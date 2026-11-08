// Package mem is the in-memory implementation of stream.Bus, for tests
// of publishers and consumers without a broker. It honours the contract
// of package stream for a single process: a record's partition is Kafka's
// default for its partition key (stream.PartitionOf), offset order per
// partition, committed offsets per group, replay from the beginning, a
// reader of no group, each group's members and offsets on a topic, a
// topic deleted with every group's offsets on it. Groups have one member
// at a time (tests need no rebalancing).
package mem

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

type partition struct {
	recs []stream.Record
}

type topic struct {
	parts   []*partition
	spec    stream.TopicSpec
	altered map[string]string // set by Alter, over what spec gives
}

// Bus is an in-memory cluster.
type Bus struct {
	mu     sync.Mutex
	cond   *sync.Cond
	topics map[string]*topic
	// committed[group][topic][partition] = next offset to deliver
	committed map[string]map[string][]int64
	// marked[group][topic][partition]: an offset committed there (Commit), as Groups gives it
	marked  map[string]map[string]map[int32]bool
	sources []*source // every source opened, closed or not
	closed  bool
}

// New returns an empty in-memory bus.
func New() *Bus {
	b := &Bus{topics: map[string]*topic{}, committed: map[string]map[string][]int64{},
		marked: map[string]map[string]map[int32]bool{}}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *Bus) Close(ctx context.Context) error {
	b.mu.Lock()
	b.closed = true
	b.cond.Broadcast()
	b.mu.Unlock()
	return nil
}

// ---- Sink ------------------------------------------------------------

type sink struct{ b *Bus }

func (b *Bus) Sink(ctx context.Context) (stream.Sink, error) { return &sink{b: b}, nil }

func (s *sink) Publish(ctx context.Context, recs ...stream.Record) error {
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	// Every record checked before any is placed: a refused publish places none.
	for _, r := range recs {
		if _, ok := s.b.topics[r.Topic]; !ok {
			return fmt.Errorf("stream/mem: publish to unknown topic %q", r.Topic)
		}
		if len(r.Key) == 0 {
			return fmt.Errorf("stream/mem: publish to %s: a record without a key", r.Topic)
		}
	}
	for _, r := range recs {
		t := s.b.topics[r.Topic]
		pk := r.PartitionKey
		if pk == nil {
			pk = r.Key
		}
		p := stream.PartitionOf(pk, int32(len(t.parts)))
		r.Partition = p
		r.PartitionKey = nil
		r.Offset = int64(len(t.parts[p].recs))
		if r.Time.IsZero() {
			r.Time = time.Now()
		}
		if r.Headers != nil {
			h := make(map[string]string, len(r.Headers))
			for k, v := range r.Headers {
				h[k] = v
			}
			r.Headers = h
		}
		t.parts[p].recs = append(t.parts[p].recs, r)
	}
	s.b.cond.Broadcast()
	return nil
}

func (s *sink) Close(ctx context.Context) error { return nil }

// ---- Source ----------------------------------------------------------

type source struct {
	b      *Bus
	group  string
	topics []string
	// cursor[topic][partition] = next offset to deliver to this member
	cursor map[string][]int64
	closed bool
}

func (b *Bus) Source(ctx context.Context, o stream.SubscribeOptions) (stream.Source, error) {
	if len(o.Topics) == 0 {
		return nil, fmt.Errorf("stream/mem: source needs topics")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s := &source{b: b, group: o.Group, topics: o.Topics, cursor: map[string][]int64{}}
	if o.Group == "" { // no group: every partition from its start, nothing committed
		for _, name := range o.Topics {
			t, ok := b.topics[name]
			if !ok {
				return nil, fmt.Errorf("stream/mem: subscribe to unknown topic %q", name)
			}
			s.cursor[name] = make([]int64, len(t.parts))
		}
		b.sources = append(b.sources, s)
		return s, nil
	}
	if b.committed[o.Group] == nil {
		b.committed[o.Group] = map[string][]int64{}
	}
	for _, name := range o.Topics {
		t, ok := b.topics[name]
		if !ok {
			return nil, fmt.Errorf("stream/mem: subscribe to unknown topic %q", name)
		}
		n := len(t.parts)
		if b.committed[o.Group][name] == nil {
			b.committed[o.Group][name] = make([]int64, n)
		}
		cur := make([]int64, n)
		switch o.Start {
		case stream.StartFromBeginning:
			for i := range cur {
				b.committed[o.Group][name][i] = 0
			}
		case stream.StartFromEnd:
			for i := range cur {
				cur[i] = int64(len(t.parts[i].recs))
				b.committed[o.Group][name][i] = cur[i]
			}
		default:
			copy(cur, b.committed[o.Group][name])
		}
		if o.Start != stream.StartFromEnd {
			copy(cur, b.committed[o.Group][name])
		}
		s.cursor[name] = cur
	}
	if o.Start == stream.StartFromBeginning {
		delete(b.marked, o.Group) // the group's offsets dropped, as the Kafka source's replay does
	}
	b.sources = append(b.sources, s)
	return s, nil
}

func (s *source) pending() []stream.Record {
	var out []stream.Record
	names := append([]string(nil), s.topics...)
	sort.Strings(names)
	for _, name := range names {
		t, ok := s.b.topics[name]
		if !ok {
			continue // deleted since
		}
		for p, part := range t.parts {
			from := s.cursor[name][p]
			if from < int64(len(part.recs)) {
				out = append(out, part.recs[from:]...)
				s.cursor[name][p] = int64(len(part.recs))
			}
		}
	}
	return out
}

func (s *source) Fetch(ctx context.Context) ([]stream.Record, error) {
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	for {
		if s.closed || s.b.closed {
			return nil, fmt.Errorf("stream/mem: source closed")
		}
		if recs := s.pending(); len(recs) > 0 {
			return recs, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Wake on publish, close, or ctx cancellation (polled).
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				s.b.mu.Lock()
				s.b.cond.Broadcast()
				s.b.mu.Unlock()
			case <-done:
			}
		}()
		s.b.cond.Wait()
		close(done)
	}
}

func (s *source) Commit(ctx context.Context, recs ...stream.Record) error {
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	if s.group == "" && len(recs) > 0 {
		return fmt.Errorf("stream/mem: commit by a source of no group")
	}
	for _, r := range recs {
		c := s.b.committed[s.group][r.Topic]
		if c == nil {
			return fmt.Errorf("stream/mem: commit on unsubscribed topic %q", r.Topic)
		}
		if next := r.Offset + 1; next > c[r.Partition] {
			c[r.Partition] = next
		}
		g := s.b.marked[s.group]
		if g == nil {
			g = map[string]map[int32]bool{}
			s.b.marked[s.group] = g
		}
		if g[r.Topic] == nil {
			g[r.Topic] = map[int32]bool{}
		}
		g[r.Topic][r.Partition] = true
	}
	return nil
}

func (s *source) Close(ctx context.Context) error {
	s.b.mu.Lock()
	s.closed = true
	s.b.cond.Broadcast()
	s.b.mu.Unlock()
	return nil
}

// ---- Admin -----------------------------------------------------------

type admin struct{ b *Bus }

func (b *Bus) Admin(ctx context.Context) (stream.Admin, error) { return &admin{b: b}, nil }

func (a *admin) Ensure(ctx context.Context, specs ...stream.TopicSpec) error {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	for _, sp := range specs {
		if _, ok := a.b.topics[sp.Name]; ok {
			continue
		}
		n := int(sp.Partitions)
		if n <= 0 {
			n = 1
		}
		t := &topic{parts: make([]*partition, n), spec: sp}
		for i := range t.parts {
			t.parts[i] = &partition{}
		}
		a.b.topics[sp.Name] = t
	}
	return nil
}

// Delete: the topics, and every group's offsets on them.
func (a *admin) Delete(ctx context.Context, names ...string) error {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	for _, n := range names {
		delete(a.b.topics, n)
		for g := range a.b.committed {
			delete(a.b.committed[g], n)
		}
		for g := range a.b.marked {
			delete(a.b.marked[g], n)
		}
	}
	return nil
}

// Groups: each group with an open source on the topic, or an offset committed on it.
func (a *admin) Groups(ctx context.Context, name string) ([]stream.Group, error) {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	if _, ok := a.b.topics[name]; !ok {
		return nil, fmt.Errorf("stream/mem: groups of unknown topic %q", name)
	}
	by := map[string]*stream.Group{}
	of := func(g string) *stream.Group {
		if by[g] == nil {
			by[g] = &stream.Group{Name: g, Committed: map[int32]int64{}}
		}
		return by[g]
	}
	for _, s := range a.b.sources {
		if s.group != "" && !s.closed && slices.Contains(s.topics, name) {
			of(s.group).Members++
		}
	}
	for g, ts := range a.b.marked {
		for p := range ts[name] {
			of(g).Committed[p] = a.b.committed[g][name][p]
		}
	}
	out := make([]stream.Group, 0, len(by))
	for _, g := range slices.Sorted(maps.Keys(by)) {
		out = append(out, *by[g])
	}
	return out, nil
}

func (a *admin) List(ctx context.Context) ([]string, error) {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	out := make([]string, 0, len(a.b.topics))
	for n := range a.b.topics {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

func (a *admin) Partitions(ctx context.Context, names ...string) (map[string]int32, error) {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	out := map[string]int32{}
	for _, n := range names {
		if t, ok := a.b.topics[n]; ok {
			out[n] = int32(len(t.parts))
		}
	}
	return out, nil
}

// Spans: the bus keeps every record, so each partition's Start is 0.
func (a *admin) Spans(ctx context.Context, name string) (map[int32]stream.Span, error) {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	t, ok := a.b.topics[name]
	if !ok {
		return nil, fmt.Errorf("stream/mem: spans of unknown topic %q", name)
	}
	out := make(map[int32]stream.Span, len(t.parts))
	for i, p := range t.parts {
		out[int32(i)] = stream.Span{Start: 0, End: int64(len(p.recs))}
	}
	return out, nil
}

// Configs: what Ensure was given, cleanup.policy delete, retention.bytes
// -1 and message.timestamp.type CreateTime when it was given none
// (Redpanda's defaults), and what Alter set over it.
func (a *admin) Configs(ctx context.Context, name string) (map[string]string, error) {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	t, ok := a.b.topics[name]
	if !ok {
		return nil, fmt.Errorf("stream/mem: configs of unknown topic %q", name)
	}
	out := map[string]string{"cleanup.policy": "delete", "retention.bytes": "-1", "message.timestamp.type": "CreateTime"}
	if t.spec.CleanupPolicy != "" {
		out["cleanup.policy"] = t.spec.CleanupPolicy
	}
	if t.spec.RetentionMS != 0 {
		out["retention.ms"] = strconv.FormatInt(t.spec.RetentionMS, 10)
	}
	if t.spec.Compression != "" {
		out["compression.type"] = t.spec.Compression
	}
	if t.spec.TimestampType != "" {
		out["message.timestamp.type"] = t.spec.TimestampType
	}
	for k, v := range t.altered {
		out[k] = v
	}
	return out, nil
}

// Alter: each named configuration set over what Ensure was given.
func (a *admin) Alter(ctx context.Context, name string, configs map[string]string) error {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	t, ok := a.b.topics[name]
	if !ok {
		return fmt.Errorf("stream/mem: alter of unknown topic %q", name)
	}
	if t.altered == nil {
		t.altered = map[string]string{}
	}
	for k, v := range configs {
		t.altered[k] = v
	}
	return nil
}

func (a *admin) Close(ctx context.Context) error { return nil }
