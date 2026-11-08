package main

import (
	"context"
	"strings"
	"testing"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/mem"
	"github.com/zhifengzhang-sz/qi-v2-platform/lib/massive"
)

const cleanTopic = "market.clean.validation.v0-1-0.massive.SPY.M1"

// cleanRec is a record of the clean topic: SPY's K-line k at its place in the clean series.
func cleanRec(t *testing.T, k massive.KLine) stream.Record {
	t.Helper()
	m := msgOf(t, massive.Source, "SPY", k, R1)
	r, err := clean.Message{Cleaning: clean.Cleaning{Name: "validation", Version: "0.1.0"}, Origin: raw.Live,
		Source: m.Source, Instrument: m.Instrument, Timeframe: m.Timeframe, Publisher: "acquisition",
		KLine: m.KLine}.Record()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// cleanBus is a bus with the clean topic, its records recs.
func cleanBus(t *testing.T, recs ...stream.Record) *mem.Bus {
	t.Helper()
	ctx := context.Background()
	b := topic(t)
	adm, _ := b.Admin(ctx)
	if _, err := clean.Provision(ctx, adm, cleanTopic, 1); err != nil {
		t.Fatal(err)
	}
	publish(t, b, recs...)
	return b
}

// -clean: the store's clean series of the topic's series emptied, then written with every K-line of the topic, a copy
// dropped, one of another value counted; another series' kept.
func TestCleanFrom(t *testing.T) {
	ctx := context.Background()
	k0, k1, k2 := kline(t, 0, "600.1"), kline(t, 1, "600.2"), kline(t, 2, "600.3")
	other := k1
	other.Close = dec(t, "600.9")
	b := cleanBus(t, cleanRec(t, k0), cleanRec(t, k1), cleanRec(t, k1), cleanRec(t, other), cleanRec(t, k2))
	st := newFake()
	stale, qqq := minute{"SPY", "M1", 1}, minute{"QQQ", "M1", k0.Date}
	st.clean[stale], st.clean[qqq] = version{k: k0}, version{k: k0}
	r, err := cleanFrom(ctx, b, st, cleanTopic, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if r.read != 5 || r.copies != 2 || r.differing != 1 || r.written != 3 {
		t.Fatalf("report %+v", r)
	}
	if !strings.Contains(r.String(), cleanTopic+": 5 K-lines read, 2 copies dropped (1 of another value); the clean "+
		"series: 3 rows written, the series emptied first") {
		t.Fatalf("report %q", r)
	}
	if _, ok := st.clean[stale]; ok || len(st.clean) != 4 || st.clean[qqq].k != k0 {
		t.Fatalf("the clean series: %+v", st.clean)
	}
	for _, k := range []massive.KLine{k0, k1, k2} {
		if v := st.clean[minute{"SPY", "M1", k.Date}]; v.k != k || v.read != 0 {
			t.Fatalf("the clean series at %d: %+v", k.Date, v)
		}
	}
	// a topic of no K-line: the series emptied
	b = cleanBus(t)
	if r, err := cleanFrom(ctx, b, st, cleanTopic, quiet); err != nil || r.written != 0 || len(st.clean) != 1 {
		t.Fatalf("an empty topic: %+v, %v, %d", r, err, len(st.clean))
	}
	// refused: a name not a clean topic's; a clean topic of another source
	for _, name := range []string{"market.raw", "market.clean.validation.v0-1-0.binance.BTCUSDT.M1"} {
		if _, err := cleanFrom(ctx, b, st, name, quiet); err == nil {
			t.Errorf("%s: written", name)
		}
	}
}
