package massive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// The saved pages (testdata) are the fetcher's, of Massive's answer to SPY's first six minutes of 2026-10-23 (04:00
// New York time), three rows a page (page1.json, page2.json), and to a Sunday's half hour, no row (empty.json).
func saved(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// the six bars' lines, indexed from 0
var six = []string{
	`{"close":763.64,"date":1792742400000,"high":764.8,"index":0,"low":763,"n":976,"open":764.7,"volume":10391.875729,"vw":763.8517}`,
	`{"close":763.71,"date":1792742460000,"high":764.31,"index":1,"low":763,"n":703,"open":763.54,"volume":17758.504868,"vw":763.635}`,
	`{"close":763.64,"date":1792742520000,"high":764.6,"index":2,"low":762.93,"n":1111,"open":763.76,"volume":37281.842425,"vw":763.6795}`,
	`{"close":763.97,"date":1792742580000,"high":764.07,"index":3,"low":763.68,"n":19,"open":763.68,"volume":976.041273,"vw":763.9437}`,
	`{"close":763.68,"date":1792742640000,"high":763.93,"index":4,"low":763,"n":75,"open":763.93,"volume":3612.094383,"vw":763.2819}`,
	`{"close":763.77,"date":1792742700000,"high":764.09,"index":5,"low":763.59,"n":26,"open":763.68,"volume":1960.043275,"vw":763.6787}`,
}

func TestSchema(t *testing.T) {
	var s map[string]any
	if err := json.Unmarshal(Schema(), &s); err != nil || s["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("the schema: %v", err)
	}
	for _, name := range []string{"page1.json", "page2.json", "empty.json"} {
		if err := CheckPage(saved(t, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A keyword the check does not know, or a schema it cannot read, is refused, never left unchecked.
	for name, s := range map[string]string{
		"additionalProperties":  `{"type":"object","additionalProperties":false}`,
		"minimum":               `{"properties":{"t":{"type":"integer","minimum":0}}}`,
		"a type of none":        `{"type":"decimal"}`,
		"a name twice":          `{"required":["t","t"]}`,
		"a type twice":          `{"type":["string","string"]}`,
		"a schema of no object": `{"items":true}`,
		"a member twice":        `{"type":"object","type":"array"}`,
	} {
		if _, err := compile([]byte(s)); err == nil {
			t.Errorf("%s: compiled", name)
		}
	}
}

func lines(t *testing.T, pages ...[]byte) []string {
	t.Helper()
	b := NewBars(Request{Instrument: "SPY", Timeframe: "M1", Spans: [][2]uint64{{1792742400000, 1792742700000}}},
		time.UnixMilli(1792742700000+60_000))
	var out []string
	for _, page := range pages {
		p, err := ReadPage(page)
		if err != nil {
			t.Fatal(err)
		}
		if err := b.Page(p, func(s string) error { out = append(out, strings.TrimSuffix(s, "\n")); return nil }); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestReadPage(t *testing.T) {
	p, err := ReadPage(saved(t, "page1.json"))
	if err != nil || p.Status != "OK" || len(p.Bars) != 3 || !strings.HasPrefix(p.NextURL, Base+"/v2/aggs/ticker/SPY/range/1/minute/1792742580000/") {
		t.Fatalf("page 1: %+v, %v", p, err)
	}
	if p, err = ReadPage(saved(t, "empty.json")); err != nil || p.Status != "OK" || len(p.Bars) != 0 || p.NextURL != "" {
		t.Fatalf("empty: %+v, %v", p, err)
	}
	got := lines(t, saved(t, "page1.json"), saved(t, "page2.json"), saved(t, "empty.json"))
	if strings.Join(got, "\n") != strings.Join(six, "\n") {
		t.Fatalf("lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(six, "\n"))
	}
	// A number in exponent form is read exactly.
	page := bytes.Replace(saved(t, "page2.json"), []byte(`"v":976.041273`), []byte(`"v":9.76041273e2`), 1)
	if p, err := ReadPage(page); err != nil || p.Bars[0].Volume.String() != "976.041273" {
		t.Fatalf("exponent: %+v, %v", p, err)
	}
}

func TestRefused(t *testing.T) {
	page := string(saved(t, "page2.json"))
	edit := func(old, new string) []byte {
		if !strings.Contains(page, old) {
			t.Fatalf("%q not in page2.json", old)
		}
		return []byte(strings.Replace(page, old, new, 1))
	}
	// A page that breaks the schema.
	for name, b := range map[string][]byte{
		"t a string":            edit(`"t":1792742580000`, `"t":"1792742580000"`),
		"no o":                  edit(`"o":763.68,`, ``),
		"no t":                  edit(`"t":1792742580000,`, ``),
		"a fraction of a trade": edit(`"n":19`, `"n":19.5`),
		"otc not a boolean":     edit(`"n":19`, `"n":19,"otc":"no"`),
		"a name twice":          edit(`"n":19`, `"n":19,"n":20`),
		"no status":             edit(`,"status":"OK"`, ``),
		"adjusted a string":     edit(`"adjusted":false`, `"adjusted":"false"`),
		"results not an array":  edit(`"results":[`, `"results":{"x":[`),
		"more after the page":   []byte(page + "{}"),
		"not JSON":              []byte(page[:100]),
	} {
		if CheckPage(b) == nil {
			t.Errorf("%s: valid by the schema", name)
		}
		if _, err := ReadPage(b); err == nil || !strings.Contains(err.Error(), "by its schema") {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A page valid by the schema whose row is not Massive's K-line, or whose status is not OK or DELAYED.
	for name, b := range map[string][]byte{
		"otc":                          edit(`"n":19`, `"n":19,"otc":true`),
		"a field the K-line has not":   edit(`"n":19`, `"n":19,"x":1`),
		"t twice, of two cases":        edit(`"n":19`, `"n":19,"T":1792742580001`),
		"vw twice, of two cases":       edit(`"n":19`, `"n":19,"VW":1`),
		"seven digits after the point": edit("976.041273", "976.0412731"),
		"low above open":               edit(`"l":763.68`, `"l":763.7`),
		"a negative trade count":       edit(`"n":19`, `"n":-19`),
		"status ERROR":                 edit(`"status":"OK"`, `"status":"ERROR"`),
	} {
		if err := CheckPage(b); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if p, err := ReadPage(b); err == nil {
			t.Errorf("%s: read, %d bars", name, len(p.Bars))
		} else if name == "otc" && !strings.Contains(err.Error(), `"otc", a field it has no place for`) {
			t.Errorf("otc: %v", err)
		}
	}
}

func TestBars(t *testing.T) {
	p1, err := ReadPage(saved(t, "page1.json"))
	if err != nil {
		t.Fatal(err)
	}
	p2, err := ReadPage(saved(t, "page2.json"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(spans [][2]uint64, now int64, pages ...Page) ([]string, Counts, error) {
		b := NewBars(Request{Instrument: "SPY", Timeframe: "M1", Spans: spans}, time.UnixMilli(now))
		var out []string
		for _, p := range pages {
			if err := b.Page(p, func(s string) error { out = append(out, s); return nil }); err != nil {
				return out, b.Counts, err
			}
		}
		return out, b.Counts, nil
	}
	whole := [][2]uint64{{1792742400000, 1792742700000}}
	// The last bar not closed yet: counted, not given.
	if out, c, err := run(whole, 1792742700000+59_999, p1, p2); err != nil || len(out) != 5 ||
		c != (Counts{Pages: 2, Rows: 6, Lines: 5, Open: 1}) {
		t.Fatalf("not closed: %d lines, %+v, %v", len(out), c, err)
	}
	// Two spans, the first minute and the fifth to the sixth: the bars between counted as outside, the lines indexed
	// from 0.
	out, c, err := run([][2]uint64{{1792742400000, 1792742400000}, {1792742640000, 1792742700000}}, 1792742700000+60_000, p1, p2)
	if err != nil || len(out) != 3 || c != (Counts{Pages: 2, Rows: 6, Lines: 3, Outside: 3}) ||
		!strings.Contains(out[0], `"date":1792742400000,`) || !strings.Contains(out[1], `"date":1792742640000,"high":763.93,"index":1,`) ||
		!strings.Contains(out[2], `"date":1792742700000,"high":764.09,"index":2,`) {
		t.Fatalf("two spans: %q, %+v, %v", out, c, err)
	}
	// Rows not ascending, across pages too.
	if _, _, err := run(whole, 1792742700000+60_000, p2, p1); err == nil || !strings.Contains(err.Error(), "not ascending") {
		t.Fatalf("not ascending: %v", err)
	}
	if err := (Request{Instrument: "SPY", Timeframe: "M1"}).Check(); err == nil {
		t.Fatal("no span: checked")
	}
	if got := strings.Join(Timeframes(), " "); got != "M1 M5 M15 M30 H1" {
		t.Fatalf("timeframes: %s", got)
	}
}

// massive is a server that answers as Massive does, each saved page with its request_id, as Massive sent it.
type massive struct {
	t     *testing.T
	srv   *httptest.Server
	key   string
	calls []string
	page1 []byte // the first page's answer, in place of page1.json's when not nil
}

func (m *massive) sent(name string) []byte {
	b := bytes.Replace(saved(m.t, name), []byte(`"status":"OK"`), []byte(`"status":"OK","request_id":"6a7e466379af0a71039d60cc78e72282"`), 1)
	return bytes.ReplaceAll(b, []byte(Base), []byte(m.srv.URL))
}

func newMassive(t *testing.T) *massive {
	m := &massive{t: t, key: "test-key"}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.calls = append(m.calls, r.URL.String())
		if r.Header.Get("Authorization") != "Bearer "+m.key || strings.Contains(r.URL.String(), m.key) {
			http.Error(w, `{"status":"ERROR","request_id":"1","error":"Unknown API Key"}`, http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Query().Get("cursor") == "":
			if r.URL.Path != "/v2/aggs/ticker/SPY/range/1/minute/1792742400000/1792742700000" ||
				r.URL.RawQuery != "adjusted=false&sort=asc&limit=3" {
				http.Error(w, `{"status":"ERROR","error":"bad request"}`, http.StatusBadRequest)
				return
			}
			if m.page1 != nil {
				w.Write(m.page1)
				return
			}
			w.Write(m.sent("page1.json"))
		default:
			w.Write(m.sent("page2.json"))
		}
	}))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *massive) fetcher(key string) *Fetcher {
	keyFile := filepath.Join(m.t.TempDir(), "key")
	if err := os.WriteFile(keyFile, []byte(key+"\n"), 0o600); err != nil {
		m.t.Fatal(err)
	}
	return &Fetcher{Base: m.srv.URL, KeyFile: keyFile, Interval: time.Nanosecond, Limit: 3, Dir: m.t.TempDir(),
		Client: m.srv.Client()}
}

var sixMinutes = Request{Instrument: "SPY", Timeframe: "M1", Spans: [][2]uint64{{1792742400000, 1792742700000}}}

func files(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			b, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(dir, p)
			out[rel] = b
		}
		return err
	})
	return out
}

// pages gives the pages saved under dir, by their path from it.
func pages(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := files(t, dir)
	for name := range out {
		if !strings.HasSuffix(name, ".json") {
			delete(out, name)
		}
	}
	return out
}

func TestFetch(t *testing.T) {
	m := newMassive(t)
	f := m.fetcher("test-key")
	var got []Page
	fetched, err := f.Fetch(context.Background(), sixMinutes, func(p Page) error { got = append(got, p); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || len(got[0].Bars) != 3 || len(got[1].Bars) != 3 || len(m.calls) != 2 {
		t.Fatalf("%d pages, %d calls", len(got), len(m.calls))
	}
	// Each page saved as Massive sent it, but its request_id, in its series' folder, named by its content: the saved
	// pages' bytes and their SHA-256; the fetch a line of the folder's log, its start, its FROM and TO, its pages.
	var want [][]byte
	var names []string
	for _, name := range []string{"page1.json", "page2.json"} {
		b := bytes.ReplaceAll(saved(t, name), []byte(Base), []byte(m.srv.URL))
		sum := sha256.Sum256(b)
		want, names = append(want, b), append(names, hex.EncodeToString(sum[:])+".json")
	}
	dir := filepath.Join(f.Dir, "SPY", "M1")
	if fetched.Dir != dir || !slices.Equal(fetched.Pages, names) || fetched.Saved != 2 {
		t.Fatalf("fetched %+v, want the pages %v in %s", fetched, names, dir)
	}
	saves := files(t, f.Dir)
	if len(saves) != 3 {
		t.Fatalf("%d files saved", len(saves))
	}
	for n, name := range names {
		rel := filepath.Join("SPY", "M1", name)
		if !bytes.Equal(saves[rel], want[n]) {
			t.Errorf("%s:\n%s\nwant:\n%s", rel, saves[rel], want[n])
		}
	}
	logged := regexp.MustCompile(`^(20\d{6}T\d{6}\.\d{3}Z 1792742400000 1792742700000 ` + names[0] + " " + names[1] + "\n)+$")
	fetches := filepath.Join("SPY", "M1", "fetches")
	if !logged.Match(saves[fetches]) {
		t.Fatalf("the log of fetches %q", saves[fetches])
	}
	// A repeat saves none: each page kept once, the fetch logged.
	if again, err := f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil }); err != nil ||
		again.Saved != 0 || !slices.Equal(again.Pages, names) {
		t.Fatalf("a repeat: %+v, %v", again, err)
	}
	if saves = files(t, f.Dir); len(saves) != 3 || !logged.Match(saves[fetches]) || bytes.Count(saves[fetches], []byte("\n")) != 2 {
		t.Fatalf("a repeat: %d files, the log %q", len(saves), saves[fetches])
	}
	// A file of a page's name holding other bytes: refused, never replaced; the page before it logged.
	os.WriteFile(filepath.Join(dir, names[1]), []byte("{}"), 0o644)
	if _, err := f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil }); err == nil ||
		!strings.Contains(err.Error(), "kept with other bytes") {
		t.Fatalf("other bytes: %v", err)
	}
	if saves = files(t, f.Dir); len(saves) != 3 || string(saves[filepath.Join("SPY", "M1", names[1])]) != "{}" ||
		!bytes.HasSuffix(saves[fetches], []byte(" "+names[0]+"\n")) {
		t.Fatalf("other bytes: %d files, the log %q", len(saves), saves[fetches])
	}
	// A refused key stops the fetch, Massive's answer named, the key never; an answer but 200 not saved.
	f = m.fetcher("other-key")
	_, err = f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "massive refused: HTTP 401") || strings.Contains(err.Error(), "other-key") ||
		len(files(t, f.Dir)) != 0 {
		t.Fatalf("a refused key: %v", err)
	}
	// Every answer of 200 saved before it is checked, its request_id taken out: one of another status, or not JSON,
	// saved, then refused; Massive's answer named but its request_id.
	for name, c := range map[string]struct{ page, err string }{
		"status ERROR": {`{"status":"ERROR","request_id":"6a7e"}`, `massive refused: HTTP 200, status "ERROR": {"status":"ERROR"}`},
		"not JSON":     {`<html>busy</html>`, `massive refused: HTTP 200, status "": <html>busy</html>`},
	} {
		m.page1 = []byte(c.page)
		f = m.fetcher("test-key")
		_, err := f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil })
		saves := pages(t, f.Dir)
		if err == nil || !strings.Contains(err.Error(), c.err) || len(saves) != 1 {
			t.Errorf("%s: %v, %d files", name, err, len(saves))
		}
		for _, b := range saves {
			if bytes.Contains(b, []byte("request_id")) {
				t.Errorf("%s: saved %s", name, b)
			}
		}
	}
	// An answer whose request_id cannot be taken out: not saved.
	for name, page := range map[string]string{
		"request_id twice":     `{"status":"OK","request_id":"1","request_id":"2"}`,
		"request_id in a row":  `{"ticker":"SPY","queryCount":1,"resultsCount":1,"adjusted":false,"status":"OK","results":[{"request_id":"1"}]}`,
		"request_id escaped":   `{"ticker":"SPY","queryCount":1,"resultsCount":1,"adjusted":false,"status":"OK","results":[{"request\u005fid":"1"}]}`,
		"request_id, not JSON": `{"status":"OK","request_id":"1"`,
	} {
		m.page1 = []byte(page)
		f = m.fetcher("test-key")
		if _, err := f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil }); err == nil ||
			!strings.Contains(err.Error(), "not saved") || len(files(t, f.Dir)) != 0 {
			t.Errorf("%s: %v, %d files", name, err, len(files(t, f.Dir)))
		}
	}
	// An answer carrying the key: not saved.
	m.page1 = []byte(`{"ticker":"SPY","queryCount":0,"resultsCount":0,"adjusted":false,"status":"OK","note":"test-key"}`)
	f = m.fetcher("test-key")
	if _, err := f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil }); err == nil ||
		!strings.Contains(err.Error(), "carrying the key") || strings.Contains(err.Error(), "test-key") || len(files(t, f.Dir)) != 0 {
		t.Fatalf("the key in a page: %v, %d files", err, len(files(t, f.Dir)))
	}
	// A page that breaks the schema: saved, then refused.
	m.page1 = bytes.Replace(m.sent("page1.json"), []byte(`"t":1792742400000`), []byte(`"t":"x"`), 1)
	f = m.fetcher("test-key")
	if _, err := f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil }); err == nil ||
		!strings.Contains(err.Error(), "by its schema") || len(pages(t, f.Dir)) != 1 {
		t.Fatalf("a page breaking the schema: %v, %d pages", err, len(pages(t, f.Dir)))
	}
	// A next_url to another host is not followed.
	m.page1 = bytes.ReplaceAll(m.sent("page1.json"), []byte(m.srv.URL), []byte("https://elsewhere.example"))
	if _, err := m.fetcher("test-key").Fetch(context.Background(), sixMinutes, func(Page) error { return nil }); err == nil ||
		!strings.Contains(err.Error(), "next_url outside") {
		t.Fatalf("next_url elsewhere: %v", err)
	}
	// No directory, or no key file named, no fetch.
	f = m.fetcher("test-key")
	f.Dir = ""
	if _, err := f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil }); err == nil {
		t.Fatal("no directory: fetched")
	}
	f = m.fetcher("test-key")
	f.KeyFile = ""
	n := len(m.calls)
	if _, err := f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil }); err == nil || len(m.calls) != n {
		t.Fatalf("no key file: %v", err)
	}
	// A refusal's body named, its request_id taken out; one still holding a request_id not named.
	if err := refused(403, []byte(`{"status":"NOT_AUTHORIZED","request_id":"6a7e"}`)); err == nil ||
		err.Error() != `massive refused: HTTP 403, status "NOT_AUTHORIZED": {"status":"NOT_AUTHORIZED"}` {
		t.Fatalf("a refusal: %v", err)
	}
	for _, body := range []string{`{"status":"ERROR","request_id":"6a7e","request_id":"6a7f"}`, `{"status":"ERROR","x":{"request_id":"6a7e"}}`,
		`<html>request_id 6a7e</html>`, `{"status":"ERROR","x":{"request\u005fid":"6a7e"}}`} {
		if err := refused(403, []byte(body)); err == nil || strings.Contains(err.Error(), "6a7e") || !strings.Contains(err.Error(), "not named") {
			t.Errorf("%s: %v", body, err)
		}
	}
	// Each name as it is written: status, not Status.
	if err := refused(200, []byte(`{"Status":"ERROR","status":"OK"}`)); err != nil {
		t.Errorf("status OK: %v", err)
	}
	if err := refused(200, []byte(`{"status":"ERROR","Status":"OK"}`)); err == nil {
		t.Error("status ERROR: not refused")
	}
}

func TestFetchWaits(t *testing.T) {
	// A 429 waited out (Retry-After) and asked again.
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n++; n == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, `{"status":"ERROR","error":"too many"}`, http.StatusTooManyRequests)
			return
		}
		w.Write(saved(t, "empty.json"))
	}))
	defer srv.Close()
	keyFile := filepath.Join(t.TempDir(), "key")
	os.WriteFile(keyFile, []byte("k3y\n"), 0o600)
	var log bytes.Buffer
	f := &Fetcher{Base: srv.URL, KeyFile: keyFile, Interval: time.Nanosecond, Dir: t.TempDir(), Client: srv.Client(), Log: &log}
	if _, err := f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil }); err != nil || n != 2 ||
		!strings.Contains(log.String(), "HTTP 429") {
		t.Fatalf("429: %v, %d calls, %q", err, n, log.String())
	}
	// No key, no call.
	f.KeyFile = filepath.Join(t.TempDir(), "none")
	if _, err := f.Fetch(context.Background(), sixMinutes, func(Page) error { return nil }); err == nil || n != 2 {
		t.Fatalf("no key: %v, %d calls", err, n)
	}
}

func TestWithout(t *testing.T) {
	for in, want := range map[string]string{
		`{"a":1,"request_id":"x","b":[2]}`:                `{"a":1,"b":[2]}`,
		`{"request_id":"x","a":1}`:                        `{"a":1}`,
		`{"a":1,"request_id":"x"}`:                        `{"a":1}`,
		`{"request_id":"x"}`:                              `{}`,
		`{ "request_id" : "x" , "a" : {"request_id":1} }`: `{  "a" : {"request_id":1} }`,
		"{\n  \"a\": 1,\n  \"request_id\": \"x\"\n}":      "{\n  \"a\": 1\n}",
		`{"a":1}`: `{"a":1}`,
	} {
		got, err := without([]byte(in), "request_id")
		if err != nil || string(got) != want {
			t.Errorf("%s: %s, %v; want %s", in, got, err, want)
		}
	}
	// Bytes not one JSON object, as they are when they do not hold the name; refused when they do.
	for _, in := range []string{`[1]`, `{"a":}`, `<html>`, `{"a":1} {}`} {
		if got, err := without([]byte(in), "request_id"); err != nil || string(got) != in {
			t.Errorf("%s: %s, %v", in, got, err)
		}
	}
	for _, in := range []string{`{"request_id":"x","request_id":"y"}`, `[{"request_id":1}]`, `{"a":1,"request_id":`,
		`{"request_id":"x"} {}`} {
		if got, err := without([]byte(in), "request_id"); err == nil {
			t.Errorf("%s: %s", in, got)
		}
	}
}

// TestImports: the library takes market's Go form and the standard library alone, its go.mod requiring market alone.
func TestImports(t *testing.T) {
	const market = "github.com/zhifengzhang-sz/qi-v2-contracts/market/go"
	const lib = "github.com/zhifengzhang-sz/qi-v2-platform/lib"
	files := 0
	err := filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		files++
		for _, s := range f.Imports {
			path := strings.Trim(s.Path.Value, `"`)
			first, _, _ := strings.Cut(path, "/")
			if !strings.Contains(first, ".") || path == market+"/market" || path == lib || strings.HasPrefix(path, lib+"/") {
				continue
			}
			t.Errorf("%s imports %s", p, path)
		}
		return nil
	})
	if err != nil || files == 0 {
		t.Fatalf("%d files: %v", files, err)
	}
	mod, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	var required []string
	block := false
	for l := range strings.SplitSeq(string(mod), "\n") {
		l, _, _ = strings.Cut(l, "//")
		f := strings.Fields(l)
		switch {
		case len(f) == 0:
		case block && f[0] == ")":
			block = false
		case block:
			required = append(required, f[0])
		case f[0] == "require" && len(f) > 1 && f[1] == "(":
			block = true
		case f[0] == "require":
			required = append(required, f[1])
		case regexp.MustCompile(`^(replace|tool|ignore)$`).MatchString(f[0]):
			t.Errorf("go.mod: %s", l)
		}
	}
	if strings.Join(required, " ") != market {
		t.Fatalf("go.mod requires %q", required)
	}
}
