package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// behind is how many events a page's feed holds that its connection has not taken: a page further behind is closed,
// and its EventSource reconnects, given a start.
const behind = 1024

// event is one server-sent event: its name and its data, one line.
type event struct{ name, data string }

// page is what the program keeps of a page: its connection's events, and how many K-lines its chart holds, those of
// its last start and each sent after it.
type page struct {
	events chan event
	held   int
}

// hub is the series' last K-lines, each its text, and the pages they are sent to (design-chart-consumer, The feed and
// the page): a start of the last window, the first event of every connection, then each K-line; a start again when a
// page's chart holds twice the window, so that no page holds more.
type hub struct {
	mu     sync.Mutex
	window int
	title  string
	klines []string // the last K-lines, at most twice the window, the window its last
	pages  map[*page]bool
	closed bool
	ended  int // the pages' feeds ended at the stop
}

func newHub(window int) *hub { return &hub{window: window, pages: map[*page]bool{}} }

// last is the last window of K-lines.
func (h *hub) last() []string { return h.klines[max(0, len(h.klines)-h.window):] }

// start is the event a page's chart is drawn whole from: the title and the last window of K-lines.
func (h *hub) start() event {
	b, err := json.Marshal(struct {
		Title  string   `json:"title"`
		KLines []string `json:"klines"`
	}{h.title, h.last()})
	if err != nil {
		panic(err) // a struct of strings
	}
	return event{"start", string(b)}
}

// send gives a page an event, or closes its feed when it is too far behind.
func (h *hub) send(p *page, e event) {
	select {
	case p.events <- e:
	default:
		delete(h.pages, p)
		close(p.events)
	}
}

// begin sends a page a start of the last window.
func (h *hub) begin(p *page) {
	p.held = len(h.last())
	h.send(p, h.start())
}

// add is the series' next K-line, its text, given to every page: a page given no start yet, since the series held no
// K-line when it came, is given one; a page whose chart then holds twice the window is given a start again.
func (h *hub) add(title, k string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.title == "" {
		h.title = title
	}
	h.klines = append(h.klines, k)
	if len(h.klines) > 2*h.window {
		h.klines = append([]string(nil), h.last()...)
	}
	for p := range h.pages {
		if p.held == 0 {
			h.begin(p)
			continue
		}
		h.send(p, event{"kline", k})
		if p.held++; p.held >= 2*h.window && h.pages[p] {
			h.begin(p)
		}
	}
}

// join is a page's feed begun: given a start when the series holds a K-line; none once the program stops.
func (h *hub) join() (*page, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, false
	}
	p := &page{events: make(chan event, behind)}
	h.pages[p] = true
	if len(h.klines) > 0 {
		h.begin(p)
	}
	return p, true
}

// leave is a page gone: its connection closed by the browser.
func (h *hub) leave(p *page) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.pages, p)
}

// close ends every page's feed, and any after it.
func (h *hub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for p := range h.pages {
		delete(h.pages, p)
		close(p.events)
		h.ended++
	}
}

// feed serves a page's events (text/event-stream) until its feed ends, the browser leaves, or a write fails. Once the
// program stops, a request is cut off with no answer, so that the page's EventSource tries again (an answer other than
// 200 would end it for good) and finds the program started again.
func (h *hub) feed(w http.ResponseWriter, r *http.Request) {
	p, ok := h.join()
	if !ok {
		panic(http.ErrAbortHandler)
	}
	defer h.leave(p)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	if rc.Flush() != nil {
		return
	}
	for {
		select {
		case e, ok := <-p.events:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, e.data); err != nil {
				return
			}
			if rc.Flush() != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// file is one of the page's files: the path it is served at, its name in the page's folder, its type.
type file struct{ path, name, kind string }

// files are the page's (design-chart-consumer, The feed and the page): its HTML, its script bundled with the library,
// the library's tv engine, and Lightweight Charts' standalone build with its licence.
var files = []file{
	{"/{$}", "index.html", "text/html; charset=utf-8"},
	{"/chart.js", "chart.js", "text/javascript; charset=utf-8"},
	{"/tv.js", "tv.js", "text/javascript; charset=utf-8"},
	{"/lightweight-charts.js", "lightweight-charts.js", "text/javascript; charset=utf-8"},
	{"/LICENSE.lightweight-charts", "LICENSE.lightweight-charts", "text/plain; charset=utf-8"},
}

// load reads each of the page's files from its folder; a file missing stops it, named.
func load(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(dir, f.name))
		if err != nil {
			return nil, fmt.Errorf("-page %s: %w", dir, err)
		}
		out[f.name] = b
	}
	return out, nil
}

// handler serves the page's files and its feed, and nothing else.
func handler(h *hub, page map[string][]byte) http.Handler {
	mux := http.NewServeMux()
	for _, f := range files {
		b := page[f.name]
		mux.HandleFunc("GET "+f.path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", f.kind)
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Write(b)
		})
	}
	mux.HandleFunc("GET /feed", h.feed)
	return mux
}
