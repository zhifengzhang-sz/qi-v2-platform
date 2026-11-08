package massive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// Base is Massive's API.
	Base = "https://api.massive.com"
	// PageSize is the rows of a page Massive is asked for, its most.
	PageSize = 50000
	// Interval is the least time between two calls of the free plan, five a minute.
	Interval = 12 * time.Second
)

// KeyFile is the key's file by default: ~/.config/qi/massive_key, one line.
func KeyFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "qi", "massive_key")
}

// PagesDir is where the pages are saved by default: ~/.local/share/qi/massive/pages, outside any repository.
func PagesDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "qi", "massive", "pages")
}

// Fetcher asks Massive for a request's pages and saves each page as Massive sent it, but its request_id, before it
// is checked or read, a page once however many fetches see it. The key is read from its file when a fetch starts and sent in the Authorization header alone,
// so that no URL, page saved, log line or error carries it.
type Fetcher struct {
	// Base is Massive's API; Base when "".
	Base string
	// KeyFile is the key's file, one line (by default KeyFile()); a fetch with none is refused.
	KeyFile string
	// Interval is the least time between two calls, none when 0 (the free plan's: Interval).
	Interval time.Duration
	// Limit is the rows a page; PageSize when 0.
	Limit int
	// Dir is where the pages are saved, a series a folder, Dir/INSTRUMENT/TIMEFRAME/: each page a file named by its
	// content, HASH.json, HASH the SHA-256 of its bytes in hex, so that a page is kept once however many fetches see
	// it; and the folder's log of fetches, fetches, a line a fetch that took a page: FETCHED FROM TO and the names of
	// its pages in order, FETCHED the fetch's start (UTC, 20061102T150405.000Z).
	Dir string
	// Client is the HTTP client; one of 60 s a call when nil.
	Client *http.Client
	// Log takes a line for each call waited out; none when nil.
	Log io.Writer

	key  string
	last time.Time // the last call's time
}

// Fetched is what a fetch took: its series' folder of pages, and the names of its pages in order, Saved of them saved
// by it and the others kept already.
type Fetched struct {
	Dir   string
	Pages []string
	Saved int
}

// Fetch asks Massive once for the whole of the request's spans, from its first FROM to its last TO, unadjusted, sorted
// ascending, page by page: each answer of 200 saved, unless kept already, then checked (status OK or DELAYED) and read
// (ReadPage) and given to page, in order; Massive's next_url followed on its own host alone, until a page gives none
// or no row. It gives the pages it took, and logs them in the folder's fetches once it ends, whatever its end. An
// answer carrying the key, or whose request_id cannot be taken out, is not saved and stops the fetch, as does an
// answer that cannot be saved, a file of its name holding other bytes, a page refused by ReadPage (kept, saved) or a
// refusal (any answer but 200 with status OK or DELAYED): Massive's answer is named, but its request_id, never the key.
func (f *Fetcher) Fetch(ctx context.Context, r Request, page func(Page) error) (got Fetched, err error) {
	if err := r.Check(); err != nil {
		return got, err
	}
	if f.Dir == "" {
		return got, errors.New("massive: no directory for the pages")
	}
	if f.KeyFile == "" {
		return got, errors.New("massive: no key: no key file")
	}
	key, err := readKey(f.KeyFile)
	if err != nil {
		return got, err
	}
	f.key = key
	defer func() { f.key = "" }()
	base, limit := f.Base, f.Limit
	if base == "" {
		base = Base
	}
	if limit == 0 {
		limit = PageSize
	}
	started := time.Now().UTC()
	got.Dir = filepath.Join(f.Dir, r.Instrument, r.Timeframe)
	if err := os.MkdirAll(got.Dir, 0o755); err != nil {
		return got, fmt.Errorf("massive: pages: %w", err)
	}
	defer func() {
		if len(got.Pages) == 0 {
			return
		}
		if lerr := logFetch(got.Dir, started, r, got.Pages); lerr != nil && err == nil {
			err = fmt.Errorf("massive: pages: %w", lerr)
		}
	}()
	u := fmt.Sprintf("%s/v2/aggs/ticker/%s/range/%s/%d/%d?adjusted=false&sort=asc&limit=%d",
		base, r.Instrument, timeframes[r.Timeframe].path, r.From(), r.To(), limit)
	for n := 1; u != ""; n++ {
		body, err := f.get(ctx, u)
		if err != nil {
			return got, err
		}
		if body, err = without(body, "request_id"); err != nil {
			return got, fmt.Errorf("massive: page %d: %w: not saved", n, err)
		}
		if holds(body, "request_id") {
			return got, fmt.Errorf("massive: page %d: request_id inside: not saved", n)
		}
		name, saved, err := keep(got.Dir, body)
		if err != nil {
			return got, fmt.Errorf("massive: page %d: %w", n, err)
		}
		got.Pages = append(got.Pages, filepath.Base(name))
		if saved {
			got.Saved++
		}
		if err := refused(http.StatusOK, body); err != nil {
			return got, fmt.Errorf("%w (%s)", err, name)
		}
		p, err := ReadPage(body)
		if err != nil {
			return got, fmt.Errorf("massive: %s: %w", name, err)
		}
		if err := page(p); err != nil {
			return got, err
		}
		if p.NextURL == "" || len(p.Bars) == 0 {
			return got, nil
		}
		// The key goes only to Massive's own host.
		if !strings.HasPrefix(p.NextURL, base+"/") {
			return got, fmt.Errorf("massive: next_url outside %s", base)
		}
		u = p.NextURL
	}
	return got, nil
}

// readKey reads the key's first line; an error names the file, never its content.
func readKey(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("massive: no key: %s: %w", path, err)
	}
	key, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	if key = strings.TrimSpace(key); key == "" {
		return "", fmt.Errorf("massive: no key: %s is empty", path)
	}
	return key, nil
}

func (f *Fetcher) wait(ctx context.Context) error {
	if d := time.Until(f.last.Add(f.Interval)); d > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	f.last = time.Now()
	return nil
}

// get asks for one page and gives the body of its answer of 200, as Massive sent it: a network fault asked again up
// to three times, a 429 (past the plan's calls a minute) or a 5xx (the provider's trouble) waited out (Retry-After,
// else a minute) and asked again up to five times; any other answer but 200 is a refusal, not saved.
func (f *Fetcher) get(ctx context.Context, u string) ([]byte, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	for attempt := 0; ; attempt++ {
		if err := f.wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+f.key)
		resp, err := client.Do(req)
		if err != nil {
			if attempt < 3 && ctx.Err() == nil {
				continue
			}
			return nil, fmt.Errorf("massive: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("massive: %w", err)
		}
		if bytes.Contains(body, []byte(f.key)) {
			return nil, fmt.Errorf("massive: HTTP %d: an answer carrying the key: not saved", resp.StatusCode)
		}
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && attempt < 5 {
			wait := time.Minute
			if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
				wait = time.Duration(s) * time.Second
			}
			if f.Log != nil {
				fmt.Fprintf(f.Log, "massive: HTTP %d, asking again in %s\n", resp.StatusCode, wait)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, refused(resp.StatusCode, body)
		}
		return body, nil
	}
}

// refused is nil for an answer of 200 with status OK or DELAYED, and else Massive's refusal, its error and message
// named, or its body when it has neither, its request_id taken out; an answer still holding one is not named. Each
// name is read as it is written, its case too.
func refused(code int, body []byte) error {
	var status, msg string
	if v, err := decode(body); err == nil {
		if m, ok := v.(map[string]any); ok {
			status, _ = m["status"].(string)
			e, _ := m["error"].(string)
			x, _ := m["message"].(string)
			msg = strings.TrimSpace(e + " " + x)
			if code == http.StatusOK && (status == "OK" || status == "DELAYED") {
				return nil
			}
		}
	}
	if msg == "" {
		if b, err := without(body, "request_id"); err == nil {
			body = b
		}
		msg = strings.TrimSpace(string(body))
	}
	if holds([]byte(msg), "request_id") {
		msg = "an answer holding a request_id, not named"
	}
	return fmt.Errorf("massive refused: HTTP %d, status %q: %s", code, status, msg)
}

// holds reports whether bytes hold a name: as it is written, or, in JSON, as a string once read (a name or a value,
// at any depth, \u005f and the like read too).
func holds(b []byte, name string) bool {
	if bytes.Contains(b, []byte(name)) {
		return true
	}
	d := json.NewDecoder(bytes.NewReader(b))
	for {
		t, err := d.Token()
		if err != nil {
			return false
		}
		if s, ok := t.(string); ok && s == name {
			return true
		}
	}
}

// keep saves a page in dir under its content's name, HASH.json, unless kept there already: saved is whether it was
// saved now. A file of its name holding other bytes is refused, never replaced. The page is written to a file of its
// own and synced, then linked to its name, so that no file of a page's name holds less than the page.
func keep(dir string, body []byte) (name string, saved bool, err error) {
	sum := sha256.Sum256(body)
	name = filepath.Join(dir, hex.EncodeToString(sum[:])+".json")
	kept := func() error {
		b, err := os.ReadFile(name)
		if err == nil && !bytes.Equal(b, body) {
			return fmt.Errorf("%s: kept with other bytes", name)
		}
		return err
	}
	if err := kept(); !errors.Is(err, os.ErrNotExist) {
		return name, false, err
	}
	w, err := os.CreateTemp(dir, ".saving-*.json")
	if err != nil {
		return name, false, err
	}
	defer os.Remove(w.Name())
	_, err = w.Write(body)
	if err == nil {
		err = w.Chmod(0o644)
	}
	if err == nil {
		err = w.Sync()
	}
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return name, false, err
	}
	if err := os.Link(w.Name(), name); err != nil {
		if errors.Is(err, os.ErrExist) {
			// saved meanwhile, by another fetch
			return name, false, kept()
		}
		return name, false, err
	}
	return name, true, nil
}

// logFetch appends a fetch's line to its folder's log of fetches, FETCHED FROM TO and its pages' names, and syncs it.
func logFetch(dir string, started time.Time, r Request, pages []string) error {
	w, err := os.OpenFile(filepath.Join(dir, "fetches"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s %d %d %s\n", started.Format("20060102T150405.000Z"), r.From(), r.To(), strings.Join(pages, " "))
	if err == nil {
		err = w.Sync()
	}
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	return err
}

// without gives a JSON object's bytes with its member of a name taken out, its comma with it, every other byte as
// it was; the bytes as they were when they have no such member, or are not a JSON object and do not hold the name.
// A name twice, or the name in bytes it cannot read, is refused.
func without(b []byte, name string) (out []byte, err error) {
	defer func() {
		if err != nil && !bytes.Contains(b, []byte(name)) {
			out, err = b, nil
		}
	}()
	d := json.NewDecoder(bytes.NewReader(b))
	if t, err := d.Token(); err != nil || t != json.Delim('{') {
		return nil, fmt.Errorf("not a JSON object, holding %s", name)
	}
	cut := [2]int64{-1, -1}
	for i := 0; d.More(); i++ {
		start := d.InputOffset() // at the member's name, or at the comma before it: More has passed the spaces
		k, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("not JSON, holding %s: %w", name, err)
		}
		var v json.RawMessage
		if err := d.Decode(&v); err != nil {
			return nil, fmt.Errorf("not JSON, holding %s: %w", name, err)
		}
		if k != name {
			continue
		}
		if cut[0] >= 0 {
			return nil, fmt.Errorf("%s twice", name)
		}
		cut = [2]int64{start, d.InputOffset()}
		if i == 0 {
			// The first member: its comma is after it, if a member follows.
			rest := bytes.TrimLeft(b[cut[1]:], " \t\r\n")
			if len(rest) > 0 && rest[0] == ',' {
				cut[1] = int64(len(b)-len(rest)) + 1
			}
		}
	}
	if _, err := d.Token(); err != nil {
		return nil, fmt.Errorf("not JSON, holding %s: %w", name, err)
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("not one JSON object, holding %s", name)
	}
	if cut[0] < 0 {
		return b, nil
	}
	return append(append(make([]byte, 0, len(b)), b[:cut[0]]...), b[cut[1]:]...), nil
}
