// Package stream is the core's abstraction over the message bus (core/README.md).
//
// It exposes exactly what the core's publishers and consumers use: an
// ordered, partitioned source with offset commit; a keyed sink whose
// partition may be chosen by a key of its own; topic provisioning; and
// each partition's span of offsets. Kafka (via franz-go, against Redpanda) is one
// implementation; an in-memory one serves tests.
//
// The contract every implementation must honour:
//
//   - Ordering: records of the same partition key are delivered in the
//     order they were published, and a partition is delivered in offset
//     order to a single consumer at a time.
//   - Replay: a consumer may start from a committed offset or from the
//     beginning; the source is durable for the topic's retention.
//   - At-least-once: a record is redelivered unless its offset was
//     committed; consumers deduplicate as the topic's schema says.
//   - Idempotent publish: retries never duplicate a record within a
//     producer session.
package stream

import (
	"context"
	"time"
)

// Record is one message on a topic. PartitionKey decides the partition
// (Key when it is nil); Key is the record's key on the bus. Headers
// carry what consumers filter on without decoding Value.
type Record struct {
	Topic        string
	Key          []byte
	PartitionKey []byte // the bytes the partition is chosen by; nil: Key. Not carried on the bus: nil on delivery
	Value        []byte
	Headers      map[string]string
	Time         time.Time
	Partition    int32 // set on delivery; ignored on publish
	Offset       int64 // set on delivery; ignored on publish
	Epoch        int32 // set on delivery: the bus's leader epoch for the offset; carried back on Commit, otherwise opaque
}

// Sink publishes records. Publish returns once the record is durably
// accepted by the bus; retries are idempotent within the Sink's life. A
// record without a key is refused: the partition is the key's.
type Sink interface {
	Publish(ctx context.Context, recs ...Record) error
	Close(ctx context.Context) error
}

// Source delivers records of the topics a consumer group subscribes to,
// in partition order, and lets the consumer commit progress. A source of
// no group (SubscribeOptions.Group "") delivers every partition of its
// topics from its start, and commits nothing: Commit refuses.
//
// Fetch blocks until at least one record is available or ctx ends; it
// returns records in delivery order across the partitions this member
// owns, and never interleaves two records of one partition out of
// order. Commit marks every record up to and including rec as
// processed; a member restarts after its last commit.
type Source interface {
	Fetch(ctx context.Context) ([]Record, error)
	Commit(ctx context.Context, recs ...Record) error
	Close(ctx context.Context) error
}

// StartFrom says where a consumer group with no committed offset begins.
type StartFrom int

const (
	StartFromCommitted StartFrom = iota // committed offset, else the beginning
	StartFromBeginning                  // always the beginning (replay)
	StartFromEnd                        // only new records
)

// SubscribeOptions configure a Source. Group "" is no group: a reader of
// every partition of the topics from its start (Start is not read), which
// joins no group and commits nothing.
type SubscribeOptions struct {
	Group  string
	Topics []string
	Start  StartFrom
}

// TopicSpec is what provisioning needs to know about a topic.
type TopicSpec struct {
	Name          string
	Partitions    int32
	Replication   int16
	RetentionMS   int64  // retention.ms: 0, the broker's default; -1, unlimited
	CleanupPolicy string // "delete" (default) or "compact"
	Compression   string // compression.type ("lz4", ...); "": broker default
	TimestampType string // message.timestamp.type ("CreateTime", "LogAppendTime"); "": broker default
}

// Span is a partition's offsets: Start the first it holds, End the next it
// will write; Start == End when it holds none (retention may have
// removed every record before End).
type Span struct{ Start, End int64 }

// Group is a consumer group as it stands on a topic: its name; its members
// reading the topic, each subscribed to it or assigned a partition of it;
// and its committed offsets on it, the next offset it reads, by partition,
// a partition it has committed none on absent.
type Group struct {
	Name      string
	Members   int
	Committed map[int32]int64
}

// Admin provisions topics. Ensure is idempotent: an existing topic is
// left as is (partition count is never reduced).
type Admin interface {
	Ensure(ctx context.Context, specs ...TopicSpec) error
	// Delete removes each named topic, and every group's committed offsets
	// on it, so that a topic made again under its name is read from its
	// start; it returns once none of them is listed. A topic not there is
	// none to remove.
	Delete(ctx context.Context, names ...string) error
	List(ctx context.Context) ([]string, error)
	// Partitions is each named topic's partition count; a topic that does
	// not exist is absent from the map.
	Partitions(ctx context.Context, names ...string) (map[string]int32, error)
	// Spans is the topic's offsets, partition by partition: a consumer that
	// has read every record before each End has read the topic as it stood.
	Spans(ctx context.Context, topic string) (map[int32]Span, error)
	// Configs is the topic's configuration as the bus has it, the key of
	// each value it holds (retention.ms, cleanup.policy, …) to its value.
	Configs(ctx context.Context, topic string) (map[string]string, error)
	// Alter sets each named configuration of an existing topic to its
	// value, every other left as it is.
	Alter(ctx context.Context, topic string, configs map[string]string) error
	// Groups is each consumer group with a member reading the topic or an
	// offset committed on it, in the order of their names.
	Groups(ctx context.Context, topic string) ([]Group, error)
	Close(ctx context.Context) error
}

// Bus is one implementation of the abstraction: it opens sinks, sources
// and an admin against one cluster.
type Bus interface {
	Sink(ctx context.Context) (Sink, error)
	Source(ctx context.Context, opts SubscribeOptions) (Source, error)
	Admin(ctx context.Context) (Admin, error)
	Close(ctx context.Context) error
}
