package abstract

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"
)

// The tests of this file hold what the definition's theorems state, each on a run: the left side is what a run of
// acquisition, of warehousing or of a working space wrote; the right side is made by calling the parts themselves,
// one on the other's output, as the theorem composes them.

// consumed gives what a consumer pushes for one partition: the consumer on that partition alone.
func consumed[K, V any, P comparable](t *testing.T, c Consumer[K, V, P], partition *memoryLog[Record[K, V]]) []P {
	t.Helper()
	pushes := logOf[P]()
	if err := c.Consume(within(t), partition.Read(), pushes); err != nil {
		t.Fatal(err)
	}
	return pushes.all()
}

// logsPushed fails the test unless, for every partition of the topic, the log the database was pushed is what the
// consumer gives on that partition of the topic.
func logsPushed[K, V any, P comparable](t *testing.T, db *memoryDB[P], c Consumer[K, V, P], topic memoryTopic[K, V]) {
	t.Helper()
	name := topic.topic.Name()
	for p, partition := range topic.partitions {
		got := db.log(pushed{table: tables[name], topic: name, partition: p})
		sameLog(t, fmt.Sprintf("the pushes for partition %d of topic %q", p, name), got, consumed(t, c, partition))
	}
}

// pushesAre: every log pushed into got's databases is the consumer of its topic's schema on that partition of on's
// topics, for every partition of every one of acquisition's topics.
func pushesAre(t *testing.T, got, on *fixture) {
	t.Helper()
	logsPushed(t, got.texts, barConsumer, on.rawBars)
	logsPushed(t, got.numbers, countConsumer, on.rawCount)
	logsPushed(t, got.texts, barConsumer, on.cleanBars)
	logsPushed(t, got.texts, barConsumer, on.cleanOdd)
	logsPushed(t, got.texts, fixConsumer, on.fixes)
	if n := len(on.acquired()); n != 5 {
		t.Errorf("acquisition has %d topics, and the pushes of 5 were compared", n)
	}
}

// rowsHeld fails the test unless, for every partition of the topic, the topic's table holds from that partition
// exactly the messages the consumer pushes for it, in no order.
func rowsHeld[K, V any, P comparable](t *testing.T, db *memoryDB[P], c Consumer[K, V, P], topic memoryTopic[K, V]) {
	t.Helper()
	name := topic.topic.Name()
	for p, partition := range topic.partitions {
		want := map[P]bool{}
		for _, m := range consumed(t, c, partition) {
			want[m] = true
		}
		if len(want) == 0 {
			t.Errorf("partition %d of topic %q: nothing to compare: the consumer pushes nothing for it", p, name)
		}
		got := db.rowsOf(tables[name], pushed{table: tables[name], topic: name, partition: p})
		if !maps.Equal(got, want) {
			t.Errorf("table %q, the rows from partition %d of topic %q:\n got  %v\n want %v", tables[name], p, name, got, want)
		}
	}
}

// Acquisition.raw_topics: the raw topics hold what the publisher makes of the raw series alone, the reader's from
// the source: whatever the corrections are.
func TestRawTopics(t *testing.T) {
	for _, fixes := range [][]fix{fixes, otherFixes} {
		got := ranAcquisition(t, pages, fixes)

		want := newFixture(t)
		raw := logOf[bar]()
		if err := (reader{}).Read(within(t), logOf(pages...).Read(), raw); err != nil {
			t.Fatal(err)
		}
		if err := want.rawPublisher().PublishRaw(within(t), raw.Read(), appendSet(t, want.topics().Raw...)); err != nil {
			t.Fatal(err)
		}
		sameTopicLogs(t, got.rawBars, want.rawBars)
		sameTopicLogs(t, got.rawCount, want.rawCount)
	}
}

// cleanTopicsOf gives, on a new fixture, what the publisher makes of the clean series, the cleaner's on the raw
// series, the reader's from the source.
func cleanTopicsOf(t *testing.T, pages []page) *fixture {
	t.Helper()
	want := newFixture(t)
	raw, clean := logOf[bar](), logOf[bar]()
	if err := (reader{}).Read(within(t), logOf(pages...).Read(), raw); err != nil {
		t.Fatal(err)
	}
	if err := (cleaner{}).Clean(within(t), raw.Read(), clean); err != nil {
		t.Fatal(err)
	}
	if err := want.cleanPublisher().PublishClean(within(t), clean.Read(), appendSet(t, want.topics().Clean...)); err != nil {
		t.Fatal(err)
	}
	return want
}

// Acquisition.clean_topics: the clean topics hold what the publisher makes of the clean series alone, the cleaner's
// on the raw series: whatever the corrections are.
func TestCleanTopics(t *testing.T) {
	for _, fixes := range [][]fix{fixes, otherFixes} {
		got := ranAcquisition(t, pages, fixes)
		want := cleanTopicsOf(t, pages)
		sameTopicLogs(t, got.cleanBars, want.cleanBars)
		sameTopicLogs(t, got.cleanOdd, want.cleanOdd)
	}
}

// Acquisition.corrections_topics: the corrections topics hold what the publisher makes of the vendor's corrections
// alone: whatever the source is.
func TestCorrectionsTopics(t *testing.T) {
	for _, pages := range [][]page{pages, otherPages} {
		got := ranAcquisition(t, pages, fixes)

		want := newFixture(t)
		if err := want.correctionsPublisher().PublishCorrections(within(t), logOf(fixes...).Read(), appendSet(t, want.topics().Corrections...)); err != nil {
			t.Fatal(err)
		}
		sameTopicLogs(t, got.fixes, want.fixes)
	}
}

// Warehousing.run_apply: partition p of topic t goes through the consumer of t's schema.
func TestRunApply(t *testing.T) {
	x := ranAcquisition(t, pages, fixes)
	if err := x.warehousing(t).Run(within(t), x.acquired(), tables); err != nil {
		t.Fatal(err)
	}
	pushesAre(t, x, x)
}

// Wiring.acquisition_topics, and Workspace.acquisition_topics of the working space the wiring makes: acquisition's
// topics hold what acquisition writes from the inputs alone, with the engine or without it.
func TestAcquisitionTopics(t *testing.T) {
	for _, engine := range []bool{true, false} {
		got, _ := ranSpace(t, engine, pages, fixes)
		want := ranAcquisition(t, pages, fixes)
		sameTopicLogs(t, got.rawBars, want.rawBars)
		sameTopicLogs(t, got.rawCount, want.rawCount)
		sameTopicLogs(t, got.cleanBars, want.cleanBars)
		sameTopicLogs(t, got.cleanOdd, want.cleanOdd)
		sameTopicLogs(t, got.fixes, want.fixes)
	}
}

// Wiring.clean_topics: in the working space, the clean topics hold what the publisher makes of the clean series
// alone: the cleaner on the raw series, the reader's from the source.
func TestWiringCleanTopics(t *testing.T) {
	got, _ := ranSpace(t, true, pages, fixes)
	want := cleanTopicsOf(t, pages)
	sameTopicLogs(t, got.cleanBars, want.cleanBars)
	sameTopicLogs(t, got.cleanOdd, want.cleanOdd)
}

// modelOf gives, on a new fixture, what the engine's worker writes from acquisition's topics: from its clean
// topics, and no others.
func modelOf(t *testing.T, acquired *fixture) *fixture {
	t.Helper()
	want := newFixture(t)
	if err := want.worker().Work(within(t), readSet(t, acquired.cleanBars.logs), appendSet(t, want.model.logs)); err != nil {
		t.Fatal(err)
	}
	return want
}

// Wiring.engine_topics: the engine's topics hold what the engine writes from acquisition's topics: with the engine,
// what its worker writes from its clean topics; without it, nothing, there being no such topic.
func TestEngineTopics(t *testing.T) {
	got, space := ranSpace(t, true, pages, fixes)
	sameTopicLogs(t, got.model, modelOf(t, ranAcquisition(t, pages, fixes)).model)
	if names := topicNames(space.Topics(ComponentMarketModelEngine)); !slices.Equal(names, []string{"model"}) {
		t.Errorf("the engine's topics are %v, and should be the one it writes", names)
	}

	got, space = ranSpace(t, false, pages, fixes)
	if topics := space.Topics(ComponentMarketModelEngine); len(topics) != 0 {
		t.Errorf("without the engine, the engine's topics are %v, and there should be none", topicNames(topics))
	}
	sameEmpty(t, "without the engine, the topic the engine would write", got.model.partitions[0].all())
}

// Wiring.warehousing_pushes: warehousing pushes, for partition p of acquisition's topic t, the consumer of t's
// schema on that partition of what acquisition writes.
func TestWarehousingPushes(t *testing.T) {
	got, _ := ranSpace(t, true, pages, fixes)
	pushesAre(t, got, ranAcquisition(t, pages, fixes))
}

// Wiring.table_rows: the table named name holds, for partition p of a topic t pushed into it, exactly the messages
// the consumer of t's schema pushes for that partition of what acquisition writes, in no order. A message the
// consumer pushes twice is one row; a table two topics are pushed into holds the rows of both.
func TestTableRows(t *testing.T) {
	got, _ := ranSpace(t, true, pages, fixes)
	on := ranAcquisition(t, pages, fixes)
	rowsHeld(t, got.texts, barConsumer, on.rawBars)
	rowsHeld(t, got.numbers, countConsumer, on.rawCount)
	rowsHeld(t, got.texts, barConsumer, on.cleanBars)
	rowsHeld(t, got.texts, barConsumer, on.cleanOdd)
	rowsHeld(t, got.texts, fixConsumer, on.fixes)

	// and a table holds rows of the logs pushed into it alone: those of the partitions of its own topics
	into := map[string][]pushed{}
	for _, table := range slices.Sorted(maps.Values(tables)) {
		into[table] = slices.Concat(got.texts.logsInto(table), got.numbers.logsInto(table))
	}
	for table, logs := range into {
		for _, from := range logs {
			if tables[from.topic] != table {
				t.Errorf("table %q holds rows from partition %d of topic %q, whose table is %q", table, from.partition, from.topic, tables[from.topic])
			}
		}
	}
	// the raw and the clean bars go into one table: both topics' partitions, four logs
	if n := len(into["bars"]); n != 4 {
		t.Errorf("table %q holds rows from %d logs, and two topics of two partitions are pushed into it", "bars", n)
	}
}

// chartOf gives the updates of the chart drawn from those clean topics and those of the engine's.
func chartOf(t *testing.T, c chart, clean, model ReadSet) []string {
	t.Helper()
	updates := logOf[string]()
	if err := c.Draw(within(t), clean, model, updates); err != nil {
		t.Fatal(err)
	}
	return updates.all()
}

// Wiring.visualization_chart: visualization's chart is drawn from its clean topics, of what acquisition writes, and
// the engine's topics, all of them, of what the engine writes from acquisition's.
func TestVisualizationChart(t *testing.T) {
	got, _ := ranSpace(t, true, pages, fixes)
	acquired := ranAcquisition(t, pages, fixes)
	model := modelOf(t, acquired)
	want := chartOf(t, got.chart(), readSet(t, acquired.cleanBars.logs, acquired.cleanOdd.logs), readSet(t, model.model.logs))
	sameLog(t, "the chart", got.views.all(), want)
	if first := "2 clean topics, 1 of the engine's"; !slices.Contains(got.views.all()[:min(1, len(got.views.all()))], first) {
		t.Errorf("the chart's updates begin %q, and should begin with %q", got.views.all(), first)
	}
}

// Wiring.chart_without_engine: without the engine, the chart is drawn from its clean topics alone: it is the same
// whatever stands for the engine's topics, of which there are none.
func TestChartWithoutEngine(t *testing.T) {
	got, _ := ranSpace(t, false, pages, fixes)
	acquired := ranAcquisition(t, pages, fixes)
	clean := readSet(t, acquired.cleanBars.logs, acquired.cleanOdd.logs)
	// what can stand for the engine's topics where there are none: a set of no topic, however it is made
	for _, none := range []ReadSet{{}, readSet(t)} {
		sameLog(t, "the chart without the engine", got.views.all(), chartOf(t, got.chart(), clean, none))
	}
	if first := "2 clean topics, 0 of the engine's"; !slices.Contains(got.views.all()[:min(1, len(got.views.all()))], first) {
		t.Errorf("the chart's updates begin %q, and should begin with %q", got.views.all(), first)
	}
}

func topicNames(topics []AnyTopic) []string {
	names := make([]string, len(topics))
	for i, topic := range topics {
		names[i] = topic.Name()
	}
	return names
}

// Wiring.names_distinct, and what it rests on (Wiring.acq_names, Wiring.Engine.names_distinct, Wiring.cross_names,
// Wiring.engineTopic_names): no two topics of the working space have the same name, and a wiring in which a name is
// used twice is refused when its working space is made.
func TestNamesDistinct(t *testing.T) {
	f := newFixture(t)
	space, err := f.wiring(t, true).Workspace()
	if err != nil {
		t.Fatal(err)
	}
	writer := map[string]Component{}
	for _, c := range []Component{ComponentAcquisition, ComponentWarehousing, ComponentVisualization, ComponentMarketModelEngine} {
		for _, name := range topicNames(space.Topics(c)) {
			if other, twice := writer[name]; twice {
				t.Errorf("two topics named %q, of %v and of %v", name, other, c)
			}
			writer[name] = c
		}
	}
	want := map[string]Component{
		"raw.bars": ComponentAcquisition, "raw.count": ComponentAcquisition, "clean.bars": ComponentAcquisition,
		"clean.odd": ComponentAcquisition, "corrections": ComponentAcquisition, "model": ComponentMarketModelEngine,
	}
	if !maps.Equal(writer, want) {
		t.Errorf("the topics and their writers are %v, and should be %v", writer, want)
	}

	// a name used twice: a topic of that name with logs of its own, put where the wiring takes a topic
	again := func(name string) TopicLogs { return newMemoryTopic[string, bar](t, name, 1).logs }
	twice := map[string]func(w *Wiring[page, bar, fix, string]){
		"two of acquisition's topics of one kind": func(w *Wiring[page, bar, fix, string]) {
			w.Topics.Raw = append(w.Topics.Raw, again("raw.bars"))
		},
		"two of acquisition's topics of two kinds": func(w *Wiring[page, bar, fix, string]) {
			w.Topics.Corrections = append(w.Topics.Corrections, again("clean.odd"))
		},
		"two of the engine's topics": func(w *Wiring[page, bar, fix, string]) {
			w.Engine.Topics = append(w.Engine.Topics, again("model"))
		},
		"one of acquisition's topics and one of the engine's": func(w *Wiring[page, bar, fix, string]) {
			w.Engine.Topics = append(w.Engine.Topics, again("raw.count"))
		},
	}
	for what, use := range twice {
		w := newFixture(t).wiring(t, true)
		use(&w)
		if _, err := w.Workspace(); !errors.Is(err, ErrSameName) {
			t.Errorf("%s: the working space is made, or refused for another reason: %v", what, err)
		}
	}
}
