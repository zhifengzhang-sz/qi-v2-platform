package abstract

import (
	"errors"
	"fmt"
	"reflect"
)

// What a refusal is for, under the text that says which topic and where (errors.Is).
var (
	// ErrSameName: two topics of one name. Each topic has one writer, so no two have the same name.
	ErrSameName = errors.New("two topics of one name")
	// ErrNotGiven: a part asks for a topic or a partition it is not given.
	ErrNotGiven = errors.New("not among the topics given")
	// ErrSameLog: one log given for two partitions, of one topic or of two. A partition's log is its own.
	ErrSameLog = errors.New("one log for two partitions")
	// ErrSameInput: one cursor given as both the source and the corrections stream. The two are two logs, each read
	// whole by its own part.
	ErrSameInput = errors.New("one cursor for the source and the corrections")
	// ErrNotClean: a topic the engine or visualization is to read is not one of acquisition's clean topics.
	ErrNotClean = errors.New("not one of acquisition's clean topics")
)

// A Header is one header of a record: a key, and a value that may be absent (nil; a value of no bytes is not nil).
type Header struct {
	Key   string
	Value []byte
}

// A Record is a record as Kafka gives it to a reader: a key and a value, either of which may be absent (nil), a
// timestamp and headers. Its partition and offset are where it is: which log, and which position in it.
type Record[K, V any] struct {
	Key       *K
	Value     *V
	Timestamp int64
	Headers   []Header
}

// A Topic is a topic's description: its name, its number of partitions, at least one, and the types of its keys and
// values, K and V, which its schema fixes. It is made by NewTopic; the zero Topic, of no partition, is refused
// wherever a topic is taken. What a topic holds is its logs (Logs).
type Topic[K, V any] struct {
	name       string
	partitions int
}

// NewTopic gives the topic of that name and number of partitions, with keys of type K and values of type V. A topic
// has at least one partition: fewer is refused.
func NewTopic[K, V any](name string, partitions int) (Topic[K, V], error) {
	if partitions < 1 {
		return Topic[K, V]{}, fmt.Errorf("abstract: topic %q: %d partitions: a topic has at least one", name, partitions)
	}
	return Topic[K, V]{name: name, partitions: partitions}, nil
}

// Name is the topic's name.
func (t Topic[K, V]) Name() string { return t.name }

// Partitions is the topic's number of partitions.
func (t Topic[K, V]) Partitions() int { return t.partitions }

// Key is the type of the topic's keys, K.
func (t Topic[K, V]) Key() reflect.Type { return reflect.TypeFor[K]() }

// Value is the type of the topic's values, V.
func (t Topic[K, V]) Value() reflect.Type { return reflect.TypeFor[V]() }

func (t Topic[K, V]) sealed() {}

// Logs gives what the topic holds: one log of records for each partition, logs[p] that of partition p. As many logs
// as the topic has partitions, each there, or it is refused; so is a log given twice (ErrSameLog), where the two
// are given as one pointer or one channel.
func (t Topic[K, V]) Logs(logs ...Log[Record[K, V]]) (TopicLogs, error) {
	if t.partitions < 1 {
		return TopicLogs{}, fmt.Errorf("abstract: topic %q: %d partitions: a topic has at least one, and is made by NewTopic", t.name, t.partitions)
	}
	if len(logs) != t.partitions {
		return TopicLogs{}, fmt.Errorf("abstract: topic %q: %d logs for %d partitions: one log a partition", t.name, len(logs), t.partitions)
	}
	held := make([]Log[Record[K, V]], len(logs))
	for p, l := range logs {
		if l == nil {
			return TopicLogs{}, fmt.Errorf("abstract: topic %q: no log for partition %d", t.name, p)
		}
		held[p] = l
	}
	l := TopicLogs{topic: t, logs: held}
	for _, log := range held {
		l.each = append(l.each, log)
	}
	if err := (logsSeen{}).add(l); err != nil {
		return TopicLogs{}, fmt.Errorf("abstract: %w", err)
	}
	return l, nil
}

// AnyTopic is a topic of any schema: a Topic[K, V]. No other type of this package is one, and a TopicLogs is made
// of a Topic alone.
type AnyTopic interface {
	Name() string
	Partitions() int
	// Key and Value are the types of the topic's keys and values, which make its schema.
	Key() reflect.Type
	Value() reflect.Type
	sealed()
}

// sameTopic: the two are one description: one name, one number of partitions, one schema.
func sameTopic(a, b AnyTopic) bool {
	return a.Name() == b.Name() && a.Partitions() == b.Partitions() && a.Key() == b.Key() && a.Value() == b.Value()
}

// TopicLogs is a topic with what it holds: one log of records for each partition (Topic.Logs). The zero TopicLogs
// is no topic, and is refused wherever one is taken.
type TopicLogs struct {
	topic AnyTopic
	logs  any   // a []Log[Record[K, V]] for a Topic[K, V], one log a partition
	each  []any // the same logs, each as it was given, to tell one given twice
}

// logsSeen is the logs of some topics, each with the partition it is the log of.
type logsSeen map[any]string

// told: whether two values given as v are told to be one: where v is a pointer or a channel, which compare by what
// they point to. That two values of another kind are two is their giver's to keep.
func told(v any) bool {
	switch reflect.ValueOf(v).Kind() {
	case reflect.Pointer, reflect.Chan:
		return true
	}
	return false
}

// add notes the logs of one more topic, and refuses a log that is already another partition's, where the two are
// told to be one (told).
func (seen logsSeen) add(l TopicLogs) error {
	for p, log := range l.each {
		if !told(log) {
			continue
		}
		of := fmt.Sprintf("partition %d of topic %q", p, l.topic.Name())
		if other, twice := seen[log]; twice {
			return fmt.Errorf("%s and %s: %w", other, of, ErrSameLog)
		}
		seen[log] = of
	}
	return nil
}

// Topic is the topic whose logs these are; nil for the zero TopicLogs.
func (l TopicLogs) Topic() AnyTopic { return l.topic }

// topicSet is some topics with their logs, each found by its name.
type topicSet struct {
	topics []TopicLogs
	byName map[string]int
}

// newTopicSet gives the set of those topics: each one a topic, no two of one name, no log in two partitions.
func newTopicSet(topics []TopicLogs) (topicSet, error) {
	s := topicSet{byName: make(map[string]int, len(topics))}
	seen := logsSeen{}
	for i, l := range topics {
		if l.topic == nil {
			return topicSet{}, fmt.Errorf("topic %d of %d: no topic: a topic comes with its logs (Topic.Logs)", i+1, len(topics))
		}
		name := l.topic.Name()
		if _, twice := s.byName[name]; twice {
			return topicSet{}, fmt.Errorf("topic %q: %w", name, ErrSameName)
		}
		if err := seen.add(l); err != nil {
			return topicSet{}, err
		}
		s.byName[name] = len(s.topics)
		s.topics = append(s.topics, l)
	}
	return s, nil
}

func (s topicSet) list() []AnyTopic {
	ts := make([]AnyTopic, len(s.topics))
	for i, l := range s.topics {
		ts[i] = l.topic
	}
	return ts
}

// partitionOf gives the log of one partition of t, where the set holds t.
func partitionOf[K, V any](s topicSet, t Topic[K, V], partition int) (Log[Record[K, V]], error) {
	i, ok := s.byName[t.name]
	if !ok {
		return nil, fmt.Errorf("topic %q: %w", t.name, ErrNotGiven)
	}
	held := s.topics[i]
	if !sameTopic(held.topic, t) {
		return nil, fmt.Errorf("topic %q: asked with %d partitions, keys %v and values %v; the topic given has %d partitions, keys %v and values %v: %w",
			t.name, t.partitions, t.Key(), t.Value(), held.topic.Partitions(), held.topic.Key(), held.topic.Value(), ErrNotGiven)
	}
	logs, ok := held.logs.([]Log[Record[K, V]])
	if !ok {
		return nil, fmt.Errorf("topic %q: its logs are not of its schema, keys %v and values %v", t.name, t.Key(), t.Value())
	}
	if partition < 0 || partition >= len(logs) {
		return nil, fmt.Errorf("topic %q: partition %d of %d: %w", t.name, partition, len(logs), ErrNotGiven)
	}
	return logs[partition], nil
}

// A ReadSet is the topics a part is given to read, and nothing else. The zero ReadSet holds no topic. It is not an
// AppendSet, and no conversion makes it one.
type ReadSet struct{ read topicSet }

// NewReadSet gives the set of those topics, to read: no two of one name, no log in two partitions.
func NewReadSet(topics ...TopicLogs) (ReadSet, error) {
	s, err := newTopicSet(topics)
	if err != nil {
		return ReadSet{}, fmt.Errorf("abstract: %w", err)
	}
	return ReadSet{read: s}, nil
}

// Topics lists the set's topics, in the order they were given.
func (s ReadSet) Topics() []AnyTopic { return s.read.list() }

// ReadFrom gives a cursor on one partition of t, at its first record. A topic the set does not hold, or a partition
// the topic does not have, is refused (ErrNotGiven).
func ReadFrom[K, V any](s ReadSet, t Topic[K, V], partition int) (Cursor[Record[K, V]], error) {
	log, err := partitionOf(s.read, t, partition)
	if err != nil {
		return nil, fmt.Errorf("abstract: to read: %w", err)
	}
	return readOnly[Record[K, V]]{from: log.Read()}, nil
}

// An AppendSet is the topics a part is given to write, and nothing else: it can only append to them. The zero
// AppendSet holds no topic. It is not a ReadSet, and no conversion makes it one.
type AppendSet struct{ appendTo topicSet }

// NewAppendSet gives the set of those topics, to append to: no two of one name, no log in two partitions.
func NewAppendSet(topics ...TopicLogs) (AppendSet, error) {
	s, err := newTopicSet(topics)
	if err != nil {
		return AppendSet{}, fmt.Errorf("abstract: %w", err)
	}
	return AppendSet{appendTo: s}, nil
}

// Topics lists the set's topics, in the order they were given.
func (s AppendSet) Topics() []AnyTopic { return s.appendTo.list() }

// AppendTo gives the writing end of one partition of t; which partition a record goes to is the writer's choice. A
// topic the set does not hold, or a partition the topic does not have, is refused (ErrNotGiven).
func AppendTo[K, V any](s AppendSet, t Topic[K, V], partition int) (Appender[Record[K, V]], error) {
	log, err := partitionOf(s.appendTo, t, partition)
	if err != nil {
		return nil, fmt.Errorf("abstract: to append: %w", err)
	}
	return appendOnly[Record[K, V]]{to: log}, nil
}
