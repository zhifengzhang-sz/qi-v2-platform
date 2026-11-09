package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// options are a run's: the clean topic, the name and group it reads in, the K-lines a page is drawn whole with.
type options struct {
	topic  string
	name   string
	window int
}

// tally is what a run has read: the K-lines taken, the copies dropped and, of those, the ones of another value.
type tally struct{ klines, copies, others int }

// run reads the clean topic from its start in the group of its name and, once it has read to the topic's end as it
// stood at the start, serves the page and its feed on ln; then it follows the topic, each K-line given to every page,
// until ctx ends (nil) or a fault (the fault). At the end, the listener closed and each page's feed ended, then the
// source closed, so that its group has no member.
func run(ctx context.Context, bus stream.Bus, o options, page map[string][]byte, ln net.Listener, log io.Writer) error {
	src, start, end, err := open(ctx, bus, o)
	if err != nil {
		ln.Close()
		return err
	}
	h := newHub(o.window)
	srv := &http.Server{Handler: handler(h, page), ReadHeaderTimeout: 10 * time.Second}
	srv.RegisterOnShutdown(h.close)
	rctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	served := false
	ready := func(t tally) {
		served = true
		h.mu.Lock()
		w := h.last()
		h.mu.Unlock()
		window := "none"
		if len(w) > 0 {
			window = fmt.Sprintf("%d, %s to %s", len(w), index(w[0]), index(w[len(w)-1]))
		}
		fmt.Fprintf(log, "chart: %s: %d K-lines read, %d copies dropped (%d of another value); the window %s; "+
			"serving http://%s/\n", o.topic, t.klines, t.copies, t.others, window, ln.Addr())
		go func() {
			if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
				cancel(fmt.Errorf("serve %s: %w", ln.Addr(), err))
			}
		}()
	}
	t, err := follow(rctx, src, o.topic, start, end, h, ready)
	if cause := context.Cause(rctx); err == nil && ctx.Err() == nil && cause != nil {
		err = cause // the server's fault
	}
	if served {
		sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
		if serr := srv.Shutdown(sctx); serr != nil && err == nil {
			err = fmt.Errorf("stop: %w", serr)
		}
		scancel()
	} else {
		ln.Close()
	}
	src.Close(context.Background())
	if err != nil {
		return err
	}
	fmt.Fprintf(log, "chart: stopped: %d K-lines read, %d copies dropped; %d pages' feeds ended\n", t.klines,
		t.copies, h.ended)
	return nil
}

// index is a K-line text's index, for a line.
func index(k string) string {
	var v struct{ Index uint64 }
	if json.Unmarshal([]byte(k), &v) != nil {
		return "?"
	}
	return fmt.Sprintf("index %d", v.Index)
}

// open is the topic's span as it stands (no such topic stops it, named; a clean topic has one partition), and its
// source in the group of its name, from its start: StartFromBeginning drops the group's committed offsets, and it
// commits none.
func open(ctx context.Context, bus stream.Bus, o options) (stream.Source, int64, int64, error) {
	adm, err := bus.Admin(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	spans, err := adm.Spans(ctx, o.topic)
	adm.Close(context.Background())
	if err != nil {
		return nil, 0, 0, err
	}
	s, ok := spans[0]
	if len(spans) != clean.Partitions || !ok {
		return nil, 0, 0, fmt.Errorf("%s: %d partitions, a clean topic's %d", o.topic, len(spans), clean.Partitions)
	}
	src, err := bus.Source(ctx, stream.SubscribeOptions{Group: o.name, Topics: []string{o.topic},
		Start: stream.StartFromBeginning})
	if err != nil {
		return nil, 0, 0, err
	}
	return src, s.Start, s.End, nil
}

// follow reads each value of the topic as clean market data 0.1.0 (clean.FromRecord) and of each: its offset after the
// last one's, else it stops; a copy, a K-line of a date it has taken (a clean topic's key is its series', so its date
// alone), dropped, the first taken, one of another value counted, each date's value kept as a digest; else the next
// K-line, its index the last one's and 1 (from 0) and its date after the last one's, else it stops, as a value not of
// the schema; given to the hub. Once it has read to end, the topic's end at the start, it calls ready; then it follows,
// until ctx ends (nil, what it has read) or a fetch fails (the fault).
func follow(ctx context.Context, src stream.Source, topic string, start, end int64, h *hub,
	ready func(tally)) (tally, error) {
	var t tally
	last, next, date := int64(-1), uint64(0), uint64(0)
	dates := map[uint64][sha256.Size]byte{}
	served := false
	serve := func() {
		if !served {
			served = true
			ready(t)
		}
	}
	if start >= end {
		serve() // nothing to read
	}
	for {
		recs, err := src.Fetch(ctx)
		if ctx.Err() != nil {
			return t, nil
		}
		if err != nil {
			return t, err
		}
		for _, r := range recs {
			at := fmt.Sprintf("%s/%d@%d", topic, r.Partition, r.Offset)
			if r.Offset <= last {
				return t, fmt.Errorf("%s: after offset %d: the offsets of a partition ascend", at, last)
			}
			last = r.Offset
			m, err := clean.FromRecord(r)
			if err != nil {
				return t, err
			}
			k := m.KLine.Base()
			sum := sha256.Sum256(r.Value)
			if was, ok := dates[k.Date]; ok {
				t.copies++
				if was != sum {
					t.others++
				}
			} else {
				switch {
				case k.Index != next:
					return t, fmt.Errorf("%s: index %d, the next %d: not a value of %s %s", at, k.Index, next,
						clean.Schema, clean.Version)
				case next > 0 && k.Date <= date:
					return t, fmt.Errorf("%s: date %d, not after the last, %d: not a value of %s %s", at, k.Date,
						date, clean.Schema, clean.Version)
				}
				dates[k.Date] = sum
				next, date = next+1, k.Date
				t.klines++
				h.add(m.Key()+", "+m.Cleaning.Name+" "+m.Cleaning.Version, text(k))
			}
			if r.Offset >= end-1 {
				serve()
			}
		}
	}
}
