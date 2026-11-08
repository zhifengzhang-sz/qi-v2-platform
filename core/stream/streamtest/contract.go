// Package streamtest runs the stream contract against any stream.Bus.
// Both implementations (kafka, mem) must pass it; a component tested on
// mem may then rely on the same properties on kafka.
package streamtest

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// Run exercises the contract on bus. topicPrefix keeps parallel runs
// apart on a shared cluster; the topics created are deleted at the end,
// and the run fails if one cannot be.
func Run(t *testing.T, bus stream.Bus, topicPrefix string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const parts = 3
	topic := fmt.Sprintf("%s-%d", topicPrefix, time.Now().UnixNano())
	adm, err := bus.Admin(ctx)
	if err != nil {
		t.Fatalf("admin: %v", err)
	}
	defer adm.Close(ctx)
	if err := adm.Ensure(ctx, stream.TopicSpec{Name: topic, Partitions: parts, Replication: 1, RetentionMS: 3_600_000,
		CleanupPolicy: "delete", TimestampType: "CreateTime"}); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	defer func() {
		if err := adm.Delete(context.Background(), topic); err != nil { // made again before the end
			t.Errorf("delete %s: %v", topic, err)
		}
	}()
	// Ensure is idempotent.
	if err := adm.Ensure(ctx, stream.TopicSpec{Name: topic, Partitions: parts}); err != nil {
		t.Fatalf("ensure twice: %v", err)
	}
	names, err := adm.List(ctx)
	if err != nil || !containsStr(names, topic) {
		t.Fatalf("list: %v, has=%v", err, containsStr(names, topic))
	}
	if n, err := adm.Partitions(ctx, topic); err != nil || n[topic] != parts {
		t.Fatalf("partitions: %v, %v", n, err)
	}
	if c, err := adm.Configs(ctx, topic); err != nil || c["retention.ms"] != "3600000" || c["cleanup.policy"] != "delete" ||
		c["message.timestamp.type"] != "CreateTime" {
		t.Fatalf("configs: %v, %v", c, err)
	}
	// Alter sets what it names, unlimited retention among them, and leaves the rest.
	if err := adm.Alter(ctx, topic, map[string]string{"retention.ms": "-1"}); err != nil {
		t.Fatalf("alter: %v", err)
	}
	if c, err := adm.Configs(ctx, topic); err != nil || c["retention.ms"] != "-1" || c["cleanup.policy"] != "delete" ||
		c["message.timestamp.type"] != "CreateTime" {
		t.Fatalf("configs after alter: %v, %v", c, err)
	}

	sink, err := bus.Sink(ctx)
	if err != nil {
		t.Fatalf("sink: %v", err)
	}
	// A record without a key is refused, and nothing of its publish placed.
	if err := sink.Publish(ctx, stream.Record{Topic: topic, Key: []byte("k"), Value: []byte("v")},
		stream.Record{Topic: topic, Value: []byte("no key")}); err == nil {
		t.Fatalf("publish of a record without a key: no error")
	}
	// 3 keys × 10 records, interleaved, partitioned by their keys; and 2
	// keys × 10 records sharing one partition key, which their partition is.
	keys := []string{"BTCUSDT:1m", "ETHUSDT:1m", "SOLUSDT:1m"}
	shared := []string{"one:SPY:M1", "other:SPY:M1"}
	var batch []stream.Record
	for i := 0; i < 10; i++ {
		for _, k := range keys {
			batch = append(batch, stream.Record{
				Topic:   topic,
				Key:     []byte(k),
				Value:   []byte(k + "#" + strconv.Itoa(i)),
				Headers: map[string]string{"pk": k, "seq": strconv.Itoa(i)},
			})
		}
		for j, k := range shared {
			batch = append(batch, stream.Record{
				Topic:        topic,
				Key:          []byte(k),
				PartitionKey: []byte("SPY"),
				Value:        []byte(k + "#" + strconv.Itoa(i)),
				Headers:      map[string]string{"pk": "SPY", "seq": strconv.Itoa(len(shared)*i + j)},
			})
		}
	}
	total := len(batch)
	if err := sink.Publish(ctx, batch...); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := sink.Close(ctx); err != nil {
		t.Fatalf("sink close: %v", err)
	}

	group := topic + "-g"
	// Read everything; check per-key order, the partition of each record
	// and the header round-trip.
	src, err := bus.Source(ctx, stream.SubscribeOptions{Group: group, Topics: []string{topic}})
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	got := collect(t, ctx, src, total)
	checkKeyOrder(t, got)
	checkPartitions(t, got, parts)
	checkSharedOrder(t, got)
	if got[0].Headers["pk"] == "" || got[0].Headers["seq"] == "" {
		t.Fatalf("headers lost: %+v", got[0].Headers)
	}
	// Each partition's span: from 0, nothing removed yet, to the next
	// offset, every record read before it.
	spans, err := adm.Spans(ctx, topic)
	if err != nil {
		t.Fatalf("spans: %v", err)
	}
	next := map[int32]int64{}
	for _, r := range got {
		if r.Offset+1 > next[r.Partition] {
			next[r.Partition] = r.Offset + 1
		}
	}
	if len(spans) != parts {
		t.Fatalf("spans: %d partitions, want %d", len(spans), parts)
	}
	for p, s := range spans {
		if s.Start != 0 || s.End != next[p] {
			t.Fatalf("spans: partition %d [%d, %d), the records read end at %d", p, s.Start, s.End, next[p])
		}
	}
	// Commit only the first half, in delivery order.
	half := got[:total/2]
	if err := src.Commit(ctx, half...); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := src.Close(ctx); err != nil {
		t.Fatalf("source close: %v", err)
	}

	// Reopen the same group: must resume after the committed offsets.
	src2, err := bus.Source(ctx, stream.SubscribeOptions{Group: group, Topics: []string{topic}})
	if err != nil {
		t.Fatalf("source 2: %v", err)
	}
	expected := total - committedCount(half, got)
	again := collect(t, ctx, src2, expected)
	checkKeyOrder(t, again)
	for _, r := range again {
		for _, h := range half {
			if h.Partition == r.Partition && h.Offset == r.Offset {
				t.Fatalf("redelivered a committed record %s/%d", r.Topic, r.Offset)
			}
		}
	}
	if err := src2.Commit(ctx, again...); err != nil {
		t.Fatalf("commit 2: %v", err)
	}
	src2.Close(ctx)

	// Replay from the beginning ignores the group's commits.
	src3, err := bus.Source(ctx, stream.SubscribeOptions{Group: group, Topics: []string{topic}, Start: stream.StartFromBeginning})
	if err != nil {
		t.Fatalf("source 3: %v", err)
	}
	replay := collect(t, ctx, src3, total)
	checkKeyOrder(t, replay)
	// A group's members on the topic and its offsets: one member while
	// src3 is open, its offsets each partition's end once committed, no
	// member once it is closed.
	if err := src3.Commit(ctx, replay...); err != nil {
		t.Fatalf("commit 3: %v", err)
	}
	ends := map[int32]int64{}
	for p, s := range spans {
		if s.End > s.Start {
			ends[p] = s.End
		}
	}
	waitGroups(t, ctx, adm, topic, []stream.Group{{Name: group, Members: 1, Committed: ends}})
	src3.Close(ctx)
	waitGroups(t, ctx, adm, topic, []stream.Group{{Name: group, Committed: ends}})

	// A source of no group reads every record from the start, commits
	// nothing, and is no group's.
	src4, err := bus.Source(ctx, stream.SubscribeOptions{Topics: []string{topic}})
	if err != nil {
		t.Fatalf("source of no group: %v", err)
	}
	all := collect(t, ctx, src4, total)
	checkKeyOrder(t, all)
	checkPartitions(t, all, parts)
	if err := src4.Commit(ctx, all...); err == nil {
		t.Fatalf("commit by a source of no group: no error")
	}
	waitGroups(t, ctx, adm, topic, []stream.Group{{Name: group, Committed: ends}})
	src4.Close(ctx)

	// A topic deleted takes every group's offsets on it: made again under
	// its name, it is read from its start.
	if err := adm.Delete(ctx, topic); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if names, err := adm.List(ctx); err != nil || containsStr(names, topic) {
		t.Fatalf("list after delete: %v, has=%v", err, containsStr(names, topic))
	}
	if err := adm.Ensure(ctx, stream.TopicSpec{Name: topic, Partitions: parts, Replication: 1}); err != nil {
		t.Fatalf("ensure again: %v", err)
	}
	waitGroups(t, ctx, adm, topic, nil)
	sink2, err := bus.Sink(ctx)
	if err != nil {
		t.Fatalf("sink again: %v", err)
	}
	if err := sink2.Publish(ctx, stream.Record{Topic: topic, Key: []byte("again"), Value: []byte("v"),
		Headers: map[string]string{"pk": "again", "seq": "0"}}); err != nil {
		t.Fatalf("publish again: %v", err)
	}
	sink2.Close(ctx)
	src5, err := bus.Source(ctx, stream.SubscribeOptions{Group: group, Topics: []string{topic}})
	if err != nil {
		t.Fatalf("source again: %v", err)
	}
	if got := collect(t, ctx, src5, 1); string(got[0].Key) != "again" || got[0].Offset != 0 {
		t.Fatalf("the topic made again: %s at %d, want again at 0", got[0].Key, got[0].Offset)
	}
	src5.Close(ctx)
}

// waitGroups: the topic's groups as want, 15 s at most (a member's leave, an offset's removal, take effect a moment
// after on a broker).
func waitGroups(t *testing.T, ctx context.Context, adm stream.Admin, topic string, want []stream.Group) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		got, err := adm.Groups(ctx, topic)
		if err != nil {
			t.Fatalf("groups: %v", err)
		}
		if sameGroups(got, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("groups of %s: %+v, want %+v", topic, got, want)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func sameGroups(a, b []stream.Group) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Members != b[i].Members || len(a[i].Committed) != len(b[i].Committed) {
			return false
		}
		for p, o := range a[i].Committed {
			if b[i].Committed[p] != o {
				return false
			}
		}
	}
	return true
}

// committedCount is the number of records not redelivered after
// committing half: per partition, everything up to the highest committed
// offset is skipped, even records not in the committed slice.
func committedCount(half, all []stream.Record) int {
	high := map[int32]int64{}
	for _, r := range half {
		if o, ok := high[r.Partition]; !ok || r.Offset > o {
			high[r.Partition] = r.Offset
		}
	}
	n := 0
	for _, r := range all {
		if o, ok := high[r.Partition]; ok && r.Offset <= o {
			n++
		}
	}
	return n
}

func collect(t *testing.T, ctx context.Context, src stream.Source, n int) []stream.Record {
	t.Helper()
	var out []stream.Record
	deadline := time.Now().Add(30 * time.Second)
	for len(out) < n {
		if time.Now().After(deadline) {
			t.Fatalf("collect: got %d of %d before deadline", len(out), n)
		}
		fctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		recs, err := src.Fetch(fctx)
		cancel()
		if err != nil && fctx.Err() == nil {
			t.Fatalf("fetch: %v", err)
		}
		out = append(out, recs...)
	}
	if len(out) != n {
		t.Fatalf("collect: got %d, want exactly %d", len(out), n)
	}
	return out
}

func checkKeyOrder(t *testing.T, recs []stream.Record) {
	t.Helper()
	last := map[string]int{}
	for _, r := range recs {
		seq, _ := strconv.Atoi(r.Headers["seq"])
		k := string(r.Key)
		if prev, ok := last[k]; ok && seq <= prev {
			t.Fatalf("key %s: seq %d after %d", k, seq, prev)
		}
		last[k] = seq
	}
}

// checkPartitions: each record in the partition Kafka's default
// partitioner gives its partition key (stream.PartitionOf).
func checkPartitions(t *testing.T, recs []stream.Record, parts int32) {
	t.Helper()
	for _, r := range recs {
		if r.PartitionKey != nil {
			t.Fatalf("%s: a partition key on delivery", r.Key)
		}
		if want := stream.PartitionOf([]byte(r.Headers["pk"]), parts); r.Partition != want {
			t.Fatalf("%s: partition %d, its partition key %q gives %d", r.Key, r.Partition, r.Headers["pk"], want)
		}
	}
}

// checkSharedOrder: the records of one partition key, of two keys, in
// the order they were published.
func checkSharedOrder(t *testing.T, recs []stream.Record) {
	t.Helper()
	last := -1
	for _, r := range recs {
		if r.Headers["pk"] != "SPY" {
			continue
		}
		seq, _ := strconv.Atoi(r.Headers["seq"])
		if seq <= last {
			t.Fatalf("partition key SPY: seq %d after %d", seq, last)
		}
		last = seq
	}
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
