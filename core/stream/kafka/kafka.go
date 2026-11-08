// Package kafka is the Kafka implementation of stream.Bus, via franz-go,
// run against Redpanda. It is the production bus. A record's partition
// is Kafka's default partitioner's for its partition key
// (stream.PartitionOf), its key when it has none.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// Config is what the bus needs to reach the cluster.
type Config struct {
	Brokers  []string // seed brokers, host:port
	ClientID string
}

// Bus implements stream.Bus over franz-go. One Bus per process; sinks,
// sources and admins share nothing but the configuration.
type Bus struct{ cfg Config }

// New returns a Bus. It does not connect; each Sink/Source/Admin does.
func New(cfg Config) *Bus {
	if cfg.ClientID == "" {
		cfg.ClientID = "qi-v2-platform"
	}
	return &Bus{cfg: cfg}
}

func (b *Bus) base() []kgo.Opt {
	return []kgo.Opt{
		kgo.SeedBrokers(b.cfg.Brokers...),
		kgo.ClientID(b.cfg.ClientID),
	}
}

// Close is a no-op: the Bus holds no connection.
func (b *Bus) Close(ctx context.Context) error { return nil }

// ---- Sink ------------------------------------------------------------

type sink struct{ cl *kgo.Client }

// Sink opens an idempotent producer (franz-go enables idempotence by
// default) with acks from all in-sync replicas, partitioning by each
// record's partition key.
func (b *Bus) Sink(ctx context.Context) (stream.Sink, error) {
	opts := append(b.base(),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.Lz4Compression()),
		kgo.RecordPartitioner(byPartitionKey()),
	)
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("stream/kafka: sink: %w", err)
	}
	if err := cl.Ping(ctx); err != nil {
		cl.Close()
		return nil, fmt.Errorf("stream/kafka: sink ping: %w", err)
	}
	return &sink{cl: cl}, nil
}

// partitionKey is the context key that carries a record's partition key
// from Publish to the partitioner: kgo.Record has no field for it, and
// its Context is the field franz-go keeps for a record's enrichment.
type partitionKey struct{}

// byPartitionKey places a record as Kafka's default partitioner places a
// key, by its partition key: franz-go's StickyKeyPartitioner (murmur2,
// as Kafka's) given that key. Every record has a key (Publish refuses
// one without), so none is placed by stickiness.
func byPartitionKey() kgo.Partitioner {
	keyed := kgo.StickyKeyPartitioner(nil)
	return kgo.BasicConsistentPartitioner(func(topic string) func(*kgo.Record, int) int {
		tp := keyed.ForTopic(topic)
		return func(r *kgo.Record, n int) int {
			if r.Context != nil {
				if pk, ok := r.Context.Value(partitionKey{}).([]byte); ok {
					return tp.Partition(&kgo.Record{Key: pk}, n)
				}
			}
			return tp.Partition(&kgo.Record{Key: r.Key}, n)
		}
	})
}

func toKgo(ctx context.Context, r stream.Record) *kgo.Record {
	rec := &kgo.Record{Topic: r.Topic, Key: r.Key, Value: r.Value, Context: ctx}
	if r.PartitionKey != nil {
		rec.Context = context.WithValue(ctx, partitionKey{}, r.PartitionKey)
	}
	if !r.Time.IsZero() {
		rec.Timestamp = r.Time
	}
	for k, v := range r.Headers {
		rec.Headers = append(rec.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}
	return rec
}

func fromKgo(r *kgo.Record) stream.Record {
	out := stream.Record{
		Topic:     r.Topic,
		Key:       r.Key,
		Value:     r.Value,
		Time:      r.Timestamp,
		Partition: r.Partition,
		Offset:    r.Offset,
		Epoch:     r.LeaderEpoch,
	}
	if len(r.Headers) > 0 {
		out.Headers = make(map[string]string, len(r.Headers))
		for _, h := range r.Headers {
			out.Headers[h.Key] = string(h.Value)
		}
	}
	return out
}

// Publish sends the records and waits for all of them to be acked.
// Records with the same partition key go to the same partition, in order.
func (s *sink) Publish(ctx context.Context, recs ...stream.Record) error {
	if len(recs) == 0 {
		return nil
	}
	rs := make([]*kgo.Record, len(recs))
	for i, r := range recs {
		if len(r.Key) == 0 {
			return fmt.Errorf("stream/kafka: publish to %s: a record without a key", r.Topic)
		}
		rs[i] = toKgo(ctx, r)
	}
	if err := s.cl.ProduceSync(ctx, rs...).FirstErr(); err != nil {
		return fmt.Errorf("stream/kafka: publish: %w", err)
	}
	return nil
}

func (s *sink) Close(ctx context.Context) error {
	if err := s.cl.Flush(ctx); err != nil {
		s.cl.Close()
		return err
	}
	s.cl.Close()
	return nil
}

// ---- Source ----------------------------------------------------------

type source struct {
	cl      *kgo.Client
	noGroup bool // a reader of no group, which commits nothing
}

// Source joins the consumer group with auto-commit disabled; progress
// is committed only through Commit. Partitions are consumed in offset
// order and a partition is never fetched by two members at once
// (Kafka's group protocol). With no group, it consumes every partition
// of the topics directly, from its start, and commits nothing.
func (b *Bus) Source(ctx context.Context, o stream.SubscribeOptions) (stream.Source, error) {
	if len(o.Topics) == 0 {
		return nil, fmt.Errorf("stream/kafka: source needs topics")
	}
	if o.Group == "" {
		cl, err := kgo.NewClient(append(b.base(), kgo.ConsumeTopics(o.Topics...),
			kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()), kgo.FetchIsolationLevel(kgo.ReadCommitted()))...)
		if err != nil {
			return nil, fmt.Errorf("stream/kafka: source: %w", err)
		}
		if err := cl.Ping(ctx); err != nil {
			cl.Close()
			return nil, fmt.Errorf("stream/kafka: source ping: %w", err)
		}
		return &source{cl: cl, noGroup: true}, nil
	}
	reset := kgo.NewOffset().AtStart()
	if o.Start == stream.StartFromEnd {
		reset = kgo.NewOffset().AtEnd()
	}
	opts := append(b.base(),
		kgo.ConsumerGroup(o.Group),
		kgo.ConsumeTopics(o.Topics...),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(reset),
		kgo.Balancers(kgo.CooperativeStickyBalancer()),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()),
	)
	if o.Start == stream.StartFromBeginning {
		// Replay: ignore any committed offset for this group.
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("stream/kafka: source: %w", err)
	}
	if err := cl.Ping(ctx); err != nil {
		cl.Close()
		return nil, fmt.Errorf("stream/kafka: source ping: %w", err)
	}
	if o.Start == stream.StartFromBeginning {
		// Drop the group's committed offsets so the reset applies.
		adm := kadm.NewClient(cl)
		if _, err := adm.DeleteGroup(ctx, o.Group); err != nil && !isGroupNotFound(err) {
			cl.Close()
			return nil, fmt.Errorf("stream/kafka: source replay reset: %w", err)
		}
	}
	return &source{cl: cl}, nil
}

func isGroupNotFound(err error) bool { return errors.Is(err, kerr.GroupIDNotFound) }

// Fetch blocks until records arrive or ctx ends. The returned slice is
// in delivery order: per partition, offsets ascend.
func (s *source) Fetch(ctx context.Context) ([]stream.Record, error) {
	fetches := s.cl.PollFetches(ctx)
	if err := fetches.Err0(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	var firstErr error
	fetches.EachError(func(t string, p int32, err error) {
		if firstErr == nil {
			firstErr = fmt.Errorf("stream/kafka: fetch %s/%d: %w", t, p, err)
		}
	})
	if firstErr != nil {
		return nil, firstErr
	}
	var out []stream.Record
	fetches.EachRecord(func(r *kgo.Record) { out = append(out, fromKgo(r)) })
	return out, nil
}

// Commit commits the offsets after the given records, synchronously.
func (s *source) Commit(ctx context.Context, recs ...stream.Record) error {
	if len(recs) == 0 {
		return nil
	}
	if s.noGroup {
		return fmt.Errorf("stream/kafka: commit by a source of no group")
	}
	rs := make([]*kgo.Record, len(recs))
	for i, r := range recs {
		rs[i] = &kgo.Record{Topic: r.Topic, Partition: r.Partition, Offset: r.Offset, LeaderEpoch: r.Epoch}
	}
	if err := s.cl.CommitRecords(ctx, rs...); err != nil {
		return fmt.Errorf("stream/kafka: commit: %w", err)
	}
	return nil
}

func (s *source) Close(ctx context.Context) error {
	if !s.noGroup {
		// Leave the group cleanly so its partitions rebalance at once.
		s.cl.LeaveGroupContext(ctx)
	}
	s.cl.Close()
	return nil
}

// ---- Admin -----------------------------------------------------------

type admin struct {
	cl  *kgo.Client
	adm *kadm.Client
}

// Admin opens an admin client. Its reads of the cluster's metadata are at most 10 ms old (franz-go's least): kadm
// answers them from the client's cached metadata, 5 s old by default, so that a topic deleted or made would be read as
// it was.
func (b *Bus) Admin(ctx context.Context) (stream.Admin, error) {
	cl, err := kgo.NewClient(append(b.base(), kgo.MetadataMinAge(10*time.Millisecond))...)
	if err != nil {
		return nil, fmt.Errorf("stream/kafka: admin: %w", err)
	}
	if err := cl.Ping(ctx); err != nil {
		cl.Close()
		return nil, fmt.Errorf("stream/kafka: admin ping: %w", err)
	}
	return &admin{cl: cl, adm: kadm.NewClient(cl)}, nil
}

// Ensure creates each topic that does not exist. Existing topics are
// left untouched. The controller, asked to create each, says which exists
// (TopicAlreadyExists), not a listing, which a core of Redpanda's may give
// from metadata older than a topic deleted.
func (a *admin) Ensure(ctx context.Context, specs ...stream.TopicSpec) error {
	for _, sp := range specs {
		parts := sp.Partitions
		if parts <= 0 {
			parts = 1
		}
		repl := sp.Replication
		if repl <= 0 {
			repl = 1
		}
		configs := map[string]*string{}
		if sp.RetentionMS != 0 {
			v := strconv.FormatInt(sp.RetentionMS, 10)
			configs["retention.ms"] = &v
		}
		if sp.CleanupPolicy != "" {
			v := sp.CleanupPolicy
			configs["cleanup.policy"] = &v
		}
		if sp.Compression != "" {
			v := sp.Compression
			configs["compression.type"] = &v
		}
		if sp.TimestampType != "" {
			v := sp.TimestampType
			configs["message.timestamp.type"] = &v
		}
		// A topic created moments ago may not be in the listing yet;
		// "already exists" is therefore success, not failure.
		if _, err := a.adm.CreateTopic(ctx, parts, repl, configs, sp.Name); err != nil && !errors.Is(err, kerr.TopicAlreadyExists) {
			return fmt.Errorf("stream/kafka: create %s: %w", sp.Name, err)
		}
	}
	// Ensure's postcondition is that every topic is visible in metadata,
	// so a Source or Sink opened right after it finds the topic.
	for _, sp := range specs {
		if err := a.waitVisible(ctx, sp.Name, 10*time.Second); err != nil {
			return err
		}
	}
	return nil
}

func (a *admin) waitVisible(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		td, err := a.adm.ListTopics(ctx, name)
		if err == nil && td.Has(name) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("stream/kafka: topic %s not visible after %s", name, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Delete deletes the topics, then waits, 30 s at most, until none is
// listed, not even as unknown, and no group holds an offset on one, in ten
// reads in a row, 50 ms apart: Redpanda answers each connection from the
// metadata of the core that holds it, and its cores learn of a topic
// deleted one after another; it drops a group's offsets on a topic deleted
// (v26.2.6, tried on the test stack), and Delete makes sure of it, so that
// a topic made again under its name is read from its start.
func (a *admin) Delete(ctx context.Context, names ...string) error {
	if len(names) == 0 {
		return nil
	}
	resps, err := a.adm.DeleteTopics(ctx, names...)
	if err != nil {
		return fmt.Errorf("stream/kafka: delete topics: %w", err)
	}
	for _, r := range resps.Sorted() {
		if r.Err != nil && !errors.Is(r.Err, kerr.UnknownTopicOrPartition) {
			return fmt.Errorf("stream/kafka: delete %s: %w", r.Topic, r.Err)
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	for gone := 0; gone < 10; {
		left, err := a.left(ctx, names)
		if err != nil {
			return err
		}
		if gone++; left != "" {
			gone = 0
			if time.Now().After(deadline) {
				return fmt.Errorf("stream/kafka: delete: %s 30 s after it was deleted", left)
			}
		}
		if gone < 10 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	return nil
}

// left: what is left of the topics deleted, a topic listed or a group's offset on one; "" for nothing.
func (a *admin) left(ctx context.Context, names []string) (string, error) {
	td, err := a.adm.ListTopics(ctx)
	if err != nil {
		return "", fmt.Errorf("stream/kafka: list topics: %w", err)
	}
	for _, n := range names {
		if _, ok := td[n]; ok { // listed still, as unknown too while Redpanda deletes it
			return n + " listed", nil
		}
	}
	gs, err := a.groups(ctx)
	if err != nil {
		return "", err
	}
	for _, g := range gs {
		for _, n := range names {
			if len(g.offsets[n]) > 0 {
				return fmt.Sprintf("group %s's offsets on %s", g.name, n), nil
			}
		}
	}
	return "", nil
}

// group is a consumer group as the cluster holds it: each member's topics, subscribed or assigned, and its committed
// offsets by topic and partition.
type group struct {
	name    string
	members []map[string]bool
	offsets map[string]map[int32]int64
}

// groups: every consumer group (ListGroups, DescribeGroups, OffsetFetch).
func (a *admin) groups(ctx context.Context) ([]group, error) {
	listed, err := a.adm.ListGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("stream/kafka: list groups: %w", err)
	}
	names := listed.Groups()
	if len(names) == 0 {
		return nil, nil
	}
	described, err := a.adm.DescribeGroups(ctx, names...)
	if err == nil {
		err = described.Error()
	}
	if err != nil {
		return nil, fmt.Errorf("stream/kafka: describe groups: %w", err)
	}
	var out []group
	for _, n := range names {
		g := group{name: n, offsets: map[string]map[int32]int64{}}
		for _, m := range described[n].Members {
			ts := map[string]bool{}
			if c, ok := m.Join.AsConsumer(); ok {
				for _, t := range c.Topics {
					ts[t] = true
				}
			}
			if c, ok := m.Assigned.AsConsumer(); ok {
				for _, t := range c.Topics {
					if len(t.Partitions) > 0 {
						ts[t.Topic] = true
					}
				}
			}
			g.members = append(g.members, ts)
		}
		offs, err := a.adm.FetchOffsets(ctx, n)
		if err == nil {
			err = offs.Error()
		}
		if err != nil {
			return nil, fmt.Errorf("stream/kafka: the offsets of group %s: %w", n, err)
		}
		for t, ps := range offs {
			for p, o := range ps {
				if o.At < 0 {
					continue // none committed
				}
				if g.offsets[t] == nil {
					g.offsets[t] = map[int32]int64{}
				}
				g.offsets[t][p] = o.At
			}
		}
		out = append(out, g)
	}
	return out, nil
}

// Groups: each group with a member subscribed to the topic or assigned a partition of it, or an offset committed on
// it.
func (a *admin) Groups(ctx context.Context, topic string) ([]stream.Group, error) {
	gs, err := a.groups(ctx)
	if err != nil {
		return nil, err
	}
	var out []stream.Group
	for _, g := range gs {
		sg := stream.Group{Name: g.name, Committed: map[int32]int64{}}
		for _, m := range g.members {
			if m[topic] {
				sg.Members++
			}
		}
		for p, o := range g.offsets[topic] {
			sg.Committed[p] = o
		}
		if sg.Members > 0 || len(sg.Committed) > 0 {
			out = append(out, sg)
		}
	}
	slices.SortFunc(out, func(x, y stream.Group) int { return strings.Compare(x.Name, y.Name) })
	return out, nil
}

// List: the topics there, not one listed as unknown, as a topic deleted is a while.
func (a *admin) List(ctx context.Context) ([]string, error) {
	td, err := a.adm.ListTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("stream/kafka: list topics: %w", err)
	}
	var out []string
	for _, n := range td.Names() {
		if td.Has(n) {
			out = append(out, n)
		}
	}
	return out, nil
}

func (a *admin) Partitions(ctx context.Context, names ...string) (map[string]int32, error) {
	td, err := a.adm.ListTopics(ctx, names...)
	if err != nil {
		return nil, fmt.Errorf("stream/kafka: describe topics: %w", err)
	}
	out := map[string]int32{}
	for _, d := range td {
		if d.Err == nil {
			out[d.Topic] = int32(len(d.Partitions))
		}
	}
	return out, nil
}

func (a *admin) Spans(ctx context.Context, topic string) (map[int32]stream.Span, error) {
	list := func(what string, f func(context.Context, ...string) (kadm.ListedOffsets, error)) (map[int32]kadm.ListedOffset, error) {
		listed, err := f(ctx, topic)
		if err == nil {
			err = listed.Error()
		}
		if err != nil {
			return nil, fmt.Errorf("stream/kafka: %s offsets of %s: %w", what, topic, err)
		}
		if len(listed[topic]) == 0 {
			return nil, fmt.Errorf("stream/kafka: %s offsets of %s: no such topic", what, topic)
		}
		return listed[topic], nil
	}
	starts, err := list("start", a.adm.ListStartOffsets)
	if err != nil {
		return nil, err
	}
	ends, err := list("end", a.adm.ListEndOffsets)
	if err != nil {
		return nil, err
	}
	out := make(map[int32]stream.Span, len(ends))
	for p, e := range ends {
		s, ok := starts[p]
		if !ok {
			return nil, fmt.Errorf("stream/kafka: start offsets of %s: no partition %d", topic, p)
		}
		out[p] = stream.Span{Start: s.Offset, End: e.Offset}
	}
	return out, nil
}

func (a *admin) Configs(ctx context.Context, topic string) (map[string]string, error) {
	rcs, err := a.adm.DescribeTopicConfigs(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("stream/kafka: configs of %s: %w", topic, err)
	}
	rc, err := rcs.On(topic, nil)
	if err == nil {
		err = rc.Err
	}
	if err != nil {
		return nil, fmt.Errorf("stream/kafka: configs of %s: %w", topic, err)
	}
	out := map[string]string{}
	for _, c := range rc.Configs {
		if c.Value != nil {
			out[c.Key] = *c.Value
		}
	}
	return out, nil
}

// Alter sets the configurations incrementally: each named one set, the
// others as they were.
func (a *admin) Alter(ctx context.Context, topic string, configs map[string]string) error {
	if len(configs) == 0 {
		return nil
	}
	alters := make([]kadm.AlterConfig, 0, len(configs))
	for k, v := range configs {
		alters = append(alters, kadm.AlterConfig{Op: kadm.SetConfig, Name: k, Value: &v})
	}
	rs, err := a.adm.AlterTopicConfigs(ctx, alters, topic)
	if err == nil {
		var r kadm.AlterConfigsResponse
		if r, err = rs.On(topic, nil); err == nil {
			err = r.Err
		}
	}
	if err != nil {
		return fmt.Errorf("stream/kafka: alter %s: %w", topic, err)
	}
	return nil
}

func (a *admin) Close(ctx context.Context) error {
	a.cl.Close()
	return nil
}

// WaitReady polls the cluster until it answers or the deadline passes.
// Used by tests and by runners at startup.
func WaitReady(ctx context.Context, cfg Config, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		cl, err := kgo.NewClient(kgo.SeedBrokers(cfg.Brokers...))
		if err == nil {
			err = cl.Ping(ctx)
			cl.Close()
		}
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("stream/kafka: not ready after %s: %w", timeout, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}
