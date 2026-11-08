package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

const cleanTopic = "market.clean.validation.v0-1-0.massive.SPY.M1"

// highs is the plugin's text with the high of each minute named changed to its value.
func highs(text string, hs map[uint64]string) string {
	var b strings.Builder
	for _, l := range strings.SplitAfter(text, "\n") {
		for d, h := range hs {
			if strings.Contains(l, `"date":`+itoa(d)+`,`) {
				l = strings.Replace(l, `"high":601`, `"high":`+h, 1)
			}
		}
		b.WriteString(l)
	}
	return b.String()
}

// topicOf is the clean topic's K-lines, in offset order.
func (r *rig) topicOf(t *testing.T, topic string) []clean.Message {
	t.Helper()
	ctx := context.Background()
	adm, _ := r.bus.Admin(ctx)
	spans, err := adm.Spans(ctx, topic)
	if err != nil {
		t.Fatalf("the clean topic: %v", err)
	}
	if len(spans) != 1 {
		t.Fatalf("the clean topic: %d partitions", len(spans))
	}
	var out []clean.Message
	if spans[0].End == 0 {
		return nil
	}
	src, _ := r.bus.Source(ctx, stream.SubscribeOptions{Topics: []string{topic}})
	defer src.Close(ctx)
	for int64(len(out)) < spans[0].End {
		recs, _ := src.Fetch(ctx)
		for _, rec := range recs {
			m, err := clean.FromRecord(rec)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, m)
		}
	}
	return out
}

// cleanLine is the report's line of the clean step.
func (r *rig) cleanLine(t *testing.T) string {
	t.Helper()
	for _, l := range strings.Split(r.out(), "\n") {
		if strings.Contains(l, ": the clean: ") {
			return l
		}
	}
	t.Fatalf("no line of the clean in %q", r.out())
	return ""
}

// sameAsTopic: each store's clean series, as written at the last reset, is the topic's.
func (r *rig) sameAsTopic(t *testing.T, topic string, want []clean.Message) {
	t.Helper()
	for _, s := range []*memStore{r.ts, r.ch} {
		if !slices.Equal(s.series[topic], want) {
			t.Fatalf("%s's clean series: %d K-lines, the topic's %d", s.nm, len(s.series[topic]), len(want))
		}
	}
}

// The first run of a series makes its clean topic, its first reset: every minute's latest version from the store's
// corrected raw through the cleaning, the zero K-line left out, numbered from 0; one partition; each store's clean
// series the topic's; the damage after it 0. A run straight after publishes nothing on it.
func TestFirstReset(t *testing.T) {
	r := newRig(t)
	now := at(2025, 11, 26, 12, 0)
	from, to := ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59)
	hole := ms(2025, 11, 24, 9, 31)
	text := plugin{skip: map[uint64]bool{hole: true}}.text(r.g, from, to)
	if _, err := r.acquire(t, now, text, ""); err != nil {
		t.Fatal(err)
	}
	got := r.topicOf(t, cleanTopic)
	grid := r.g.within(from, to)
	if len(got) != 779 {
		t.Fatalf("the clean topic: %d K-lines, want 779", len(got))
	}
	for i, m := range got {
		d := grid[i]
		if i > 0 {
			d = grid[i+1] // the minute without a bar left out
		}
		k := m.KLine.Base()
		if k.Index != uint64(i) || k.Date != d || m.Cleaning != (clean.Cleaning{Name: "validation", Version: "0.1.0"}) ||
			m.Origin != raw.Live || m.Publisher != "acquisition" || m.Key() != "massive:SPY:M1" {
			t.Fatalf("K-line %d: %+v", i, m)
		}
		if m.KLine.Text() != kline(d, i, "600.5", "600.25") { // the raw's but its index
			t.Fatalf("K-line %d: %s", i, m.KLine.Text())
		}
	}
	r.sameAsTopic(t, cleanTopic, got)
	if !slices.Equal(r.ts.calls, []string{cleanTopic}) || !slices.Equal(r.ch.calls, []string{cleanTopic}) {
		t.Fatalf("the stores written: %v, %v", r.ts.calls, r.ch.calls)
	}
	line := r.cleanLine(t)
	for _, s := range []string{"the clean: " + cleanTopic + ", by validation 0.1.0: reset at 2025-11-26T17:00:00.000Z, " +
		"its first reset, no clean topic before; the topics derived from it deleted: none; made again from " +
		"timescaledb-writer's corrected raw, 780 minutes: 779 published, 1 refused (2025-11-24 09:31 EST: open 0: a " +
		"price not above 0); the stores' clean series written from it: timescaledb-writer, clickhouse-writer; now 779 " +
		"K-lines, index 0 to 778, 2025-11-24 09:30 EST to 2025-11-25 15:59 EST; the damage after it 0"} {
		if !strings.Contains(line, s) {
			t.Fatalf("%q not in %q", s, line)
		}
	}
	// Straight after: nothing on the clean topic, no store written.
	if _, err := r.acquire(t, now, text, ""); err != nil {
		t.Fatal(err)
	}
	if n := len(r.topicOf(t, cleanTopic)); n != 779 || len(r.ts.calls) != 1 {
		t.Fatalf("again: %d K-lines, the stores written %d times", n, len(r.ts.calls))
	}
	if line := r.cleanLine(t); !strings.Contains(line, "779 K-lines before the run (0 copies); the tail: no minute "+
		"after its last; now 779 K-lines") || !strings.Contains(line, "the latest versions cleaned, 1 refused; 0 "+
		"minutes damaged, the tolerance 10") {
		t.Fatalf("again: %q", line)
	}
}

// The tail: each minute after the clean topic's last, its first version, through the cleaning, published after the
// head, numbered after it; a minute refused at the end of the clean series given again with the next; the stores'
// clean series not written between resets.
func TestTail(t *testing.T) {
	r := newRig(t)
	if _, err := r.acquire(t, at(2025, 11, 26, 12, 0), plugin{}.text(r.g, ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59)), ""); err != nil {
		t.Fatal(err)
	}
	// Thursday noon: Wednesday, its 15:59 without a bar.
	last := ms(2025, 11, 26, 15, 59)
	text := plugin{skip: map[uint64]bool{last: true}}.text(r.g, ms(2025, 11, 24, 9, 30), last)
	if _, err := r.acquire(t, at(2025, 11, 27, 12, 0), text, ""); err != nil {
		t.Fatal(err)
	}
	if line := r.cleanLine(t); !strings.Contains(line, "780 K-lines before the run (0 copies); the tail: 390 minutes "+
		"given, 389 published (index 780 to 1168), 1 refused (2025-11-26 15:59 EST: open 0: a price not above 0); now "+
		"1169 K-lines, index 0 to 1168") || !strings.Contains(line, "0 minutes damaged") {
		t.Fatalf("Wednesday: %q", line)
	}
	// Saturday 09:00: Friday, to its early close; Wednesday's 15:59 given again, refused again.
	friday := r.g.within(ms(2025, 11, 28, 9, 30), ms(2025, 11, 28, 12, 59))
	text = plugin{skip: map[uint64]bool{last: true}}.text(r.g, ms(2025, 11, 24, 9, 30), friday[len(friday)-1])
	if _, err := r.acquire(t, at(2025, 11, 29, 9, 0), text, ""); err != nil {
		t.Fatal(err)
	}
	if line := r.cleanLine(t); !strings.Contains(line, "the tail: 211 minutes given, 210 published (index 1169 to "+
		"1378), 1 refused (2025-11-26 15:59 EST: open 0: a price not above 0); now 1379 K-lines, index 0 to 1378") {
		t.Fatalf("Friday: %q", line)
	}
	got := r.topicOf(t, cleanTopic)
	for i, m := range got {
		if m.KLine.Base().Index != uint64(i) || (i > 0 && m.KLine.Base().Date <= got[i-1].KLine.Base().Date) {
			t.Fatalf("K-line %d: %+v", i, m.KLine.Base())
		}
	}
	if got[1169].KLine.Base().Date != friday[0] || got[1168].KLine.Base().Date != ms(2025, 11, 26, 15, 58) {
		t.Fatalf("the tail after the head: %+v, %+v", got[1168].KLine.Base(), got[1169].KLine.Base())
	}
	if len(r.ts.calls) != 1 || len(r.ts.series[cleanTopic]) != 780 {
		t.Fatalf("the stores written between resets: %d, %d", len(r.ts.calls), len(r.ts.series[cleanTopic]))
	}
}

// The damage: a minute whose high or low in the clean topic differs from its latest version once cleaned; another
// field's change none. With 10 damaged minutes, the tolerance, no reset; a correction back to the first value, its
// minute no longer damaged; with 11, one reset: the topic made again under its name, its highs the corrections', each
// store's clean series the new topic's, the damage after it 0.
func TestDamage(t *testing.T) {
	r := newRig(t)
	from, to := ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59)
	base := plugin{}
	if _, err := r.acquire(t, at(2025, 11, 26, 12, 0), base.text(r.g, from, to), ""); err != nil {
		t.Fatal(err)
	}
	grid := r.g.within(from, to)
	changed := map[uint64]string{}
	for i := range 10 {
		changed[grid[10*i]] = "602"
	}
	// ten highs changed and a vw: 11 corrections, 10 minutes damaged
	vw := plugin{vws: map[uint64]string{grid[5]: "600.3"}}
	got, err := r.acquire(t, at(2025, 11, 26, 12, 1), highs(vw.text(r.g, from, to), changed), "")
	if err != nil || len(got.corrections) != 11 {
		t.Fatalf("ten highs: %d corrections, %v", len(got.corrections), err)
	}
	line := r.cleanLine(t)
	if !strings.Contains(line, "10 minutes damaged, the tolerance 10: 2025-11-24 09:30 EST, 2025-11-24 09:40 EST") ||
		strings.Contains(line, "reset at") || len(r.ts.calls) != 1 {
		t.Fatalf("ten: %q", line)
	}
	// one back to its first value: 9
	delete(changed, grid[0])
	if got, err = r.acquire(t, at(2025, 11, 26, 12, 2), highs(vw.text(r.g, from, to), changed), ""); err != nil ||
		len(got.corrections) != 1 || got.corrections[0].KLine.Base().Date != grid[0] {
		t.Fatalf("back: %+v, %v", got.corrections, err)
	}
	if line := r.cleanLine(t); !strings.Contains(line, "9 minutes damaged, the tolerance 10: 2025-11-24 09:40 EST") {
		t.Fatalf("back: %q", line)
	}
	// two more: 11, a reset
	changed[grid[200]], changed[grid[201]] = "603", "603"
	when := at(2025, 11, 26, 12, 3)
	if _, err = r.acquire(t, when, highs(vw.text(r.g, from, to), changed), ""); err != nil {
		t.Fatal(err)
	}
	line = r.cleanLine(t)
	if !strings.Contains(line, "reset at "+when.UTC().Format("2006-01-02T15:04:05.000Z")+", 11 minutes damaged, more "+
		"than the tolerance 10: 2025-11-24 09:40 EST,") || !strings.Contains(line, "made again from timescaledb-writer's "+
		"corrected raw, 780 minutes: 780 published, 0 refused; the stores' clean series written from it: "+
		"timescaledb-writer, clickhouse-writer; now 780 K-lines, index 0 to 779") ||
		!strings.Contains(line, "the damage after it 0") {
		t.Fatalf("eleven: %q", line)
	}
	topic := r.topicOf(t, cleanTopic)
	if len(topic) != 780 {
		t.Fatalf("made again: %d K-lines", len(topic))
	}
	for i, m := range topic {
		h := "601"
		if v, ok := changed[grid[i]]; ok {
			h = v
		}
		if m.KLine.Base().Index != uint64(i) || m.KLine.Base().High.String() != h {
			t.Fatalf("made again, K-line %d: %+v, its high %s", i, m.KLine.Base(), h)
		}
	}
	r.sameAsTopic(t, cleanTopic, topic)
	if len(r.ts.calls) != 2 || len(r.ch.calls) != 2 {
		t.Fatalf("the stores written: %v", r.ts.calls)
	}
	// straight after: no damage, nothing
	if _, err = r.acquire(t, when, highs(vw.text(r.g, from, to), changed), ""); err != nil {
		t.Fatal(err)
	}
	if line := r.cleanLine(t); !strings.Contains(line, "0 minutes damaged") || len(r.topicOf(t, cleanTopic)) != 780 {
		t.Fatalf("after the reset: %q", line)
	}
}

// The zero K-line, left out of the clean series; a correction of it to a bar is damage: the cleaned latest versions
// hold the minute, the clean topic not.
func TestZeroCorrected(t *testing.T) {
	r := newRig(t)
	from, to := ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59)
	hole := ms(2025, 11, 24, 9, 31)
	if _, err := r.acquire(t, at(2025, 11, 26, 12, 0), plugin{skip: map[uint64]bool{hole: true}}.text(r.g, from, to), ""); err != nil {
		t.Fatal(err)
	}
	got, err := r.acquire(t, at(2025, 11, 26, 12, 1), plugin{}.text(r.g, from, to), "")
	if err != nil || len(got.corrections) != 1 {
		t.Fatalf("the bar: %+v, %v", got, err)
	}
	if line := r.cleanLine(t); !strings.Contains(line, "the latest versions cleaned, 0 refused; 1 minutes damaged, the "+
		"tolerance 10: 2025-11-24 09:31 EST") {
		t.Fatalf("%q", line)
	}
}

// An instrument whose name holds a dot is refused a clean topic, with that reason; its raw published.
func TestDotted(t *testing.T) {
	r := newRig(t)
	r.a.dataset.Instruments = []string{"BRK.B"}
	got, err := r.acquire(t, at(2025, 11, 26, 12, 0), plugin{}.text(r.g, ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59)), "")
	if err != nil || len(got.firsts) != 780 {
		t.Fatalf("%d first versions, %v", len(got.firsts), err)
	}
	if !strings.Contains(r.out(), `acquisition: massive:BRK.B:M1: the clean: validation 0.1.0 refused a clean topic: `+
		`clean: instrument "BRK.B" holds a dot, which parts a topic's names: no clean topic for its series until such `+
		`names are settled`) {
		t.Fatalf("%q", r.out())
	}
	adm, _ := r.bus.Admin(context.Background())
	names, _ := adm.List(context.Background())
	for _, n := range names {
		if strings.HasPrefix(n, clean.Kind) {
			t.Fatalf("a clean topic made: %s", n)
		}
	}
}

// A reset refused: the store's corrected raw not the record's latest versions, nothing deleted or made; a reader of
// the clean topic running. Stopped, the reset made: the topic and those derived from it deleted with the readers'
// offsets, so that a reader started again reads from the start.
func TestResetRefused(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	from, to := ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59)
	text := plugin{}.text(r.g, from, to)
	r.ts.alter = func(ks []raw.KLine) []raw.KLine {
		k, _ := raw.DecodeKLine([]byte(strings.Replace(ks[3].Text(), `"close":600.5`, `"close":600.75`, 1)))
		ks[3] = k
		return ks
	}
	if _, err := r.acquire(t, at(2025, 11, 26, 12, 0), text, ""); err == nil ||
		!strings.Contains(err.Error(), "timescaledb-writer's corrected raw of massive:SPY:M1 at 2025-11-24 09:33 EST") {
		t.Fatalf("the corrected raw not the record's: %v", err)
	}
	adm, _ := r.bus.Admin(ctx)
	if names, _ := adm.List(ctx); slices.Contains(names, cleanTopic) {
		t.Fatalf("a clean topic made")
	}
	r.ts.alter = nil
	if _, err := r.acquire(t, at(2025, 11, 26, 12, 0), text, ""); err != nil {
		t.Fatal(err)
	}
	// a reader of the clean topic, its offset committed; a topic derived from it
	derived := "engine.go.v0-2-0.validation.v0-1-0.massive.SPY.M1"
	if err := adm.Ensure(ctx, stream.TopicSpec{Name: derived, Partitions: 1}); err != nil {
		t.Fatal(err)
	}
	reader, _ := r.bus.Source(ctx, stream.SubscribeOptions{Group: "engine", Topics: []string{cleanTopic}})
	recs, _ := reader.Fetch(ctx)
	if err := reader.Commit(ctx, recs[len(recs)-1]); err != nil {
		t.Fatal(err)
	}
	grid := r.g.within(from, to)
	changed := map[uint64]string{}
	for i := range 11 {
		changed[grid[i]] = "602"
	}
	text = highs(text, changed)
	if _, err := r.acquire(t, at(2025, 11, 26, 12, 1), text, ""); err == nil ||
		!strings.Contains(err.Error(), "the reader in group engine of "+cleanTopic+" is running: the reset waits for it stopped") {
		t.Fatalf("a reader running: %v", err)
	}
	if n := len(r.topicOf(t, cleanTopic)); n != 780 {
		t.Fatalf("the topic touched: %d", n)
	}
	reader.Close(ctx)
	if _, err := r.acquire(t, at(2025, 11, 26, 12, 2), text, ""); err != nil {
		t.Fatal(err)
	}
	if line := r.cleanLine(t); !strings.Contains(line, "11 minutes damaged, more than the tolerance 10") ||
		!strings.Contains(line, "the topics derived from it deleted: "+derived) || !strings.Contains(line, "the damage after it 0") {
		t.Fatalf("stopped: %q", line)
	}
	if names, _ := adm.List(ctx); slices.Contains(names, derived) {
		t.Fatalf("%s not deleted", derived)
	}
	if gs, _ := adm.Groups(ctx, cleanTopic); len(gs) != 0 {
		t.Fatalf("the reader's offsets kept: %+v", gs)
	}
	again, _ := r.bus.Source(ctx, stream.SubscribeOptions{Group: "engine", Topics: []string{cleanTopic}})
	defer again.Close(ctx)
	if recs, _ := again.Fetch(ctx); len(recs) == 0 || recs[0].Offset != 0 {
		t.Fatalf("the reader started again: from %d", recs[0].Offset)
	}
}

// A reset that fails once its topic is made (a store's clean series not written): the topic deleted, the run failing;
// the next run makes it again whole, its first reset, every store's clean series with it.
func TestResetFailed(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	text := plugin{}.text(r.g, ms(2025, 11, 24, 9, 30), ms(2025, 11, 25, 15, 59))
	r.ch.fail = true
	_, err := r.acquire(t, at(2025, 11, 26, 12, 0), text, "")
	if err == nil || !strings.Contains(err.Error(), "clickhouse-writer -clean "+cleanTopic+": exit status 1; "+cleanTopic+
		" deleted, the next run makes it again") {
		t.Fatalf("a store failing: %v", err)
	}
	adm, _ := r.bus.Admin(ctx)
	if names, _ := adm.List(ctx); slices.Contains(names, cleanTopic) {
		t.Fatalf("the topic kept")
	}
	r.ch.fail = false
	if _, err := r.acquire(t, at(2025, 11, 26, 12, 0), text, ""); err != nil {
		t.Fatal(err)
	}
	got := r.topicOf(t, cleanTopic)
	if len(got) != 780 || !strings.Contains(r.cleanLine(t), "its first reset") {
		t.Fatalf("made again: %d, %q", len(got), r.cleanLine(t))
	}
	r.sameAsTopic(t, cleanTopic, got)
}
