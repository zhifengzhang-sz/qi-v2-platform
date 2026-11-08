// printer, a consumer on the core (core/README.md), built on the core and the contracts alone, as any user's
// consumer would be: it reads the raw market data topic and writes each message, every field kept, a line a message
// in its canonical text, to stdout or a file. Each message is read as raw market data 0.3.0, or 0.2.0 or 0.1.0 as
// before (raw.FromRecord: its value in its canonical text, its key the value's, its timestamp the bar's date); one that
// is not stops it.
//
//	printer [-brokers localhost:19092] [-name printer] [-out FILE] [-from-beginning] [-to-end [-dedup]]
//
// Its consumer group is its name. It reads from its group's committed offset (-from-beginning: from the beginning)
// and on, committing what it has written; -to-end reads the topic from its beginning to its end as it stood when it
// began, and stops. -dedup (with -to-end) drops a copy as the schema's delivery says, a message whose key and bar's date
// it has taken already, the first taken, and writes each bar once, by key, then date. On stderr, at the end: what it
// read and wrote.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/kafka"
)

// options is a run's.
type options struct {
	name          string
	fromBeginning bool
	toEnd         bool
	dedup         bool
}

// report is what a run read and wrote.
type report struct {
	read, written, repeats, changed int
	keys                            map[string]int
}

func main() {
	var (
		brokers = flag.String("brokers", "localhost:19092", "the brokers, host:port, comma-separated (the local stack's Redpanda by default)")
		name    = flag.String("name", "printer", "this component's name, its consumer group")
		outPath = flag.String("out", "", "the file to write (default stdout)")
		begin   = flag.Bool("from-beginning", false, "read from the topic's beginning, not the group's committed offset")
		toEnd   = flag.Bool("to-end", false, "read the topic from its beginning to its end as it stands now, then stop")
		dedup   = flag.Bool("dedup", false, "with -to-end: each bar once, by the key and the bar's date, its first copy")
	)
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	o := options{name: *name, fromBeginning: *begin || *toEnd, toEnd: *toEnd, dedup: *dedup}
	if o.dedup && !o.toEnd {
		fatal(log, errors.New("-dedup needs -to-end: each bar once, by key, then date, is known once the topic is read"))
	}
	var out io.Writer = os.Stdout
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			fatal(log, err)
		}
		defer f.Close()
		out = f
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg := kafka.Config{Brokers: strings.Split(*brokers, ","), ClientID: o.name}
	if err := kafka.WaitReady(ctx, cfg, 30*time.Second); err != nil {
		fatal(log, err)
	}
	r, err := consume(ctx, kafka.New(cfg), o, out)
	if r.keys != nil {
		fmt.Fprintf(os.Stderr, "printer: %d messages read from %s, of %d keys; %d written; %d repeats of a bar read before, %d of them with another text, the first kept\n",
			r.read, raw.Topic, len(r.keys), r.written, r.repeats, r.changed)
	}
	if err != nil && !(errors.Is(err, context.Canceled) && !o.toEnd) {
		fatal(log, err)
	}
}

func fatal(log *slog.Logger, err error) {
	log.Error("fatal", "err", err.Error())
	os.Exit(1)
}

// consume reads the topic as o says and writes its messages to out.
func consume(ctx context.Context, bus stream.Bus, o options, out io.Writer) (report, error) {
	r := report{keys: map[string]int{}}
	var spans map[int32]stream.Span
	if o.toEnd {
		adm, err := bus.Admin(ctx)
		if err != nil {
			return r, err
		}
		spans, err = adm.Spans(ctx, raw.Topic)
		adm.Close(ctx)
		if err != nil {
			return r, err
		}
	}
	start := stream.StartFromCommitted
	if o.fromBeginning {
		start = stream.StartFromBeginning
	}
	src, err := bus.Source(ctx, stream.SubscribeOptions{Group: o.name, Topics: []string{raw.Topic}, Start: start})
	if err != nil {
		return r, err
	}
	defer src.Close(context.Background())
	w := bufio.NewWriter(out)
	first := map[raw.Identity][]byte{}
	next := map[int32]int64{}
	// done: every partition read to its end as it stood, or holding nothing then
	done := func() bool {
		for p, s := range spans {
			if max(next[p], s.Start) < s.End {
				return false
			}
		}
		return true
	}
	for !(o.toEnd && done()) {
		recs, err := src.Fetch(ctx)
		if err != nil {
			return r, err
		}
		for _, rec := range recs {
			if o.toEnd && rec.Offset >= spans[rec.Partition].End {
				continue // published after the run began
			}
			m, err := raw.FromRecord(rec)
			if err != nil {
				return r, err
			}
			r.read++
			r.keys[m.Key()]++
			next[rec.Partition] = rec.Offset + 1
			id := m.Identity()
			if before, ok := first[id]; ok {
				r.repeats++
				if string(before) != string(rec.Value) {
					r.changed++
				}
			} else {
				first[id] = rec.Value
			}
			if o.dedup {
				continue
			}
			if err := line(w, rec.Value); err != nil {
				return r, err
			}
			r.written++
		}
		if !o.dedup {
			if err := w.Flush(); err != nil {
				return r, err
			}
			if err := src.Commit(ctx, recs...); err != nil {
				return r, err
			}
		}
	}
	if o.dedup {
		ids := make([]raw.Identity, 0, len(first))
		for id := range first {
			ids = append(ids, id)
		}
		slices.SortFunc(ids, func(a, b raw.Identity) int {
			if c := strings.Compare(a.Key, b.Key); c != 0 {
				return c
			}
			switch {
			case a.Date < b.Date:
				return -1
			case a.Date > b.Date:
				return 1
			}
			return 0
		})
		for _, id := range ids {
			if err := line(w, first[id]); err != nil {
				return r, err
			}
			r.written++
		}
	}
	return r, w.Flush()
}

func line(w *bufio.Writer, v []byte) error {
	if _, err := w.Write(v); err != nil {
		return err
	}
	return w.WriteByte('\n')
}
