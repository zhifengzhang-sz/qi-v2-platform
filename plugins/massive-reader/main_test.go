package main

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/lib/massive"
)

// The pages are Massive's answer to SPY's first six minutes of 2026-10-23 (04:00 New York time), recorded three bars
// a page; the third page is Massive's answer of no row. The reading of pages is lib/massive's, tested there; here the
// plugin's lines, as they were before the library held its rules.
func server(t *testing.T) *httptest.Server {
	t.Helper()
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	p1, p2 := read("massive-page1.json"), read("massive-page2.json")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" || strings.Contains(r.URL.String(), "test-key") {
			http.Error(w, `{"status":"ERROR","error":"unknown API Key"}`, http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Query().Get("cursor") == "":
			if r.URL.Query().Get("adjusted") != "false" || r.URL.Path != "/v2/aggs/ticker/SPY/range/1/minute/1792742400000/1792742700000" {
				http.Error(w, `{"status":"ERROR","error":"bad request"}`, http.StatusBadRequest)
				return
			}
			w.Write([]byte(strings.ReplaceAll(p1, "https://api.massive.com", srv.URL)))
		case strings.Contains(r.URL.Path, "/1792742580000/"):
			w.Write([]byte(strings.ReplaceAll(p2, "https://api.massive.com", srv.URL)))
		default:
			w.Write([]byte(`{"ticker":"SPY","queryCount":0,"resultsCount":0,"adjusted":false,"status":"OK","request_id":"0"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func backfill(t *testing.T, srv *httptest.Server, key string, r massive.Request, now time.Time) (string, massive.Counts, error) {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyFile, []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &massive.Fetcher{Base: srv.URL, KeyFile: keyFile, Interval: time.Nanosecond, Dir: t.TempDir(), Client: srv.Client()}
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	c, _, err := run(context.Background(), f, r, now, w)
	w.Flush()
	return b.String(), c, err
}

var six = massive.Request{Instrument: "SPY", Timeframe: "M1", Spans: [][2]uint64{{1792742400000, 1792742700000}}}

func TestBackfill(t *testing.T) {
	out, c, err := backfill(t, server(t), "test-key", six, time.UnixMilli(1792742700000+60_000))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		`{"close":763.64,"date":1792742400000,"high":764.8,"index":0,"low":763,"n":976,"open":764.7,"volume":10391.875729,"vw":763.8517}`,
		`{"close":763.71,"date":1792742460000,"high":764.31,"index":1,"low":763,"n":703,"open":763.54,"volume":17758.504868,"vw":763.635}`,
		`{"close":763.64,"date":1792742520000,"high":764.6,"index":2,"low":762.93,"n":1111,"open":763.76,"volume":37281.842425,"vw":763.6795}`,
		`{"close":763.97,"date":1792742580000,"high":764.07,"index":3,"low":763.68,"n":19,"open":763.68,"volume":976.041273,"vw":763.9437}`,
		`{"close":763.68,"date":1792742640000,"high":763.93,"index":4,"low":763,"n":75,"open":763.93,"volume":3612.094383,"vw":763.2819}`,
		`{"close":763.77,"date":1792742700000,"high":764.09,"index":5,"low":763.59,"n":26,"open":763.68,"volume":1960.043275,"vw":763.6787}`,
	}, "\n") + "\n"
	if out != want {
		t.Fatalf("lines:\n%s\nwant:\n%s", out, want)
	}
	if c != (massive.Counts{Pages: 3, Rows: 6, Lines: 6}) {
		t.Fatalf("counts %+v", c)
	}
	// A refused key stops the run.
	if _, _, err := backfill(t, server(t), "other", six, time.UnixMilli(1792742700000+60_000)); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("a refused key: %v", err)
	}
}

func TestSpan(t *testing.T) {
	if r, err := parseSpan([]string{"SPY:M1", "1", "2", "4", "4"}); err != nil || r.Instrument != "SPY" || r.Timeframe != "M1" ||
		len(r.Spans) != 2 || r.Spans[0] != [2]uint64{1, 2} || r.Spans[1] != [2]uint64{4, 4} {
		t.Fatalf("%+v, %v", r, err)
	}
	for _, args := range [][]string{{"SPY:M1", "2", "1"}, {"SPY", "1", "2"}, {"SPY:D1", "1", "2"}, {"SPY:M1", "-1", "2"},
		{"SPY/x:M1", "1", "2"}, {"SPY:M1", "1"}, {"SPY:M1", "1", "2", "3"}, {"SPY:M1", "1", "5", "5", "6"}, {"SPY:M1", "4", "5", "1", "2"}} {
		if _, err := parseSpan(args); err == nil {
			t.Errorf("%v: no fault", args)
		}
	}
}
