package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/lib/massive"
)

// The clean series (README.md, The store): written when a clean topic is made, by acquisition's reset, which runs the
// writer with -clean TOPIC, a run of its own, which reads in no group and commits nothing, beside the writer that
// follows the core's topics.

// cleanReport is what -clean read and wrote.
type cleanReport struct {
	topic                   string
	read, copies, differing int // the topic's K-lines read; of them, copies dropped and those of another value
	written                 int // the rows of the store's clean series of the topic's series after it
}

func (r cleanReport) String() string {
	return fmt.Sprintf("%s: %d K-lines read, %d copies dropped (%d of another value); the clean series: %d rows "+
		"written, the series emptied first", r.topic, r.read, r.copies, r.differing, r.written)
}

// seriesOf is a clean topic's series, its name's last three names (core/README.md, The topics' names).
func seriesOf(topic string) (source, instrument, timeframe string, err error) {
	n := strings.Split(topic, ".")
	if !strings.HasPrefix(topic, clean.Kind+".") || len(n) != 7 {
		return "", "", "", fmt.Errorf("%q: not a clean topic, market.clean.<cleaning>.<version>.<source>.<instrument>."+
			"<timeframe>", topic)
	}
	return n[4], n[5], n[6], nil
}

// cleanFrom writes the store's clean series of the clean topic's series from the topic: every K-line of it, from its
// start to its end as it stood when it began, read in no group (clean market data 0.1.0, clean.FromRecord), a copy
// dropped by its key and date, the first taken, one of another value logged; each Massive's K-line (massive.ReadKLine);
// the series emptied in the store, then written with them, at once.
func cleanFrom(ctx context.Context, bus stream.Bus, st store, topic string, log *slog.Logger) (cleanReport, error) {
	r := cleanReport{topic: topic}
	source, instrument, timeframe, err := seriesOf(topic)
	if err != nil {
		return r, err
	}
	if source != massive.Source {
		return r, fmt.Errorf("%s: a clean topic of %s, the writer keeps %s's", topic, source, massive.Source)
	}
	adm, err := bus.Admin(ctx)
	if err != nil {
		return r, err
	}
	spans, err := adm.Spans(ctx, topic)
	adm.Close(ctx)
	if err != nil {
		return r, err
	}
	next := map[int32]int64{}
	done := func() bool {
		for p, s := range spans {
			if max(next[p], s.Start) < s.End {
				return false
			}
		}
		return true
	}
	var rs []row
	at := map[uint64]int{}
	if !done() {
		src, err := bus.Source(ctx, stream.SubscribeOptions{Topics: []string{topic}})
		if err != nil {
			return r, err
		}
		defer src.Close(context.Background())
		for !done() {
			recs, err := src.Fetch(ctx)
			if err != nil {
				return r, err
			}
			for _, rec := range recs {
				if rec.Offset >= spans[rec.Partition].End {
					continue // published after the run began
				}
				next[rec.Partition] = rec.Offset + 1
				m, err := clean.FromRecord(rec)
				if err != nil {
					return r, err
				}
				r.read++
				k, err := massive.ReadKLine([]byte(m.KLine.Text()))
				if err != nil {
					return r, fmt.Errorf("%s/%d@%d: %w", rec.Topic, rec.Partition, rec.Offset, err)
				}
				if i, ok := at[k.Date]; ok {
					r.copies++
					if rs[i].v.k != k {
						r.differing++
						log.Warn("a copy of another value dropped", "topic", topic, "date", k.Date, "offset", rec.Offset)
					}
					continue
				}
				at[k.Date] = len(rs)
				rs = append(rs, row{m: minute{instrument: instrument, timeframe: timeframe, date: k.Date},
					v: version{k: k}})
			}
		}
	}
	if r.written, err = st.Clean(ctx, instrument, timeframe, rs); err != nil {
		return r, err
	}
	if r.written != len(rs) {
		return r, fmt.Errorf("%s: %d rows of the clean series after the write, %d K-lines", topic, r.written, len(rs))
	}
	return r, nil
}
