package abstract

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

// What the tests run a working space on: logs, topics and a database in memory.

// memoryLog is a log in memory: it holds every message appended to it.
type memoryLog[T any] struct {
	mu       sync.Mutex
	messages []T
	grown    chan struct{} // closed when the log grows, for the cursors that wait at its end
}

// logOf gives a log that holds those messages.
func logOf[T any](messages ...T) *memoryLog[T] {
	return &memoryLog[T]{messages: slices.Clone(messages)}
}

func (l *memoryLog[T]) Append(ctx context.Context, m T) error {
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

func (l *memoryLog[T]) Read() Cursor[T] { return &memoryCursor[T]{log: l} }

// all gives what the log holds.
func (l *memoryLog[T]) all() []T {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.messages)
}

// memoryCursor reads a memoryLog from its first message, waiting at its end.
type memoryCursor[T any] struct {
	log  *memoryLog[T]
	next int
}

func (c *memoryCursor[T]) Next(ctx context.Context) (T, error) {
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

// memoryTopic is a topic with its logs in memory.
type memoryTopic[K, V any] struct {
	topic      Topic[K, V]
	partitions []*memoryLog[Record[K, V]]
	logs       TopicLogs
}

func newMemoryTopic[K, V any](t *testing.T, name string, partitions int) memoryTopic[K, V] {
	t.Helper()
	topic, err := NewTopic[K, V](name, partitions)
	if err != nil {
		t.Fatal(err)
	}
	m := memoryTopic[K, V]{topic: topic}
	var logs []Log[Record[K, V]]
	for range partitions {
		l := logOf[Record[K, V]]()
		m.partitions = append(m.partitions, l)
		logs = append(logs, l)
	}
	if m.logs, err = topic.Logs(logs...); err != nil {
		t.Fatal(err)
	}
	return m
}

// pushed names one log pushed into the database: the table it goes into, and the partition of the topic it is from.
type pushed struct {
	table, topic string
	partition    int
}

// memoryDB is a database in memory, for pushes of type P: each log pushed into it, in its order, and each table as
// the set of its rows, a row a message marked with the log it is from.
type memoryDB[P comparable] struct {
	mu   sync.Mutex
	logs map[pushed][]P
	rows map[string]map[pushed]map[P]bool
}

func newMemoryDB[P comparable]() *memoryDB[P] {
	return &memoryDB[P]{logs: map[pushed][]P{}, rows: map[string]map[pushed]map[P]bool{}}
}

func (d *memoryDB[P]) Pushes(table string, topic string, partition int) (Appender[P], error) {
	from := pushed{table: table, topic: topic, partition: partition}
	return appendFunc[P](func(ctx context.Context, m P) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		d.logs[from] = append(d.logs[from], m)
		if d.rows[table] == nil {
			d.rows[table] = map[pushed]map[P]bool{}
		}
		if d.rows[table][from] == nil {
			d.rows[table][from] = map[P]bool{}
		}
		d.rows[table][from][m] = true
		return nil
	}), nil
}

// log gives one log pushed, in its order.
func (d *memoryDB[P]) log(from pushed) []P {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.logs[from])
}

// rowsOf gives the rows the table holds that are from one log: their messages, as a set.
func (d *memoryDB[P]) rowsOf(table string, from pushed) map[P]bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	rows := map[P]bool{}
	for m := range d.rows[table][from] {
		rows[m] = true
	}
	return rows
}

// logsInto lists the logs that have a row in the table.
func (d *memoryDB[P]) logsInto(table string) []pushed {
	d.mu.Lock()
	defer d.mu.Unlock()
	var from []pushed
	for l := range d.rows[table] {
		from = append(from, l)
	}
	return from
}

// appendFunc is a writing end made of a function.
type appendFunc[T any] func(ctx context.Context, m T) error

func (f appendFunc[T]) Append(ctx context.Context, m T) error { return f(ctx, m) }

// within gives a context for one run of a test: a run that has not returned by then is stopped, and fails its test
// by the error it returns.
func within(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}
