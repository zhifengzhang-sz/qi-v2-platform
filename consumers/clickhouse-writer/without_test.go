package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/lib/massive"
)

// page is a page of Massive's as its spec allows one: its first row without vw and n, its second with both.
const page = `{"ticker":"SPY","queryCount":2,"resultsCount":2,"adjusted":false,"results":[` +
	`{"v":976.041273,"o":763.68,"c":763.97,"h":764.07,"l":763.68,"t":1792742580000},` +
	`{"v":3612.094383,"vw":763.2819,"o":763.93,"c":763.68,"h":763.93,"l":763,"t":1792742640000,"n":75}],` +
	`"status":"OK","count":2}`

// TestWithoutVwAndN: a K-line without vw and n, read from a page by the library's reader, its line published on the
// topic as acquisition publishes a plugin's line (raw.DecodeKLine, raw.Message), consumed and stored with both empty
// (NULL), in the raw and the corrected raw, and read again the same.
func TestWithoutVwAndN(t *testing.T) {
	serverURL, database := scratch(t)
	ctx := context.Background()
	p, err := massive.ReadPage([]byte(page))
	if err != nil {
		t.Fatal(err)
	}
	bars := massive.NewBars(massive.Request{Instrument: "SPY", Timeframe: "M1",
		Spans: [][2]uint64{{1792742580000, 1792742640000}}}, time.UnixMilli(1792742700000))
	var ms []stream.Record
	var lines []string
	err = bars.Page(p, func(line string) error {
		lines = append(lines, line)
		k, err := raw.DecodeKLine([]byte(strings.TrimSuffix(line, "\n")))
		if err != nil {
			return err
		}
		ms = append(ms, rec(t, raw.Message{Origin: raw.Live, Source: massive.Source, Instrument: "SPY", Timeframe: "M1",
			Publisher: "acquisition", KLine: k, Read: R1}))
		return nil
	})
	if err != nil || len(ms) != 2 {
		t.Fatalf("%d lines, %v", len(ms), err)
	}
	if want := `{"close":763.97,"date":1792742580000,"high":764.07,"index":0,"low":763.68,"open":763.68,"volume":976.041273}` + "\n"; lines[0] != want {
		t.Fatalf("line 0: %q, want %q", lines[0], want)
	}
	st, err := open(ctx, serverURL, database)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close(ctx)
	b := topic(t, ms...)
	r, err := consume(ctx, b, st, options{name: "w", toEnd: true}, quiet)
	if err != nil || r.read != 2 || r.raw.stored != 2 || r.corrected != 2 {
		t.Fatalf("%+v, %v", r, err)
	}
	want := "SPY\tM1\t0\t1792742580000\t763.68\t764.07\t763.68\t763.97\t976.041273\t\\N\t\\N\t1764280800000\n" +
		"SPY\tM1\t1\t1792742640000\t763.93\t763.93\t763\t763.68\t3612.094383\t763.2819\t75\t1764280800000"
	for _, table := range []string{"klines", "klines_corrected"} {
		if got := query(t, st, "SELECT "+columns+" FROM "+table+" FINAL ORDER BY instrument, date"); got != want {
			t.Fatalf("%s:\n%s\nwant:\n%s", table, got, want)
		}
	}
	r, err = consume(ctx, b, st, options{name: "w", toEnd: true}, quiet)
	if err != nil || r.read != 2 || r.raw.copies != 2 || r.raw.stored+r.corrected != 0 {
		t.Fatalf("again: %+v, %v", r, err)
	}
}
