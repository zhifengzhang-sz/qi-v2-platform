package main

import (
	"context"
	"fmt"
	"slices"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// version is a bar's version as the record holds it: its index, its K-line's text, when its run read it (0: none,
// a first version of 0.1.0 or 0.2.0) and its origin.
type version struct {
	index  uint64
	text   string
	read   uint64
	origin string
}

// record is what the core's two topics hold of one series, the record (core/README.md): each bar's first version on
// market.raw, a copy dropped, and its latest correction on market.corrections, that of the greatest read, a copy
// dropped.
type record struct {
	first  map[uint64]version    // by the bar's date
	latest map[uint64]version    // by the bar's date
	seen   map[[2]uint64]version // each correction taken, by the bar's date and its read
	// copies dropped, of market.raw and of market.corrections; of them, those of another K-line or read (differing)
	copies, differing, corrections, ccopies, cdiffering int
}

func newRecord() *record {
	return &record{first: map[uint64]version{}, latest: map[uint64]version{}, seen: map[[2]uint64]version{}}
}

// dates is the series' minutes on market.raw, in the order of their dates.
func (r *record) dates() []uint64 {
	out := make([]uint64, 0, len(r.first))
	for d := range r.first {
		out = append(out, d)
	}
	slices.Sort(out)
	return out
}

// latestOf is the minute's latest version: its correction of the greatest read, else its first version.
func (r *record) latestOf(d uint64) version {
	if v, ok := r.latest[d]; ok {
		return v
	}
	return r.first[d]
}

// readRecord reads the core's topics from their start to their end as they stood when it began, as a consumer of the
// group named (which commits nothing), and gives each series' record by its key.
func readRecord(ctx context.Context, bus stream.Bus, group string) (map[string]*record, error) {
	adm, err := bus.Admin(ctx)
	if err != nil {
		return nil, err
	}
	spans := map[string]map[int32]stream.Span{}
	for _, t := range raw.Topics {
		if spans[t], err = adm.Spans(ctx, t); err != nil {
			adm.Close(ctx)
			return nil, err
		}
	}
	adm.Close(ctx)
	out := map[string]*record{}
	next := map[string]map[int32]int64{raw.Topic: {}, raw.CorrectionsTopic: {}}
	// done: every partition of both read to its end as it stood, or holding nothing then
	done := func() bool {
		for t, ps := range spans {
			for p, s := range ps {
				if max(next[t][p], s.Start) < s.End {
					return false
				}
			}
		}
		return true
	}
	if done() {
		return out, nil
	}
	src, err := bus.Source(ctx, stream.SubscribeOptions{Group: group, Topics: raw.Topics,
		Start: stream.StartFromBeginning})
	if err != nil {
		return nil, err
	}
	defer src.Close(context.Background())
	of := func(key string) *record {
		r := out[key]
		if r == nil {
			r = newRecord()
			out[key] = r
		}
		return r
	}
	for !done() {
		recs, err := src.Fetch(ctx)
		if err != nil {
			return nil, err
		}
		for _, rec := range recs {
			if rec.Offset >= spans[rec.Topic][rec.Partition].End {
				continue // published after the read began
			}
			next[rec.Topic][rec.Partition] = rec.Offset + 1
			switch rec.Topic {
			case raw.Topic:
				m, err := raw.FromRecord(rec)
				if err != nil {
					return nil, err
				}
				r, k := of(m.Key()), m.KLine.Base()
				v := version{index: k.Index, text: m.KLine.Text(), read: m.Read, origin: m.Origin}
				if was, ok := r.first[k.Date]; ok {
					r.copies++
					if was != v {
						r.differing++
					}
					continue
				}
				r.first[k.Date] = v
			case raw.CorrectionsTopic:
				c, err := raw.CorrectionFromRecord(rec)
				if err != nil {
					return nil, err
				}
				r, k := of(c.Key()), c.KLine.Base()
				v := version{index: k.Index, text: c.KLine.Text(), read: c.Read, origin: c.Origin}
				id := [2]uint64{k.Date, c.Read}
				if was, ok := r.seen[id]; ok { // a copy, by its key, date and read: the first taken
					r.ccopies++
					if was != v {
						r.cdiffering++
					}
					continue
				}
				r.seen[id] = v
				r.corrections++
				if was, ok := r.latest[k.Date]; !ok || v.read > was.read {
					r.latest[k.Date] = v
				}
			default:
				return nil, fmt.Errorf("%s/%d@%d: a topic not asked for", rec.Topic, rec.Partition, rec.Offset)
			}
		}
	}
	return out, nil
}
