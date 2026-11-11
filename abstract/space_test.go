package abstract

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// The tests of this file hold what the package builds in beside the theorems: a topic's shape, what each part of a
// working space is given, what a wiring is refused for, and how a run ends.

// A topic has at least one partition, and one log for each.
func TestTopic(t *testing.T) {
	for _, partitions := range []int{0, -1} {
		if _, err := NewTopic[string, bar]("bars", partitions); err == nil {
			t.Errorf("a topic of %d partitions is made", partitions)
		}
	}
	if _, err := (Topic[string, bar]{}).Logs(); err == nil {
		t.Error("the zero topic, of no partition, is given logs")
	}
	topic, err := NewTopic[string, bar]("bars", 2)
	if err != nil {
		t.Fatal(err)
	}
	if topic.Name() != "bars" || topic.Partitions() != 2 || topic.Key().Kind().String() != "string" || topic.Value().Name() != "bar" {
		t.Errorf("the topic is %q, %d partitions, keys %v, values %v", topic.Name(), topic.Partitions(), topic.Key(), topic.Value())
	}
	one, two := logOf[Record[string, bar]](), logOf[Record[string, bar]]()
	for what, logs := range map[string][]Log[Record[string, bar]]{"one log": {one}, "three logs": {one, two, one}, "a log missing": {one, nil}} {
		if _, err := topic.Logs(logs...); err == nil {
			t.Errorf("a topic of 2 partitions is given %s", what)
		}
	}
	logs, err := topic.Logs(one, two)
	if err != nil {
		t.Fatal(err)
	}
	if logs.Topic() != AnyTopic(topic) {
		t.Errorf("the logs' topic is %v", logs.Topic())
	}

	// a part is given a partition of a topic it holds, by the topic's own description, and nothing else
	other, err := NewTopic[string, bar]("others", 2)
	if err != nil {
		t.Fatal(err)
	}
	fewer, err := NewTopic[string, bar]("bars", 1)
	if err != nil {
		t.Fatal(err)
	}
	numbers, err := NewTopic[int, int]("bars", 2)
	if err != nil {
		t.Fatal(err)
	}
	reads, appends := readSet(t, logs), appendSet(t, logs)
	refused := map[string]error{}
	_, refused["to read, a topic the set does not hold"] = ReadFrom(reads, other, 0)
	_, refused["to read, the name with another number of partitions"] = ReadFrom(reads, fewer, 0)
	_, refused["to read, the name with another schema"] = ReadFrom(reads, numbers, 0)
	_, refused["to read, a partition the topic has not"] = ReadFrom(reads, topic, 2)
	_, refused["to read, a partition below 0"] = ReadFrom(reads, topic, -1)
	_, refused["to append, a topic the set does not hold"] = AppendTo(appends, other, 0)
	_, refused["to append, the name with another schema"] = AppendTo(appends, numbers, 0)
	_, refused["to append, a partition the topic has not"] = AppendTo(appends, topic, 2)
	_, refused["to read, of the set of no topic"] = ReadFrom(ReadSet{}, topic, 0)
	_, refused["to append, of the set of no topic"] = AppendTo(AppendSet{}, topic, 0)
	for what, err := range refused {
		if !errors.Is(err, ErrNotGiven) {
			t.Errorf("%s: given, or refused for another reason: %v", what, err)
		}
	}
	if _, err := NewReadSet(logs, logs); !errors.Is(err, ErrSameName) {
		t.Errorf("a set of two topics of one name: %v", err)
	}
	if _, err := NewAppendSet(TopicLogs{}); err == nil {
		t.Error("a set is made of the zero TopicLogs, which is no topic")
	}

	// a partition's log is its own: one log is not two partitions, of one topic or of two
	if _, err := topic.Logs(one, one); !errors.Is(err, ErrSameLog) {
		t.Errorf("a topic given one log for its two partitions: %v", err)
	}
	single, err := NewTopic[string, bar]("single", 1)
	if err != nil {
		t.Fatal(err)
	}
	another, err := NewTopic[string, bar]("another", 1)
	if err != nil {
		t.Fatal(err)
	}
	shared := logOf[Record[string, bar]]()
	singleLogs, err := single.Logs(shared)
	if err != nil {
		t.Fatal(err)
	}
	anotherLogs, err := another.Logs(shared)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewReadSet(singleLogs, anotherLogs); !errors.Is(err, ErrSameLog) {
		t.Errorf("a set to read of two topics on one log: %v", err)
	}
	if _, err := NewAppendSet(singleLogs, anotherLogs); !errors.Is(err, ErrSameLog) {
		t.Errorf("a set to append to of two topics on one log: %v", err)
	}
	if _, err := NewAppendSet(logs, singleLogs); err != nil {
		t.Errorf("a set of two topics, each on logs of its own: %v", err)
	}

	// what is given reads the partition's log from its first record, and appends at its end
	to, err := AppendTo(appends, topic, 1)
	if err != nil {
		t.Fatal(err)
	}
	for n := range 3 {
		if err := to.Append(within(t), barRecord(bar{N: n}, n)); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		from, err := ReadFrom(reads, topic, 1)
		if err != nil {
			t.Fatal(err)
		}
		for n := range 3 {
			r, err := from.Next(within(t))
			if err != nil {
				t.Fatal(err)
			}
			if r.Value.N != n {
				t.Errorf("record %d of the partition is bar %d", n, r.Value.N)
			}
		}
	}
	sameEmpty(t, "the partition nothing was appended to", one.all())
}

// given is what the parts of a run were given, as each saw it, and what any of them could do beyond it.
type given struct {
	mu     sync.Mutex
	topics map[string][]string
	more   []string
}

func (g *given) note(part string, topics []AnyTopic) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.topics[part] = topicNames(topics)
}

func (g *given) could(what string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.more = append(g.more, what)
}

// onlyAppends notes it where a writing end is more than one: where it can be read too.
func onlyAppends[T any](g *given, part string, end Appender[T]) {
	if _, reads := end.(interface{ Read() Cursor[T] }); reads {
		g.could(part + " can read what it is given to append to")
	}
}

// onlyReads notes it where a reading end is more than one: where it can be appended to.
func onlyReads[T any](g *given, part string, end Cursor[T]) {
	if _, appends := end.(Appender[T]); appends {
		g.could(part + " can append to what it is given to read")
	}
	if _, log := end.(*memoryCursor[T]); log {
		g.could(part + " is given the log's own cursor")
	}
}

// The parts of the fixture, each noting what it is given before it does its work.
type (
	notingReader struct {
		reader
		g *given
	}
	notingCleaner struct {
		cleaner
		g *given
	}
	notingRaw struct {
		rawPublisher
		g *given
		f *fixture
	}
	notingClean struct {
		cleanPublisher
		g *given
		f *fixture
	}
	notingCorrections struct {
		correctionsPublisher
		g *given
		f *fixture
	}
	notingWorker struct {
		worker
		g *given
		f *fixture
	}
	notingChart struct {
		chart
		g *given
		f *fixture
	}
	countingConsumer[K, V any, P comparable] struct {
		consumer[K, V, P]
		calls *atomic.Int32
		g     *given
	}
)

func (p notingReader) Read(ctx context.Context, source Cursor[page], raw Appender[bar]) error {
	onlyReads(p.g, "the reader", source)
	onlyAppends(p.g, "the reader", raw)
	return p.reader.Read(ctx, source, raw)
}

func (p notingCleaner) Clean(ctx context.Context, raw Cursor[bar], clean Appender[bar]) error {
	onlyReads(p.g, "the cleaner", raw)
	onlyAppends(p.g, "the cleaner", clean)
	return p.cleaner.Clean(ctx, raw, clean)
}

// notGiven notes it where a part can append to a topic that is not of its kind.
func notGiven[K, V any](g *given, part string, topics AppendSet, other Topic[K, V]) {
	if _, err := AppendTo(topics, other, 0); !errors.Is(err, ErrNotGiven) {
		g.could(part + " can append to topic " + other.Name())
	}
}

func (p notingRaw) PublishRaw(ctx context.Context, raw Cursor[bar], topics AppendSet) error {
	p.g.note("the raw publisher", topics.Topics())
	notGiven(p.g, "the raw publisher", topics, p.f.cleanBars.topic)
	notGiven(p.g, "the raw publisher", topics, p.f.fixes.topic)
	notGiven(p.g, "the raw publisher", topics, p.f.model.topic)
	if to, err := AppendTo(topics, p.bars, 0); err == nil {
		onlyAppends(p.g, "the raw publisher", to)
	}
	return p.rawPublisher.PublishRaw(ctx, raw, topics)
}

func (p notingClean) PublishClean(ctx context.Context, clean Cursor[bar], topics AppendSet) error {
	p.g.note("the clean publisher", topics.Topics())
	notGiven(p.g, "the clean publisher", topics, p.f.rawBars.topic)
	notGiven(p.g, "the clean publisher", topics, p.f.fixes.topic)
	notGiven(p.g, "the clean publisher", topics, p.f.model.topic)
	return p.cleanPublisher.PublishClean(ctx, clean, topics)
}

func (p notingCorrections) PublishCorrections(ctx context.Context, corrections Cursor[fix], topics AppendSet) error {
	p.g.note("the corrections publisher", topics.Topics())
	onlyReads(p.g, "the corrections publisher", corrections)
	notGiven(p.g, "the corrections publisher", topics, p.f.rawBars.topic)
	notGiven(p.g, "the corrections publisher", topics, p.f.cleanBars.topic)
	notGiven(p.g, "the corrections publisher", topics, p.f.model.topic)
	return p.correctionsPublisher.PublishCorrections(ctx, corrections, topics)
}

func (p notingWorker) Work(ctx context.Context, clean ReadSet, own AppendSet) error {
	p.g.note("the worker, to read", clean.Topics())
	p.g.note("the worker, to append to", own.Topics())
	notGiven(p.g, "the worker", own, p.f.cleanBars.topic)
	notGiven(p.g, "the worker", own, p.f.rawBars.topic)
	if from, err := ReadFrom(clean, p.f.cleanBars.topic, 0); err == nil {
		onlyReads(p.g, "the worker", from)
	}
	return p.worker.Work(ctx, clean, own)
}

func (p notingChart) Draw(ctx context.Context, clean ReadSet, model ReadSet, updates Appender[string]) error {
	p.g.note("the chart, clean", clean.Topics())
	p.g.note("the chart, the engine's", model.Topics())
	onlyAppends(p.g, "the chart", updates)
	return p.chart.Draw(ctx, clean, model, updates)
}

func (c countingConsumer[K, V, P]) Consume(ctx context.Context, partition Cursor[Record[K, V]], pushes Appender[P]) error {
	c.calls.Add(1)
	onlyReads(c.g, "the consumer", partition)
	return c.consumer.Consume(ctx, partition, pushes)
}

// What each part of a working space is given: each publisher the topics of its kind alone; the engine clean topics
// only, those the wiring names, and its own to write; visualization its clean topics and all of the engine's; the
// consumer one partition a call, every partition of every topic of acquisition's; and of each, the one end.
func TestGiven(t *testing.T) {
	f := newFixture(t)
	g := &given{topics: map[string][]string{}}
	calls := &atomic.Int32{}
	w := f.wiring(t, true)
	w.Acquisition = Acquisition[page, bar, fix]{
		Reader:             notingReader{g: g},
		Cleaner:            notingCleaner{g: g},
		PublishRaw:         notingRaw{rawPublisher: f.rawPublisher(), g: g, f: f},
		PublishClean:       notingClean{cleanPublisher: f.cleanPublisher(), g: g, f: f},
		PublishCorrections: notingCorrections{correctionsPublisher: f.correctionsPublisher(), g: g, f: f},
	}
	var err error
	w.Warehousing, err = NewWarehousing(
		ConsumerOf(countingConsumer[string, bar, string]{consumer: barConsumer, calls: calls, g: g}, f.texts),
		ConsumerOf(countingConsumer[int, string, int]{consumer: countConsumer, calls: calls, g: g}, f.numbers),
		ConsumerOf(countingConsumer[string, fix, string]{consumer: fixConsumer, calls: calls, g: g}, f.texts),
	)
	if err != nil {
		t.Fatal(err)
	}
	w.Engine.Worker = notingWorker{worker: f.worker(), g: g, f: f}
	w.Chart = notingChart{chart: f.chart(), g: g, f: f}
	space, err := w.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	if err := space.Run(within(t), inputs(pages, fixes), f.views); err != nil {
		t.Fatal(err)
	}

	want := map[string][]string{
		"the raw publisher":         {"raw.bars", "raw.count"},
		"the clean publisher":       {"clean.bars", "clean.odd"},
		"the corrections publisher": {"corrections"},
		"the worker, to read":       {"clean.bars"},
		"the worker, to append to":  {"model"},
		"the chart, clean":          {"clean.bars", "clean.odd"},
		"the chart, the engine's":   {"model"},
	}
	for part, topics := range want {
		if !slices.Equal(g.topics[part], topics) {
			t.Errorf("%s is given %v, and should be given %v", part, g.topics[part], topics)
		}
	}
	if len(g.topics) != len(want) {
		t.Errorf("the parts that noted their topics: %v", g.topics)
	}
	for _, more := range g.more {
		t.Error(more)
	}
	// raw.bars 2, raw.count 1, clean.bars 2, clean.odd 1, corrections 1
	if n := calls.Load(); n != 7 {
		t.Errorf("the consumers were called %d times, and acquisition's topics have 7 partitions", n)
	}
	for topic, table := range tables {
		if got, ok := space.Table(topic); !ok || got != table {
			t.Errorf("the table of topic %q is %q, %v", topic, got, ok)
		}
	}
	if _, ok := space.Table("model"); ok {
		t.Error("the engine's topic has a table")
	}
}

// The two sets a part is given are two types, and no conversion makes one of the other: a part given topics to read
// cannot make of them topics to append to.
func TestSetsApart(t *testing.T) {
	read, appendTo := reflect.TypeFor[ReadSet](), reflect.TypeFor[AppendSet]()
	if read.ConvertibleTo(appendTo) || appendTo.ConvertibleTo(read) {
		t.Error("a ReadSet and an AppendSet convert into each other")
	}
}

// What a wiring is refused for, when its working space is made.
func TestRefused(t *testing.T) {
	type wiring = Wiring[page, bar, fix, string]
	refused := map[string]struct {
		change func(f *fixture, w *wiring)
		why    error
	}{
		"no reader":                      {change: func(f *fixture, w *wiring) { w.Acquisition.Reader = nil }},
		"no cleaner":                     {change: func(f *fixture, w *wiring) { w.Acquisition.Cleaner = nil }},
		"no publisher of the raw":        {change: func(f *fixture, w *wiring) { w.Acquisition.PublishRaw = nil }},
		"no publisher of the clean":      {change: func(f *fixture, w *wiring) { w.Acquisition.PublishClean = nil }},
		"no publisher of corrections":    {change: func(f *fixture, w *wiring) { w.Acquisition.PublishCorrections = nil }},
		"no chart":                       {change: func(f *fixture, w *wiring) { w.Chart = nil }},
		"an engine with no worker":       {change: func(f *fixture, w *wiring) { w.Engine.Worker = nil }},
		"a topic that is none":           {change: func(f *fixture, w *wiring) { w.Topics.Raw = append(w.Topics.Raw, TopicLogs{}) }},
		"an engine's topic that is none": {change: func(f *fixture, w *wiring) { w.Engine.Topics = append(w.Engine.Topics, TopicLogs{}) }},
		"a topic with no table": {change: func(f *fixture, w *wiring) {
			w.Tables = map[string]string{"raw.bars": "bars", "raw.count": "counts", "clean.bars": "bars", "clean.odd": "odd"}
		}},
		"a table for no topic": {change: func(f *fixture, w *wiring) {
			w.Tables = map[string]string{"raw.bars": "bars", "raw.count": "counts", "clean.bars": "bars", "clean.odd": "odd", "corrections": "fixes", "others": "others"}
		}},
		"a table for the engine's topic": {change: func(f *fixture, w *wiring) {
			w.Tables = map[string]string{"raw.bars": "bars", "raw.count": "counts", "clean.bars": "bars", "clean.odd": "odd", "corrections": "fixes", "model": "models"}
		}},
		"a schema with no consumer": {change: func(f *fixture, w *wiring) {
			w.Warehousing, _ = NewWarehousing(ConsumerOf(barConsumer, f.texts), ConsumerOf(fixConsumer, f.texts))
		}},
		"no consumer at all":        {change: func(f *fixture, w *wiring) { w.Warehousing = Warehousing{} }},
		"the engine on a raw topic": {change: func(f *fixture, w *wiring) { w.Engine.Clean = []AnyTopic{f.rawBars.topic} }, why: ErrNotClean},
		"the engine on its own":     {change: func(f *fixture, w *wiring) { w.Engine.Clean = []AnyTopic{f.model.topic} }, why: ErrNotClean},
		"the engine on corrections": {change: func(f *fixture, w *wiring) { w.Engine.Clean = []AnyTopic{f.cleanBars.topic, f.fixes.topic} }, why: ErrNotClean},
		"the engine on no topic":    {change: func(f *fixture, w *wiring) { w.Engine.Clean = []AnyTopic{nil} }},
		"the chart on a raw topic":  {change: func(f *fixture, w *wiring) { w.Shown = []AnyTopic{f.cleanOdd.topic, f.rawCount.topic} }, why: ErrNotClean},
		"the chart on the engine's": {change: func(f *fixture, w *wiring) { w.Shown = []AnyTopic{f.model.topic} }, why: ErrNotClean},
		"the chart on a topic that is not acquisition's": {change: func(f *fixture, w *wiring) {
			w.Shown = []AnyTopic{newMemoryTopic[string, bar](t, "others", 1).topic}
		}, why: ErrNotClean},
		"the chart on a clean topic's name with another number of partitions": {change: func(f *fixture, w *wiring) {
			w.Shown = []AnyTopic{newMemoryTopic[string, bar](t, "clean.bars", 3).topic}
		}, why: ErrNotClean},
		"the chart on a clean topic's name with another schema": {change: func(f *fixture, w *wiring) {
			w.Shown = []AnyTopic{newMemoryTopic[int, int](t, "clean.bars", 2).topic}
		}, why: ErrNotClean},
		"a clean topic on a raw topic's log": {change: func(f *fixture, w *wiring) {
			onRaw, err := f.cleanOdd.topic.Logs(f.rawBars.partitions[0])
			if err != nil {
				t.Fatal(err)
			}
			w.Topics.Clean = []TopicLogs{f.cleanBars.logs, onRaw}
		}, why: ErrSameLog},
		"an engine's topic on a clean topic's log": {change: func(f *fixture, w *wiring) {
			onClean, err := newMemoryTopic[string, bar](t, "model.bars", 1).topic.Logs(f.cleanOdd.partitions[0])
			if err != nil {
				t.Fatal(err)
			}
			w.Engine.Topics = append(w.Engine.Topics, onClean)
		}, why: ErrSameLog},
	}
	for what, r := range refused {
		f := newFixture(t)
		w := f.wiring(t, true)
		r.change(f, &w)
		_, err := w.Workspace()
		if err == nil {
			t.Errorf("%s: the working space is made", what)
		} else if r.why != nil && !errors.Is(err, r.why) {
			t.Errorf("%s: refused for another reason: %v", what, err)
		}
	}

	// a clean topic named twice among those to read is given once
	f := newFixture(t)
	w := f.wiring(t, true)
	w.Shown = []AnyTopic{f.cleanOdd.topic, f.cleanOdd.topic}
	if _, err := w.Workspace(); err != nil {
		t.Errorf("a clean topic shown twice: %v", err)
	}

	// warehousing: one consumer for each schema, each with its database
	notMade := map[string][]SchemaConsumer{
		"two consumers of one schema": {ConsumerOf(barConsumer, f.texts), ConsumerOf(barConsumer, f.texts)},
		"a consumer that is none":     {ConsumerOf[string, bar, string](nil, f.texts)},
		"a database that is none":     {ConsumerOf[string, bar, string](barConsumer, nil)},
		"the zero SchemaConsumer":     {{}},
	}
	for what, consumers := range notMade {
		if _, err := NewWarehousing(consumers...); err == nil {
			t.Errorf("%s: warehousing is made", what)
		}
	}
	// and alone, it refuses a topic with no table or no consumer, and a name twice
	alone := f.warehousing(t)
	if err := alone.Run(within(t), f.acquired(), map[string]string{}); err == nil {
		t.Error("warehousing runs on topics with no table")
	}
	if err := (Warehousing{}).Run(within(t), f.acquired(), tables); err == nil {
		t.Error("warehousing runs with no consumer")
	}
	if err := alone.Run(within(t), []TopicLogs{f.rawBars.logs, f.rawBars.logs}, tables); !errors.Is(err, ErrSameName) {
		t.Errorf("warehousing on a topic twice: %v", err)
	}
	// acquisition alone refuses a part or an input that is missing
	if err := (Acquisition[page, bar, fix]{}).Run(within(t), inputs(pages, fixes), f.topics()); err == nil {
		t.Error("acquisition runs with no part")
	}
	if err := f.acquisition().Run(within(t), Inputs[page, fix]{Source: logOf(pages...).Read()}, f.topics()); err == nil {
		t.Error("acquisition runs with no corrections stream")
	}
	if err := f.acquisition().Run(within(t), Inputs[page, fix]{Corrections: logOf(fixes...).Read()}, f.topics()); err == nil {
		t.Error("acquisition runs with no source")
	}
	// and a log in two partitions, of two kinds: two publishers would write it
	onRaw, err := f.cleanOdd.topic.Logs(f.rawBars.partitions[1])
	if err != nil {
		t.Fatal(err)
	}
	twoKinds := f.topics()
	twoKinds.Clean = []TopicLogs{f.cleanBars.logs, onRaw}
	if err := f.acquisition().Run(within(t), inputs(pages, fixes), twoKinds); !errors.Is(err, ErrSameLog) {
		t.Errorf("acquisition on a log that is a raw and a clean topic's: %v", err)
	}
	// the two inputs are two logs: one cursor is not both, for acquisition alone or in a working space; two cursors
	// on one log are two, each reading the whole of it
	both := Acquisition[page, bar, page]{
		Reader: reader{}, Cleaner: cleaner{}, PublishRaw: f.rawPublisher(), PublishClean: f.cleanPublisher(), PublishCorrections: stopped{},
	}
	one := logOf(pages...).Read()
	if err := both.Run(within(t), Inputs[page, page]{Source: one, Corrections: one}, f.topics()); !errors.Is(err, ErrSameInput) {
		t.Errorf("acquisition on one cursor as its source and its corrections: %v", err)
	}
	space, err := Wiring[page, bar, page, string]{
		Topics: f.topics(), Acquisition: both, Warehousing: f.warehousing(t), Tables: tables,
		Shown: []AnyTopic{f.cleanBars.topic}, Chart: f.chart(),
	}.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	if err := space.Run(within(t), Inputs[page, page]{Source: one, Corrections: one}, f.views); !errors.Is(err, ErrSameInput) {
		t.Errorf("a working space on one cursor as its source and its corrections: %v", err)
	}
	sameEmpty(t, "a topic of runs that were refused", f.rawBars.partitions[0].all())
	log := logOf(pages...)
	if err := both.Run(within(t), Inputs[page, page]{Source: log.Read(), Corrections: log.Read()}, f.topics()); err != nil {
		t.Errorf("acquisition on two cursors of one log: %v", err)
	}
}

// A working space is run once: within one, nothing starts again.
func TestRunOnce(t *testing.T) {
	f, space := ranSpace(t, true, pages, fixes)
	before := f.rawCount.partitions[0].all()
	if err := space.Run(within(t), inputs(pages, fixes), f.views); err == nil {
		t.Error("a working space is run a second time")
	}
	if after := f.rawCount.partitions[0].all(); len(after) != len(before) {
		t.Errorf("the refused run wrote: %d records before it, %d after", len(before), len(after))
	}
	// a run refused for what it is given is no run: the working space can still be run
	g := newFixture(t)
	space, err := g.wiring(t, false).Workspace()
	if err != nil {
		t.Fatal(err)
	}
	if err := space.Run(within(t), inputs(pages, fixes), nil); err == nil {
		t.Error("a working space is run with nothing to append the chart to")
	}
	if err := space.Run(within(t), Inputs[page, fix]{}, g.views); err == nil {
		t.Error("a working space is run with no inputs")
	}
	if err := space.Run(within(t), inputs(pages, fixes), g.views); err != nil {
		t.Errorf("the run after the refused ones: %v", err)
	}

	// a copy of a working space is the same working space: run once between them
	h := newFixture(t)
	space, err = h.wiring(t, false).Workspace()
	if err != nil {
		t.Fatal(err)
	}
	copied := *space
	if err := copied.Run(within(t), inputs(pages, fixes), h.views); err != nil {
		t.Fatal(err)
	}
	if err := space.Run(within(t), inputs(pages, fixes), h.views); err == nil {
		t.Error("a working space is run after its copy was")
	}
	// and a working space no wiring made is none
	var none *Workspace[page, bar, fix, string]
	if err := none.Run(within(t), inputs(pages, fixes), h.views); err == nil {
		t.Error("a nil working space is run")
	}
	if err := (&Workspace[page, bar, fix, string]{}).Run(within(t), inputs(pages, fixes), h.views); err == nil {
		t.Error("a working space no wiring made is run")
	}
}

var errFails = errors.New("the part fails")

// failing is a cleaner that fails at its third bar.
type failing struct{}

func (failing) Clean(ctx context.Context, raw Cursor[bar], clean Appender[bar]) error {
	for range 2 {
		b, err := raw.Next(ctx)
		if err != nil {
			return err
		}
		if err := clean.Append(ctx, b); err != nil {
			return err
		}
	}
	return errFails
}

// A part that fails stops the run: every other part's context is done, and the run returns the part's error.
func TestPartFails(t *testing.T) {
	f := newFixture(t)
	w := f.wiring(t, true)
	w.Acquisition.Cleaner = failing{}
	space, err := w.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	err = space.Run(within(t), inputs(pages, fixes), f.views)
	if !errors.Is(err, errFails) || !strings.Contains(err.Error(), "the cleaner") {
		t.Errorf("the run of a failing cleaner: %v", err)
	}
}

// A run whose context is done returns, with the context's error, once every part has returned: the inputs here
// hold nothing and never say stop, so every part waits until then.
func TestCancel(t *testing.T) {
	f := newFixture(t)
	space, err := f.wiring(t, true).Workspace()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(within(t))
	done := make(chan error)
	go func() { done <- space.Run(ctx, inputs(nil, nil), f.views) }()
	select {
	case err := <-done:
		t.Fatalf("the run returned before its context was done: %v", err)
	default:
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("the run whose context is done: %v", err)
	}
}

// stopped is a part that returns at once, having read nothing.
type stopped struct{}

func (stopped) Clean(ctx context.Context, raw Cursor[bar], clean Appender[bar]) error { return nil }

func (stopped) PublishClean(ctx context.Context, clean Cursor[bar], topics AppendSet) error {
	return nil
}

func (stopped) PublishCorrections(ctx context.Context, corrections Cursor[page], topics AppendSet) error {
	return nil
}

// A part that has returned reads no more, and the part that writes what it read does not wait for it: with a
// cleaner that stops at once, the raw topics still hold the whole of the raw series.
func TestStopped(t *testing.T) {
	f := newFixture(t)
	a := f.acquisition()
	a.Cleaner, a.PublishClean = stopped{}, stopped{}
	if err := a.Run(within(t), inputs(pages, fixes), f.topics()); err != nil {
		t.Fatal(err)
	}
	want := ranAcquisition(t, pages, fixes)
	sameTopicLogs(t, f.rawBars, want.rawBars)
	sameTopicLogs(t, f.rawCount, want.rawCount)
	sameEmpty(t, "the clean topic of a cleaner that stopped", f.cleanBars.partitions[0].all())
}

// unread is a cleaner that reads nothing, gives nothing, and does not return until it is released: a process, the
// one that gives nothing whatever it is given.
type unread struct{ release <-chan struct{} }

func (u unread) Clean(ctx context.Context, raw Cursor[bar], clean Appender[bar]) error {
	select {
	case <-u.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// finishing is the raw publisher, saying when it has returned.
type finishing struct {
	rawPublisher
	done chan<- struct{}
}

func (p finishing) PublishRaw(ctx context.Context, raw Cursor[bar], topics AppendSet) error {
	defer close(p.done)
	return p.rawPublisher.PublishRaw(ctx, raw, topics)
}

// Acquisition.raw_topics holds for every cleaner, one that reads nothing among them: the raw topics hold the whole
// of what the publisher makes of the raw series while the cleaner still waits, having taken none of it. No part
// waits for another to read.
func TestUnread(t *testing.T) {
	f := newFixture(t)
	release, published := make(chan struct{}), make(chan struct{})
	a := f.acquisition()
	a.Cleaner, a.PublishClean = unread{release: release}, stopped{}
	a.PublishRaw = finishing{rawPublisher: f.rawPublisher(), done: published}
	ctx := within(t)
	ran := make(chan error, 1)
	go func() { ran <- a.Run(ctx, inputs(pages, fixes), f.topics()) }()
	select {
	case <-published:
	case err := <-ran:
		t.Fatalf("acquisition returned while its cleaner waited, the raw publisher not done: %v", err)
	}
	want := ranAcquisition(t, pages, fixes)
	sameTopicLogs(t, f.rawBars, want.rawBars)
	sameTopicLogs(t, f.rawCount, want.rawCount)
	close(release)
	if err := <-ran; err != nil {
		t.Fatal(err)
	}
}
