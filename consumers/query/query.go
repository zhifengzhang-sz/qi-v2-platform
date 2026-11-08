package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// baseFields are the base K-line's, market 1.1.0's, as a row shows them (date as its minute); every other field of a
// K-line is its source's own (core/README.md).
var baseFields = []string{"index", "open", "high", "low", "close", "volume"}

// version is one version of a minute as the record holds it: on market.raw, its first version; on
// market.corrections, a correction, with the fields it changed.
type version struct {
	kline     raw.KLine
	read      uint64   // 0: none, a first version of 0.1.0 or 0.2.0
	corrected []string // a correction's; nil for a first version
}

func (v version) date() uint64 { return v.kline.Base().Date }

// fields is the K-line's fields, each its text as its canonical text writes it, by name.
func (v version) fields() (map[string]string, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(v.kline.Text()), &m); err != nil {
		return nil, fmt.Errorf("query: the K-line %s: %w", v.kline.Text(), err)
	}
	out := make(map[string]string, len(m))
	for n, t := range m {
		out[n] = string(t)
	}
	return out, nil
}

// own is the source's own fields' names across the versions, in the order of their names (ASCII).
func own(vs []version) ([]string, error) {
	set := map[string]bool{}
	for _, v := range vs {
		f, err := v.fields()
		if err != nil {
			return nil, err
		}
		for n := range f {
			set[n] = true
		}
	}
	delete(set, "date")
	for _, n := range baseFields {
		delete(set, n)
	}
	return slices.Sorted(maps.Keys(set)), nil
}

// table is a table written aligned, its cells parted by two spaces at least.
type table struct{ w *tabwriter.Writer }

func newTable(out io.Writer, header ...string) *table {
	t := &table{w: tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)}
	t.row(header...)
	return t
}

func (t *table) row(cells ...string) { fmt.Fprintln(t.w, strings.Join(cells, "\t")) }

func (t *table) flush() error { return t.w.Flush() }

func itoa[T int | int64 | uint64](n T) string { return strconv.FormatInt(int64(n), 10) }

// counted is n and its noun, one or many.
func counted(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return itoa(n) + " " + many
}

// isClean: whether a topic is a clean topic, by its name.
func isClean(topic string) bool { return strings.HasPrefix(topic, clean.Kind+".") }

// cleanSeries is a clean topic's series, its key, from its name's last three names (core/README.md, The topics' names).
func cleanSeries(topic string) (string, error) {
	n := strings.Split(topic, ".")
	if !isClean(topic) || len(n) != 7 {
		return "", fmt.Errorf("not market.clean.<cleaning>.<version>.<source>.<instrument>.<timeframe>")
	}
	return n[4] + ":" + n[5] + ":" + n[6], nil
}

// partitions is a topic's partitions by its schema: a clean topic's one, a core topic's the core's.
func partitions(topic string) int32 {
	if isClean(topic) {
		return clean.Partitions
	}
	return raw.Partitions
}

// spans is a topic's spans, its partitions the schema's.
func spans(ctx context.Context, rec record, topic string) (map[int32]stream.Span, error) {
	s, err := rec.Spans(ctx, topic)
	if err != nil {
		return nil, err
	}
	if n := partitions(topic); int32(len(s)) != n {
		return nil, fmt.Errorf("query: %s has %d partitions, the schema %d", topic, len(s), n)
	}
	return s, nil
}

// readSeries gives each record of the series on a topic, on a core topic from its instrument's partition alone
// (core/README.md), on a clean topic from its one, in offset order.
func readSeries(ctx context.Context, rec record, topic string, s series, each func(stream.Record) error) error {
	sp, err := spans(ctx, rec, topic)
	if err != nil {
		return err
	}
	p := stream.PartitionOf([]byte(s.instrument), partitions(topic))
	return rec.Read(ctx, topic, p, sp[p], func(r stream.Record) error {
		if string(r.Key) != s.key {
			return nil
		}
		return each(r)
	})
}

// firsts is the series' versions on market.raw, each minute once, its first copy (the schema's delivery: a message
// whose key and date were taken is a copy, dropped), in the order of their dates; and the copies dropped. at, when not
// nil, keeps the minute of that date alone.
func firsts(ctx context.Context, rec record, s series, at *uint64) ([]version, int, error) {
	seen := map[uint64]bool{}
	var out []version
	copies := 0
	err := readSeries(ctx, rec, raw.Topic, s, func(r stream.Record) error {
		m, err := raw.FromRecord(r)
		if err != nil {
			return err
		}
		d := m.KLine.Base().Date
		if at != nil && d != *at {
			return nil
		}
		if seen[d] {
			copies++
			return nil
		}
		seen[d] = true
		out = append(out, version{kline: m.KLine, read: m.Read})
		return nil
	})
	slices.SortStableFunc(out, func(a, b version) int { return compare(a.date(), b.date()) })
	return out, copies, err
}

// corrections is the series' versions on market.corrections, each once (the schema's delivery: a message whose key,
// date and read were taken is a copy, dropped, the first taken), in the order of their dates, then of their reads;
// and the copies dropped. at, when not nil, keeps the minute of that date alone.
func corrections(ctx context.Context, rec record, s series, at *uint64) ([]version, int, error) {
	seen := map[[2]uint64]bool{}
	var out []version
	copies := 0
	err := readSeries(ctx, rec, raw.CorrectionsTopic, s, func(r stream.Record) error {
		c, err := raw.CorrectionFromRecord(r)
		if err != nil {
			return err
		}
		d := c.KLine.Base().Date
		if at != nil && d != *at {
			return nil
		}
		id := [2]uint64{d, c.Read}
		if seen[id] {
			copies++
			return nil
		}
		seen[id] = true
		out = append(out, version{kline: c.KLine, read: c.Read, corrected: c.Corrected})
		return nil
	})
	slices.SortStableFunc(out, func(a, b version) int {
		if c := compare(a.date(), b.date()); c != 0 {
			return c
		}
		return compare(a.read, b.read)
	})
	return out, copies, err
}

// cleans is the series' K-lines on a clean topic, each minute once, its first copy (the schema's delivery: a message
// whose key and date were taken is a copy, dropped), in the order of their dates; and the copies dropped.
func cleans(ctx context.Context, rec record, topic string, s series) ([]version, int, error) {
	seen := map[uint64]bool{}
	var out []version
	copies := 0
	err := readSeries(ctx, rec, topic, s, func(r stream.Record) error {
		m, err := clean.FromRecord(r)
		if err != nil {
			return err
		}
		d := m.KLine.Base().Date
		if seen[d] {
			copies++
			return nil
		}
		seen[d] = true
		out = append(out, version{kline: m.KLine})
		return nil
	})
	slices.SortStableFunc(out, func(a, b version) int { return compare(a.date(), b.date()) })
	return out, copies, err
}

func compare(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// topics: every topic, each partition with its offsets and its messages, then the topic's partitions and its messages
// in all.
func topics(ctx context.Context, rec record, out io.Writer) error {
	names, err := rec.List(ctx)
	if err != nil {
		return err
	}
	slices.Sort(names)
	t := newTable(out, "topic", "partition", "start", "end", "messages")
	for _, n := range names {
		sp, err := rec.Spans(ctx, n)
		if err != nil {
			return err
		}
		var total int64
		for _, p := range slices.Sorted(maps.Keys(sp)) {
			s := sp[p]
			total += s.End - s.Start
			t.row(n, itoa(int64(p)), itoa(s.Start), itoa(s.End), itoa(s.End-s.Start))
		}
		t.row(n, "all "+itoa(len(sp)), "", "", itoa(total))
	}
	return t.flush()
}

// seriesOn: each series on a topic, by its key: its messages; its minutes, each date once; its copies, the messages
// dropped by the topic's delivery; its first and last minute, in its zone. A series on a partition not its
// instrument's is refused, since a series is read from that partition alone; on a clean topic, a series not the
// topic's.
func seriesOn(ctx context.Context, rec record, cat *catalog, topic string, out io.Writer) error {
	sp, err := spans(ctx, rec, topic)
	if err != nil {
		return err
	}
	type stat struct {
		s                series
		messages, copies int
		minutes          map[uint64]bool
		seen             map[[2]uint64]bool
		first, last      uint64
	}
	stats := map[string]*stat{}
	for _, p := range slices.Sorted(maps.Keys(sp)) {
		err := rec.Read(ctx, topic, p, sp[p], func(r stream.Record) error {
			var m raw.Message
			var read uint64
			switch topic {
			case raw.Topic:
				var err error
				if m, err = raw.FromRecord(r); err != nil {
					return err
				}
			case raw.CorrectionsTopic:
				c, err := raw.CorrectionFromRecord(r)
				if err != nil {
					return err
				}
				m, read = c.Message, c.Read
			default:
				c, err := clean.FromRecord(r)
				if err != nil {
					return err
				}
				m = raw.Message{Source: c.Source, Instrument: c.Instrument, Timeframe: c.Timeframe, KLine: c.KLine}
			}
			if want := stream.PartitionOf([]byte(m.Instrument), partitions(topic)); want != r.Partition {
				return fmt.Errorf("query: %s/%d@%d: %s on partition %d, its instrument's %d", r.Topic, r.Partition,
					r.Offset, m.Key(), r.Partition, want)
			}
			st := stats[m.Key()]
			if st == nil {
				st = &stat{s: series{key: m.Key(), source: m.Source, instrument: m.Instrument, timeframe: m.Timeframe},
					minutes: map[uint64]bool{}, seen: map[[2]uint64]bool{}}
				stats[m.Key()] = st
			}
			st.messages++
			d := m.KLine.Base().Date
			id := [2]uint64{d, read}
			if st.seen[id] {
				st.copies++
				return nil
			}
			st.seen[id] = true
			if len(st.minutes) == 0 || d < st.first {
				st.first = d
			}
			if len(st.minutes) == 0 || d > st.last {
				st.last = d
			}
			st.minutes[d] = true
			return nil
		})
		if err != nil {
			return err
		}
	}
	t := newTable(out, "series", "messages", "minutes", "copies", "first", "last", "zone")
	for _, k := range slices.Sorted(maps.Keys(stats)) {
		st := stats[k]
		loc, err := cat.zone(st.s)
		if err != nil {
			return err
		}
		t.row(k, itoa(st.messages), itoa(len(st.minutes)), itoa(st.copies), minute(st.first, loc),
			minute(st.last, loc), loc.String())
	}
	return t.flush()
}

// window is which of a series' versions bars shows: the first or the last n, or those from and to a minute, both
// included; all when none is given.
type window struct {
	first, last int
	from, to    *uint64
}

func (w window) of(vs []version) []version {
	if w.from != nil || w.to != nil {
		var out []version
		for _, v := range vs {
			if (w.from == nil || v.date() >= *w.from) && (w.to == nil || v.date() <= *w.to) {
				out = append(out, v)
			}
		}
		return out
	}
	if w.first > 0 {
		return vs[:min(w.first, len(vs))]
	}
	if w.last > 0 {
		return vs[max(len(vs)-w.last, 0):]
	}
	return vs
}

// bars: the series' K-lines on a topic in the window, a line each: the minute in the series' zone and in UTC, the
// base's fields, the source's own; on market.corrections each correction's read and the fields it changed too; on a
// clean topic, its index the clean series'.
func bars(ctx context.Context, rec record, s series, loc *time.Location, topic string, w window,
	out io.Writer) error {
	var vs []version
	var err error
	switch topic {
	case raw.Topic:
		vs, _, err = firsts(ctx, rec, s, nil)
	case raw.CorrectionsTopic:
		vs, _, err = corrections(ctx, rec, s, nil)
	default:
		vs, _, err = cleans(ctx, rec, topic, s)
	}
	if err != nil {
		return err
	}
	vs = w.of(vs)
	names, err := own(vs)
	if err != nil {
		return err
	}
	header := append(append([]string{loc.String(), "UTC"}, baseFields...), names...)
	if topic == raw.CorrectionsTopic {
		header = append(header, "read", "corrected")
	}
	t := newTable(out, header...)
	for _, v := range vs {
		f, err := v.fields()
		if err != nil {
			return err
		}
		cells := []string{minute(v.date(), loc), minute(v.date(), time.UTC)}
		for _, n := range append(slices.Clone(baseFields), names...) {
			cells = append(cells, cell(f, n))
		}
		if topic == raw.CorrectionsTopic {
			cells = append(cells, readTime(v.read), strings.Join(v.corrected, ","))
		}
		t.row(cells...)
	}
	return t.flush()
}

func cell(f map[string]string, n string) string {
	if v, ok := f[n]; ok {
		return v
	}
	return "-"
}

// minuteOf: one minute of the series, its first version on market.raw, then each of its corrections in the order of
// their reads, each with the fields it changed.
func minuteOf(ctx context.Context, rec record, s series, loc *time.Location, at time.Time, out io.Writer) error {
	date := uint64(at.UnixMilli())
	fs, copies, err := firsts(ctx, rec, s, &date)
	if err != nil {
		return err
	}
	cs, ccopies, err := corrections(ctx, rec, s, &date)
	if err != nil {
		return err
	}
	if len(fs) == 0 && len(cs) == 0 {
		return fmt.Errorf("query: no version of %s at %s on %s or %s", s.key, minute(date, loc), raw.Topic,
			raw.CorrectionsTopic)
	}
	first := "its first version"
	if len(fs) == 0 {
		first = "no first version"
	}
	fmt.Fprintf(out, "%s at %s (%s): %s on %s, %s dropped; %s on %s, %s dropped\n", s.key, minute(date, loc),
		minute(date, time.UTC), first, raw.Topic, counted(copies, "copy", "copies"),
		counted(len(cs), "correction", "corrections"), raw.CorrectionsTopic, counted(ccopies, "copy", "copies"))
	vs := append(fs, cs...)
	names, err := own(vs)
	if err != nil {
		return err
	}
	t := newTable(out, append(append([]string{"version", "read", "corrected"}, baseFields...), names...)...)
	for _, v := range vs {
		f, err := v.fields()
		if err != nil {
			return err
		}
		cells := []string{"first", readTime(v.read), "-"}
		if v.corrected != nil {
			cells = []string{"correction", readTime(v.read), strings.Join(v.corrected, ",")}
		}
		for _, n := range append(slices.Clone(baseFields), names...) {
			cells = append(cells, cell(f, n))
		}
		t.row(cells...)
	}
	return t.flush()
}

// correctionsBy: the series' corrections by day, in its zone (each day's corrections, the minutes they correct, how
// many change each field, how many change high or low), then by day and the fields changed.
func correctionsBy(ctx context.Context, rec record, s series, loc *time.Location, out io.Writer) error {
	cs, copies, err := corrections(ctx, rec, s, nil)
	if err != nil {
		return err
	}
	type day struct {
		corrections, highOrLow int
		minutes                map[uint64]bool
		fields                 map[string]int
		sets                   map[string]int
	}
	days := map[string]*day{}
	all := &day{minutes: map[uint64]bool{}, fields: map[string]int{}}
	for _, c := range cs {
		k := time.UnixMilli(int64(c.date())).In(loc).Format(time.DateOnly)
		d := days[k]
		if d == nil {
			d = &day{minutes: map[uint64]bool{}, fields: map[string]int{}, sets: map[string]int{}}
			days[k] = d
		}
		hl := slices.Contains(c.corrected, "high") || slices.Contains(c.corrected, "low")
		for _, x := range []*day{d, all} {
			x.corrections++
			x.minutes[c.date()] = true
			for _, n := range c.corrected {
				x.fields[n]++
			}
			if hl {
				x.highOrLow++
			}
		}
		d.sets[strings.Join(c.corrected, ",")]++
	}
	fmt.Fprintf(out, "%s: %s on %s, %s dropped, of %s; %d changing high or low; the days in %s\n", s.key,
		counted(all.corrections, "correction", "corrections"), raw.CorrectionsTopic, counted(copies, "copy", "copies"),
		counted(len(all.minutes), "minute", "minutes"), all.highOrLow, loc)
	if len(cs) == 0 {
		return nil
	}
	names := slices.Sorted(maps.Keys(all.fields))
	t := newTable(out, append(append([]string{"day", "corrections", "minutes"}, names...), "high-or-low")...)
	keys := slices.Sorted(maps.Keys(days))
	for _, x := range append(slices.Clone(keys), "all") {
		d := all
		if x != "all" {
			d = days[x]
		}
		cells := []string{x, itoa(d.corrections), itoa(len(d.minutes))}
		for _, n := range names {
			cells = append(cells, itoa(d.fields[n]))
		}
		t.row(append(cells, itoa(d.highOrLow))...)
	}
	if err := t.flush(); err != nil {
		return err
	}
	fmt.Fprintln(out)
	t = newTable(out, "day", "corrected", "corrections")
	for _, k := range keys {
		for _, set := range slices.Sorted(maps.Keys(days[k].sets)) {
			t.row(k, set, itoa(days[k].sets[set]))
		}
	}
	return t.flush()
}

// groupsOf: each consumer group, for each topic it has committed on or its members are assigned, each partition it
// has committed on, is assigned or that holds a message: its committed offset, the partition's end, and how far
// behind it is, from its committed offset, or with none from the partition's start, where the core's consumers begin.
func groupsOf(ctx context.Context, rec record, out io.Writer) error {
	gs, err := rec.Groups(ctx)
	if err != nil {
		return err
	}
	names, err := rec.List(ctx)
	if err != nil {
		return err
	}
	cache := map[string]map[int32]stream.Span{}
	t := newTable(out, "group", "state", "topic", "partition", "committed", "end", "behind")
	for _, g := range gs {
		set := map[string]bool{}
		for tp := range g.committed {
			set[tp] = true
		}
		for tp := range g.assigned {
			set[tp] = true
		}
		if len(set) == 0 {
			t.row(g.name, g.state, "-", "-", "-", "-", "-")
			continue
		}
		for _, tp := range slices.Sorted(maps.Keys(set)) {
			if !slices.Contains(names, tp) {
				for _, p := range slices.Sorted(maps.Keys(g.committed[tp])) {
					t.row(g.name, g.state, tp, itoa(int64(p)), itoa(g.committed[tp][p]), "-", "-")
				}
				continue
			}
			sp, ok := cache[tp]
			if !ok {
				if sp, err = rec.Spans(ctx, tp); err != nil {
					return err
				}
				cache[tp] = sp
			}
			ps := map[int32]bool{}
			for p := range g.committed[tp] {
				ps[p] = true
			}
			for _, p := range g.assigned[tp] {
				ps[p] = true
			}
			for p, s := range sp {
				if s.End > s.Start {
					ps[p] = true
				}
			}
			for _, p := range slices.Sorted(maps.Keys(ps)) {
				s := sp[p]
				c, ok := g.committed[tp][p]
				committed, behind := "-", itoa(s.End-s.Start)
				if ok {
					committed, behind = itoa(c), itoa(s.End-c)
				}
				t.row(g.name, g.state, tp, itoa(int64(p)), committed, itoa(s.End), behind)
			}
		}
	}
	return t.flush()
}
