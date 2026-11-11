// Package taker takes the flake package abstract-level as any taker does (abstract/README.md, Taking it): make
// check-abstract-package copies this file into a module in a temporary folder whose go.mod requires the module and
// replaces its path with the package's store path. Its test uses what the package exports, and nothing else: it
// wires a working space of the four components on topics, a database and a chart in memory, runs it to its end and
// reads what each component wrote.
package taker

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/abstract"
)

// memory is a log in memory: every message appended to it, in order.
type memory[T any] struct {
	mu       sync.Mutex
	messages []T
	grown    chan struct{} // closed when the log grows, for the cursors that wait at its end
}

func logOf[T any](messages ...T) *memory[T] { return &memory[T]{messages: messages} }

func (l *memory[T]) Append(ctx context.Context, m T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, m)
	if l.grown != nil {
		close(l.grown)
		l.grown = nil
	}
	return nil
}

func (l *memory[T]) Read() abstract.Cursor[T] { return &cursor[T]{log: l} }

func (l *memory[T]) all() []T {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.messages)
}

// cursor reads a log in memory from its first message, waiting at its end.
type cursor[T any] struct {
	log  *memory[T]
	next int
}

func (c *cursor[T]) Next(ctx context.Context) (T, error) {
	for {
		c.log.mu.Lock()
		if c.next < len(c.log.messages) {
			m := c.log.messages[c.next]
			c.next++
			c.log.mu.Unlock()
			return m, nil
		}
		if c.log.grown == nil {
			c.log.grown = make(chan struct{})
		}
		grown := c.log.grown
		c.log.mu.Unlock()
		select {
		case <-grown:
		case <-ctx.Done():
			var none T
			return none, ctx.Err()
		}
	}
}

// A log has no end, and the test's inputs are finite: each ends with a message that says stop, which a part passes
// on to what it writes, as a record of no value, and returns.
const (
	stop     = -1
	stopping = "stop"
)

// reader gives each page's number as a k-line.
type reader struct{}

func (reader) Read(ctx context.Context, source abstract.Cursor[int], raw abstract.Appender[int]) error {
	for {
		n, err := source.Next(ctx)
		if err != nil {
			return err
		}
		if err := raw.Append(ctx, n); err != nil || n == stop {
			return err
		}
	}
}

// cleaner keeps the k-lines of an even number.
type cleaner struct{}

func (cleaner) Clean(ctx context.Context, raw abstract.Cursor[int], clean abstract.Appender[int]) error {
	for {
		n, err := raw.Next(ctx)
		if err != nil {
			return err
		}
		if n != stop && n%2 != 0 {
			continue
		}
		if err := clean.Append(ctx, n); err != nil || n == stop {
			return err
		}
	}
}

// publisher writes each k-line of its series to its topic's one partition: the raw series' or the clean series'.
type publisher struct{ topic abstract.Topic[int, int] }

func (p publisher) PublishRaw(ctx context.Context, raw abstract.Cursor[int], topics abstract.AppendSet) error {
	return p.publish(ctx, raw, topics)
}

func (p publisher) PublishClean(ctx context.Context, clean abstract.Cursor[int], topics abstract.AppendSet) error {
	return p.publish(ctx, clean, topics)
}

func (p publisher) publish(ctx context.Context, series abstract.Cursor[int], topics abstract.AppendSet) error {
	to, err := abstract.AppendTo(topics, p.topic, 0)
	if err != nil {
		return err
	}
	for {
		n, err := series.Next(ctx)
		if err != nil {
			return err
		}
		r := abstract.Record[int, int]{Value: &n}
		if n == stop {
			r.Value = nil
		}
		if err := to.Append(ctx, r); err != nil || n == stop {
			return err
		}
	}
}

// corrections writes each correction, as it comes, to its topic's one partition.
type corrections struct{ topic abstract.Topic[int, string] }

func (p corrections) PublishCorrections(ctx context.Context, in abstract.Cursor[string], topics abstract.AppendSet) error {
	to, err := abstract.AppendTo(topics, p.topic, 0)
	if err != nil {
		return err
	}
	for {
		c, err := in.Next(ctx)
		if err != nil {
			return err
		}
		r := abstract.Record[int, string]{Value: &c}
		if c == stopping {
			r.Value = nil
		}
		if err := to.Append(ctx, r); err != nil || c == stopping {
			return err
		}
	}
}

// consumer is the warehouse consumer of the schema of keys int and values V: it pushes each record's value as text.
type consumer[V any] struct{}

func (consumer[V]) Consume(ctx context.Context, partition abstract.Cursor[abstract.Record[int, V]], pushes abstract.Appender[string]) error {
	for {
		r, err := partition.Next(ctx)
		if err != nil {
			return err
		}
		if r.Value == nil {
			return nil
		}
		if err := pushes.Append(ctx, fmt.Sprint(*r.Value)); err != nil {
			return err
		}
	}
}

// database holds each log pushed into it, by its table, its topic and its partition. Warehousing pushes the
// partitions' logs at once, so it is safe for calls at once.
type database struct {
	mu   sync.Mutex
	logs map[string][]string
}

func (d *database) Pushes(table string, topic string, partition int) (abstract.Appender[string], error) {
	from := fmt.Sprintf("%s from %s/%d", table, topic, partition)
	return appender(func(ctx context.Context, m string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		d.logs[from] = append(d.logs[from], m)
		return nil
	}), nil
}

func (d *database) all() map[string][]string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return maps.Clone(d.logs)
}

type appender func(ctx context.Context, m string) error

func (f appender) Append(ctx context.Context, m string) error { return f(ctx, m) }

// worker is the engine's: the sum of the clean k-lines so far, to its own topic.
type worker struct{ clean, sums abstract.Topic[int, int] }

func (w worker) Work(ctx context.Context, clean abstract.ReadSet, own abstract.AppendSet) error {
	from, err := abstract.ReadFrom(clean, w.clean, 0)
	if err != nil {
		return err
	}
	to, err := abstract.AppendTo(own, w.sums, 0)
	if err != nil {
		return err
	}
	sum := 0
	for {
		r, err := from.Next(ctx)
		if err != nil {
			return err
		}
		if r.Value == nil {
			return to.Append(ctx, abstract.Record[int, int]{})
		}
		sum += *r.Value
		if err := to.Append(ctx, abstract.Record[int, int]{Value: new(sum)}); err != nil {
			return err
		}
	}
}

// chart draws its clean topic to its stop, then the engine's sums to theirs.
type chart struct{ clean, sums abstract.Topic[int, int] }

func (c chart) Draw(ctx context.Context, clean abstract.ReadSet, model abstract.ReadSet, updates abstract.Appender[string]) error {
	for _, read := range []struct {
		what  string
		set   abstract.ReadSet
		topic abstract.Topic[int, int]
	}{{"clean", clean, c.clean}, {"sum", model, c.sums}} {
		from, err := abstract.ReadFrom(read.set, read.topic, 0)
		if err != nil {
			return err
		}
		for {
			r, err := from.Next(ctx)
			if err != nil {
				return err
			}
			if r.Value == nil {
				break
			}
			if err := updates.Append(ctx, fmt.Sprintf("%s %d", read.what, *r.Value)); err != nil {
				return err
			}
		}
	}
	return nil
}

// topic is a topic of one partition, its log in memory.
type topic[V any] struct {
	topic abstract.Topic[int, V]
	log   *memory[abstract.Record[int, V]]
	logs  abstract.TopicLogs
}

func topicOf[V any](t *testing.T, name string) topic[V] {
	t.Helper()
	described, err := abstract.NewTopic[int, V](name, 1)
	if err != nil {
		t.Fatal(err)
	}
	log := logOf[abstract.Record[int, V]]()
	logs, err := described.Logs(log)
	if err != nil {
		t.Fatal(err)
	}
	return topic[V]{topic: described, log: log, logs: logs}
}

// values gives what the topic holds, a record by its value, "none" for a record of no value.
func (tp topic[V]) values() []string {
	var vs []string
	for _, r := range tp.log.all() {
		if r.Value == nil {
			vs = append(vs, "none")
			continue
		}
		vs = append(vs, fmt.Sprint(*r.Value))
	}
	return vs
}

// A working space of the four components, wired from what the package exports, made and run to its end: each topic,
// each table and the chart hold what its writer made of what it was given.
func TestWorkingSpace(t *testing.T) {
	raw, clean := topicOf[int](t, "raw"), topicOf[int](t, "clean")
	fixes, sums := topicOf[string](t, "corrections"), topicOf[int](t, "sums")
	db := &database{logs: map[string][]string{}}
	warehousing, err := abstract.NewWarehousing(abstract.ConsumerOf(consumer[int]{}, db), abstract.ConsumerOf(consumer[string]{}, db))
	if err != nil {
		t.Fatal(err)
	}
	wiring := abstract.Wiring[int, int, string, string]{
		Topics: abstract.AllTopics{Raw: []abstract.TopicLogs{raw.logs}, Clean: []abstract.TopicLogs{clean.logs}, Corrections: []abstract.TopicLogs{fixes.logs}},
		Acquisition: abstract.Acquisition[int, int, string]{
			Reader: reader{}, Cleaner: cleaner{}, PublishRaw: publisher{raw.topic}, PublishClean: publisher{clean.topic}, PublishCorrections: corrections{fixes.topic},
		},
		Warehousing: warehousing,
		Tables:      map[string]string{"raw": "klines", "clean": "klines", "corrections": "corrections"},
		Engine:      &abstract.Engine{Clean: []abstract.AnyTopic{clean.topic}, Topics: []abstract.TopicLogs{sums.logs}, Worker: worker{clean: clean.topic, sums: sums.topic}},
		Shown:       []abstract.AnyTopic{clean.topic},
		Chart:       chart{clean: clean.topic, sums: sums.topic},
	}
	space, err := wiring.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	views := logOf[string]()
	in := abstract.Inputs[int, string]{Source: logOf(1, 2, 3, 4, stop).Read(), Corrections: logOf("split", stopping).Read()}
	if err := space.Run(ctx, in, views); err != nil {
		t.Fatal(err)
	}

	for _, held := range []struct {
		what      string
		got, want []string
	}{
		{"the raw topic", raw.values(), []string{"1", "2", "3", "4", "none"}},
		{"the clean topic", clean.values(), []string{"2", "4", "none"}},
		{"the corrections topic", fixes.values(), []string{"split", "none"}},
		{"the engine's topic", sums.values(), []string{"2", "6", "none"}},
		{"the chart", views.all(), []string{"clean 2", "clean 4", "sum 2", "sum 6"}},
	} {
		if !slices.Equal(held.got, held.want) {
			t.Errorf("%s holds %q, and should hold %q", held.what, held.got, held.want)
		}
	}
	want := map[string][]string{
		"klines from raw/0":              {"1", "2", "3", "4"},
		"klines from clean/0":            {"2", "4"},
		"corrections from corrections/0": {"split"},
	}
	if got := db.all(); !maps.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Errorf("the database holds %q, and should hold %q", got, want)
	}

	// a wiring that uses a name twice is refused when its working space is made
	engine := *wiring.Engine
	engine.Topics = append(slices.Clone(engine.Topics), topicOf[int](t, "raw").logs)
	wiring.Engine = &engine
	if _, err := wiring.Workspace(); !errors.Is(err, abstract.ErrSameName) {
		t.Errorf("a wiring with two topics named raw: %v", err)
	}
}
