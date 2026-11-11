package abstract

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// A Consumer is the warehouse consumer of one schema, that of keys K and values V: it reads one partition of a topic
// of that schema and gives what it pushes into the database, messages P, which the definition leaves open. It is
// the same for every topic and partition of its schema: Consume is called once for each partition, with that
// partition alone, and what a call keeps between records is the call's.
type Consumer[K, V, P any] interface {
	Consume(ctx context.Context, partition Cursor[Record[K, V]], pushes Appender[P]) error
}

// A Database is the database as warehousing writes it, for pushes of type P. A table of it is a set of rows: it
// holds the rows of every log pushed into it, in no order. A row is a message marked with the log it is from, a
// partition of a topic: a message one log pushes twice is one row, and a message two logs push is two. Only
// warehousing writes into the database.
//
// The consumers of several schemas may push messages of several types, each into a Database of its own type: these
// stand for one database, in which a table's name names one table. That they are one is their giver's to keep.
type Database[P any] interface {
	// Pushes gives the writing end of one log pushed into the table named table: that of one partition of the topic
	// named topic.
	Pushes(table string, topic string, partition int) (Appender[P], error)
}

// schema is a topic's schema: the types of its keys and values.
type schema struct{ key, value reflect.Type }

func schemaOf(t AnyTopic) schema { return schema{key: t.Key(), value: t.Value()} }

func (s schema) String() string { return fmt.Sprintf("keys %v and values %v", s.key, s.value) }

// A SchemaConsumer is the consumer of one schema with the database its pushes go into (ConsumerOf).
type SchemaConsumer struct {
	schema  schema
	missing string
	// consume runs the consumer on one partition of a topic of its schema, its pushes into the table
	consume func(ctx context.Context, topic TopicLogs, partition int, table string) error
}

// ConsumerOf gives the consumer of the schema of keys K and values V, its pushes going into db.
func ConsumerOf[K, V, P any](c Consumer[K, V, P], db Database[P]) SchemaConsumer {
	s := SchemaConsumer{schema: schema{key: reflect.TypeFor[K](), value: reflect.TypeFor[V]()}}
	switch {
	case c == nil:
		s.missing = "no consumer"
	case db == nil:
		s.missing = "no database"
	}
	s.consume = func(ctx context.Context, topic TopicLogs, partition int, table string) error {
		logs, ok := topic.logs.([]Log[Record[K, V]])
		if !ok || partition < 0 || partition >= len(logs) {
			return fmt.Errorf("no log of %v at partition %d", s.schema, partition)
		}
		pushes, err := db.Pushes(table, topic.topic.Name(), partition)
		if err != nil {
			return fmt.Errorf("the pushes into table %q: %w", table, err)
		}
		if pushes == nil {
			return fmt.Errorf("the pushes into table %q: the database gives no log", table)
		}
		return c.Consume(ctx, readOnly[Record[K, V]]{from: logs[partition].Read()}, appendOnly[P]{to: pushes})
	}
	return s
}

// Warehousing is the warehouse consumers, one for each schema (NewWarehousing). The zero Warehousing has none.
type Warehousing struct {
	consumers map[schema]SchemaConsumer
}

// NewWarehousing gives warehousing with those consumers: one for each schema, a second for a schema refused.
func NewWarehousing(consumers ...SchemaConsumer) (Warehousing, error) {
	w := Warehousing{consumers: make(map[schema]SchemaConsumer, len(consumers))}
	for _, c := range consumers {
		if c.consume == nil {
			return Warehousing{}, errors.New("abstract: warehousing: a consumer not made by ConsumerOf")
		}
		if c.missing != "" {
			return Warehousing{}, fmt.Errorf("abstract: warehousing: the consumer of %v: %s", c.schema, c.missing)
		}
		if _, twice := w.consumers[c.schema]; twice {
			return Warehousing{}, fmt.Errorf("abstract: warehousing: two consumers of %v: one for each schema", c.schema)
		}
		w.consumers[c.schema] = c
	}
	return w, nil
}

// check: every topic is one, no two of one name, each with the consumer of its schema and its table.
func (w Warehousing) check(topics []TopicLogs, tables map[string]string) error {
	if _, err := newTopicSet(topics); err != nil {
		return fmt.Errorf("warehousing: %w", err)
	}
	for _, l := range topics {
		name := l.topic.Name()
		if _, ok := w.consumers[schemaOf(l.topic)]; !ok {
			return fmt.Errorf("warehousing: topic %q: no consumer of its schema, %v", name, schemaOf(l.topic))
		}
		if _, ok := tables[name]; !ok {
			return fmt.Errorf("warehousing: topic %q: no table to push it into", name)
		}
	}
	return nil
}

// Run is warehousing as one process: for every partition of every topic, the consumer of the topic's schema on that
// partition alone, what it pushes going into the topic's table, tables[the topic's name]; several topics may go
// into one table. A topic with no consumer of its schema, or with no table, is refused; so are two topics of one
// name (ErrSameName), a topic's table being found by its name, and one log given for two partitions (ErrSameLog).
// It returns when every consumer has returned, with the first error.
func (w Warehousing) Run(ctx context.Context, topics []TopicLogs, tables map[string]string) error {
	if err := w.check(topics, tables); err != nil {
		return fmt.Errorf("abstract: %w", err)
	}
	if err := w.run(ctx, topics, tables); err != nil {
		return fmt.Errorf("abstract: warehousing: %w", err)
	}
	return nil
}

func (w Warehousing) run(ctx context.Context, topics []TopicLogs, tables map[string]string) error {
	g := newGroup(ctx)
	for _, l := range topics {
		consumer, table := w.consumers[schemaOf(l.topic)], tables[l.topic.Name()]
		for p := range l.topic.Partitions() {
			g.run(fmt.Sprintf("the consumer on partition %d of topic %q", p, l.topic.Name()), func(ctx context.Context) error {
				return consumer.consume(ctx, l, p, table)
			})
		}
	}
	return g.wait()
}
