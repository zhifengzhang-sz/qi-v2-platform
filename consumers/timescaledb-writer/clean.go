package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/lib/massive"
)

// The clean series (README.md, The store): written when a clean topic is made, by acquisition's reset, which runs the
// writer with -clean TOPIC; and the corrected raw a reset reads, -corrected KEY. Each a run of its own, which reads in
// no group and commits nothing, beside the writer that follows the core's topics.

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

// waitAtEnds waits until the group has committed the core's topics to their ends as they stood when it began: each
// partition holding records, its committed offset at its end (stream.Admin.Groups); at most wait.
func waitAtEnds(ctx context.Context, bus stream.Bus, group string, wait time.Duration, log *slog.Logger) error {
	adm, err := bus.Admin(ctx)
	if err != nil {
		return err
	}
	defer adm.Close(context.Background())
	ends := map[string]map[int32]int64{}
	for _, t := range raw.Topics {
		sp, err := adm.Spans(ctx, t)
		if err != nil {
			return err
		}
		ends[t] = map[int32]int64{}
		for p, s := range sp {
			if s.End > s.Start {
				ends[t][p] = s.End
			}
		}
	}
	deadline := time.Now().Add(wait)
	for {
		behind := ""
		for _, t := range raw.Topics {
			gs, err := adm.Groups(ctx, t)
			if err != nil {
				return err
			}
			committed := map[int32]int64{}
			for _, g := range gs {
				if g.Name == group {
					committed = g.Committed
				}
			}
			for p, e := range ends[t] {
				if c, ok := committed[p]; !ok || c < e {
					behind = fmt.Sprintf("%s/%d at %d of %d", t, p, c, e)
				}
			}
		}
		if behind == "" {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the group %s has not read the core's topics to their ends in %s: %s", group, wait, behind)
		}
		log.Info("waiting for the group at the topics' ends", "group", group, "behind", behind)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// correctedOf prints the series' corrected raw, once the writer's group has read the core's topics to their ends as
// they stood when it began (waitAtEnds): each minute's row of klines_corrected, in the order of their dates, Massive's
// K-line in its canonical text (massive.Text), one a line. It gives how many.
func correctedOf(ctx context.Context, bus stream.Bus, st *pg, group, key string, wait time.Duration, out io.Writer,
	log *slog.Logger) (int, error) {
	n := strings.Split(key, ":")
	if len(n) != 3 || n[0] != massive.Source {
		return 0, fmt.Errorf("-corrected %q: a series of %s, massive:<instrument>:<timeframe>", key, massive.Source)
	}
	if err := waitAtEnds(ctx, bus, group, wait, log); err != nil {
		return 0, err
	}
	w := bufio.NewWriter(out)
	count := 0
	err := st.Corrected(ctx, n[1], n[2], func(k massive.KLine) error {
		text, err := massive.Text(k)
		if err != nil {
			return err
		}
		count++
		_, err = w.WriteString(strings.TrimSuffix(text, "\n") + "\n")
		return err
	})
	if err != nil {
		return count, err
	}
	return count, w.Flush()
}
