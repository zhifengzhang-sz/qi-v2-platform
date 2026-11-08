package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// batch is the number of messages published at once.
const batch = 1000

// acquirer runs over a dataset.
type acquirer struct {
	bus       stream.Bus
	dataset   dataset
	publisher string // its name: the messages' publisher, and the group it reads the record in
	now       func() time.Time
	// sleep waits d, or until ctx ends, which it reports false; nil: the clock's
	sleep  func(ctx context.Context, d time.Duration) bool
	out    io.Writer
	stderr io.Writer // the plugins'; os.Stderr when nil
	lines  io.Writer // the plugin's lines as read, when not nil
	// source is the store whose corrected raw a reset reads, the TimescaleDB store; stores, each store whose clean
	// series a reset writes
	source store
	stores []store
}

// tally is a series' run.
type tally struct {
	from, to                     uint64 // the span asked, when minutes > 0
	minutes, spans, lines        int    // the span's minutes of the grid, the session spans asked, the plugin's lines
	firsts, zeros, corrected     int    // published: first versions, of them zero K-lines; corrections
	unchanged                    int
	firstIndex, lastIndex        uint64 // the first versions' index, when firsts > 0
	recorded                     int    // the series' minutes on market.raw before the run
	recFirst, recLast            uint64 // their first and last, when recorded > 0
	copies, differing            int    // market.raw's copies dropped; of them, of another K-line or read
	corrections, ccopies, cdiffs int    // market.corrections': its corrections of the series, copies, differing copies
}

// run is one run over the dataset (plugins.md, 3): the record read from the core's two topics, then each series asked
// for its span and its minutes published, a line a series on out.
func (a *acquirer) run(ctx context.Context) error {
	adm, err := a.bus.Admin(ctx)
	if err != nil {
		return err
	}
	n, err := adm.Partitions(ctx, raw.Topics...)
	adm.Close(ctx)
	if err != nil {
		return err
	}
	for _, t := range raw.Topics {
		if n[t] != raw.Partitions {
			return fmt.Errorf("%s: %d partitions, the schema's %d: provision it (core/cmd/provision)", t, n[t], raw.Partitions)
		}
	}
	rec, err := readRecord(ctx, a.bus, a.publisher)
	if err != nil {
		return fmt.Errorf("the record: %w", err)
	}
	sink, err := a.bus.Sink(ctx)
	if err != nil {
		return err
	}
	defer sink.Close(context.Background())
	now := a.now()
	history, _ := a.dataset.from(now)
	for _, instrument := range a.dataset.Instruments {
		for _, timeframe := range a.dataset.Timeframes {
			m := raw.Message{Origin: raw.Live, Source: a.dataset.Source, Instrument: instrument, Timeframe: timeframe,
				Publisher: a.publisher}
			r := rec[m.Key()]
			if r == nil {
				r = newRecord()
				rec[m.Key()] = r
			}
			t, err := a.series(ctx, sink, m, r, history, now)
			if err != nil {
				return fmt.Errorf("%s: %w", m.Key(), err)
			}
			fmt.Fprintf(a.out, "acquisition: %s: %s\n", m.Key(), t.report(a.dataset.Session))
			ct, err := a.cleanSeriesOf(ctx, sink, m, r)
			if err != nil {
				return fmt.Errorf("%s: the clean: %w", m.Key(), err)
			}
			fmt.Fprintf(a.out, "acquisition: %s: %s\n", m.Key(), ct.report())
		}
	}
	return nil
}

func (t tally) report(s *session) string {
	rec := "none on market.raw"
	if t.recorded > 0 {
		rec = fmt.Sprintf("%d minutes on market.raw, index 0 to %d, %s to %s, %d copies (%d of another value); %d "+
			"corrections on market.corrections, %d copies (%d of another value)", t.recorded, t.recorded-1,
			stamp(t.recFirst, s), stamp(t.recLast, s), t.copies, t.differing, t.corrections, t.ccopies, t.cdiffs)
	}
	if t.minutes == 0 {
		return "the record: " + rec + "; nothing asked, nothing published"
	}
	idx := "none"
	if t.firsts > 0 {
		idx = fmt.Sprintf("%d to %d", t.firstIndex, t.lastIndex)
	}
	return fmt.Sprintf("the record: %s; asked %s to %s, %d minutes of the grid in %d spans; %d lines; published %d "+
		"first versions on market.raw (index %s; %d zero K-lines), %d corrections on market.corrections; %d unchanged",
		rec, stamp(t.from, s), stamp(t.to, s), t.minutes, t.spans, t.lines, t.firsts, idx, t.zeros, t.corrected,
		t.unchanged)
}

// series is one series' run, its record r taking the versions it publishes. Its record is checked against its grid: every bar of the grid from the series' first to
// its last on market.raw, each at its place in the grid, and each correction of a bar there at its index. The span
// asked is from the later of the series' first bar and reread before now, or the bar after its last on market.raw
// when that is earlier, so that no bar of the grid is left out (a series with none: from the history's first bar),
// to the last bar of the grid at least delay old, its end a delay before now; the plugin is run over the span's
// session spans, and its lines, once it has read the whole of them (exit 0), are the span's bars: a bar of the grid
// with no line is the zero K-line. Each bar after the series' last on market.raw is published there, its first
// version; each bar already there whose K-line differs from its latest version is published on market.corrections,
// naming the fields that differ; any other publishes nothing.
func (a *acquirer) series(ctx context.Context, sink stream.Sink, m raw.Message, r *record, history, now time.Time) (tally, error) {
	var t tally
	g := grid{step: uint64(steps[m.Timeframe].Milliseconds()), s: a.dataset.Session}
	dates := make([]uint64, 0, len(r.first))
	for d := range r.first {
		dates = append(dates, d)
	}
	slices.Sort(dates)
	n := len(dates)
	t.recorded, t.copies, t.differing = n, r.copies, r.differing
	t.corrections, t.ccopies, t.cdiffs = r.corrections, r.ccopies, r.cdiffering
	if n > 0 {
		t.recFirst, t.recLast = dates[0], dates[n-1]
		if err := r.check(g, dates); err != nil {
			return t, err
		}
	}
	// the span
	nowMS := uint64(now.UnixMilli())
	var first uint64
	if n > 0 {
		first = dates[0]
	} else {
		f, err := g.after(uint64(max(history.UnixMilli(), 0)))
		if err != nil {
			return t, err
		}
		first = f
	}
	back, err := g.after(nowMS - min(nowMS, a.dataset.reread))
	if err != nil {
		return t, err
	}
	from, next := max(first, back), first
	if n > 0 {
		if next, err = g.after(dates[n-1] + 1); err != nil {
			return t, err
		}
	}
	from = min(from, next)
	to, ok := g.before(nowMS - min(nowMS, g.step+a.dataset.delay))
	if !ok || from > to {
		return t, nil
	}
	minutes := g.within(from, to)
	if len(minutes) == 0 {
		return t, nil
	}
	t.from, t.to, t.minutes = from, to, len(minutes)
	spans := [][2]uint64{{from, to}}
	if s := a.dataset.Session; s != nil {
		spans = s.spans(from, to)
	}
	t.spans = len(spans)
	got, err := a.plugin(ctx, m, spans, minutes, &t)
	if err != nil {
		return t, err
	}
	read := uint64(a.now().UnixMilli()) // the span read whole
	// the versions
	var firsts []stream.Record
	var corrections []stream.Record
	var published []dated // the versions published, of the record after the run
	place := uint64(n)
	for _, d := range minutes {
		idx := place
		if n > 0 && d <= dates[n-1] {
			idx = r.first[d].index
		} else {
			place++
		}
		var k raw.KLine
		if line, ok := got[d]; ok {
			k, err = raw.WithIndex(line, idx)
		} else {
			k, err = raw.Zero(idx, d)
		}
		if err != nil {
			return t, fmt.Errorf("the bar at %d: %w", d, err)
		}
		v := m
		v.KLine, v.Read = k, read
		published = append(published, dated{date: d, version: version{index: idx, text: k.Text(), read: read,
			origin: m.Origin}})
		if n == 0 || d > dates[n-1] {
			rec, err := v.Record()
			if err != nil {
				return t, err
			}
			if t.firsts == 0 {
				t.firstIndex = idx
			}
			t.firsts++
			t.lastIndex = idx
			if _, ok := got[d]; !ok {
				t.zeros++
			}
			firsts = append(firsts, rec)
			continue
		}
		latest, ok := r.latest[d]
		if !ok {
			latest = r.first[d]
		}
		if latest.text == k.Text() {
			t.unchanged++
			published = published[:len(published)-1]
			continue
		}
		before, err := raw.DecodeKLine([]byte(latest.text))
		if err != nil {
			return t, fmt.Errorf("the bar at %d, its latest version: %w", d, err)
		}
		fields, err := raw.Differ(before, k)
		if err != nil {
			return t, fmt.Errorf("the bar at %d: %w", d, err)
		}
		rec, err := raw.Correction{Message: v, Corrected: fields}.Record()
		if err != nil {
			return t, fmt.Errorf("the bar at %d: %w", d, err)
		}
		corrections = append(corrections, rec)
		t.corrected++
	}
	for _, recs := range [][]stream.Record{firsts, corrections} {
		for i := 0; i < len(recs); i += batch {
			if err := sink.Publish(ctx, recs[i:min(i+batch, len(recs))]...); err != nil {
				return t, err
			}
		}
	}
	for _, v := range published {
		if _, ok := r.first[v.date]; !ok {
			r.first[v.date] = v.version
			continue
		}
		r.latest[v.date] = v.version
		r.seen[[2]uint64{v.date, v.read}] = v.version
		r.corrections++
	}
	return t, nil
}

// dated is a version and its minute's date.
type dated struct {
	date uint64
	version
}

// check is the series' record against its grid: its first versions' dates every bar of the grid from the first to
// the last, each at its place in it; each correction of a bar there, at its index.
func (r *record) check(g grid, dates []uint64) error {
	want := g.within(dates[0], dates[len(dates)-1])
	for i := range max(len(want), len(dates)) {
		switch {
		case i >= len(want) || (i < len(dates) && dates[i] < want[i]):
			return fmt.Errorf("market.raw: a bar at %s, not of the grid (the catalog's session)", stamp(dates[i], g.s))
		case i >= len(dates) || dates[i] > want[i]:
			return fmt.Errorf("market.raw: the grid's bar at %s not there, though bars after it are",
				stamp(want[i], g.s))
		}
		if idx := r.first[dates[i]].index; idx != uint64(i) {
			return fmt.Errorf("market.raw: the bar at %s has index %d, its place in the grid %d", stamp(dates[i], g.s),
				idx, i)
		}
	}
	for d, v := range r.latest {
		f, ok := r.first[d]
		if !ok {
			return fmt.Errorf("market.corrections: a correction of a bar at %s, not on market.raw", stamp(d, g.s))
		}
		if v.index != f.index {
			return fmt.Errorf("market.corrections: the bar at %s at index %d, on market.raw %d", stamp(d, g.s), v.index,
				f.index)
		}
	}
	return nil
}

// plugin runs the dataset's plugin over the spans and gives its bars by date, once it has read the whole of them
// (its exit 0): each line the source's K-line in its canonical text, ascending, in the spans, a bar of the grid.
func (a *acquirer) plugin(ctx context.Context, m raw.Message, spans [][2]uint64, minutes []uint64, t *tally) (map[uint64]raw.KLine, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	d := a.dataset.Command
	args := append(append([]string(nil), d[1:]...), "--backfill", m.Instrument+":"+m.Timeframe)
	for _, sp := range spans {
		args = append(args, strconv.FormatUint(sp[0], 10), strconv.FormatUint(sp[1], 10))
	}
	cmd := exec.CommandContext(ctx, d[0], args...)
	cmd.Stderr = a.stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("plugin %s: %w", d[0], err)
	}
	fault := func(err error) (map[uint64]raw.KLine, error) {
		cancel()
		_ = cmd.Wait()
		return nil, err
	}
	got := map[uint64]raw.KLine{}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	last := int64(-1)
	at := 0 // the span the lines have reached
	on := 0 // the grid's bar the lines have reached
	for sc.Scan() {
		t.lines++
		if a.lines != nil {
			if _, err := fmt.Fprintf(a.lines, "%s\n", sc.Bytes()); err != nil {
				return fault(err)
			}
		}
		k, err := raw.DecodeKLine(sc.Bytes())
		if err != nil {
			return fault(fmt.Errorf("the plugin's line %d: %w", t.lines, err))
		}
		date := k.Base().Date
		if int64(date) <= last {
			return fault(fmt.Errorf("the plugin's line %d: a bar at %d after one at %d", t.lines, date, last))
		}
		last = int64(date)
		for at < len(spans) && date > spans[at][1] {
			at++
		}
		if at == len(spans) || date < spans[at][0] {
			return fault(fmt.Errorf("the plugin's line %d: a bar at %d outside the spans asked", t.lines, date))
		}
		for on < len(minutes) && minutes[on] < date {
			on++
		}
		if on == len(minutes) || minutes[on] != date {
			return fault(fmt.Errorf("the plugin's line %d: a bar at %d, not of the grid", t.lines, date))
		}
		got[date] = k
	}
	if err := sc.Err(); err != nil {
		return fault(fmt.Errorf("the plugin's output: %w", err))
	}
	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("plugin %s: %w; nothing published", d[0], err)
		}
		return nil, fmt.Errorf("plugin %s: %w", d[0], err)
	}
	return got, nil
}

// follow runs the dataset at its start, then whenever the last bar of the next session of the grid becomes delay old
// (for a dataset without a session, its next bar), each run as run is: a line on out for the next run's time. A run
// that fails is a line on out, its fault; the next run comes at its time all the same, and asks from the bar after the
// series' last on market.raw, so that nothing a failed run left out is lost. It ends when ctx does.
func (a *acquirer) follow(ctx context.Context) error {
	sleep := a.sleep
	if sleep == nil {
		sleep = func(ctx context.Context, d time.Duration) bool {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(d):
				return true
			}
		}
	}
	for {
		if err := a.run(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(a.out, "acquisition: the run failed: %v\n", err)
		}
		due, err := a.due(a.now())
		if err != nil {
			return err
		}
		fmt.Fprintf(a.out, "acquisition: the next run at %s, when the last bar of the next session is %s old\n",
			stamp(uint64(due.UnixMilli()), a.dataset.Session), a.dataset.Delay)
		// the wall clock, read again each minute at most, so that a machine asleep in between wakes to the time
		for {
			left := due.Sub(a.now())
			if left <= 0 {
				break
			}
			if !sleep(ctx, min(left, time.Minute)) {
				return ctx.Err()
			}
		}
	}
}

// due is the time of the next run after now: the earliest, of the dataset's timeframes, at which the last bar of the
// session after the last bar delay old becomes delay old, its end a delay before then.
func (a *acquirer) due(now time.Time) (time.Time, error) {
	var out time.Time
	nowMS := uint64(now.UnixMilli())
	for _, tf := range a.dataset.Timeframes {
		g := grid{step: uint64(steps[tf].Milliseconds()), s: a.dataset.Session}
		edge := nowMS - min(nowMS, g.step+a.dataset.delay)
		next := edge + 1
		if l, ok := g.before(edge); ok {
			next = l + 1
		}
		f, err := g.after(next)
		if err != nil {
			return out, err
		}
		at := time.UnixMilli(int64(g.last(f) + g.step + a.dataset.delay))
		if out.IsZero() || at.Before(out) {
			out = at
		}
	}
	return out, nil
}
