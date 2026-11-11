package abstract

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"
)

// The parts the tests wire, and the data they work on. Each part is a small function of what it reads, with no
// meaning beyond being told apart from the others: a wiring that gave a part another's input, or its output to
// another's topics, would show in what the topics hold.
//
// A test's logs are finite, and a log has no end, so the data carries its own: a message that says stop. A part
// passes it on to everything it writes and returns, so that a run returns, and what it wrote is then all there is
// to compare.

// page is a message of the source, bar the k-line, fix a message of the corrections stream.
type (
	page struct {
		N    int
		Stop bool
	}
	bar struct {
		N    int
		Stop bool
	}
	fix struct {
		N    int
		Stop bool
	}
)

// part gives the partition, of count, that the number n goes to.
func part(n, count int) int { return ((n % count) + count) % count }

// appenders gives the writing end of each partition of t.
func appenders[K, V any](topics AppendSet, t Topic[K, V]) ([]Appender[Record[K, V]], error) {
	var to []Appender[Record[K, V]]
	for p := range t.Partitions() {
		a, err := AppendTo(topics, t, p)
		if err != nil {
			return nil, err
		}
		to = append(to, a)
	}
	return to, nil
}

// barRecord is the record of the n-th bar a publisher is given: the bar's number its key, n its timestamp and a
// header.
func barRecord(b bar, n int) Record[string, bar] {
	return Record[string, bar]{Key: new(strconv.Itoa(b.N)), Value: &b, Timestamp: int64(n), Headers: []Header{{Key: "n", Value: []byte(strconv.Itoa(n))}}}
}

// reader: a page gives the bar of its number, and a page of an even number a second bar, of ten times it.
type reader struct{}

func (reader) Read(ctx context.Context, source Cursor[page], raw Appender[bar]) error {
	for {
		p, err := source.Next(ctx)
		if err != nil {
			return err
		}
		if p.Stop {
			return raw.Append(ctx, bar{Stop: true})
		}
		if err := raw.Append(ctx, bar{N: p.N}); err != nil {
			return err
		}
		if p.N%2 == 0 {
			if err := raw.Append(ctx, bar{N: 10 * p.N}); err != nil {
				return err
			}
		}
	}
}

// cleaner: a bar below zero is dropped, and so is a bar of a number it has given before; what it has given it
// keeps in the call.
type cleaner struct{}

func (cleaner) Clean(ctx context.Context, raw Cursor[bar], clean Appender[bar]) error {
	given := map[int]bool{}
	for {
		b, err := raw.Next(ctx)
		if err != nil {
			return err
		}
		if b.Stop {
			return clean.Append(ctx, b)
		}
		if b.N < 0 || given[b.N] {
			continue
		}
		given[b.N] = true
		if err := clean.Append(ctx, b); err != nil {
			return err
		}
	}
}

// rawPublisher: each bar to the partition of bars its number chooses, and its number, keyed by its place in the
// series, to count.
type rawPublisher struct {
	bars  Topic[string, bar]
	count Topic[int, string]
}

func (p rawPublisher) PublishRaw(ctx context.Context, raw Cursor[bar], topics AppendSet) error {
	bars, err := appenders(topics, p.bars)
	if err != nil {
		return err
	}
	count, err := AppendTo(topics, p.count, 0)
	if err != nil {
		return err
	}
	for n := 1; ; n++ {
		b, err := raw.Next(ctx)
		if err != nil {
			return err
		}
		if b.Stop {
			for _, to := range bars {
				if err := to.Append(ctx, Record[string, bar]{Value: &b}); err != nil {
					return err
				}
			}
			return count.Append(ctx, Record[int, string]{Value: new("stop")})
		}
		if err := bars[part(b.N, len(bars))].Append(ctx, barRecord(b, n)); err != nil {
			return err
		}
		if err := count.Append(ctx, Record[int, string]{Key: new(n), Value: new(strconv.Itoa(b.N)), Timestamp: int64(n)}); err != nil {
			return err
		}
	}
}

// cleanPublisher: each bar to the partition of bars its number chooses, and a bar of an odd number to odd too.
type cleanPublisher struct {
	bars Topic[string, bar]
	odd  Topic[string, bar]
}

func (p cleanPublisher) PublishClean(ctx context.Context, clean Cursor[bar], topics AppendSet) error {
	bars, err := appenders(topics, p.bars)
	if err != nil {
		return err
	}
	odd, err := AppendTo(topics, p.odd, 0)
	if err != nil {
		return err
	}
	for n := 1; ; n++ {
		b, err := clean.Next(ctx)
		if err != nil {
			return err
		}
		if b.Stop {
			for _, to := range append(slices.Clone(bars), odd) {
				if err := to.Append(ctx, Record[string, bar]{Value: &b}); err != nil {
					return err
				}
			}
			return nil
		}
		if err := bars[part(b.N, len(bars))].Append(ctx, barRecord(b, n)); err != nil {
			return err
		}
		if b.N%2 != 0 {
			if err := odd.Append(ctx, barRecord(b, n)); err != nil {
				return err
			}
		}
	}
}

// correctionsPublisher: each correction, as it comes, to the one partition of fixes.
type correctionsPublisher struct {
	fixes Topic[string, fix]
}

func (p correctionsPublisher) PublishCorrections(ctx context.Context, corrections Cursor[fix], topics AppendSet) error {
	to, err := AppendTo(topics, p.fixes, 0)
	if err != nil {
		return err
	}
	for n := 1; ; n++ {
		f, err := corrections.Next(ctx)
		if err != nil {
			return err
		}
		if f.Stop {
			return to.Append(ctx, Record[string, fix]{Value: &f})
		}
		if err := to.Append(ctx, Record[string, fix]{Key: new("fix"), Value: &f, Timestamp: int64(n)}); err != nil {
			return err
		}
	}
}

// consumer is the warehouse consumer of the schema of keys K and values V: it pushes each record's message twice,
// which a table holds as one row. It returns at the record that says stop.
type consumer[K, V any, P comparable] struct {
	stop func(Record[K, V]) bool
	push func(Record[K, V]) P
}

func (c consumer[K, V, P]) Consume(ctx context.Context, partition Cursor[Record[K, V]], pushes Appender[P]) error {
	for {
		r, err := partition.Next(ctx)
		if err != nil {
			return err
		}
		if c.stop(r) {
			return nil
		}
		for range 2 {
			if err := pushes.Append(ctx, c.push(r)); err != nil {
				return err
			}
		}
	}
}

// The consumers of the three schemas of acquisition's topics; what each pushes is of its own type.
var (
	barConsumer = consumer[string, bar, string]{
		stop: func(r Record[string, bar]) bool { return r.Value.Stop },
		push: func(r Record[string, bar]) string { return fmt.Sprintf("bar %s at %d", *r.Key, r.Timestamp) },
	}
	countConsumer = consumer[int, string, int]{
		stop: func(r Record[int, string]) bool { return *r.Value == "stop" },
		push: func(r Record[int, string]) int { return *r.Key },
	}
	fixConsumer = consumer[string, fix, string]{
		stop: func(r Record[string, fix]) bool { return r.Value.Stop },
		push: func(r Record[string, fix]) string { return fmt.Sprintf("fix %d at %d", r.Value.N, r.Timestamp) },
	}
)

// worker is the engine's: it reads the partitions of its clean topic one after another, each to its stop, and
// writes the sum of the bars so far, keyed by the bar, to its own topic; then a record of no value, its stop.
type worker struct {
	clean Topic[string, bar]
	model Topic[string, int]
}

func (w worker) Work(ctx context.Context, clean ReadSet, own AppendSet) error {
	to, err := AppendTo(own, w.model, 0)
	if err != nil {
		return err
	}
	sum := 0
	for p := range w.clean.Partitions() {
		records, err := ReadFrom(clean, w.clean, p)
		if err != nil {
			return err
		}
		for {
			r, err := records.Next(ctx)
			if err != nil {
				return err
			}
			if r.Value.Stop {
				break
			}
			sum += r.Value.N
			if err := to.Append(ctx, Record[string, int]{Key: r.Key, Value: new(sum), Timestamp: r.Timestamp}); err != nil {
				return err
			}
		}
	}
	return to.Append(ctx, Record[string, int]{})
}

// chart is visualization: its first update says how many topics it is given; then the bars of each partition of
// each clean topic it shows, one after another, each to its stop; then the engine's topics, of which it knows one.
type chart struct {
	shown []Topic[string, bar]
	model Topic[string, int]
}

func (c chart) Draw(ctx context.Context, clean ReadSet, model ReadSet, updates Appender[string]) error {
	if err := updates.Append(ctx, fmt.Sprintf("%d clean topics, %d of the engine's", len(clean.Topics()), len(model.Topics()))); err != nil {
		return err
	}
	for _, t := range c.shown {
		for p := range t.Partitions() {
			records, err := ReadFrom(clean, t, p)
			if err != nil {
				return err
			}
			for {
				r, err := records.Next(ctx)
				if err != nil {
					return err
				}
				if r.Value.Stop {
					break
				}
				if err := updates.Append(ctx, fmt.Sprintf("%s, partition %d: bar %d", t.Name(), p, r.Value.N)); err != nil {
					return err
				}
			}
		}
	}
	for _, t := range model.Topics() {
		if t.Name() != c.model.Name() {
			return fmt.Errorf("the chart knows no topic %q of the engine's", t.Name())
		}
		records, err := ReadFrom(model, c.model, 0)
		if err != nil {
			return err
		}
		for {
			r, err := records.Next(ctx)
			if err != nil {
				return err
			}
			if r.Value == nil {
				break
			}
			if err := updates.Append(ctx, fmt.Sprintf("%s: %s is %d", t.Name(), *r.Key, *r.Value)); err != nil {
				return err
			}
		}
	}
	return nil
}

// The inputs: pages and corrections, each to its stop, and others to tell a part's own input from another's.
var (
	pages      = []page{{N: 1}, {N: 2}, {N: 2}, {N: 3}, {N: -4}, {N: 5}, {N: 6}, {Stop: true}}
	otherPages = []page{{N: 7}, {N: 8}, {Stop: true}}
	fixes      = []fix{{N: 2}, {N: 20}, {Stop: true}}
	otherFixes = []fix{{N: 9}, {Stop: true}}
)

func inputs(pages []page, fixes []fix) Inputs[page, fix] {
	return Inputs[page, fix]{Source: logOf(pages...).Read(), Corrections: logOf(fixes...).Read()}
}

// tables names the table of each of acquisition's topics: the raw and the clean bars go into one.
var tables = map[string]string{
	"raw.bars":    "bars",
	"raw.count":   "counts",
	"clean.bars":  "bars",
	"clean.odd":   "odd",
	"corrections": "fixes",
}

// fixture is the topics of a working space, each with its logs in memory, the database and the chart's log: what a
// run writes, to be read after it. Two fixtures have the same topics and logs of their own.
type fixture struct {
	rawBars   memoryTopic[string, bar]
	rawCount  memoryTopic[int, string]
	cleanBars memoryTopic[string, bar]
	cleanOdd  memoryTopic[string, bar]
	fixes     memoryTopic[string, fix]
	model     memoryTopic[string, int]
	texts     *memoryDB[string]
	numbers   *memoryDB[int]
	views     *memoryLog[string]
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return &fixture{
		rawBars:   newMemoryTopic[string, bar](t, "raw.bars", 2),
		rawCount:  newMemoryTopic[int, string](t, "raw.count", 1),
		cleanBars: newMemoryTopic[string, bar](t, "clean.bars", 2),
		cleanOdd:  newMemoryTopic[string, bar](t, "clean.odd", 1),
		fixes:     newMemoryTopic[string, fix](t, "corrections", 1),
		model:     newMemoryTopic[string, int](t, "model", 1),
		texts:     newMemoryDB[string](),
		numbers:   newMemoryDB[int](),
		views:     logOf[string](),
	}
}

// topics is acquisition's topics, by kind.
func (f *fixture) topics() AllTopics {
	return AllTopics{
		Raw:         []TopicLogs{f.rawBars.logs, f.rawCount.logs},
		Clean:       []TopicLogs{f.cleanBars.logs, f.cleanOdd.logs},
		Corrections: []TopicLogs{f.fixes.logs},
	}
}

// acquired is acquisition's topics, all of them.
func (f *fixture) acquired() []TopicLogs {
	all := f.topics()
	return slices.Concat(all.Raw, all.Clean, all.Corrections)
}

func (f *fixture) rawPublisher() rawPublisher {
	return rawPublisher{bars: f.rawBars.topic, count: f.rawCount.topic}
}

func (f *fixture) cleanPublisher() cleanPublisher {
	return cleanPublisher{bars: f.cleanBars.topic, odd: f.cleanOdd.topic}
}

func (f *fixture) correctionsPublisher() correctionsPublisher {
	return correctionsPublisher{fixes: f.fixes.topic}
}

func (f *fixture) acquisition() Acquisition[page, bar, fix] {
	return Acquisition[page, bar, fix]{
		Reader:             reader{},
		Cleaner:            cleaner{},
		PublishRaw:         f.rawPublisher(),
		PublishClean:       f.cleanPublisher(),
		PublishCorrections: f.correctionsPublisher(),
	}
}

// warehousing has the consumer of each of the three schemas, the bars' and the fixes' pushes into one database and
// the counts' into another.
func (f *fixture) warehousing(t *testing.T) Warehousing {
	t.Helper()
	w, err := NewWarehousing(ConsumerOf(barConsumer, f.texts), ConsumerOf(countConsumer, f.numbers), ConsumerOf(fixConsumer, f.texts))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func (f *fixture) worker() worker { return worker{clean: f.cleanBars.topic, model: f.model.topic} }

func (f *fixture) chart() chart {
	return chart{shown: []Topic[string, bar]{f.cleanBars.topic, f.cleanOdd.topic}, model: f.model.topic}
}

// wiring wires the fixture's parts and topics: the engine, where there is one, on the clean bars; visualization on
// both clean topics.
func (f *fixture) wiring(t *testing.T, engine bool) Wiring[page, bar, fix, string] {
	t.Helper()
	w := Wiring[page, bar, fix, string]{
		Topics:      f.topics(),
		Acquisition: f.acquisition(),
		Warehousing: f.warehousing(t),
		Tables:      tables,
		Shown:       []AnyTopic{f.cleanBars.topic, f.cleanOdd.topic},
		Chart:       f.chart(),
	}
	if engine {
		w.Engine = &Engine{Clean: []AnyTopic{f.cleanBars.topic}, Topics: []TopicLogs{f.model.logs}, Worker: f.worker()}
	}
	return w
}

// ranSpace makes the working space of a new fixture's wiring, runs it on the inputs to its end, and gives both.
func ranSpace(t *testing.T, engine bool, pages []page, fixes []fix) (*fixture, *Workspace[page, bar, fix, string]) {
	t.Helper()
	f := newFixture(t)
	space, err := f.wiring(t, engine).Workspace()
	if err != nil {
		t.Fatal(err)
	}
	if err := space.Run(within(t), inputs(pages, fixes), f.views); err != nil {
		t.Fatal(err)
	}
	return f, space
}

// ranAcquisition runs acquisition alone on the inputs, on a new fixture: what acquisition writes from the inputs.
func ranAcquisition(t *testing.T, pages []page, fixes []fix) *fixture {
	t.Helper()
	f := newFixture(t)
	if err := f.acquisition().Run(within(t), inputs(pages, fixes), f.topics()); err != nil {
		t.Fatal(err)
	}
	return f
}

// readSet and appendSet give those topics to read, or to append to.
func readSet(t *testing.T, topics ...TopicLogs) ReadSet {
	t.Helper()
	s, err := NewReadSet(topics...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func appendSet(t *testing.T, topics ...TopicLogs) AppendSet {
	t.Helper()
	s, err := NewAppendSet(topics...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// recordText writes a record out: its key and value, absent or not, its timestamp and its headers.
func recordText[K, V any](r Record[K, V]) string {
	key, value := "none", "none"
	if r.Key != nil {
		key = fmt.Sprintf("%v", *r.Key)
	}
	if r.Value != nil {
		value = fmt.Sprintf("%+v", *r.Value)
	}
	return fmt.Sprintf("key %s, value %s, at %d, headers %q", key, value, r.Timestamp, r.Headers)
}

// sameLog fails the test unless the two logs hold the same messages in the same order. A log expected to hold
// nothing compares nothing, and fails too: sameEmpty is for that.
func sameLog[T comparable](t *testing.T, what string, got, want []T) {
	t.Helper()
	if len(want) == 0 {
		t.Errorf("%s: nothing to compare: the log expected holds no message", what)
		return
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s:\n got  %v\n want %v", what, got, want)
	}
}

func sameEmpty[T any](t *testing.T, what string, got []T) {
	t.Helper()
	if len(got) != 0 {
		t.Errorf("%s: holds %v, and should hold nothing", what, got)
	}
}

// sameTopicLogs fails the test unless the two topics hold the same records, partition by partition.
func sameTopicLogs[K, V any](t *testing.T, got, want memoryTopic[K, V]) {
	t.Helper()
	for p := range want.partitions {
		var g, w []string
		for _, r := range got.partitions[p].all() {
			g = append(g, recordText(r))
		}
		for _, r := range want.partitions[p].all() {
			w = append(w, recordText(r))
		}
		sameLog(t, fmt.Sprintf("partition %d of topic %q", p, want.topic.Name()), g, w)
	}
}
