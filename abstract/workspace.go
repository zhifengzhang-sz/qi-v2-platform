package abstract

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync/atomic"
)

// Component is one of the four components of a working space.
type Component int

const (
	ComponentAcquisition Component = iota
	ComponentWarehousing
	ComponentVisualization
	// ComponentMarketModelEngine is optional: a working space may have none.
	ComponentMarketModelEngine
)

func (c Component) String() string {
	switch c {
	case ComponentAcquisition:
		return "acquisition"
	case ComponentWarehousing:
		return "warehousing"
	case ComponentVisualization:
		return "visualization"
	case ComponentMarketModelEngine:
		return "the market model engine"
	}
	return fmt.Sprintf("Component(%d)", int(c))
}

// Wiring is the components of a working space and how they connect. Src, Kline and Corr are what acquisition works
// on, which its k-line contract defines (Acquisition); View is one update of the chart.
type Wiring[Src, Kline, Corr, View any] struct {
	// Topics is acquisition's topics, of the three kinds, each with its logs: no two of one name.
	Topics AllTopics
	// Acquisition is the working space's one acquisition.
	Acquisition Acquisition[Src, Kline, Corr]
	// Warehousing is on all of acquisition's topics: it has the consumer of each one's schema.
	Warehousing Warehousing
	// Tables names, by each of acquisition's topics' names, the table of the database the topic is pushed into,
	// every partition of it; several topics may go into one table. Every topic of acquisition's has one, and a
	// name that is no topic of acquisition's is refused.
	Tables map[string]string
	// Engine is the market model engine, if the working space has one; nil where it has none.
	Engine *Engine
	// Shown is the topics visualization reads beside the engine's: each one of acquisition's clean topics.
	Shown []AnyTopic
	// Chart is visualization, on those clean topics and all of the engine's.
	Chart Chart[View]
}

// Workspace is the working space a wiring makes (Wiring.Workspace): acquisition, warehousing, visualization and,
// where the wiring has one, the market model engine, wired by the topics. It is run once (Run).
type Workspace[Src, Kline, Corr, View any] struct {
	acquisition Acquisition[Src, Kline, Corr]
	// acquisition's topics: to append to, by kind, and all of them, raw then clean then corrections, to read
	published [len(kinds)]AppendSet
	acquired  []TopicLogs
	// warehousing, and the table of each of acquisition's topics, by the topic's name
	warehousing Warehousing
	tables      map[string]string
	// the engine: nil worker where the working space has none; what it reads and what it writes
	worker      Worker
	engineClean ReadSet
	engineOwn   AppendSet
	// visualization: its clean topics, and all of the engine's
	chart Chart[View]
	shown ReadSet
	model ReadSet

	// whether it has been run: one for every copy of a working space, and nil for one no wiring made
	ran *atomic.Bool
}

// Workspace makes the working space of the wiring, or refuses it: for a part that is missing; for two topics of one
// name, of acquisition's or of the engine's or one of each (ErrSameName); for a log given for two partitions
// (ErrSameLog); for a topic of acquisition's with no
// consumer of its schema or no table, or a table for a topic acquisition has not; for a topic the engine or
// visualization is to read that is not one of acquisition's clean topics (ErrNotClean).
func (w Wiring[Src, Kline, Corr, View]) Workspace() (*Workspace[Src, Kline, Corr, View], error) {
	s, err := w.workspace()
	if err != nil {
		return nil, fmt.Errorf("abstract: the working space: %w", err)
	}
	return s, nil
}

func (w Wiring[Src, Kline, Corr, View]) workspace() (*Workspace[Src, Kline, Corr, View], error) {
	if err := w.Acquisition.check(); err != nil {
		return nil, err
	}
	if w.Chart == nil {
		return nil, errors.New("visualization: no chart")
	}
	if w.Engine != nil && w.Engine.Worker == nil {
		return nil, errors.New("the market model engine: no worker")
	}
	s := &Workspace[Src, Kline, Corr, View]{acquisition: w.Acquisition, warehousing: w.Warehousing, chart: w.Chart, ran: new(atomic.Bool)}

	// The topics: each one a topic, no two of one name and no log in two partitions, of one component or of two.
	writer := map[string]string{}
	seen := logsSeen{}
	name := func(l TopicLogs, by string) error {
		if l.topic == nil {
			return fmt.Errorf("%s: no topic: a topic comes with its logs (Topic.Logs)", by)
		}
		if other, twice := writer[l.topic.Name()]; twice {
			return fmt.Errorf("topic %q, %s and %s: %w", l.topic.Name(), other, by, ErrSameName)
		}
		writer[l.topic.Name()] = by
		return seen.add(l)
	}
	for _, k := range kinds {
		for _, l := range w.Topics.Of(k) {
			if err := name(l, fmt.Sprintf("one of acquisition's %v topics", k)); err != nil {
				return nil, err
			}
			s.acquired = append(s.acquired, l)
		}
	}
	var own []TopicLogs
	if w.Engine != nil {
		own = slices.Clone(w.Engine.Topics)
		for _, l := range own {
			if err := name(l, "one of the engine's topics"); err != nil {
				return nil, err
			}
		}
	}
	published, err := w.Topics.appendSets()
	if err != nil {
		return nil, err
	}
	s.published = published

	// Warehousing: on all of acquisition's topics, each into its table.
	s.tables = maps.Clone(w.Tables)
	if err := s.warehousing.check(s.acquired, s.tables); err != nil {
		return nil, err
	}
	acquired, err := newTopicSet(s.acquired)
	if err != nil {
		return nil, err
	}
	for _, topic := range slices.Sorted(maps.Keys(s.tables)) {
		if _, ok := acquired.byName[topic]; !ok {
			return nil, fmt.Errorf("warehousing: a table for %q, which is no topic of acquisition's", topic)
		}
	}

	// The engine and visualization read clean topics only, and visualization all of the engine's.
	clean, err := newTopicSet(w.Topics.Clean)
	if err != nil {
		return nil, err
	}
	if w.Engine != nil {
		s.worker = w.Engine.Worker
		if s.engineClean, err = cleanTopics(clean, w.Engine.Clean); err != nil {
			return nil, fmt.Errorf("the market model engine: %w", err)
		}
		set, err := newTopicSet(own)
		if err != nil {
			return nil, err
		}
		s.engineOwn, s.model = AppendSet{appendTo: set}, ReadSet{read: set}
	}
	if s.shown, err = cleanTopics(clean, w.Shown); err != nil {
		return nil, fmt.Errorf("visualization: %w", err)
	}
	return s, nil
}

// cleanTopics gives, to read, those of acquisition's clean topics that are asked for: each asked for is one of them,
// and one asked for twice is given once.
func cleanTopics(clean topicSet, asked []AnyTopic) (ReadSet, error) {
	var given []TopicLogs
	seen := map[string]bool{}
	for i, t := range asked {
		if t == nil {
			return ReadSet{}, fmt.Errorf("topic %d of %d to read: no topic", i+1, len(asked))
		}
		at, ok := clean.byName[t.Name()]
		if !ok || !sameTopic(clean.topics[at].topic, t) {
			return ReadSet{}, fmt.Errorf("topic %q, with %d partitions, keys %v and values %v: %w", t.Name(), t.Partitions(), t.Key(), t.Value(), ErrNotClean)
		}
		if seen[t.Name()] {
			continue
		}
		seen[t.Name()] = true
		given = append(given, clean.topics[at])
	}
	set, err := newTopicSet(given)
	if err != nil {
		return ReadSet{}, err
	}
	return ReadSet{read: set}, nil
}

// Topics lists the topics a component writes: acquisition's, of every kind, and the engine's; warehousing and
// visualization write none. No two topics of the working space have the same name.
func (s *Workspace[Src, Kline, Corr, View]) Topics(c Component) []AnyTopic {
	switch c {
	case ComponentAcquisition:
		ts := make([]AnyTopic, len(s.acquired))
		for i, l := range s.acquired {
			ts[i] = l.topic
		}
		return ts
	case ComponentMarketModelEngine:
		return s.engineOwn.Topics()
	}
	return nil
}

// Table names the table of the database one of acquisition's topics is pushed into; false for any other name.
func (s *Workspace[Src, Kline, Corr, View]) Table(topic string) (string, bool) {
	table, ok := s.tables[topic]
	return table, ok
}

// Run runs the working space: acquisition on the inputs, which it alone reads; warehousing on all of acquisition's
// topics; the engine, where there is one, on its clean topics, writing its own; visualization on its clean topics
// and all of the engine's, its chart's updates appended to chart. What leaves the system is warehousing's pushes,
// into the tables, and the chart. It returns when every part has returned, so that nothing of it is left running:
// with nil where each returned nil, else with the first error a part returned, which is the context's where the
// run ended because ctx was done.
//
// A working space is run once: within one, nothing starts again, and a second Run is refused, of the working space
// or of a copy of it. A Workspace that no wiring made (Wiring.Workspace) is refused.
func (s *Workspace[Src, Kline, Corr, View]) Run(ctx context.Context, in Inputs[Src, Corr], chart Appender[View]) error {
	if s == nil || s.ran == nil {
		return errors.New("abstract: the working space: not made by a wiring (Wiring.Workspace)")
	}
	if err := in.check(); err != nil {
		return fmt.Errorf("abstract: the working space: %w", err)
	}
	if chart == nil {
		return errors.New("abstract: the working space: nothing to append the chart's updates to")
	}
	if !s.ran.CompareAndSwap(false, true) {
		return errors.New("abstract: the working space has been run: within one, nothing starts again")
	}
	g := newGroup(ctx)
	g.run("acquisition", func(ctx context.Context) error {
		return s.acquisition.run(ctx, in, s.published)
	})
	g.run("warehousing", func(ctx context.Context) error {
		return s.warehousing.run(ctx, s.acquired, s.tables)
	})
	if s.worker != nil {
		g.run("the market model engine: the worker", func(ctx context.Context) error {
			return s.worker.Work(ctx, s.engineClean, s.engineOwn)
		})
	}
	g.run("visualization: the chart", func(ctx context.Context) error {
		return s.chart.Draw(ctx, s.shown, s.model, appendOnly[View]{to: chart})
	})
	if err := g.wait(); err != nil {
		return fmt.Errorf("abstract: %w", err)
	}
	return nil
}
