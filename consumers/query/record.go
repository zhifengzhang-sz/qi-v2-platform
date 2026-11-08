package main

import (
	"context"
	"fmt"
	"slices"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/kafka"
)

// record is what query reads of the stream, and all it does there: the topics, each partition's span of offsets, a
// partition's records within a span, and the consumer groups with their committed offsets. Each is a read: no topic
// written, no group joined or made, no offset committed.
type record interface {
	// List is the topics' names.
	List(ctx context.Context) ([]string, error)
	// Spans is a topic's offsets, partition by partition.
	Spans(ctx context.Context, topic string) (map[int32]stream.Span, error)
	// Read gives each record of the topic's partition from span.Start to before span.End, in offset order.
	Read(ctx context.Context, topic string, partition int32, span stream.Span, each func(stream.Record) error) error
	// Groups is each consumer group.
	Groups(ctx context.Context) ([]group, error)
	Close()
}

// group is a consumer group as the bus holds it: its name, its state, its committed offsets (the next offset it reads)
// by topic and partition, and the partitions its members are assigned now.
type group struct {
	name, state string
	committed   map[string]map[int32]int64
	assigned    map[string][]int32
}

// clientID is query's name to the brokers; it is no group's.
const clientID = "query"

// kafkaRecord is the record on Kafka (Redpanda): the core's admin for the topics and their spans, of which it calls
// List and Spans alone; a client of no group for a partition's records, each read a client of its own consuming that
// partition directly; the admin client's reads of the groups (ListGroups, DescribeGroups, OffsetFetch).
type kafkaRecord struct {
	brokers []string
	adm     stream.Admin
	cl      *kgo.Client
	groups  *kadm.Client
}

func openKafka(ctx context.Context, brokers []string) (*kafkaRecord, error) {
	adm, err := kafka.New(kafka.Config{Brokers: brokers, ClientID: clientID}).Admin(ctx)
	if err != nil {
		return nil, err
	}
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ClientID(clientID))
	if err != nil {
		adm.Close(ctx)
		return nil, fmt.Errorf("query: %w", err)
	}
	return &kafkaRecord{brokers: brokers, adm: adm, cl: cl, groups: kadm.NewClient(cl)}, nil
}

func (k *kafkaRecord) List(ctx context.Context) ([]string, error) { return k.adm.List(ctx) }

func (k *kafkaRecord) Spans(ctx context.Context, topic string) (map[int32]stream.Span, error) {
	return k.adm.Spans(ctx, topic)
}

func (k *kafkaRecord) Read(ctx context.Context, topic string, p int32, span stream.Span,
	each func(stream.Record) error) error {
	if span.Start >= span.End {
		return nil
	}
	cl, err := kgo.NewClient(kgo.SeedBrokers(k.brokers...), kgo.ClientID(clientID),
		kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{topic: {p: kgo.NewOffset().At(span.Start)}}),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()))
	if err != nil {
		return fmt.Errorf("query: read %s/%d: %w", topic, p, err)
	}
	defer cl.Close()
	for next := span.Start; next < span.End; {
		fs := cl.PollFetches(ctx)
		if err := ctx.Err(); err != nil {
			return err
		}
		var ferr error
		fs.EachError(func(t string, p int32, err error) {
			if ferr == nil {
				ferr = fmt.Errorf("query: read %s/%d: %w", t, p, err)
			}
		})
		if ferr != nil {
			return ferr
		}
		for it := fs.RecordIter(); !it.Done(); {
			r := it.Next()
			if r.Offset < next || r.Offset >= span.End {
				continue // before the span or read already; after it, published since the span was taken
			}
			if err := each(stream.Record{Topic: r.Topic, Key: r.Key, Value: r.Value, Time: r.Timestamp,
				Partition: r.Partition, Offset: r.Offset}); err != nil {
				return err
			}
			next = r.Offset + 1
		}
	}
	return nil
}

func (k *kafkaRecord) Groups(ctx context.Context) ([]group, error) {
	listed, err := k.groups.ListGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("query: list groups: %w", err)
	}
	names := make([]string, 0, len(listed))
	for n := range listed {
		names = append(names, n)
	}
	slices.Sort(names)
	if len(names) == 0 {
		return nil, nil
	}
	described, err := k.groups.DescribeGroups(ctx, names...)
	if err == nil {
		err = described.Error()
	}
	if err != nil {
		return nil, fmt.Errorf("query: describe groups: %w", err)
	}
	out := make([]group, 0, len(names))
	for _, n := range names {
		g := group{name: n, state: listed[n].State, committed: map[string]map[int32]int64{},
			assigned: map[string][]int32{}}
		if d, ok := described[n]; ok {
			g.state = d.State
			for t, ps := range d.AssignedPartitions() {
				for p := range ps {
					g.assigned[t] = append(g.assigned[t], p)
				}
				slices.Sort(g.assigned[t])
			}
		}
		offs, err := k.groups.FetchOffsets(ctx, n)
		if err != nil {
			return nil, fmt.Errorf("query: the offsets of group %s: %w", n, err)
		}
		for t, ps := range offs {
			for p, o := range ps {
				if o.Err != nil {
					return nil, fmt.Errorf("query: the offset of group %s on %s/%d: %w", n, t, p, o.Err)
				}
				if o.At < 0 {
					continue // none committed
				}
				if g.committed[t] == nil {
					g.committed[t] = map[int32]int64{}
				}
				g.committed[t][p] = o.At
			}
		}
		out = append(out, g)
	}
	return out, nil
}

func (k *kafkaRecord) Close() {
	k.adm.Close(context.Background())
	k.cl.Close()
}
