package main

import (
	"bytes"
	"context"
	"errors"
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

// memRecord is the record on the in-memory bus: the bus's admin for the topics and their spans; a partition's records
// read by a source of a group of the test's own, query having none there (the in-memory bus reads only in a group);
// and the groups named, each its topics, their committed offsets read back by a source of the group that commits
// nothing: the next record it would be given, else the partition's end.
type memRecord struct {
	b      *mem.Bus
	adm    stream.Admin
	groups map[string][]string
}

func (m *memRecord) List(ctx context.Context) ([]string, error) { return m.adm.List(ctx) }

func (m *memRecord) Spans(ctx context.Context, topic string) (map[int32]stream.Span, error) {
	return m.adm.Spans(ctx, topic)
}

func (m *memRecord) Read(ctx context.Context, topic string, p int32, span stream.Span,
	each func(stream.Record) error) error {
	if span.Start >= span.End {
		return nil
	}
	src, err := m.b.Source(ctx, stream.SubscribeOptions{Group: "query-test", Topics: []string{topic},
		Start: stream.StartFromBeginning})
	if err != nil {
		return err
	}
	defer src.Close(ctx)
	recs, err := src.Fetch(ctx)
	if err != nil {
		return err
	}
	for _, r := range recs {
		if r.Partition == p && r.Offset >= span.Start && r.Offset < span.End {
			if err := each(r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *memRecord) Groups(ctx context.Context) ([]group, error) {
	var out []group
	for _, n := range slices.Sorted(func(yield func(string) bool) {
		for k := range m.groups {
			if !yield(k) {
				return
			}
		}
	}) {
		g := group{name: n, state: "Empty", committed: map[string]map[int32]int64{}}
		for _, t := range m.groups[n] {
			sp, err := m.adm.Spans(ctx, t)
			if err != nil {
				return nil, err
			}
			next := map[int32]int64{}
			for p, s := range sp {
				next[p] = s.End
			}
			src, err := m.b.Source(ctx, stream.SubscribeOptions{Group: n, Topics: []string{t}})
			if err != nil {
				return nil, err
			}
			fctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
			recs, err := src.Fetch(fctx)
			cancel()
			src.Close(ctx)
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			for _, r := range recs {
				next[r.Partition] = min(next[r.Partition], r.Offset)
			}
			for p, o := range next {
				if o > 0 { // a commit is of an offset from 1
					if g.committed[t] == nil {
						g.committed[t] = map[int32]int64{}
					}
					g.committed[t][p] = o
				}
			}
		}
		out = append(out, g)
	}
	return out, nil
}

func (m *memRecord) Close() {}

var ny = func() *time.Location {
	l, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return l
}()

// The fixture's minutes: 2026-11-04 09:30 and 09:31 New York, index 0 and 1; 2026-11-05 09:30 and 09:31, 390 and 391.
var (
	d0 = uint64(time.Date(2026, 11, 4, 9, 30, 0, 0, ny).UnixMilli())
	d1 = d0 + 60_000
	d2 = uint64(time.Date(2026, 11, 5, 9, 30, 0, 0, ny).UnixMilli())
	d3 = d2 + 60_000
	// reads: the first run's, then two later ones
	r0 = uint64(time.Date(2026, 11, 6, 1, 0, 0, 0, time.UTC).UnixMilli())
	r1 = uint64(time.Date(2026, 11, 7, 1, 0, 0, 0, time.UTC).UnixMilli())
	r2 = uint64(time.Date(2026, 11, 8, 1, 0, 0, 0, time.UTC).UnixMilli())
)

func kline(t *testing.T, index, date uint64, close, high, vw string) raw.KLine {
	t.Helper()
	k, err := raw.DecodeKLine([]byte(`{"close":` + close + `,"date":` + strconv.FormatUint(date, 10) + `,"high":` +
		high + `,"index":` + strconv.FormatUint(index, 10) + `,"low":599,"n":3,"open":600,"volume":10,"vw":` + vw + `}`))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func msg(instrument, timeframe string, k raw.KLine, read uint64) raw.Message {
	return raw.Message{Origin: raw.Live, Source: "massive", Instrument: instrument, Timeframe: timeframe,
		Publisher: "acquisition", KLine: k, Read: read}
}

// fixture: market.raw and market.corrections on an in-memory bus. On market.raw, SPY M1's four minutes, the first
// of raw market data 0.2.0 (no read), the third the zero K-line; SPY M5's one minute and QQQ M1's, the one on SPY's
// partition, the other on its own; then a copy of SPY M1's second minute, and one of its fourth of another value,
// both dropped, the first copy taken. On market.corrections, SPY M1's first minute corrected twice, of vw, then of
// high and vw, and the first again, a copy dropped; its third, the zero K-line, corrected to a bar; QQQ's one.
func fixture(t *testing.T) *mem.Bus {
	t.Helper()
	ctx := context.Background()
	b := mem.New()
	adm, _ := b.Admin(ctx)
	if _, err := raw.Provision(ctx, adm, 1); err != nil {
		t.Fatal(err)
	}
	sink, _ := b.Sink(ctx)
	put := func(r stream.Record, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if err := sink.Publish(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	zero, err := raw.Zero(390, d2)
	if err != nil {
		t.Fatal(err)
	}
	spy := []raw.KLine{kline(t, 0, d0, "600.1", "601", "600.5"), kline(t, 1, d1, "600.2", "601", "600.6"), zero,
		kline(t, 391, d3, "600.4", "601", "600.8")}
	// the first minute as 0.2.0 wrote it, with no read
	m0 := msg("SPY", "M1", spy[0], r0)
	v, err := m0.Encode()
	if err != nil {
		t.Fatal(err)
	}
	v = bytes.Replace(bytes.Replace(v, []byte(`"read":`+strconv.FormatUint(r0, 10)+`,`), nil, 1),
		[]byte(`"version":"0.3.0"`), []byte(`"version":"0.2.0"`), 1)
	r, err := m0.Record()
	r.Value = v
	put(r, err)
	if m, err := raw.FromRecord(r); err != nil || m.Read != 0 {
		t.Fatalf("the 0.2.0 value: %+v, %v", m, err)
	}
	for _, k := range spy[1:] {
		put(msg("SPY", "M1", k, r0).Record())
	}
	put(msg("SPY", "M5", kline(t, 0, d0, "600.9", "601", "600.5"), r0).Record())
	put(msg("QQQ", "M1", kline(t, 0, d0, "600.1", "601", "600.5"), r0).Record())
	put(msg("SPY", "M1", spy[1], r0).Record())
	put(msg("SPY", "M1", kline(t, 391, d3, "600.4", "601", "600.85"), r1).Record())

	correct := func(before, after raw.KLine, instrument string, read uint64) (stream.Record, error) {
		f, err := raw.Differ(before, after)
		if err != nil {
			t.Fatal(err)
		}
		return raw.Correction{Message: msg(instrument, "M1", after, read), Corrected: f}.Record()
	}
	c1 := kline(t, 0, d0, "600.1", "601", "600.55")
	c2 := kline(t, 0, d0, "600.1", "601.5", "600.57")
	put(correct(spy[0], c1, "SPY", r1))
	put(correct(c1, c2, "SPY", r2))
	put(correct(spy[0], c1, "SPY", r1))
	put(correct(zero, kline(t, 390, d2, "600.3", "601", "600.7"), "SPY", r1))
	put(correct(kline(t, 0, d0, "600.1", "601", "600.5"), kline(t, 0, d0, "600.1", "601", "600.6"), "QQQ", r1))
	return b
}

// consumers on the fixture: timescaledb-writer reads both topics and commits each record; printer reads market.raw
// and commits its first three records on SPY's partition.
func consumers(t *testing.T, b *mem.Bus) {
	t.Helper()
	ctx := context.Background()
	src, err := b.Source(ctx, stream.SubscribeOptions{Group: "timescaledb-writer", Topics: raw.Topics})
	if err != nil {
		t.Fatal(err)
	}
	recs, err := src.Fetch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Commit(ctx, recs...); err != nil {
		t.Fatal(err)
	}
	src.Close(ctx)
	src, err = b.Source(ctx, stream.SubscribeOptions{Group: "printer", Topics: []string{raw.Topic}})
	if err != nil {
		t.Fatal(err)
	}
	if recs, err = src.Fetch(ctx); err != nil {
		t.Fatal(err)
	}
	p := stream.PartitionOf([]byte("SPY"), raw.Partitions)
	var three []stream.Record
	for _, r := range recs {
		if r.Partition == p && len(three) < 3 {
			three = append(three, r)
		}
	}
	if err := src.Commit(ctx, three...); err != nil {
		t.Fatal(err)
	}
	src.Close(ctx)
}

// catalogFile: a catalog of two datasets, SPY M1 of massive in New York's zone, and one with no session.
func catalogFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "catalog.yaml")
	if err := os.WriteFile(p, []byte(`datasets:
  - name: massive-spy-1m
    source: massive
    instruments: [SPY]
    timeframes: [M1]
    session: { zone: America/New_York, open: "09:30", close: "16:00" }
  - name: massive-qqq-1m
    source: massive
    instruments: [QQQ]
    timeframes: [M1]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// query runs query's args on the fixture, its consumers' groups too.
func query(t *testing.T, b *mem.Bus, args ...string) (string, error) {
	t.Helper()
	ctx := context.Background()
	adm, _ := b.Admin(ctx)
	open := func(context.Context, []string) (record, error) {
		return &memRecord{b: b, adm: adm, groups: map[string][]string{"timescaledb-writer": raw.Topics,
			"printer": {raw.Topic}}}, nil
	}
	var out, errs bytes.Buffer
	err := run(ctx, args, open, &out, &errs)
	return out.String(), err
}

func want(t *testing.T, b *mem.Bus, w string, args ...string) {
	t.Helper()
	got, err := query(t, b, args...)
	if err != nil {
		t.Fatalf("%s: %v", strings.Join(args, " "), err)
	}
	if got != w {
		t.Fatalf("%s:\n%s\nwant:\n%s", strings.Join(args, " "), got, w)
	}
}

func TestQQQPartition(t *testing.T) {
	// The fixture's QQQ is on a partition of its own, SPY's 7.
	if p, q := stream.PartitionOf([]byte("SPY"), raw.Partitions), stream.PartitionOf([]byte("QQQ"), raw.Partitions); p != 7 || q == p {
		t.Fatalf("SPY %d, QQQ %d", p, q)
	}
}

func TestTopics(t *testing.T) {
	b := fixture(t)
	q := stream.PartitionOf([]byte("QQQ"), raw.Partitions)
	// every partition of each topic, SPY's 7 and QQQ's holding messages, the others none
	w := "topic               partition  start  end  messages\n"
	for _, tp := range []struct {
		name         string
		spy, qqq, in int
	}{{"market.corrections", 4, 1, 5}, {"market.raw", 7, 1, 8}} {
		for p := int32(0); p < raw.Partitions; p++ {
			n := 0
			switch p {
			case 7:
				n = tp.spy
			case q:
				n = tp.qqq
			}
			w += fmt.Sprintf("%-20s%-11d%-7d%-5d%d\n", tp.name, p, 0, n, n)
		}
		w += fmt.Sprintf("%-20s%-11s%-7s%-5s%d\n", tp.name, "all 16", "", "", tp.in)
	}
	want(t, b, w, "topics")
}

func TestSeries(t *testing.T) {
	b := fixture(t)
	cat := catalogFile(t)
	want(t, b, ``+
		"series          messages  minutes  copies  first                   last                    zone\n"+
		"massive:QQQ:M1  1         1        0       2026-11-04T14:30Z       2026-11-04T14:30Z       UTC\n"+
		"massive:SPY:M1  6         4        2       2026-11-04T09:30-05:00  2026-11-05T09:31-05:00  America/New_York\n"+
		"massive:SPY:M5  1         1        0       2026-11-04T14:30Z       2026-11-04T14:30Z       UTC\n",
		"series", "-catalog", cat)
	want(t, b, ``+
		"series          messages  minutes  copies  first                   last                    zone\n"+
		"massive:QQQ:M1  1         1        0       2026-11-04T14:30Z       2026-11-04T14:30Z       UTC\n"+
		"massive:SPY:M1  4         2        1       2026-11-04T09:30-05:00  2026-11-05T09:30-05:00  America/New_York\n",
		"series", "-catalog", cat, "-topic", "market.corrections")
}

func TestBars(t *testing.T) {
	b := fixture(t)
	cat := catalogFile(t)
	head := "America/New_York        UTC                index  open  high  low  close  volume  n  vw\n"
	rows := []string{
		"2026-11-04T09:30-05:00  2026-11-04T14:30Z  0      600   601   599  600.1  10      3  600.5\n",
		"2026-11-04T09:31-05:00  2026-11-04T14:31Z  1      600   601   599  600.2  10      3  600.6\n",
		"2026-11-05T09:30-05:00  2026-11-05T14:30Z  390    0     0     0    0      0       -  -\n",
		// the first copy, its vw 600.8, the later copy's 600.85 dropped
		"2026-11-05T09:31-05:00  2026-11-05T14:31Z  391    600   601   599  600.4  10      3  600.8\n",
	}
	want(t, b, head+strings.Join(rows, ""), "bars", "-catalog", cat, "-series", "massive:SPY:M1")
	want(t, b, head+strings.Join(rows[:2], ""), "bars", "-catalog", cat, "-series", "massive:SPY:M1", "-first", "2")
	want(t, b, head+strings.Join(rows[2:], ""), "bars", "-catalog", cat, "-series", "massive:SPY:M1", "-last", "2")
	want(t, b, head+strings.Join(rows[1:3], ""), "bars", "-catalog", cat, "-series", "massive:SPY:M1",
		"-from", "2026-11-04T09:31", "-to", "2026-11-05T14:30Z")
	want(t, b, head+strings.Join(rows[3:], ""), "bars", "-catalog", cat, "-series", "massive:SPY:M1",
		"-from", "2026-11-05T09:31-05:00")
	// a zone not in the catalog: UTC
	want(t, b, ``+
		"UTC                UTC                index  open  high  low  close  volume  n  vw\n"+
		"2026-11-04T14:30Z  2026-11-04T14:30Z  0      600   601   599  600.9  10      3  600.5\n",
		"bars", "-catalog", cat, "-series", "massive:SPY:M5")
	// the corrections: each once, by minute, then read; the copy dropped
	want(t, b, ``+
		"America/New_York        UTC                index  open  high   low  close  volume  n  vw      read                      corrected\n"+
		"2026-11-04T09:30-05:00  2026-11-04T14:30Z  0      600   601    599  600.1  10      3  600.55  2026-11-07T01:00:00.000Z  vw\n"+
		"2026-11-04T09:30-05:00  2026-11-04T14:30Z  0      600   601.5  599  600.1  10      3  600.57  2026-11-08T01:00:00.000Z  high,vw\n"+
		"2026-11-05T09:30-05:00  2026-11-05T14:30Z  390    600   601    599  600.3  10      3  600.7   2026-11-07T01:00:00.000Z  close,high,low,n,open,volume,vw\n",
		"bars", "-catalog", cat, "-series", "massive:SPY:M1", "-topic", "market.corrections")
}

func TestMinute(t *testing.T) {
	b := fixture(t)
	cat := catalogFile(t)
	w := `` +
		"massive:SPY:M1 at 2026-11-04T09:30-05:00 (2026-11-04T14:30Z): its first version on market.raw, 0 copies dropped; 2 corrections on market.corrections, 1 copy dropped\n" +
		"version     read                      corrected  index  open  high   low  close  volume  n  vw\n" +
		"first       -                         -          0      600   601    599  600.1  10      3  600.5\n" +
		"correction  2026-11-07T01:00:00.000Z  vw         0      600   601    599  600.1  10      3  600.55\n" +
		"correction  2026-11-08T01:00:00.000Z  high,vw    0      600   601.5  599  600.1  10      3  600.57\n"
	want(t, b, w, "minute", "-catalog", cat, "-series", "massive:SPY:M1", "-at", "2026-11-04T09:30")
	want(t, b, w, "minute", "-catalog", cat, "-series", "massive:SPY:M1", "-at", "2026-11-04T14:30Z")
	want(t, b, ``+
		"massive:SPY:M1 at 2026-11-05T09:31-05:00 (2026-11-05T14:31Z): its first version on market.raw, 1 copy dropped; 0 corrections on market.corrections, 0 copies dropped\n"+
		"version  read                      corrected  index  open  high  low  close  volume  n  vw\n"+
		"first    2026-11-06T01:00:00.000Z  -          391    600   601   599  600.4  10      3  600.8\n",
		"minute", "-catalog", cat, "-series", "massive:SPY:M1", "-at", "2026-11-05T09:31")
	if _, err := query(t, b, "minute", "-catalog", cat, "-series", "massive:SPY:M1", "-at", "2026-11-04T10:00"); err == nil ||
		!strings.Contains(err.Error(), "no version of massive:SPY:M1 at 2026-11-04T10:00-05:00") {
		t.Fatalf("a minute of no version: %v", err)
	}
}

func TestCorrections(t *testing.T) {
	b := fixture(t)
	cat := catalogFile(t)
	want(t, b, ``+
		"massive:SPY:M1: 3 corrections on market.corrections, 1 copy dropped, of 2 minutes; 2 changing high or low; the days in America/New_York\n"+
		"day         corrections  minutes  close  high  low  n  open  volume  vw  high-or-low\n"+
		"2026-11-04  2            1        0      1     0    0  0     0       2   1\n"+
		"2026-11-05  1            1        1      1     1    1  1     1       1   1\n"+
		"all         3            2        1      2     1    1  1     1       3   2\n"+
		"\n"+
		"day         corrected                        corrections\n"+
		"2026-11-04  high,vw                          1\n"+
		"2026-11-04  vw                               1\n"+
		"2026-11-05  close,high,low,n,open,volume,vw  1\n",
		"corrections", "-catalog", cat, "-series", "massive:SPY:M1")
	want(t, b, "massive:SPY:M5: 0 corrections on market.corrections, 0 copies dropped, of 0 minutes; 0 changing high "+
		"or low; the days in UTC\n", "corrections", "-catalog", cat, "-series", "massive:SPY:M5")
}

func TestGroups(t *testing.T) {
	b := fixture(t)
	consumers(t, b)
	q := strconv.Itoa(int(stream.PartitionOf([]byte("QQQ"), raw.Partitions)))
	// printer committed its first three of SPY's partition, none of QQQ's: behind on both
	want(t, b, ``+
		"group               state  topic               partition  committed  end  behind\n"+
		"printer             Empty  market.raw          "+q+"          -          1    1\n"+
		"printer             Empty  market.raw          7          3          7    4\n"+
		"timescaledb-writer  Empty  market.corrections  "+q+"          1          1    0\n"+
		"timescaledb-writer  Empty  market.corrections  7          4          4    0\n"+
		"timescaledb-writer  Empty  market.raw          "+q+"          1          1    0\n"+
		"timescaledb-writer  Empty  market.raw          7          7          7    0\n", "groups")
}

// The record is read only: every topic's partitions and ends and every group's offsets the same after each
// subcommand (the groups the test's consumers made; the stand-in's own, in which it reads, not among them).
func TestReadOnly(t *testing.T) {
	b := fixture(t)
	consumers(t, b)
	cat := catalogFile(t)
	state := func() string {
		s, err := query(t, b, "topics")
		if err != nil {
			t.Fatal(err)
		}
		g, err := query(t, b, "groups")
		if err != nil {
			t.Fatal(err)
		}
		return s + g
	}
	before := state()
	for _, args := range [][]string{{"topics"}, {"groups"}, {"series", "-catalog", cat},
		{"series", "-catalog", cat, "-topic", "market.corrections"}, {"bars", "-catalog", cat, "-series", "massive:SPY:M1"},
		{"bars", "-catalog", cat, "-series", "massive:SPY:M1", "-topic", "market.corrections"},
		{"minute", "-catalog", cat, "-series", "massive:SPY:M1", "-at", "2026-11-04T09:30"},
		{"corrections", "-catalog", cat, "-series", "massive:SPY:M1"}} {
		if _, err := query(t, b, args...); err != nil {
			t.Fatal(err)
		}
		if after := state(); after != before {
			t.Fatalf("after %q:\n%s\nbefore:\n%s", args, after, before)
		}
	}
}

func TestRefused(t *testing.T) {
	b := fixture(t)
	cat := catalogFile(t)
	for _, c := range []struct {
		args []string
		err  string
	}{
		{nil, "usage"},
		{[]string{"nothing"}, "usage"},
		{[]string{"topics", "-stack", "prod"}, `-stack "prod": stack or test`},
		{[]string{"topics", "extra"}, "no argument but its flags"},
		{[]string{"series", "-catalog", cat, "-topic", "raw-klines"}, `-topic "raw-klines"`},
		{[]string{"bars", "-catalog", cat}, "no -series"},
		{[]string{"bars", "-catalog", cat, "-series", "SPY:M1"}, "<source>:<instrument>:<timeframe>"},
		{[]string{"bars", "-catalog", cat, "-series", "massive:SPY:M1", "-first", "2", "-last", "2"}, "one of them"},
		{[]string{"bars", "-catalog", cat, "-series", "massive:SPY:M1", "-first", "-1"}, "from 1"},
		{[]string{"bars", "-catalog", cat, "-series", "massive:SPY:M1", "-from", "2026-11-05T09:30", "-to",
			"2026-11-04T09:30"}, "after -to"},
		{[]string{"bars", "-catalog", cat, "-series", "massive:SPY:M1", "-from", "2026-11-05 09:30"}, `minute "2026-11-05 09:30"`},
		{[]string{"minute", "-catalog", cat, "-series", "massive:SPY:M1"}, "no -at"},
		// New York's clock passes 02:30 by on 2026-03-08 and 01:30 twice on 2026-11-01
		{[]string{"minute", "-catalog", cat, "-series", "massive:SPY:M1", "-at", "2026-03-08T02:30"}, "passes it by"},
		{[]string{"minute", "-catalog", cat, "-series", "massive:SPY:M1", "-at", "2026-11-01T01:30"},
			"twice a time of America/New_York (2026-11-01T01:30-04:00 and 2026-11-01T01:30-05:00)"},
		{[]string{"corrections", "-catalog", filepath.Join(t.TempDir(), "none.yaml"), "-series", "massive:SPY:M1"},
			"catalog: open"},
	} {
		_, err := query(t, b, c.args...)
		if err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%q: %v, want %q", c.args, err, c.err)
		}
	}
}

// A message not of the schema on the series' partition stops the run, with where it is.
func TestNotOfTheSchema(t *testing.T) {
	b := fixture(t)
	ctx := context.Background()
	sink, _ := b.Sink(ctx)
	if err := sink.Publish(ctx, stream.Record{Topic: raw.Topic, Key: []byte("massive:SPY:M1"), PartitionKey: []byte("SPY"),
		Value: []byte(`{"not":"raw"}`), Time: time.UnixMilli(1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := query(t, b, "bars", "-catalog", catalogFile(t), "-series", "massive:SPY:M1"); err == nil ||
		!strings.Contains(err.Error(), "market.raw/7@7") {
		t.Fatalf("a message not of the schema: %v", err)
	}
}

func TestZone(t *testing.T) {
	c := &catalog{path: "c.yaml", datasets: []dataset{
		{Name: "a", Source: "massive", Instruments: []string{"SPY"}, Timeframes: []string{"M1"}},
		{Name: "b", Source: "massive", Instruments: []string{"SPY"}, Timeframes: []string{"M1"}},
	}}
	c.datasets[0].Session = &struct {
		Zone string `yaml:"zone"`
	}{Zone: "America/New_York"}
	s, _ := parseSeries("massive:SPY:M1")
	if loc, err := c.zone(s); err != nil || loc.String() != "America/New_York" {
		t.Fatalf("one zone: %v, %v", loc, err)
	}
	c.datasets[1].Session = &struct {
		Zone string `yaml:"zone"`
	}{Zone: "Europe/London"}
	if _, err := c.zone(s); err == nil || !strings.Contains(err.Error(), "zone America/New_York, and in b, zone Europe/London") {
		t.Fatalf("two zones: %v", err)
	}
}

// A clean topic: bars and series read it from its one partition, each minute once, its first copy; its index the
// clean series'. A series not the topic's, or a name not a clean topic's, refused.
func TestClean(t *testing.T) {
	ctx := context.Background()
	b := fixture(t)
	cat := catalogFile(t)
	topic := "market.clean.validation.v0-1-0.massive.SPY.M1"
	adm, _ := b.Admin(ctx)
	if _, err := clean.Provision(ctx, adm, topic, 1); err != nil {
		t.Fatal(err)
	}
	sink, _ := b.Sink(ctx)
	for _, k := range []raw.KLine{kline(t, 0, d0, "600.1", "601", "600.5"), kline(t, 1, d1, "600.2", "601", "600.6"),
		kline(t, 1, d1, "600.2", "601", "600.65"), kline(t, 2, d3, "600.4", "601", "600.8")} {
		r, err := clean.Message{Cleaning: clean.Cleaning{Name: "validation", Version: "0.1.0"}, Origin: raw.Live,
			Source: "massive", Instrument: "SPY", Timeframe: "M1", Publisher: "acquisition", KLine: k}.Record()
		if err != nil {
			t.Fatal(err)
		}
		if err := sink.Publish(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	want(t, b, ``+
		"America/New_York        UTC                index  open  high  low  close  volume  n  vw\n"+
		"2026-11-04T09:30-05:00  2026-11-04T14:30Z  0      600   601   599  600.1  10      3  600.5\n"+
		"2026-11-04T09:31-05:00  2026-11-04T14:31Z  1      600   601   599  600.2  10      3  600.6\n"+
		"2026-11-05T09:31-05:00  2026-11-05T14:31Z  2      600   601   599  600.4  10      3  600.8\n",
		"bars", "-catalog", cat, "-series", "massive:SPY:M1", "-topic", topic)
	// no -series: the topic's name's
	want(t, b, ``+
		"America/New_York        UTC                index  open  high  low  close  volume  n  vw\n"+
		"2026-11-05T09:31-05:00  2026-11-05T14:31Z  2      600   601   599  600.4  10      3  600.8\n",
		"bars", "-catalog", cat, "-topic", topic, "-last", "1")
	want(t, b, ``+
		"series          messages  minutes  copies  first                   last                    zone\n"+
		"massive:SPY:M1  4         3        1       2026-11-04T09:30-05:00  2026-11-05T09:31-05:00  America/New_York\n",
		"series", "-catalog", cat, "-topic", topic)
	for _, c := range []struct {
		args []string
		err  string
	}{
		{[]string{"bars", "-catalog", cat, "-series", "massive:QQQ:M1", "-topic", topic},
			"the clean topic " + topic + " is massive:SPY:M1's"},
		{[]string{"bars", "-catalog", cat, "-series", "massive:SPY:M1", "-topic", "market.clean.validation.SPY"},
			"not market.clean.<cleaning>.<version>.<source>.<instrument>.<timeframe>"},
	} {
		if _, err := query(t, b, c.args...); err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%q: %v, want %q", c.args, err, c.err)
		}
	}
}
