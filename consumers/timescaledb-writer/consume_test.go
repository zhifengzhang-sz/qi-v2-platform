package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/mem"
	"github.com/zhifengzhang-sz/qi-v2-platform/lib/massive"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// The reads of the runs: R1 the first versions', R2 and R3 two later runs'.
const (
	R1 = 1764280800000
	R2 = 1764367200000
	R3 = 1764453600000
)

func dec(t *testing.T, s string) market.Decimal {
	t.Helper()
	d, err := market.ParseDecimal(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func kline(t *testing.T, i int, close string) massive.KLine {
	return massive.KLine{Index: uint64(i), Date: 1764167400000 + uint64(i)*60_000, Open: dec(t, "600"),
		High: dec(t, "601"), Low: dec(t, "599"), Close: dec(t, close), Volume: dec(t, "10"), Vw: dec(t, "600.5"),
		HasVw: true, N: 3, HasN: true}
}

// zero is the zero K-line at i: the base's fields alone, every price and the volume 0.
func zero(i int) massive.KLine {
	return massive.KLine{Index: uint64(i), Date: 1764167400000 + uint64(i)*60_000}
}

// msgOf is the message of a K-line of Massive's, in its text (massive.Text); of the base's alone, vw and n not set.
func msgOf(t *testing.T, source, instrument string, k massive.KLine, read uint64) raw.Message {
	t.Helper()
	text, err := massive.Text(k)
	if err != nil {
		t.Fatal(err)
	}
	rk, err := raw.DecodeKLine([]byte(strings.TrimSuffix(text, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	return raw.Message{Origin: raw.Live, Source: source, Instrument: instrument, Timeframe: "M1", Publisher: "acquisition",
		KLine: rk, Read: read}
}

func msg(t *testing.T, source, instrument string, i int, close string) raw.Message {
	k := kline(t, i, close)
	if source != massive.Source {
		k.Vw, k.HasVw, k.N, k.HasN = market.Decimal{}, false, 0, false
	}
	return msgOf(t, source, instrument, k, R1)
}

func rec(t *testing.T, m raw.Message) stream.Record {
	t.Helper()
	r, err := m.Record()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// crec is the record of a correction: the K-line k of instrument read at read, naming fields.
func crec(t *testing.T, instrument string, k massive.KLine, read uint64, fields ...string) stream.Record {
	t.Helper()
	r, err := raw.Correction{Message: msgOf(t, massive.Source, instrument, k, read), Corrected: fields}.Record()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// before is the record of a value of market.raw of a version before, 0.2.0 or 0.1.0, as the topic holds them.
func before(version string, k massive.KLine) stream.Record {
	text, _ := massive.Text(k)
	v := `{"instrument":"SPY","kline":` + strings.TrimSuffix(text, "\n") + `,"origin":"live","publisher":"acquisition","schema":"market.raw","source":"massive","timeframe":"M1","version":"` + version + `"}`
	return stream.Record{Topic: raw.Topic, Key: []byte("massive:SPY:M1"), PartitionKey: []byte("SPY"), Value: []byte(v),
		Time: time.UnixMilli(int64(k.Date))}
}

func publish(t *testing.T, b *mem.Bus, recs ...stream.Record) {
	t.Helper()
	ctx := context.Background()
	sink, _ := b.Sink(ctx)
	for _, r := range recs {
		if err := sink.Publish(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
}

func topic(t *testing.T, recs ...stream.Record) *mem.Bus {
	t.Helper()
	ctx := context.Background()
	b := mem.New()
	adm, _ := b.Admin(ctx)
	if _, err := raw.Provision(ctx, adm, 1); err != nil {
		t.Fatal(err)
	}
	publish(t, b, recs...)
	return b
}

// fake is a store in memory, by the store's own rules: a row of the raw and of the corrections inserted once; a row
// of the corrected raw inserted, or replaced by one of a later read. after has it end the run once the raw holds that
// many bars.
type fake struct {
	raw         map[minute]version
	corrections map[stamp]correction
	corrected   map[minute]version
	clean       map[minute]version
	writes      int
	after       int
	stop        context.CancelFunc
}

func newFake() *fake {
	return &fake{raw: map[minute]version{}, corrections: map[stamp]correction{}, corrected: map[minute]version{},
		clean: map[minute]version{}}
}

// Clean: the series' clean rows deleted, the rows put, each minute once.
func (f *fake) Clean(ctx context.Context, instrument, timeframe string, rs []row) (int, error) {
	for m := range f.clean {
		if m.instrument == instrument && m.timeframe == timeframe {
			delete(f.clean, m)
		}
	}
	for _, r := range rs {
		if _, ok := f.clean[r.m]; ok {
			return 0, errors.New("a minute twice")
		}
		f.clean[r.m] = r.v
	}
	n := 0
	for m := range f.clean {
		if m.instrument == instrument && m.timeframe == timeframe {
			n++
		}
	}
	return n, nil
}

func (f *fake) Empty(ctx context.Context) (bool, error) { return len(f.raw) == 0, nil }

func (f *fake) Held(ctx context.Context, minutes []minute, stamps []stamp) (held, error) {
	h := held{raw: map[minute]version{}, corrections: map[stamp]correction{}, corrected: map[minute]version{}}
	for _, m := range minutes {
		if v, ok := f.raw[m]; ok {
			h.raw[m] = v
		}
		if v, ok := f.corrected[m]; ok {
			h.corrected[m] = v
		}
	}
	for _, s := range stamps {
		if c, ok := f.corrections[s]; ok {
			h.corrections[s] = c
		}
	}
	return h, nil
}

func (f *fake) Write(ctx context.Context, w rows) (put, error) {
	var p put
	f.writes++
	for _, r := range w.raw {
		if _, ok := f.raw[r.m]; !ok {
			f.raw[r.m] = r.v
			p.raw++
		}
	}
	for _, c := range w.corrections {
		if _, ok := f.corrections[c.s]; !ok {
			f.corrections[c.s] = c.c
			p.corrections++
		}
	}
	for _, r := range w.corrected {
		if was, ok := f.corrected[r.m]; !ok || r.v.read > was.read {
			f.corrected[r.m] = r.v
			p.corrected++
		}
	}
	if f.stop != nil && len(f.raw) >= f.after {
		f.stop()
	}
	return p, nil
}

// closes: a series' bars by key, each its close, index and read.
func closes(s map[minute]version) map[string]string {
	out := map[string]string{}
	for m, v := range s {
		out[m.instrument+":"+strconv.FormatUint(m.date, 10)] = v.k.Close.String() + "/" + strconv.FormatUint(v.k.Index, 10) +
			"/" + strconv.FormatUint(v.read, 10)
	}
	return out
}

func same(t *testing.T, what string, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %v, want %v", what, got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: %v, want %v", what, got, want)
		}
	}
}

const (
	d0 = "1764167400000"
	d1 = "1764167460000"
	d2 = "1764167520000"
)

func TestToEnd(t *testing.T) {
	// SPY's three bars, then each again (copies), one copy of another value; a QQQ bar; a bar of another source; a
	// correction of SPY's bar 1.
	first := []stream.Record{rec(t, msg(t, "massive", "SPY", 0, "600.1")), rec(t, msg(t, "massive", "SPY", 1, "600.2")),
		rec(t, msg(t, "massive", "SPY", 2, "600.3"))}
	b := topic(t, append(append(append(append([]stream.Record(nil), first...), rec(t, msg(t, "massive", "QQQ", 0, "600")),
		rec(t, msg(t, "binance", "SPY", 7, "600"))), first...), rec(t, msg(t, "massive", "SPY", 2, "600.35")),
		crec(t, "SPY", kline(t, 1, "600.25"), R2, "close"))...)
	ctx := context.Background()
	st := newFake()
	r, err := consume(ctx, b, st, options{name: "w", toEnd: true}, quiet)
	if err != nil {
		t.Fatal(err)
	}
	// one fetch: the copies dropped, the first taken; the corrected raw each minute's latest
	want := report{read: 10, other: 1, raw: taken{stored: 4, copies: 4, differing: 1}, corrections: taken{stored: 1},
		corrected: 4}
	if r != want || st.writes != 1 {
		t.Fatalf("report %+v, want %+v; %d writes", r, want, st.writes)
	}
	same(t, "the raw", closes(st.raw), map[string]string{"SPY:" + d0: "600.1/0/" + itoa(R1),
		"SPY:" + d1: "600.2/1/" + itoa(R1), "SPY:" + d2: "600.3/2/" + itoa(R1), "QQQ:" + d0: "600/0/" + itoa(R1)})
	same(t, "the corrected raw", closes(st.corrected), map[string]string{"SPY:" + d0: "600.1/0/" + itoa(R1),
		"SPY:" + d1: "600.25/1/" + itoa(R2), "SPY:" + d2: "600.3/2/" + itoa(R1), "QQQ:" + d0: "600/0/" + itoa(R1)})
	if c := st.corrections[stamp{minute{"SPY", "M1", 1764167460000}, R2}]; len(st.corrections) != 1 ||
		!slices.Equal(c.corrected, []string{"close"}) || c.k.Close.String() != "600.25" {
		t.Fatalf("the corrections %+v", st.corrections)
	}
	// Run again to the end: the topics read again from their start, every version a copy, nothing written.
	r, err = consume(ctx, b, st, options{name: "w", toEnd: true}, quiet)
	want = report{read: 10, other: 1, raw: taken{copies: 8, differing: 1}, corrections: taken{copies: 1}}
	if err != nil || r != want {
		t.Fatalf("again: %+v, want %+v, %v", r, want, err)
	}
	// A later correction, in a later fetch: the corrected raw's minute replaced, the raw as it is.
	publish(t, b, crec(t, "SPY", kline(t, 1, "600.3"), R3, "close"))
	r, err = consume(ctx, b, st, options{name: "w", toEnd: true}, quiet)
	if err != nil || r.corrections.stored != 1 || r.corrected != 1 || closes(st.corrected)["SPY:"+d1] != "600.3/1/"+itoa(R3) ||
		closes(st.raw)["SPY:"+d1] != "600.2/1/"+itoa(R1) {
		t.Fatalf("revised: %+v, %v, %v", r, err, closes(st.corrected))
	}
}

func itoa(n uint64) string { return strconv.FormatUint(n, 10) }

// A minute without a bar, its first version the zero K-line; a correction to a bar, and back: the raw the zero
// K-line, the corrections both, the corrected raw each time the latest.
func TestZeroAndBack(t *testing.T) {
	ctx := context.Background()
	z := zero(1)
	b := topic(t, rec(t, msg(t, "massive", "SPY", 0, "600.1")), rec(t, msgOf(t, "massive", "SPY", z, R1)))
	st := newFake()
	if _, err := consume(ctx, b, st, options{name: "w", toEnd: true}, quiet); err != nil {
		t.Fatal(err)
	}
	if v := st.raw[minute{"SPY", "M1", z.Date}]; v.k != z || v.k.HasVw || v.k.HasN {
		t.Fatalf("the zero K-line: %+v", v)
	}
	all := []string{"close", "high", "low", "n", "open", "volume", "vw"}
	publish(t, b, crec(t, "SPY", kline(t, 1, "600.2"), R2, all...))
	if _, err := consume(ctx, b, st, options{name: "w", toEnd: true}, quiet); err != nil {
		t.Fatal(err)
	}
	if v := st.corrected[minute{"SPY", "M1", z.Date}]; v.k != kline(t, 1, "600.2") || v.read != R2 {
		t.Fatalf("to a bar: %+v", v)
	}
	publish(t, b, crec(t, "SPY", z, R3, all...))
	r, err := consume(ctx, b, st, options{name: "w", toEnd: true}, quiet)
	if err != nil || r.corrections.stored != 1 || r.corrections.copies != 1 || r.corrected != 1 {
		t.Fatalf("back: %+v, %v", r, err)
	}
	if v := st.corrected[minute{"SPY", "M1", z.Date}]; v.k != z || v.read != R3 || len(st.corrections) != 2 ||
		st.raw[minute{"SPY", "M1", z.Date}].k != z {
		t.Fatalf("back to the zero K-line: %+v", v)
	}
}

// A correction read before its minute's first version (another topic, an earlier fetch): the corrected raw the
// correction, kept when the first version comes.
func TestCorrectionFirst(t *testing.T) {
	ctx := context.Background()
	b := topic(t, crec(t, "SPY", kline(t, 0, "600.2"), R2, "close"))
	st := newFake()
	if _, err := consume(ctx, b, st, options{name: "w", toEnd: true}, quiet); err != nil {
		t.Fatal(err)
	}
	publish(t, b, rec(t, msg(t, "massive", "SPY", 0, "600.1")))
	ctx, cancel := context.WithCancel(ctx)
	st.after, st.stop = 1, cancel
	r, err := consume(ctx, b, st, options{name: "w"}, quiet)
	if !errors.Is(err, context.Canceled) || r.raw.stored != 1 || r.corrected != 0 {
		t.Fatalf("%+v, %v", r, err)
	}
	if v := st.corrected[minute{"SPY", "M1", 1764167400000}]; v.read != R2 || v.k.Close.String() != "600.2" {
		t.Fatalf("the corrected raw %+v", v)
	}
}

// Values of 0.2.0 and 0.1.0 on market.raw: first versions with no read; a copy dropped.
func TestVersionsBefore(t *testing.T) {
	b := topic(t, before("0.1.0", kline(t, 0, "600.1")), before("0.2.0", kline(t, 1, "600.2")),
		before("0.2.0", kline(t, 1, "600.2")), rec(t, msg(t, "massive", "SPY", 0, "600.1")))
	st := newFake()
	r, err := consume(context.Background(), b, st, options{name: "w", toEnd: true}, quiet)
	if err != nil || r.raw != (taken{stored: 2, copies: 2, differing: 1}) {
		t.Fatalf("%+v, %v", r, err)
	}
	same(t, "the raw", closes(st.raw), map[string]string{"SPY:" + d0: "600.1/0/0", "SPY:" + d1: "600.2/1/0"})
	same(t, "the corrected raw", closes(st.corrected), closes(st.raw))
}

func TestEmptyTopic(t *testing.T) {
	r, err := consume(context.Background(), topic(t), newFake(), options{name: "w", toEnd: true}, quiet)
	if err != nil || r.read != 0 {
		t.Fatalf("%+v, %v", r, err)
	}
}

// Following the topics: from the group's committed offsets while the raw holds bars, from the topics' start when it
// holds none.
func TestFollow(t *testing.T) {
	b := topic(t, rec(t, msg(t, "massive", "SPY", 0, "600.1")), rec(t, msg(t, "massive", "SPY", 1, "600.2")))
	st := newFake()
	if _, err := consume(context.Background(), b, st, options{name: "w", toEnd: true}, quiet); err != nil {
		t.Fatal(err)
	}
	publish(t, b, rec(t, msg(t, "massive", "SPY", 2, "600.3")), crec(t, "SPY", kline(t, 0, "600.15"), R2, "close"))
	ctx, cancel := context.WithCancel(context.Background())
	st.after, st.stop = 3, cancel
	r, err := consume(ctx, b, st, options{name: "w"}, quiet)
	if !errors.Is(err, context.Canceled) || r.read != 2 || r.raw.stored != 1 || r.corrections.stored != 1 || r.corrected != 2 {
		t.Fatalf("from the committed offsets: %+v, %v", r, err)
	}
	// A store made again: the topics from their start, whatever the group committed.
	again := newFake()
	ctx, cancel = context.WithCancel(context.Background())
	again.after, again.stop = 3, cancel
	r, err = consume(ctx, b, again, options{name: "w"}, quiet)
	if !errors.Is(err, context.Canceled) || r.read != 4 || r.raw.stored != 3 || len(again.corrected) != 3 ||
		len(again.corrections) != 1 {
		t.Fatalf("an empty store: %+v, %v", r, err)
	}
}

func TestRefused(t *testing.T) {
	ctx := context.Background()
	for name, r := range map[string]stream.Record{
		"a value not raw market data": {Topic: raw.Topic, Key: []byte("massive:SPY:M1"), PartitionKey: []byte("SPY"),
			Value: []byte(`{"not":"raw"}`)},
		"a value of market.raw on market.corrections": func() stream.Record {
			r := rec(t, msg(t, "massive", "SPY", 0, "600"))
			r.Topic = raw.CorrectionsTopic
			return r
		}(),
		"a correction on market.raw": func() stream.Record {
			r := crec(t, "SPY", kline(t, 0, "600"), R2, "close")
			r.Topic = raw.Topic
			return r
		}(),
	} {
		b := topic(t)
		publish(t, b, r)
		if _, err := consume(ctx, b, newFake(), options{name: "w", toEnd: true}, quiet); err == nil {
			t.Errorf("%s: read", name)
		}
	}
	// A message of massive whose K-line is not Massive's: a field Massive's K-line has not.
	k, err := raw.DecodeKLine([]byte(`{"close":600,"date":1764167400000,"high":601,"index":0,"low":599,"open":600,"volume":10,"x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	b := topic(t, rec(t, raw.Message{Origin: raw.Live, Source: massive.Source, Instrument: "SPY", Timeframe: "M1",
		Publisher: "acquisition", KLine: k, Read: R1}))
	if _, err := consume(ctx, b, newFake(), options{name: "w", toEnd: true}, quiet); err == nil ||
		!strings.Contains(err.Error(), `"x": a name that is no field of Massive's K-line`) {
		t.Fatalf("massive with a field Massive's K-line has not: %v", err)
	}
}

// TestWide: market's widest decimals, 15 digits before the point, and 9 before it and 6 after, consumed and written
// as they are: every decimal of market fits the store.
func TestWide(t *testing.T) {
	k := kline(t, 0, "999999999999999")
	k.Open, k.High, k.Low = dec(t, "999999999999999"), dec(t, "999999999999999"), dec(t, "999999999999999")
	k.Volume, k.Vw = dec(t, "999999999.999999"), dec(t, "-999999999999999")
	st := newFake()
	if _, err := consume(context.Background(), topic(t, rec(t, msgOf(t, "massive", "SPY", k, R1))), st,
		options{name: "w", toEnd: true}, quiet); err != nil || st.writes != 1 || len(st.raw) != 1 {
		t.Fatalf("%v, %d writes, %d bars", err, st.writes, len(st.raw))
	}
	for _, v := range st.raw {
		if v.k != k {
			t.Fatalf("written %+v, not %+v", v.k, k)
		}
	}
}
