package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// The clean series (plugins.md, 3; core/README.md, clean market data): after a series' run, its dataset's cleaning
// makes its clean topic, one series', kept as the record changes. The tail: each minute after the clean topic's last,
// its first version, given to the cleaning, each K-line it gives published on the clean topic after the last. The
// damage: the minutes whose high or low in the clean topic differs from the minute's latest version once cleaned, or
// that one holds and the other not. The reset, when the damage is more than the dataset's tolerance, and as the
// series' first clean topic: the topic made again under its name from the TimescaleDB store's corrected raw, through
// the cleaning, numbered from 0, and each store's clean series written from it.

// store is a store on the core as a reset uses it: its corrected raw of a series read, and its clean series written
// from a clean topic.
type store interface {
	name() string
	// corrected is the series' corrected raw as the store holds it, each minute's latest version in the order of their
	// dates, once the store's writer has read the core's topics to their ends.
	corrected(ctx context.Context, key string) ([]raw.KLine, error)
	// clean empties the store's clean series of the topic's series and writes every bar of the topic.
	clean(ctx context.Context, topic string) error
}

// command is a store reached through its writer's program (its README under consumers/): `argv… -corrected KEY` prints
// the series' corrected raw, a K-line in its canonical text a line; `argv… -clean TOPIC` writes the store's clean
// series from the topic. Each must exit 0; its stderr is acquisition's.
type command struct {
	argv   []string
	stderr io.Writer
}

func (c command) name() string { return filepath.Base(c.argv[0]) }

func (c command) run(ctx context.Context, stdout io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, c.argv[0], append(append([]string(nil), c.argv[1:]...), args...)...)
	cmd.Stdout, cmd.Stderr = stdout, c.stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", c.name(), strings.Join(args, " "), err)
	}
	return nil
}

func (c command) corrected(ctx context.Context, key string) ([]raw.KLine, error) {
	var out bytes.Buffer
	if err := c.run(ctx, &out, "-corrected", key); err != nil {
		return nil, err
	}
	var ks []raw.KLine
	sc := bufio.NewScanner(&out)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for n := 1; sc.Scan(); n++ {
		k, err := raw.DecodeKLine(sc.Bytes())
		if err != nil {
			return nil, fmt.Errorf("%s -corrected %s: line %d: %w", c.name(), key, n, err)
		}
		ks = append(ks, k)
	}
	return ks, sc.Err()
}

func (c command) clean(ctx context.Context, topic string) error {
	return c.run(ctx, c.stderr, "-clean", topic)
}

// answer is the cleaning's answer to a K-line: the K-line it gives, or its refusal and the reason.
type answer struct {
	kline   raw.KLine
	given   bool
	refused string
}

// cleaning runs the dataset's cleaning over the K-lines, one series' in the order of their dates (plugins.md, 3):
// `command… --clean [--after DATE]`, the K-lines on its stdin, one a line, in their canonical text; it gives the
// cleaning's name and version, its first line, and its answer to each K-line, in order, once it has exited 0. A K-line
// it gives is in its canonical text and of its input's date; its index is set by its publisher.
func (a *acquirer) cleaning(ctx context.Context, ks []raw.KLine, after *uint64) (clean.Cleaning, []answer, error) {
	var c clean.Cleaning
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	d := a.dataset.Cleaning
	args := append(append([]string(nil), d[1:]...), "--clean")
	if after != nil {
		args = append(args, "--after", strconv.FormatUint(*after, 10))
	}
	cmd := exec.CommandContext(ctx, d[0], args...)
	cmd.Stderr = a.stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return c, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return c, nil, err
	}
	if err := cmd.Start(); err != nil {
		return c, nil, fmt.Errorf("cleaning %s: %w", d[0], err)
	}
	wrote := make(chan error, 1)
	go func() {
		w := bufio.NewWriterSize(stdin, 1<<16)
		var err error
		for _, k := range ks {
			if _, err = w.WriteString(k.Text() + "\n"); err != nil {
				break
			}
		}
		if err == nil {
			err = w.Flush()
		}
		if cerr := stdin.Close(); err == nil {
			err = cerr
		}
		wrote <- err
	}()
	fault := func(err error) (clean.Cleaning, []answer, error) {
		cancel()
		_ = cmd.Wait()
		<-wrote
		return c, nil, fmt.Errorf("cleaning %s: %w", d[0], err)
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	if !sc.Scan() {
		return fault(fmt.Errorf("no line naming the cleaning and its version (%v)", sc.Err()))
	}
	var named struct {
		Cleaning string `json:"cleaning"`
		Version  string `json:"version"`
	}
	if err := strict(sc.Bytes(), &named); err != nil {
		return fault(fmt.Errorf("its first line %q, not {\"cleaning\":…,\"version\":…}: %w", sc.Bytes(), err))
	}
	c = clean.Cleaning{Name: named.Cleaning, Version: named.Version}
	if err := c.Check(); err != nil {
		return fault(err)
	}
	out := make([]answer, 0, len(ks))
	for sc.Scan() {
		n := len(out)
		if n == len(ks) {
			return fault(fmt.Errorf("a line after its answers to the %d K-lines given: %q", n, sc.Bytes()))
		}
		var v struct {
			KLine   json.RawMessage `json:"kline"`
			Refused *string         `json:"refused"`
		}
		if err := strict(sc.Bytes(), &v); err != nil || (v.KLine == nil) == (v.Refused == nil) {
			return fault(fmt.Errorf("its answer %d %q, not {\"kline\":…} or {\"refused\":…} (%v)", n+1, sc.Bytes(), err))
		}
		if v.Refused != nil {
			out = append(out, answer{refused: *v.Refused})
			continue
		}
		k, err := raw.DecodeKLine(v.KLine)
		if err != nil {
			return fault(fmt.Errorf("its answer %d: %w", n+1, err))
		}
		if k.Base().Date != ks[n].Base().Date {
			return fault(fmt.Errorf("its answer %d: a K-line at %d to one at %d", n+1, k.Base().Date,
				ks[n].Base().Date))
		}
		out = append(out, answer{kline: k, given: true})
	}
	if err := sc.Err(); err != nil {
		return fault(err)
	}
	if err := cmd.Wait(); err != nil {
		<-wrote
		return c, nil, fmt.Errorf("cleaning %s: %w", d[0], err)
	}
	if err := <-wrote; err != nil {
		return c, nil, fmt.Errorf("cleaning %s: its input: %w", d[0], err)
	}
	if len(out) != len(ks) {
		return c, nil, fmt.Errorf("cleaning %s: %d answers to %d K-lines", d[0], len(out), len(ks))
	}
	return c, out, nil
}

// strict decodes one JSON object, no field but v's, nothing after it.
func strict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("more after the object")
	}
	return nil
}

// cleanSeries is a series' clean topic as it stands: its K-lines by date, in the order of their dates, a copy dropped
// (by its key and date, the first taken).
type cleanSeries struct {
	dates  []uint64
	klines map[uint64]raw.KLine
	copies int
}

// readClean reads the clean topic from its start to its end as it stood, in no group: each value clean market data of
// the cleaning and the series, each K-line after the one before it at its place in the clean series, its date a
// minute of the record.
func readClean(ctx context.Context, bus stream.Bus, topic string, c clean.Cleaning, key string, r *record) (cleanSeries, error) {
	cs := cleanSeries{klines: map[uint64]raw.KLine{}}
	adm, err := bus.Admin(ctx)
	if err != nil {
		return cs, err
	}
	spans, err := adm.Spans(ctx, topic)
	adm.Close(ctx)
	if err != nil {
		return cs, err
	}
	next := map[int32]int64{}
	done := func() bool {
		for p, s := range spans {
			if max(next[p], s.Start) < s.End {
				return false
			}
		}
		return true
	}
	if done() {
		return cs, nil
	}
	src, err := bus.Source(ctx, stream.SubscribeOptions{Topics: []string{topic}})
	if err != nil {
		return cs, err
	}
	defer src.Close(context.Background())
	for !done() {
		recs, err := src.Fetch(ctx)
		if err != nil {
			return cs, err
		}
		for _, rec := range recs {
			if rec.Offset >= spans[rec.Partition].End {
				continue // published after the read began
			}
			next[rec.Partition] = rec.Offset + 1
			m, err := clean.FromRecord(rec)
			if err != nil {
				return cs, err
			}
			if m.Cleaning != c || m.Key() != key {
				return cs, fmt.Errorf("%s/%d@%d: a value of %s %s of %s, not %s %s of %s", rec.Topic, rec.Partition,
					rec.Offset, m.Cleaning.Name, m.Cleaning.Version, m.Key(), c.Name, c.Version, key)
			}
			k := m.KLine.Base()
			if _, ok := cs.klines[k.Date]; ok {
				cs.copies++
				continue
			}
			n := len(cs.dates)
			if n > 0 && k.Date < cs.dates[n-1] {
				return cs, fmt.Errorf("%s/%d@%d: a K-line at %d after one at %d", rec.Topic, rec.Partition,
					rec.Offset, k.Date, cs.dates[n-1])
			}
			if k.Index != uint64(n) {
				return cs, fmt.Errorf("%s/%d@%d: the K-line at %d has index %d, its place in the clean series %d",
					rec.Topic, rec.Partition, rec.Offset, k.Date, k.Index, n)
			}
			if _, ok := r.first[k.Date]; !ok {
				return cs, fmt.Errorf("%s/%d@%d: a K-line at %d, a minute not on %s", rec.Topic, rec.Partition,
					rec.Offset, k.Date, raw.Topic)
			}
			cs.dates = append(cs.dates, k.Date)
			cs.klines[k.Date] = m.KLine
		}
	}
	return cs, nil
}

// damaged is the series' damaged minutes, in the order of their dates: of the cleaned latest versions and the clean
// topic's K-lines, each minute in one and not the other, or in both with another high or low. A minute's latest
// version is its last correction, else its first, so that a minute corrected and corrected back counts nothing, and
// one corrected twice once.
func damaged(latest map[uint64]raw.KLine, cs cleanSeries) []uint64 {
	var out []uint64
	for d, k := range latest {
		c, ok := cs.klines[d]
		if !ok || c.Base().High != k.Base().High || c.Base().Low != k.Base().Low {
			out = append(out, d)
		}
	}
	for d := range cs.klines {
		if _, ok := latest[d]; !ok {
			out = append(out, d)
		}
	}
	slices.Sort(out)
	return out
}

// cleaned is what the cleaning gave of a series' K-lines: those it gave by date, in the order of their dates, and its
// refusals, each minute and its reason.
type cleaned struct {
	dates   []uint64
	klines  map[uint64]raw.KLine
	refused []refusal
}

type refusal struct {
	date   uint64
	reason string
}

func cleanedOf(ks []raw.KLine, as []answer) cleaned {
	out := cleaned{klines: map[uint64]raw.KLine{}}
	for i, a := range as {
		d := ks[i].Base().Date
		if !a.given {
			out.refused = append(out.refused, refusal{date: d, reason: a.refused})
			continue
		}
		out.dates = append(out.dates, d)
		out.klines[d] = a.kline
	}
	return out
}

// cleanTally is a series' clean step of a run, for its line.
type cleanTally struct {
	topic                    string
	cleaning                 clean.Cleaning
	head, copies             int // the clean topic's K-lines before the tail, its copies dropped
	given, published         int // the tail: minutes given to the cleaning, K-lines published
	firstIndex               uint64
	refused                  []refusal // the tail's
	damaged                  []uint64
	tolerance                int
	reset                    *resetTally
	refusedTopic, noMinutes  string
	sessionOf                *session
	cleanFirst, cleanLast    uint64
	cleanCount, cleanRefused int
}

// resetTally is a reset, for its line.
type resetTally struct {
	at                 time.Time
	first              bool     // the series' first clean topic
	damaged            []uint64 // what made it
	tolerance          int
	derived            []string
	from               string // the store read
	minutes, published int
	refused            []refusal
	stores             []string
	after              int // the damage after it
}

// cleanSeriesOf is the series' clean step after its run (r, the record with the run's versions): the cleaning of every
// minute's latest version; then the series' clean topic, made by its first reset when it is not there, else its tail
// published; then the damage, and a reset when it is more than the tolerance.
func (a *acquirer) cleanSeriesOf(ctx context.Context, sink stream.Sink, m raw.Message, r *record) (cleanTally, error) {
	t := cleanTally{tolerance: *a.dataset.Tolerance, sessionOf: a.dataset.Session}
	dates := r.dates()
	if len(dates) == 0 {
		t.noMinutes = "no minute on " + raw.Topic
		return t, nil
	}
	latest := make([]raw.KLine, len(dates))
	for i, d := range dates {
		k, err := raw.DecodeKLine([]byte(r.latestOf(d).text))
		if err != nil {
			return t, fmt.Errorf("the minute at %d, its latest version: %w", d, err)
		}
		latest[i] = k
	}
	c, as, err := a.cleaning(ctx, latest, nil)
	if err != nil {
		return t, err
	}
	t.cleaning = c
	cl := cleanedOf(latest, as)
	topic, err := clean.Topic(c, m.Source, m.Instrument, m.Timeframe)
	if err != nil {
		t.refusedTopic = err.Error()
		return t, nil
	}
	t.topic = topic
	adm, err := a.bus.Admin(ctx)
	if err != nil {
		return t, err
	}
	names, err := adm.List(ctx)
	adm.Close(ctx)
	if err != nil {
		return t, err
	}
	if !slices.Contains(names, topic) {
		rt, err := a.reset(ctx, sink, m, r, c, topic, false, nil, cl, latest)
		t.reset = rt
		if err == nil {
			t.cleanCount, t.cleanFirst, t.cleanLast = rt.published, cl.first(), cl.last()
		}
		return t, err
	}
	cs, err := readClean(ctx, a.bus, topic, c, m.Key(), r)
	if err != nil {
		return t, err
	}
	t.head, t.copies = len(cs.dates), cs.copies
	// the tail: each minute after the clean topic's last, its first version
	var tail []raw.KLine
	var after *uint64
	if n := len(cs.dates); n > 0 {
		after = &cs.dates[n-1]
	}
	for _, d := range dates {
		if after != nil && d <= *after {
			continue
		}
		k, err := raw.DecodeKLine([]byte(r.first[d].text))
		if err != nil {
			return t, fmt.Errorf("the minute at %d, its first version: %w", d, err)
		}
		tail = append(tail, k)
	}
	t.given = len(tail)
	if len(tail) > 0 {
		tc, tas, err := a.cleaning(ctx, tail, after)
		if err != nil {
			return t, err
		}
		if tc != c {
			return t, fmt.Errorf("cleaning %s: %s %s, then %s %s", a.dataset.Cleaning[0], c.Name, c.Version, tc.Name,
				tc.Version)
		}
		tcl := cleanedOf(tail, tas)
		t.refused = tcl.refused
		t.firstIndex = uint64(len(cs.dates))
		n, err := a.publishClean(ctx, sink, m, r, c, tcl, len(cs.dates))
		if err != nil {
			return t, err
		}
		t.published = n
		for _, d := range tcl.dates {
			cs.dates = append(cs.dates, d)
			cs.klines[d] = tcl.klines[d]
		}
	}
	t.cleanCount = len(cs.dates)
	if len(cs.dates) > 0 {
		t.cleanFirst, t.cleanLast = cs.dates[0], cs.dates[len(cs.dates)-1]
	}
	t.cleanRefused = len(cl.refused)
	t.damaged = damaged(cl.klines, cs)
	if len(t.damaged) > t.tolerance {
		rt, err := a.reset(ctx, sink, m, r, c, topic, true, t.damaged, cl, latest)
		t.reset = rt
		if err == nil {
			t.cleanCount, t.cleanFirst, t.cleanLast = rt.published, cl.first(), cl.last()
		}
		return t, err
	}
	return t, nil
}

func (c cleaned) first() uint64 {
	if len(c.dates) == 0 {
		return 0
	}
	return c.dates[0]
}

func (c cleaned) last() uint64 {
	if len(c.dates) == 0 {
		return 0
	}
	return c.dates[len(c.dates)-1]
}

// publishClean publishes the K-lines the cleaning gave on the clean topic, in the order of their dates, each its
// index its place in the clean series from from; it gives how many.
func (a *acquirer) publishClean(ctx context.Context, sink stream.Sink, m raw.Message, r *record, c clean.Cleaning,
	cl cleaned, from int) (int, error) {
	recs := make([]stream.Record, 0, len(cl.dates))
	for i, d := range cl.dates {
		k, err := raw.WithIndex(cl.klines[d], uint64(from+i))
		if err != nil {
			return 0, err
		}
		v := clean.Message{Cleaning: c, Origin: r.latestOf(d).origin, Source: m.Source, Instrument: m.Instrument,
			Timeframe: m.Timeframe, Publisher: a.publisher, KLine: k}
		rec, err := v.Record()
		if err != nil {
			return 0, fmt.Errorf("the clean K-line at %d: %w", d, err)
		}
		recs = append(recs, rec)
	}
	for i := 0; i < len(recs); i += batch {
		if err := sink.Publish(ctx, recs[i:min(i+batch, len(recs))]...); err != nil {
			return 0, err
		}
	}
	return len(recs), nil
}

// reset makes the series' clean topic again under its name (plugins.md, 3), or its first, when exists is false: once
// the TimescaleDB writer has read the core's topics to their ends, the store's corrected raw of the series read,
// every minute to the series' last, and checked against the record's latest versions; the clean topic's readers,
// and those of any topic derived from it, stopped (a group with a member reading it refuses the reset); the topic and
// those derived from it deleted, with every group's offsets on them, so that a reader started again reads from its
// start; the topic provisioned; the corrected raw through the cleaning, each K-line it gives published, numbered from
// 0; each store's clean series emptied and written from it; the topic read again, the damage after it 0. A step that
// fails once the topic is made deletes it, so that the next run makes it again whole, as the series' first reset, and
// every store's clean series with it.
func (a *acquirer) reset(ctx context.Context, sink stream.Sink, m raw.Message, r *record, c clean.Cleaning,
	topic string, exists bool, why []uint64, latestCleaned cleaned, latest []raw.KLine) (*resetTally, error) {
	rt := &resetTally{at: a.now().UTC(), first: !exists, damaged: why, tolerance: *a.dataset.Tolerance}
	if a.source == nil || len(a.stores) == 0 {
		return rt, errors.New("a reset needs the store whose corrected raw it reads (-corrected) and the stores " +
			"whose clean series it writes (-store)")
	}
	rt.from = a.source.name()
	corrected, err := a.source.corrected(ctx, m.Key())
	if err != nil {
		return rt, err
	}
	if len(corrected) != len(latest) {
		return rt, fmt.Errorf("%s's corrected raw of %s: %d minutes, the record's %d", rt.from, m.Key(),
			len(corrected), len(latest))
	}
	for i, k := range corrected {
		if k.Text() != latest[i].Text() {
			return rt, fmt.Errorf("%s's corrected raw of %s at %s: %s, the record's latest version %s", rt.from,
				m.Key(), stamp(latest[i].Base().Date, a.dataset.Session), k.Text(), latest[i].Text())
		}
	}
	rt.minutes = len(corrected)
	adm, err := a.bus.Admin(ctx)
	if err != nil {
		return rt, err
	}
	defer adm.Close(context.Background())
	names, err := adm.List(ctx)
	if err != nil {
		return rt, err
	}
	gone := clean.Derived(topic, names)
	rt.derived = gone
	if exists {
		gone = append([]string{topic}, gone...)
	}
	for _, t := range gone {
		gs, err := adm.Groups(ctx, t)
		if err != nil {
			return rt, err
		}
		for _, g := range gs {
			if g.Members > 0 {
				return rt, fmt.Errorf("the reader in group %s of %s is running: the reset waits for it stopped", g.Name, t)
			}
		}
	}
	if err := adm.Delete(ctx, gone...); err != nil {
		return rt, err
	}
	// undo: the topic made deleted, a fault after it is made; the next run makes it again, its first reset
	undo := func(err error) (*resetTally, error) {
		dctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if derr := adm.Delete(dctx, topic); derr != nil {
			return rt, fmt.Errorf("%w; %s not deleted (%v): delete it, so that the next run makes it again", err,
				topic, derr)
		}
		return rt, fmt.Errorf("%w; %s deleted, the next run makes it again", err, topic)
	}
	if _, err := clean.Provision(ctx, adm, topic, 1); err != nil {
		return undo(err)
	}
	rc, as, err := a.cleaning(ctx, corrected, nil)
	if err != nil {
		return undo(err)
	}
	if rc != c {
		return undo(fmt.Errorf("cleaning %s: %s %s, then %s %s", a.dataset.Cleaning[0], c.Name, c.Version, rc.Name,
			rc.Version))
	}
	cl := cleanedOf(corrected, as)
	rt.refused = cl.refused
	if rt.published, err = a.publishClean(ctx, sink, m, r, c, cl, 0); err != nil {
		return undo(err)
	}
	for _, s := range a.stores {
		if err := s.clean(ctx, topic); err != nil {
			return undo(err)
		}
		rt.stores = append(rt.stores, s.name())
	}
	cs, err := readClean(ctx, a.bus, topic, c, m.Key(), r)
	if err != nil {
		return undo(err)
	}
	if len(cs.dates) != rt.published {
		return undo(fmt.Errorf("%s: %d K-lines read again, %d published", topic, len(cs.dates), rt.published))
	}
	after := damaged(latestCleaned.klines, cs)
	rt.after = len(after)
	if len(after) > 0 {
		return undo(fmt.Errorf("%s made again: %d minutes damaged after it: %s", topic, len(after),
			minutes(after, a.dataset.Session)))
	}
	return rt, nil
}

// minutes is a list of minutes for a line, in the session's zone.
func minutes(ds []uint64, s *session) string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = stamp(d, s)
	}
	return strings.Join(out, ", ")
}

func refusals(rs []refusal, s *session) string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = stamp(r.date, s) + ": " + r.reason
	}
	return strings.Join(out, "; ")
}

// report is the series' clean step as a line: the clean topic, its K-lines, the tail, the damage, and a reset made.
func (t cleanTally) report() string {
	switch {
	case t.noMinutes != "":
		return "the clean: " + t.noMinutes + ", no clean topic"
	case t.refusedTopic != "":
		return fmt.Sprintf("the clean: %s %s refused a clean topic: %s", t.cleaning.Name, t.cleaning.Version,
			t.refusedTopic)
	}
	s := t.sessionOf
	held := func() string {
		if t.cleanCount == 0 {
			return "none"
		}
		return fmt.Sprintf("%d K-lines, index 0 to %d, %s to %s", t.cleanCount, t.cleanCount-1, stamp(t.cleanFirst, s),
			stamp(t.cleanLast, s))
	}
	if rt := t.reset; rt != nil {
		why := "its first reset, no clean topic before"
		if !rt.first {
			why = fmt.Sprintf("%d minutes damaged, more than the tolerance %d: %s", len(rt.damaged), rt.tolerance,
				minutes(rt.damaged, s))
		}
		derived := "none"
		if len(rt.derived) > 0 {
			derived = strings.Join(rt.derived, ", ")
		}
		ref := ""
		if len(rt.refused) > 0 {
			ref = " (" + refusals(rt.refused, s) + ")"
		}
		return fmt.Sprintf("the clean: %s, by %s %s: reset at %s, %s; the topics derived from it deleted: %s; made "+
			"again from %s's corrected raw, %d minutes: %d published, %d refused%s; the stores' clean series written "+
			"from it: %s; now %s; the damage after it %d", t.topic, t.cleaning.Name, t.cleaning.Version,
			rt.at.Format("2006-01-02T15:04:05.000Z"), why, derived, rt.from, rt.minutes, rt.published, len(rt.refused),
			ref, strings.Join(rt.stores, ", "), held(), rt.after)
	}
	tail := "no minute after its last"
	if t.given > 0 {
		idx := "none"
		if t.published > 0 {
			idx = fmt.Sprintf("index %d to %d", t.firstIndex, t.firstIndex+uint64(t.published)-1)
		}
		ref := ""
		if len(t.refused) > 0 {
			ref = " (" + refusals(t.refused, s) + ")"
		}
		tail = fmt.Sprintf("%d minutes given, %d published (%s), %d refused%s", t.given, t.published, idx,
			len(t.refused), ref)
	}
	dmg := ""
	if len(t.damaged) > 0 {
		dmg = ": " + minutes(t.damaged, s)
	}
	return fmt.Sprintf("the clean: %s, by %s %s: %d K-lines before the run (%d copies); the tail: %s; now %s; the "+
		"latest versions cleaned, %d refused; %d minutes damaged, the tolerance %d%s", t.topic, t.cleaning.Name,
		t.cleaning.Version, t.head, t.copies, tail, held(), t.cleanRefused, len(t.damaged), t.tolerance, dmg)
}
