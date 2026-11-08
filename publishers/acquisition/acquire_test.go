package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/mem"
)

// The test binary is the plugin too: with ACQ_PLUGIN_LINES set, it writes its arguments to ACQ_PLUGIN_ARGS, prints
// that file's lines (with ACQ_PLUGIN_FILTER=1, those whose date is in a span it is asked for, as a plugin prints them)
// and exits with ACQ_PLUGIN_EXIT (0 when unset). Run with --clean, it is the cleaning (cleaning).
func TestMain(m *testing.M) {
	if slices.Contains(os.Args[1:], "--clean") {
		cleaning()
	}
	if lines := os.Getenv("ACQ_PLUGIN_LINES"); lines != "" {
		_ = os.WriteFile(os.Getenv("ACQ_PLUGIN_ARGS"), []byte(strings.Join(os.Args[1:], " ")), 0o644)
		b, _ := os.ReadFile(lines)
		if os.Getenv("ACQ_PLUGIN_FILTER") == "1" {
			args := os.Args[slices.Index(os.Args, "--backfill")+2:]
			var kept []byte
			for _, l := range strings.SplitAfter(string(b), "\n") {
				_, after, _ := strings.Cut(l, `"date":`)
				d, _ := strconv.ParseUint(after[:strings.IndexByte(after+",", ',')], 10, 64)
				for i := 0; i+1 < len(args); i += 2 {
					from, _ := strconv.ParseUint(args[i], 10, 64)
					to, _ := strconv.ParseUint(args[i+1], 10, 64)
					if d >= from && d <= to {
						kept = append(kept, l...)
					}
				}
			}
			b = kept
		}
		os.Stdout.Write(b)
		if os.Getenv("ACQ_PLUGIN_EXIT") == "1" {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// cleaning is the cleaning's protocol (plugins.md, 3) with the validation's rules on the prices and the dates alone:
// its name and version, then for each K-line on stdin {"kline":…} as it came, or {"refused":…} for one whose open is not
// above 0 or whose date is not after the last accepted (--after, the last before); with ACQ_CLEANING_NAME, that name.
func cleaning() {
	name := os.Getenv("ACQ_CLEANING_NAME")
	if name == "" {
		name = "validation"
	}
	fmt.Printf(`{"cleaning":%q,"version":"0.1.0"}`+"\n", name)
	var last uint64
	any := false
	if i := slices.Index(os.Args, "--after"); i > 0 {
		last, _ = strconv.ParseUint(os.Args[i+1], 10, 64)
		any = true
	}
	w := bufio.NewWriter(os.Stdout)
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		k, err := raw.DecodeKLine(sc.Bytes())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		switch b := k.Base(); {
		case b.Open.Sign() <= 0:
			fmt.Fprintf(w, `{"refused":"open %s: a price not above 0"}`+"\n", b.Open)
		case any && b.Date <= last:
			fmt.Fprintf(w, `{"refused":"date %d not after %d, the last accepted"}`+"\n", b.Date, last)
		default:
			fmt.Fprintf(w, `{"kline":%s}`+"\n", sc.Bytes())
			last, any = b.Date, true
		}
	}
	w.Flush()
	os.Exit(0)
}

// The test catalog: a history of 3 days, so that a series with none asks for few days; Thanksgiving closed, the day
// after it closing at 13:00.
const catalogText = `datasets:
  - name: massive-spy-1m
    source: massive
    command: [bin/plugin, -x]
    instruments: [SPY]
    timeframes: [M1]
    history: 3d
    reread: 30d
    delay: 20h
    cleaning: [bin/validation]
    tolerance: 10
    session:
      zone: America/New_York
      open: "09:30"
      close: "16:00"
      closed: [2025-11-27]
      early: { 2025-11-28: "13:00" }
    store: { retention: forever, compressAfter: 7d }
  - name: other
    source: binance
    instruments: [BTCUSDT]
    timeframes: [M1]
    history: 30d
`

func writeCatalog(t *testing.T, text string) string {
	p := filepath.Join(t.TempDir(), "catalog.yaml")
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

var ny, _ = time.LoadLocation("America/New_York")

// at is a time in New York; ms, its milliseconds since 1970.
func at(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, ny) }

func ms(y int, m time.Month, d, h, min int) uint64 { return uint64(at(y, m, d, h, min).UnixMilli()) }

func itoa(n uint64) string { return strconv.FormatUint(n, 10) }

func TestCatalog(t *testing.T) {
	p := writeCatalog(t, catalogText)
	d, err := loadDataset(p, "massive-spy-1m")
	if err != nil {
		t.Fatal(err)
	}
	if d.Source != "massive" || d.Command[0] != filepath.Join(filepath.Dir(p), "bin/plugin") || d.Command[1] != "-x" ||
		d.Cleaning[0] != filepath.Join(filepath.Dir(p), "bin/validation") || *d.Tolerance != 10 {
		t.Fatalf("dataset %+v", d)
	}
	// reread and delay, as the catalog writes them, in milliseconds
	if d.reread != 30*24*3600*1000 || d.delay != 20*3600*1000 {
		t.Fatalf("reread %d, delay %d", d.reread, d.delay)
	}
	if _, err := loadDataset(p, "other"); err == nil {
		t.Fatalf("a dataset without a command: loaded")
	}
	if _, err := loadDataset(p, "none"); err == nil {
		t.Fatalf("no such dataset: loaded")
	}
	for name, c := range map[string]struct{ from, to, err string }{
		"a timeframe not the catalog's": {"timeframes: [M1]", "timeframes: [M2]", `timeframe "M2"`},
		"no reread":                     {"    reread: 30d\n", "", "no reread"},
		"no delay":                      {"    delay: 20h\n", "", "no delay"},
		"a reread not a duration":       {"reread: 30d", "reread: 30 days", "reread"},
		"a delay not a duration":        {"delay: 20h", "delay: 20s", "delay"},
		"no cleaning":                   {"    cleaning: [bin/validation]\n", "", "no cleaning"},
		"no tolerance":                  {"    tolerance: 10\n", "", "no tolerance"},
		"a tolerance below 0":           {"tolerance: 10", "tolerance: -1", "no tolerance"},
		"a tolerance not a number":      {"tolerance: 10", "tolerance: ten", "`ten` into int"},
	} {
		p := writeCatalog(t, strings.Replace(catalogText, c.from, c.to, 1))
		if _, err := loadDataset(p, "massive-spy-1m"); err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	for s, want := range map[string]bool{
		"09:29 26": false, "09:30 26": true, "15:59 26": true, "16:00 26": false,
		"10:00 27": false,                   // closed
		"12:59 28": true, "13:00 28": false, // early
		"10:00 29": false, // a Saturday
	} {
		var h, m, day int
		fmt.Sscanf(s, "%d:%d %d", &h, &m, &day)
		if got := d.Session.contains(ms(2025, 11, day, h, m)); got != want {
			t.Errorf("2025-11-%d %02d:%02d: in the session %v, want %v", day, h, m, got, want)
		}
	}
	// The spans of 2025-11-24 10:00 to 2025-11-30: Monday from 10:00, Tuesday and Wednesday whole, Thursday closed,
	// Friday to its early close, the weekend none; each [open, close − 1 ms].
	got := d.Session.spans(ms(2025, 11, 24, 10, 0), ms(2025, 11, 30, 23, 59))
	want := [][2]uint64{
		{ms(2025, 11, 24, 10, 0), ms(2025, 11, 24, 16, 0) - 1}, {ms(2025, 11, 25, 9, 30), ms(2025, 11, 25, 16, 0) - 1},
		{ms(2025, 11, 26, 9, 30), ms(2025, 11, 26, 16, 0) - 1}, {ms(2025, 11, 28, 9, 30), ms(2025, 11, 28, 13, 0) - 1},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("spans %v, want %v", got, want)
	}
}

func testGrid(t *testing.T) grid {
	d, err := loadDataset(writeCatalog(t, catalogText), "massive-spy-1m")
	if err != nil {
		t.Fatal(err)
	}
	return grid{step: 60_000, s: d.Session}
}

func TestGrid(t *testing.T) {
	g := testGrid(t)
	// A week: Monday to Wednesday 390 minutes each, Thanksgiving none, Friday 210, the weekend none.
	week := g.within(ms(2025, 11, 24, 0, 0), ms(2025, 11, 30, 23, 59))
	if len(week) != 3*390+210 || week[0] != ms(2025, 11, 24, 9, 30) || week[389] != ms(2025, 11, 24, 15, 59) ||
		week[390] != ms(2025, 11, 25, 9, 30) || week[len(week)-1] != ms(2025, 11, 28, 12, 59) {
		t.Fatalf("a week: %d minutes", len(week))
	}
	for i := 1; i < len(week); i++ {
		if week[i] <= week[i-1] {
			t.Fatalf("not ascending at %d", i)
		}
	}
	// Across the clock's change (2025-11-02, a Sunday): Friday and Monday each 390, 09:30 New York in both zones.
	if w := g.within(ms(2025, 10, 31, 0, 0), ms(2025, 11, 3, 23, 59)); len(w) != 780 || w[390] != ms(2025, 11, 3, 9, 30) {
		t.Fatalf("across the clock's change: %d", len(w))
	}
	for _, c := range []struct {
		t, after, before uint64
	}{
		{ms(2025, 11, 26, 10, 0), ms(2025, 11, 26, 10, 0), ms(2025, 11, 26, 10, 0)},
		{ms(2025, 11, 26, 10, 0) + 1, ms(2025, 11, 26, 10, 1), ms(2025, 11, 26, 10, 0)},
		{ms(2025, 11, 26, 9, 0), ms(2025, 11, 26, 9, 30), ms(2025, 11, 25, 15, 59)},
		{ms(2025, 11, 26, 16, 0), ms(2025, 11, 28, 9, 30), ms(2025, 11, 26, 15, 59)}, // Thanksgiving passed over
		{ms(2025, 11, 28, 14, 0), ms(2025, 12, 1, 9, 30), ms(2025, 11, 28, 12, 59)},  // the early close; the weekend
	} {
		a, err := g.after(c.t)
		b, ok := g.before(c.t)
		if err != nil || !ok || a != c.after || b != c.before {
			t.Errorf("at %s: after %s, before %s; want %s, %s", stamp(c.t, g.s), stamp(a, g.s), stamp(b, g.s),
				stamp(c.after, g.s), stamp(c.before, g.s))
		}
	}
	if l := g.last(ms(2025, 11, 28, 10, 0)); l != ms(2025, 11, 28, 12, 59) {
		t.Fatalf("the early day's last: %s", stamp(l, g.s))
	}
	// Without a session: every multiple of the step.
	n := grid{step: 60_000}
	if w := n.within(59_999, 180_000); !slices.Equal(w, []uint64{60_000, 120_000, 180_000}) {
		t.Fatalf("without a session: %v", w)
	}
	if a, _ := n.after(60_001); a != 120_000 {
		t.Fatalf("without a session, after: %d", a)
	}
}

// kline is the plugin's K-line of SPY at date: index the line's place; its close c; vw v (none when "").
func kline(date uint64, index int, c, v string) string {
	s := `{"close":` + c + `,"date":` + itoa(date) + `,"high":601,"index":` + strconv.Itoa(index) + `,"low":599,"n":7,"open":600,"volume":10`
	if v != "" {
		s += `,"vw":` + v
	}
	return s + "}"
}

// lines are the plugin's lines for the minutes of the grid in [from, to], but those skipped; a minute's close given
// by closes, else 600.5, its vw by vws, else 600.25.
type plugin struct {
	skip   map[uint64]bool
	closes map[uint64]string
	vws    map[uint64]string
}

func (p plugin) text(g grid, from, to uint64) string {
	var b strings.Builder
	i := 0
	for _, d := range g.within(from, to) {
		if p.skip[d] {
			continue
		}
		c, v := "600.5", "600.25"
		if s, ok := p.closes[d]; ok {
			c = s
		}
		if s, ok := p.vws[d]; ok {
			v = s
		}
		b.WriteString(kline(d, i, c, v) + "\n")
		i++
	}
	return b.String()
}

type rig struct {
	bus    *mem.Bus
	a      *acquirer
	g      grid
	dir    string
	group  int
	now    time.Time
	ts, ch *memStore
}

func newRig(t *testing.T) *rig {
	ctx := context.Background()
	bus := mem.New()
	adm, _ := bus.Admin(ctx)
	if _, err := raw.Provision(ctx, adm, 1); err != nil {
		t.Fatal(err)
	}
	d, err := loadDataset(writeCatalog(t, catalogText), "massive-spy-1m")
	if err != nil {
		t.Fatal(err)
	}
	d.Command = []string{os.Args[0], "-x"}
	d.Cleaning = []string{os.Args[0]}
	r := &rig{bus: bus, dir: t.TempDir(), g: grid{step: 60_000, s: d.Session}}
	r.ts = &memStore{bus: bus, nm: "timescaledb-writer", series: map[string][]clean.Message{}}
	r.ch = &memStore{bus: bus, nm: "clickhouse-writer", series: map[string][]clean.Message{}}
	r.a = &acquirer{bus: bus, dataset: d, publisher: "acquisition", now: func() time.Time { return r.now },
		out: &bytes.Buffer{}, stderr: &bytes.Buffer{}, source: r.ts, stores: []store{r.ts, r.ch}}
	return r
}

// memStore is a store on the in-memory bus: its corrected raw each minute's latest version as the record holds it
// (changed by alter, when set); its clean series of a topic, the topic's K-lines as it stood, written at each call.
type memStore struct {
	bus    *mem.Bus
	nm     string
	fail   bool // its clean series' write fails
	alter  func([]raw.KLine) []raw.KLine
	series map[string][]clean.Message // the clean series, by topic
	calls  []string                   // the topics written from, in order
}

func (s *memStore) name() string { return s.nm }

func (s *memStore) corrected(ctx context.Context, key string) ([]raw.KLine, error) {
	rec, err := readRecord(ctx, s.bus, s.nm)
	if err != nil {
		return nil, err
	}
	r := rec[key]
	var out []raw.KLine
	for _, d := range r.dates() {
		k, err := raw.DecodeKLine([]byte(r.latestOf(d).text))
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	if s.alter != nil {
		out = s.alter(out)
	}
	return out, nil
}

func (s *memStore) clean(ctx context.Context, topic string) error {
	if s.fail {
		return fmt.Errorf("%s -clean %s: exit status 1", s.nm, topic)
	}
	adm, _ := s.bus.Admin(ctx)
	spans, err := adm.Spans(ctx, topic)
	if err != nil {
		return err
	}
	end := spans[0].End
	var got []clean.Message
	if end > 0 {
		src, _ := s.bus.Source(ctx, stream.SubscribeOptions{Topics: []string{topic}})
		defer src.Close(ctx)
		for int64(len(got)) < end {
			recs, err := src.Fetch(ctx)
			if err != nil {
				return err
			}
			for _, rec := range recs {
				if rec.Offset < end {
					m, err := clean.FromRecord(rec)
					if err != nil {
						return err
					}
					got = append(got, m)
				}
			}
		}
	}
	s.series[topic] = got
	s.calls = append(s.calls, topic)
	return nil
}

// args gives the plugin's arguments of the last run that ran it.
func (r *rig) args(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r.dir, "args"))
	if err != nil {
		t.Fatalf("the plugin not run: %v", err)
	}
	return strings.Fields(string(b))
}

// out gives acquisition's report of the last run.
func (r *rig) out() string { return r.a.out.(*bytes.Buffer).String() }

// published is what a run published on each topic.
type published struct {
	firsts      []raw.Message
	corrections []raw.Correction
}

// acquire runs acquisition at now with the plugin printing text, and gives what the run published.
func (r *rig) acquire(t *testing.T, now time.Time, text string, exit string) (published, error) {
	t.Helper()
	r.now = now
	r.a.out.(*bytes.Buffer).Reset()
	os.Remove(filepath.Join(r.dir, "args"))
	p := filepath.Join(r.dir, "lines")
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ACQ_PLUGIN_LINES", p)
	t.Setenv("ACQ_PLUGIN_ARGS", filepath.Join(r.dir, "args"))
	t.Setenv("ACQ_PLUGIN_EXIT", exit)
	return r.watch(t, func() error { return r.a.run(context.Background()) })
}

// watch runs f, and gives what was published on each topic while it ran.
func (r *rig) watch(t *testing.T, f func() error) (published, error) {
	t.Helper()
	ctx := context.Background()
	adm, _ := r.bus.Admin(ctx)
	before := map[string]map[int32]stream.Span{}
	for _, topic := range raw.Topics {
		before[topic], _ = adm.Spans(ctx, topic)
	}
	err := f()
	var out published
	for _, topic := range raw.Topics {
		after, _ := adm.Spans(ctx, topic)
		want := 0
		for p, s := range after {
			want += int(s.End - before[topic][p].End)
		}
		r.group++
		src, _ := r.bus.Source(ctx, stream.SubscribeOptions{Group: "g" + strconv.Itoa(r.group), Topics: []string{topic}})
		for n := 0; n < want; {
			recs, _ := src.Fetch(ctx)
			for _, rec := range recs {
				if rec.Offset < before[topic][rec.Partition].End {
					continue
				}
				n++
				if topic == raw.Topic {
					m, ferr := raw.FromRecord(rec)
					if ferr != nil {
						t.Fatal(ferr)
					}
					out.firsts = append(out.firsts, m)
					continue
				}
				c, ferr := raw.CorrectionFromRecord(rec)
				if ferr != nil {
					t.Fatal(ferr)
				}
				out.corrections = append(out.corrections, c)
			}
		}
	}
	return out, err
}

// put publishes values on market.raw as they are, a record each, its key, partition and timestamp the value's.
func (r *rig) put(t *testing.T, values ...string) {
	t.Helper()
	ctx := context.Background()
	sink, _ := r.bus.Sink(ctx)
	for _, v := range values {
		m, err := raw.Decode([]byte(v))
		if err != nil {
			t.Fatal(err)
		}
		if err := sink.Publish(ctx, stream.Record{Topic: raw.Topic, Key: []byte(m.Key()),
			PartitionKey: []byte(m.Instrument), Value: []byte(v), Time: time.UnixMilli(int64(m.KLine.Base().Date))}); err != nil {
			t.Fatal(err)
		}
	}
}

// value is a value of market.raw of version 0.2.0 or 0.1.0, SPY's K-line text.
func value(version, kline string) string {
	return `{"instrument":"SPY","kline":` + kline + `,"origin":"live","publisher":"acquisition","schema":"market.raw","source":"massive","timeframe":"M1","version":"` + version + `"}`
}

// The first run of a series with none: every minute of the grid from the history's first, a minute the plugin gives
// no line for the zero K-line; the plugin asked for the session's spans, to the last minute delay old.
func TestFirstRun(t *testing.T) {
	r := newRig(t)
	now := at(2025, 11, 26, 12, 0) // Wednesday noon: the last minute 20 h old, Tuesday's 15:59
	from, to := ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59)
	hole := ms(2025, 11, 24, 9, 31)
	got, err := r.acquire(t, now, plugin{skip: map[uint64]bool{hole: true}}.text(r.g, from, to), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.firsts) != 780 || len(got.corrections) != 0 {
		t.Fatalf("published %d first versions, %d corrections", len(got.firsts), len(got.corrections))
	}
	for i, m := range got.firsts {
		k := m.KLine.Base()
		if k.Index != uint64(i) || k.Date != r.g.within(from, to)[i] || m.Read != uint64(now.UnixMilli()) ||
			m.Origin != raw.Live || m.Publisher != "acquisition" || m.Key() != "massive:SPY:M1" {
			t.Fatalf("message %d: %+v", i, m)
		}
	}
	// The minute without a bar: the zero K-line, at its place.
	if z := got.firsts[1].KLine.Text(); z != `{"close":0,"date":`+itoa(hole)+`,"high":0,"index":1,"low":0,"open":0,"volume":0}` {
		t.Fatalf("the minute without a bar: %s", z)
	}
	if k := got.firsts[2].KLine.Text(); k != kline(ms(2025, 11, 24, 9, 32), 2, "600.5", "600.25") {
		t.Fatalf("the minute after it: %s", k)
	}
	// The history (3 days back, Sunday) begins at Monday's open; to Tuesday's 15:59, each day [open, close − 1 ms]
	// cut to the span.
	a := r.args(t)
	want := []string{"-x", "--backfill", "SPY:M1", itoa(from), itoa(ms(2025, 11, 24, 16, 0) - 1), itoa(ms(2025, 11, 25, 9, 30)), itoa(to)}
	if !slices.Equal(a, want) {
		t.Fatalf("the plugin's arguments %q, want %q", a, want)
	}
	if !strings.Contains(r.out(), "massive:SPY:M1: the record: none on market.raw; asked 2025-11-24 09:30 EST to 2025-11-25 15:59 EST, "+
		"780 minutes of the grid in 2 spans; 779 lines; published 780 first versions on market.raw (index 0 to 779; 1 zero K-lines), "+
		"0 corrections on market.corrections; 0 unchanged") {
		t.Fatalf("report %q", r.out())
	}
	// At once again: the span read again, nothing published, every minute the same.
	again, err := r.acquire(t, now, plugin{skip: map[uint64]bool{hole: true}}.text(r.g, from, to), "")
	if err != nil || len(again.firsts)+len(again.corrections) != 0 {
		t.Fatalf("again: %+v, %v", again, err)
	}
	if !strings.Contains(r.out(), "780 minutes on market.raw, index 0 to 779, 2025-11-24 09:30 EST to 2025-11-25 15:59 EST, "+
		"0 copies (0 of another value); 0 corrections") || !strings.Contains(r.out(), "published 0 first versions") ||
		!strings.Contains(r.out(), "; 780 unchanged") {
		t.Fatalf("again: report %q", r.out())
	}
}

// A minute without a bar published as the zero K-line, then a correction to a bar, and back; a bar revised; each
// correction naming the fields that differ from the minute's latest version.
func TestCorrections(t *testing.T) {
	r := newRig(t)
	from, to := ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59)
	hole, revised := ms(2025, 11, 24, 9, 31), ms(2025, 11, 25, 10, 0)
	if _, err := r.acquire(t, at(2025, 11, 26, 12, 0), plugin{skip: map[uint64]bool{hole: true}}.text(r.g, from, to), ""); err != nil {
		t.Fatal(err)
	}
	// Thanksgiving noon: Wednesday read too; the minute without a bar has one now, and a bar's vw is revised.
	now := at(2025, 11, 27, 12, 0)
	to = ms(2025, 11, 26, 15, 59)
	got, err := r.acquire(t, now, plugin{vws: map[uint64]string{revised: "600.3"}}.text(r.g, from, to), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.firsts) != 390 || got.firsts[0].KLine.Base().Index != 780 || got.firsts[389].KLine.Base().Index != 1169 {
		t.Fatalf("Wednesday: %d first versions", len(got.firsts))
	}
	if len(got.corrections) != 2 {
		t.Fatalf("corrections %d, want 2", len(got.corrections))
	}
	c := got.corrections[0]
	if c.KLine.Text() != kline(hole, 1, "600.5", "600.25") || c.Read != uint64(now.UnixMilli()) ||
		!slices.Equal(c.Corrected, []string{"close", "high", "low", "n", "open", "volume", "vw"}) {
		t.Fatalf("the minute's bar: %s %v", c.KLine.Text(), c.Corrected)
	}
	c = got.corrections[1]
	if c.KLine.Text() != kline(revised, 390+30, "600.5", "600.3") || !slices.Equal(c.Corrected, []string{"vw"}) {
		t.Fatalf("the bar revised: %s %v", c.KLine.Text(), c.Corrected)
	}
	if !strings.Contains(r.out(), "published 390 first versions on market.raw (index 780 to 1169; 0 zero K-lines), "+
		"2 corrections on market.corrections; 778 unchanged") {
		t.Fatalf("report %q", r.out())
	}
	// Again: the bar gone once more (back to the zero K-line), against its latest version; the revised bar as it was
	// revised, against its correction: unchanged.
	later := at(2025, 11, 27, 13, 0)
	got, err = r.acquire(t, later, plugin{skip: map[uint64]bool{hole: true}, vws: map[uint64]string{revised: "600.3"}}.text(r.g, from, to), "")
	if err != nil || len(got.firsts) != 0 || len(got.corrections) != 1 {
		t.Fatalf("back: %+v, %v", got, err)
	}
	c = got.corrections[0]
	if c.KLine.Text() != `{"close":0,"date":`+itoa(hole)+`,"high":0,"index":1,"low":0,"open":0,"volume":0}` ||
		c.Read != uint64(later.UnixMilli()) || !slices.Equal(c.Corrected, []string{"close", "high", "low", "n", "open", "volume", "vw"}) {
		t.Fatalf("back to the zero K-line: %s %v", c.KLine.Text(), c.Corrected)
	}
	if !strings.Contains(r.out(), "1170 minutes on market.raw, index 0 to 1169") ||
		!strings.Contains(r.out(), "; 2 corrections on market.corrections, 0 copies") ||
		!strings.Contains(r.out(), "1 corrections on market.corrections; 1169 unchanged") {
		t.Fatalf("back: report %q", r.out())
	}
	// And again: nothing.
	got, err = r.acquire(t, later, plugin{skip: map[uint64]bool{hole: true}, vws: map[uint64]string{revised: "600.3"}}.text(r.g, from, to), "")
	if err != nil || len(got.firsts)+len(got.corrections) != 0 {
		t.Fatalf("and again: %+v, %v", got, err)
	}
}

// Values of 0.2.0 and 0.1.0 on market.raw are read as first versions, with no read; two copies of a value are one,
// the first taken; the series continues after them at its place in the grid.
func TestVersionsBefore(t *testing.T) {
	r := newRig(t)
	monday := r.g.within(ms(2025, 11, 24, 9, 30), ms(2025, 11, 24, 15, 59))
	var vs []string
	for i, d := range monday {
		v := value("0.2.0", kline(d, i, "600.5", "600.25"))
		if i < 100 {
			v = value("0.1.0", kline(d, i, "600.5", "600.25"))
		}
		vs = append(vs, v)
	}
	r.put(t, vs...)
	r.put(t, vs...) // every value twice
	// One copy of another value: dropped, the first taken; counted.
	r.put(t, value("0.2.0", kline(monday[5], 5, "601", "600.25")))
	from, to := monday[0], ms(2025, 11, 25, 15, 59)
	got, err := r.acquire(t, at(2025, 11, 26, 12, 0), plugin{}.text(r.g, from, to), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.corrections) != 0 || len(got.firsts) != 390 || got.firsts[0].KLine.Base().Index != 390 ||
		got.firsts[0].KLine.Base().Date != ms(2025, 11, 25, 9, 30) {
		t.Fatalf("published %d first versions, %d corrections", len(got.firsts), len(got.corrections))
	}
	if !strings.Contains(r.out(), "390 minutes on market.raw, index 0 to 389, 2025-11-24 09:30 EST to 2025-11-24 15:59 EST, "+
		"391 copies (1 of another value)") || !strings.Contains(r.out(), "; 390 unchanged") {
		t.Fatalf("report %q", r.out())
	}
}

// The span: from reread before now, or the minute after the series' last when that is earlier; to the last minute
// delay old.
func TestSpan(t *testing.T) {
	r := newRig(t)
	from, to := ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59)
	if _, err := r.acquire(t, at(2025, 11, 26, 12, 0), plugin{}.text(r.g, from, to), ""); err != nil {
		t.Fatal(err)
	}
	// Before 12:00 the next day, Wednesday's 15:59 not 20 h old: to 15:58.
	if _, err := r.acquire(t, at(2025, 11, 27, 11, 59), plugin{}.text(r.g, from, ms(2025, 11, 26, 15, 58)), ""); err != nil {
		t.Fatal(err)
	}
	if a := r.args(t); a[3] != itoa(from) || a[len(a)-1] != itoa(ms(2025, 11, 26, 15, 58)) {
		t.Fatalf("to 15:58: %q", a)
	}
	// Seven weeks later: reread 30 days back would leave out Wednesday's 15:59 and Friday, so from the minute after
	// the series' last, 15:59, to the last minute delay old.
	now := at(2026, 1, 15, 12, 0)
	got, err := r.acquire(t, now, plugin{}.text(r.g, ms(2025, 11, 26, 15, 59), ms(2026, 1, 14, 15, 59)), "")
	if err != nil {
		t.Fatal(err)
	}
	if a := r.args(t); a[3] != itoa(ms(2025, 11, 26, 15, 59)) || a[len(a)-1] != itoa(ms(2026, 1, 14, 15, 59)) {
		t.Fatalf("seven weeks later: %q…%q", a[:5], a[len(a)-2:])
	}
	if got.firsts[0].KLine.Base().Index != 1169 || got.firsts[0].KLine.Base().Date != ms(2025, 11, 26, 15, 59) {
		t.Fatalf("seven weeks later: the first %+v", got.firsts[0].KLine.Base())
	}
	// The next day: 30 days back, the series' first minute long before.
	if _, err := r.acquire(t, at(2026, 1, 16, 12, 0), plugin{}.text(r.g, ms(2025, 12, 17, 12, 0), ms(2026, 1, 15, 15, 59)), ""); err != nil {
		t.Fatal(err)
	}
	if a := r.args(t); a[3] != itoa(ms(2025, 12, 17, 12, 0)) || a[len(a)-1] != itoa(ms(2026, 1, 15, 15, 59)) {
		t.Fatalf("30 days back: %q…%q", a[:5], a[len(a)-2:])
	}
}

// A record not the grid's, a line not the plugin's, or the plugin's fault stops the run, nothing of the series
// published.
func TestRefused(t *testing.T) {
	from, to := ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59)
	now := at(2025, 11, 26, 12, 0)
	monday := testGrid(t).within(from, ms(2025, 11, 24, 15, 59))
	for name, c := range map[string]struct {
		record []string
		err    string
	}{
		"a minute missing": {[]string{value("0.2.0", kline(monday[0], 0, "600", "")), value("0.2.0", kline(monday[2], 2, "600", ""))},
			"not there, though bars after it are"},
		"an index not its place": {[]string{value("0.2.0", kline(monday[0], 0, "600", "")), value("0.2.0", kline(monday[1], 2, "600", ""))},
			"has index 2, its place in the grid 1"},
		"a minute not of the grid": {[]string{value("0.2.0", kline(monday[0]-60_000, 0, "600", "")), value("0.2.0", kline(monday[0], 1, "600", ""))},
			"not of the grid"},
	} {
		r := newRig(t)
		r.put(t, c.record...)
		got, err := r.acquire(t, now, plugin{}.text(r.g, from, to), "")
		if err == nil || !strings.Contains(err.Error(), c.err) || len(got.firsts)+len(got.corrections) != 0 {
			t.Errorf("%s: %v, %d published", name, err, len(got.firsts)+len(got.corrections))
		}
	}
	text := plugin{}.text(testGrid(t), from, to)
	ls := strings.SplitAfter(text, "\n")
	for name, text := range map[string]string{
		"not ascending":     ls[1] + ls[0],
		"outside the spans": kline(from-60_000, 0, "600", "") + "\n",
		"not of the grid":   kline(from+30_000, 0, "600", "") + "\n",
		"an own field text": strings.Replace(ls[0], `"n":7`, `"n":"7"`, 1),
		"asOf":              strings.Replace(ls[0], `{`, `{"asOf":1764167400000,`, 1),
		"not canonical":     strings.Replace(ls[0], `"close":`, `"close": `, 1),
		"not a K-line":      "hello\n",
		"a broken K-line":   strings.Replace(ls[0], `"low":599`, `"low":600.6`, 1),
		"a duplicate date":  ls[0] + ls[0],
	} {
		r := newRig(t)
		if got, err := r.acquire(t, now, text, ""); err == nil || len(got.firsts)+len(got.corrections) != 0 {
			t.Errorf("%s: %v, %d published", name, err, len(got.firsts)+len(got.corrections))
		}
	}
	r := newRig(t)
	if got, err := r.acquire(t, now, text, "1"); err == nil || !strings.Contains(err.Error(), "exit status 1; nothing published") ||
		len(got.firsts) != 0 {
		t.Fatalf("the plugin's exit 1: %v, %d published", err, len(got.firsts))
	}
	// The next run asks again, and publishes the span whole.
	if got, err := r.acquire(t, now, text, ""); err != nil || len(got.firsts) != 780 {
		t.Fatalf("after the plugin's fault: %d, %v", len(got.firsts), err)
	}
}

// Following: a run at the start, then one when the next session's last minute becomes delay old, the clock read
// again each minute at most.
func TestFollow(t *testing.T) {
	r := newRig(t)
	r.now = at(2025, 11, 26, 12, 0)
	t.Setenv("ACQ_PLUGIN_ARGS", filepath.Join(r.dir, "args"))
	p := filepath.Join(r.dir, "lines")
	t.Setenv("ACQ_PLUGIN_LINES", p)
	t.Setenv("ACQ_PLUGIN_FILTER", "1")
	os.WriteFile(p, []byte(plugin{}.text(r.g, ms(2025, 11, 24, 9, 30), ms(2025, 11, 26, 15, 59))), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waits, longest := 0, time.Duration(0)
	t.Setenv("ACQ_PLUGIN_EXIT", "")
	r.a.sleep = func(ctx context.Context, d time.Duration) bool {
		waits++
		longest = max(longest, d)
		r.now = r.now.Add(d)
		// the second run's plugin fails, the third's does not
		if !r.now.Before(at(2025, 11, 27, 12, 0)) && r.now.Before(at(2025, 11, 28, 0, 0)) {
			os.Setenv("ACQ_PLUGIN_EXIT", "1")
		} else {
			os.Setenv("ACQ_PLUGIN_EXIT", "")
		}
		// at the third run's wait, after Saturday 09:00: stopped
		if r.now.After(at(2025, 11, 29, 9, 0)) {
			cancel()
			return false
		}
		return true
	}
	var err error
	got, _ := r.watch(t, func() error { err = r.a.follow(ctx); return nil })
	if err != context.Canceled {
		t.Fatalf("follow: %v", err)
	}
	// Run 1: Monday and Tuesday; run 2 at Thursday 12:00 (Wednesday's 15:59 20 h old), its plugin failing: nothing
	// published, the run said failed, and the next at its time; run 3 at Saturday 09:00 (Friday's 12:59 20 h old):
	// Wednesday, which run 2 left out, and Friday, its plugin giving no line of Friday: its 210 minutes the zero K-line.
	if len(got.firsts) != 780+390+210 || got.firsts[len(got.firsts)-1].KLine.Base().Date != ms(2025, 11, 28, 12, 59) ||
		got.firsts[1170].KLine.Base().Volume.Sign() != 0 {
		t.Fatalf("published %d", len(got.firsts))
	}
	out := r.out()
	for _, s := range []string{"the next run at 2025-11-27 12:00 EST, when the last bar of the next session is 20h old",
		"acquisition: the run failed: massive:SPY:M1: plugin", "exit status 1; nothing published",
		"the next run at 2025-11-29 09:00 EST", "the next run at 2025-12-02 12:00 EST"} {
		if !strings.Contains(out, s) {
			t.Fatalf("%q not in %q", s, out)
		}
	}
	if longest > time.Minute {
		t.Fatalf("a wait of %s", longest)
	}
}

// The record: a correction's copy, by its key, date and read, is dropped, whichever correction of its minute is the
// latest; the latest the one of the greatest read.
func TestRecordCopies(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	monday := r.g.within(ms(2025, 11, 24, 9, 30), ms(2025, 11, 24, 9, 31))
	r.put(t, value("0.2.0", kline(monday[0], 0, "600.5", "600.25")), value("0.2.0", kline(monday[1], 1, "600.5", "600.25")))
	sink, _ := r.bus.Sink(ctx)
	correct := func(read uint64, vw string) {
		k, err := raw.DecodeKLine([]byte(kline(monday[0], 0, "600.5", vw)))
		if err != nil {
			t.Fatal(err)
		}
		rec, err := raw.Correction{Message: raw.Message{Origin: raw.Live, Source: "massive", Instrument: "SPY",
			Timeframe: "M1", Publisher: "acquisition", KLine: k, Read: read}, Corrected: []string{"vw"}}.Record()
		if err != nil {
			t.Fatal(err)
		}
		if err := sink.Publish(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	correct(100, "600.3")
	correct(200, "600.35")
	correct(100, "600.3") // a copy of the earlier
	correct(200, "600.4") // a copy of another value
	rec, err := readRecord(ctx, r.bus, "acquisition")
	if err != nil {
		t.Fatal(err)
	}
	s := rec["massive:SPY:M1"]
	if s.corrections != 2 || s.ccopies != 2 || s.cdiffering != 1 || s.latest[monday[0]].read != 200 ||
		!strings.Contains(s.latest[monday[0]].text, `"vw":600.35`) {
		t.Fatalf("record %+v", s)
	}
}
