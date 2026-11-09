package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/kafka"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/mem"
)

// day is a K-line's date in the tests: a minute of 2026-09-15, from 14:00 UTC.
func day(i int) uint64 { return 1789480800000 + uint64(i)*60000 }

// source is a clean value's K-line as Massive's source gives it, its own fields n and vw beside the base's, its prices
// at Massive's precision; volume tells two values of one minute apart.
func source(t *testing.T, i int, volume string) raw.KLine {
	t.Helper()
	k, err := raw.DecodeKLine([]byte(fmt.Sprintf(`{"close":759.225,"date":%d,"high":759.5406,"index":%d,`+
		`"low":759.21,"n":1282,"open":759.5406,"volume":%s,"vw":759.3544}`, day(i), i, volume)))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// series is a test's clean topic and its series.
type series struct{ topic, source, instrument string }

func (s series) message(k raw.KLine) clean.Message {
	return clean.Message{Cleaning: clean.Cleaning{Name: "validation", Version: "0.1.0"}, Origin: raw.Live,
		Source: s.source, Instrument: s.instrument, Timeframe: "M1", Publisher: "acquisition", KLine: k}
}

func (s series) title() string { return s.source + ":" + s.instrument + ":M1, validation 0.1.0" }

func publish(t *testing.T, bus stream.Bus, s series, ks ...raw.KLine) {
	t.Helper()
	ctx := context.Background()
	sink, err := bus.Sink(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close(ctx)
	for _, k := range ks {
		r, err := s.message(k).Record()
		if err != nil {
			t.Fatal(err)
		}
		if err := sink.Publish(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
}

func provision(t *testing.T, bus stream.Bus, topic string) {
	t.Helper()
	ctx := context.Background()
	adm, err := bus.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer adm.Close(ctx)
	if _, err := clean.Provision(ctx, adm, topic, 1); err != nil {
		t.Fatal(err)
	}
}

// lines is a run's log, read as it is written.
type lines struct {
	mu   sync.Mutex
	text strings.Builder
}

func (l *lines) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.Write(b)
}

func (l *lines) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.text.String()
}

// wait is the first line holding what, within a minute.
func (l *lines) wait(t *testing.T, what string) string {
	t.Helper()
	for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		for _, line := range strings.Split(l.String(), "\n") {
			if strings.Contains(line, what) {
				return line
			}
		}
	}
	t.Fatalf("no line with %q in a minute: %q", what, l.String())
	return ""
}

// running is a run of the chart on a listener of its own.
type running struct {
	addr string
	log  *lines
	stop context.CancelFunc
	done chan error
}

func start(t *testing.T, bus stream.Bus, o options, page map[string][]byte) *running {
	t.Helper()
	return startAt(t, "127.0.0.1:0", bus, o, page)
}

// startAt is a run listening at addr: a run started again where the one before served.
func startAt(t *testing.T, addr string, bus stream.Bus, o options, page map[string][]byte) *running {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{addr: ln.Addr().String(), log: &lines{}, stop: cancel, done: make(chan error, 1)}
	go func() { r.done <- run(ctx, bus, o, page, ln, r.log) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-r.done:
		case <-time.After(time.Minute):
			t.Error("the run not ended a minute after it was stopped")
		}
	})
	return r
}

// end is the run's end, within a minute.
func (r *running) end(t *testing.T) error { return r.endIn(t, time.Minute) }

func (r *running) endIn(t *testing.T, d time.Duration) error {
	t.Helper()
	select {
	case err := <-r.done:
		r.done <- err
		return err
	case <-time.After(d):
		t.Fatalf("the run not ended in %s", d)
		return nil
	}
}

// feed is a page's connection, its events as they come.
type feed struct {
	events chan event
	close  context.CancelFunc
}

func connect(t *testing.T, addr string) *feed {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+addr+"/feed", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("/feed: %s, %s", resp.Status, resp.Header.Get("Content-Type"))
	}
	f := &feed{events: make(chan event, 100), close: cancel}
	go func() {
		defer resp.Body.Close()
		defer close(f.events)
		s := bufio.NewScanner(resp.Body)
		s.Buffer(nil, 1<<24)
		var e event
		for s.Scan() {
			switch line := s.Text(); {
			case line == "":
				f.events <- e
				e = event{}
			case strings.HasPrefix(line, "event: "):
				e.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				e.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return f
}

// next is the feed's next event, within a minute; ok false when its stream has ended.
func (f *feed) next(t *testing.T) (event, bool) {
	t.Helper()
	select {
	case e, ok := <-f.events:
		return e, ok
	case <-time.After(time.Minute):
		t.Fatal("no event in a minute")
		return event{}, false
	}
}

// none: no event within a while.
func (f *feed) none(t *testing.T) {
	t.Helper()
	select {
	case e, ok := <-f.events:
		t.Fatalf("an event, %v %q, where none was due", ok, e)
	case <-time.After(300 * time.Millisecond):
	}
}

// texts are K-lines' texts as the feed sends them, of each test K-line.
func texts(t *testing.T, ks ...raw.KLine) []string {
	t.Helper()
	out := make([]string, len(ks))
	for i, k := range ks {
		out[i] = text(k.Base())
	}
	return out
}

func wantStart(t *testing.T, f *feed, title string, want []string) {
	t.Helper()
	e, ok := f.next(t)
	if !ok || e.name != "start" {
		t.Fatalf("want a start, got %v %q", ok, e)
	}
	var s struct {
		Title  string   `json:"title"`
		KLines []string `json:"klines"`
	}
	if err := json.Unmarshal([]byte(e.data), &s); err != nil {
		t.Fatal(err)
	}
	if s.Title != title || !slices.Equal(s.KLines, want) {
		t.Fatalf("start %q %q, want %q %q", s.Title, s.KLines, title, want)
	}
}

func wantKLine(t *testing.T, f *feed, want string) {
	t.Helper()
	if e, ok := f.next(t); !ok || e.name != "kline" || e.data != want {
		t.Fatalf("want the K-line %s, got %v %q", want, ok, e)
	}
}

// pageFiles is a page's folder of empty files, every one the program serves.
func pageFiles(t *testing.T) map[string][]byte {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	page, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

// A K-line's text is its base's alone, in market's canonical text, and the core's reader reads it back to the same
// base.
func TestText(t *testing.T) {
	// SPY's minute of 2026-09-15 10:00 (New York) as Massive's source gives it (docs/tutorial/advanced/README.md, 1)
	k, err := raw.DecodeKLine([]byte(`{"close":759.225,"date":1789480800000,"high":759.5406,"index":178920,` +
		`"low":759.21,"n":1282,"open":759.5406,"volume":52684.86967,"vw":759.3544}`))
	if err != nil {
		t.Fatal(err)
	}
	got := text(k.Base())
	want := `{"close":759.225,"date":1789480800000,"high":759.5406,"index":178920,"low":759.21,"open":759.5406,` +
		`"volume":52684.86967}`
	if got != want {
		t.Fatalf("text %s, want %s", got, want)
	}
	back, err := raw.DecodeKLine([]byte(got))
	if err != nil {
		t.Fatal(err)
	}
	if back.Base() != k.Base() || back.Text() != got {
		t.Fatalf("read back %+v %s, the base %+v", back.Base(), back.Text(), k.Base())
	}
	z, err := raw.Zero(3, day(3))
	if err != nil {
		t.Fatal(err)
	}
	if got := text(z.Base()); got != z.Text() {
		t.Fatalf("the zero K-line's text %s, the core's %s", got, z.Text())
	}
}

// feedCase is the feed of a run on a series' clean topic, the window 3: a start of the last window; each K-line after;
// a copy dropped, of the same value or another; a start again when the page's chart holds twice the window; a
// reconnect given a start; the group listed with a member while it serves and none after the stop.
func feedCase(t *testing.T, bus stream.Bus, s series, group string) {
	ctx := context.Background()
	provision(t, bus, s.topic)
	ks := make([]raw.KLine, 10)
	for i := range ks {
		ks[i] = source(t, i, fmt.Sprintf("%d.5", 1000+i))
	}
	publish(t, bus, s, ks[0], ks[1], ks[2], ks[1], source(t, 2, "7"), ks[3], ks[4])
	page := pageFiles(t)
	r := start(t, bus, options{topic: s.topic, name: group, window: 3}, page)
	line := r.log.wait(t, "serving")
	if want := fmt.Sprintf("chart: %s: 5 K-lines read, 2 copies dropped (1 of another value); the window 3, index 2 "+
		"to index 4; serving http://%s/", s.topic, r.addr); line != want {
		t.Fatalf("line %q, want %q", line, want)
	}
	for _, f := range files {
		resp, err := http.Get("http://" + r.addr + strings.TrimSuffix(f.path, "{$}"))
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		bufio.NewReader(resp.Body).WriteTo(&b)
		resp.Body.Close()
		if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != f.kind || b.String() != f.name {
			t.Fatalf("%s: %s %s %q", f.path, resp.Status, resp.Header.Get("Content-Type"), b.String())
		}
	}
	if resp, err := http.Get("http://" + r.addr + "/index.html"); err != nil || resp.StatusCode != 404 {
		t.Fatalf("/index.html: %v %v, want 404", resp, err)
	}
	adm, err := bus.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer adm.Close(ctx)
	members := func() int {
		gs, err := adm.Groups(ctx, s.topic)
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range gs {
			if g.Name == group {
				if len(g.Committed) > 0 {
					t.Fatalf("group %s committed %v: the chart commits none", group, g.Committed)
				}
				return g.Members
			}
		}
		return 0
	}
	if n := members(); n != 1 {
		t.Fatalf("group %s: %d members while it serves, want 1", group, n)
	}

	f := connect(t, r.addr)
	wantStart(t, f, s.title(), texts(t, ks[2], ks[3], ks[4]))
	publish(t, bus, s, ks[5], ks[6])
	wantKLine(t, f, text(ks[5].Base()))
	wantKLine(t, f, text(ks[6].Base()))
	publish(t, bus, s, ks[4], ks[7])
	wantKLine(t, f, text(ks[7].Base()))
	// six K-lines, twice the window: drawn whole again from the last window
	wantStart(t, f, s.title(), texts(t, ks[5], ks[6], ks[7]))
	publish(t, bus, s, ks[8])
	wantKLine(t, f, text(ks[8].Base()))
	f.none(t)
	f.close()

	again := connect(t, r.addr)
	wantStart(t, again, s.title(), texts(t, ks[6], ks[7], ks[8]))
	publish(t, bus, s, ks[9])
	wantKLine(t, again, text(ks[9].Base()))

	r.stop()
	if err := r.end(t); err != nil {
		t.Fatal(err)
	}
	if _, ok := again.next(t); ok {
		t.Fatal("the feed not ended at the stop")
	}
	if line, want := r.log.wait(t, "stopped"), "chart: stopped: 10 K-lines read, 3 copies dropped; 1 pages' feeds ended"; line != want {
		t.Fatalf("line %q, want %q", line, want)
	}
	if _, err := net.DialTimeout("tcp", r.addr, time.Second); err == nil {
		t.Fatal("the listener open after the stop")
	}
	for deadline := time.Now().Add(30 * time.Second); members() > 0; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("group %s has a member 30 s after the stop", group)
		}
	}
}

func TestFeed(t *testing.T) {
	s := series{topic: "market.clean.validation.v0-1-0.massive.SPY.M1", source: "massive", instrument: "SPY"}
	feedCase(t, mem.New(), s, "chart")
}

// A topic that holds no K-line yet: served at once, a page given its start at the first K-line.
func TestEmpty(t *testing.T) {
	bus := mem.New()
	s := series{topic: "market.clean.validation.v0-1-0.massive.SPY.M1", source: "massive", instrument: "SPY"}
	provision(t, bus, s.topic)
	r := start(t, bus, options{topic: s.topic, name: "chart", window: 3}, pageFiles(t))
	r.log.wait(t, "0 K-lines read, 0 copies dropped (0 of another value); the window none; serving")
	f := connect(t, r.addr)
	f.none(t)
	k := source(t, 0, "1")
	publish(t, bus, s, k)
	wantStart(t, f, s.title(), texts(t, k))
}

// stubBus is a bus whose source gives the records it holds, then waits.
type stubBus struct {
	stream.Bus
	recs []stream.Record
}

type stubSource struct{ recs []stream.Record }

func (b stubBus) Source(ctx context.Context, o stream.SubscribeOptions) (stream.Source, error) {
	return &stubSource{recs: b.recs}, nil
}

func (s *stubSource) Fetch(ctx context.Context) ([]stream.Record, error) {
	if len(s.recs) > 0 {
		recs := s.recs
		s.recs = nil
		return recs, nil
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s *stubSource) Commit(ctx context.Context, recs ...stream.Record) error { return nil }
func (s *stubSource) Close(ctx context.Context) error                         { return nil }

// What stops a run: no such topic; an offset not after the last; an index not the next; a date not after the last; a
// value not clean market data's; a page's file missing.
func TestStops(t *testing.T) {
	s := series{topic: "market.clean.validation.v0-1-0.massive.SPY.M1", source: "massive", instrument: "SPY"}
	o := options{topic: s.topic, name: "chart", window: 3}
	record := func(k raw.KLine, offset int64) stream.Record {
		r, err := s.message(k).Record()
		if err != nil {
			t.Fatal(err)
		}
		r.Offset = offset
		return r
	}
	k0, k1, k2 := source(t, 0, "1"), source(t, 1, "1"), source(t, 2, "1")
	early, err := raw.DecodeKLine([]byte(fmt.Sprintf(`{"close":1,"date":%d,"high":1,"index":2,"low":1,"open":1,`+
		`"volume":1}`, day(0)-60000)))
	if err != nil {
		t.Fatal(err)
	}
	garbage := record(k2, 2)
	garbage.Value = []byte(`{"schema":"market.raw"}`)
	cases := []struct {
		name string
		recs []stream.Record
		want string
	}{
		{"offset", []stream.Record{record(k0, 0), record(k1, 1), record(k2, 1)}, s.topic + "/0@1: after offset 1"},
		{"index", []stream.Record{record(k0, 0), record(k2, 1)}, s.topic + "/0@1: index 2, the next 1: not a value of market.clean 0.1.0"},
		{"date", []stream.Record{record(k0, 0), record(k1, 1), record(early, 2)}, s.topic + "/0@2: date 1789480740000, not after the last, 1789480860000"},
		{"value", []stream.Record{record(k0, 0), garbage}, s.topic + "/0@2: clean: "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bus := mem.New()
			provision(t, bus, s.topic)
			publish(t, bus, s, k0, k1, k2) // the topic's end: 3
			r := start(t, stubBus{Bus: bus, recs: c.recs}, o, pageFiles(t))
			if err := r.end(t); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err %v, want one with %q", err, c.want)
			}
			if strings.Contains(r.log.String(), "serving") {
				t.Fatalf("served before the fault: %q", r.log.String())
			}
		})
	}
	t.Run("no topic", func(t *testing.T) {
		r := start(t, mem.New(), o, pageFiles(t))
		if err := r.end(t); err == nil || !strings.Contains(err.Error(), s.topic) {
			t.Fatalf("err %v, want one naming %s", err, s.topic)
		}
	})
	t.Run("a page's file", func(t *testing.T) {
		dir := t.TempDir()
		for _, f := range files[:len(files)-1] {
			os.WriteFile(filepath.Join(dir, f.name), nil, 0o644)
		}
		if _, err := load(dir); err == nil || !strings.Contains(err.Error(), files[len(files)-1].name) {
			t.Fatalf("err %v, want one naming %s", err, files[len(files)-1].name)
		}
	})
}

// brokers is the test stack's Redpanda (QI_KAFKA_BROKERS; unset, localhost:29092, never the stack's), as the stream
// contract test reaches it: within 5 s, else a failure when the variable is set and a skip when not. It reads
// QI_STACK_STATE, which make check sets, so that Go's test cache keeps its result by the stack's state as well
// (go.toolchain.md, 4).
func brokers(t *testing.T) kafka.Config {
	t.Helper()
	_ = os.Getenv("QI_STACK_STATE")
	b := os.Getenv("QI_KAFKA_BROKERS")
	set := b != ""
	if !set {
		b = "localhost:29092"
	}
	cfg := kafka.Config{Brokers: strings.Split(b, ","), ClientID: "chart-test"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := kafka.WaitReady(ctx, cfg, 5*time.Second); err != nil {
		if set {
			t.Fatalf("no broker at QI_KAFKA_BROKERS=%s: %v", b, err)
		}
		t.Skipf("no broker at %s: %v", b, err)
	}
	return cfg
}

// stackSeries is a series and a clean topic of the test's own on the test stack, deleted after it, the test failing if
// it cannot be.
func stackSeries(t *testing.T, bus stream.Bus, what string) series {
	t.Helper()
	s := series{source: "chart-test", instrument: fmt.Sprintf("%s%d", what, time.Now().UnixNano())}
	topic, err := clean.Topic(clean.Cleaning{Name: "validation", Version: "0.1.0"}, s.source, s.instrument, "M1")
	if err != nil {
		t.Fatal(err)
	}
	s.topic = topic
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		adm, err := bus.Admin(ctx)
		if err != nil {
			t.Errorf("%s not deleted: %v", topic, err)
			return
		}
		defer adm.Close(ctx)
		if err := adm.Delete(ctx, topic); err != nil {
			t.Errorf("%s not deleted: %v", topic, err)
		}
	})
	return s
}

// On the test stack, a clean topic of its own: the feed as on the in-memory stream, its group listed with a member
// while it serves, which is what holds a reset off.
func TestStackFeed(t *testing.T) {
	bus := kafka.New(brokers(t))
	s := stackSeries(t, bus, "F")
	feedCase(t, bus, s, "chart-test-"+s.instrument)
}

// On the test stack, its topic deleted and made again under it as it runs (design-chart-consumer, At a reset, the one
// way): it stops with a fault, its client finding the topic's ID gone (UNKNOWN_TOPIC_ID), never stepping a K-line of
// the topic made again onto the chart of the one before.
func TestStackRemade(t *testing.T) {
	ctx := context.Background()
	bus := kafka.New(brokers(t))
	s := stackSeries(t, bus, "R")
	provision(t, bus, s.topic)
	ks := []raw.KLine{source(t, 0, "1"), source(t, 1, "1"), source(t, 2, "1")}
	publish(t, bus, s, ks...)
	r := start(t, bus, options{topic: s.topic, name: "chart-test-" + s.instrument, window: 3}, pageFiles(t))
	r.log.wait(t, "serving")
	f := connect(t, r.addr)
	wantStart(t, f, s.title(), texts(t, ks...))
	adm, err := bus.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer adm.Close(ctx)
	began := time.Now()
	if err := adm.Delete(ctx, s.topic); err != nil {
		t.Fatal(err)
	}
	provision(t, bus, s.topic)
	publish(t, bus, s, source(t, 0, "2"), source(t, 1, "2"), source(t, 2, "2"), source(t, 3, "2"))
	made := time.Since(began)
	// franz-go keeps a cursor's topic ID and gives UNKNOWN_TOPIC_ID back on the sixth fetch that fails with it, each a
	// metadata refresh after the one before (pkg/kgo/source.go, cursor): about 50 s on the test stack
	err = r.endIn(t, 2*time.Minute)
	if err == nil || !strings.Contains(err.Error(), "UNKNOWN_TOPIC_ID") {
		t.Fatalf("err %v, want the fetch's UNKNOWN_TOPIC_ID", err)
	}
	t.Logf("the topic made again in %s, the run stopped %s after: %v", made.Round(time.Millisecond),
		(time.Since(began) - made).Round(time.Millisecond), err)
	for {
		e, ok := f.next(t)
		if !ok {
			break
		}
		t.Fatalf("an event after the topic was made again: %q", e)
	}
}
