// clean-follower, the tutorial's consumer on the core (docs/tutorial/medium/README.md): it follows a clean topic in the
// group of its name, as the core's rule on a component's identity says (core/README.md), and keeps each K-line it
// takes as a line of its file, the line written and synced before its offset is committed.
//
//	clean-follower -topic TOPIC -out FILE [-brokers localhost:29092] [-name clean-follower] [-max N]
//
// Its file is what it has done: a line a K-line taken, its index, its date (UTC), open, high, low, close and volume,
// as clean-reader prints them (../../basic/clean-reader). Its group's committed offset on the topic is where it takes
// up. Started with one, it continues from it, its file as the run before left it; started with none (its first run, or
// the topic made again, which deletes every group's offsets on it), it reads the topic from its start and writes its
// file anew. A copy, a value of a key and a date taken already, is dropped, the first taken, as the schema's delivery
// says: a clean topic is one series', each value's key the topic's (clean.FromRecord checks it), so its key and date are
// its date, and its file, the one topic's, holds the dates taken. So a value delivered again after a stop, its line
// written but its offset not yet committed, is not taken twice. One file a topic: a file another topic's would hold
// another series' dates.
// It follows the topic until it is stopped (SIGINT, SIGTERM), or until it has taken N K-lines in this run (-max).
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/kafka"
)

// minute is how a K-line's date is written in its line.
const minute = "2006-01-02T15:04Z07:00"

func main() {
	var (
		brokers = flag.String("brokers", "localhost:29092", "the brokers, host:port, comma-separated (the test stack's)")
		topic   = flag.String("topic", "", "the clean topic it follows")
		out     = flag.String("out", "", "its file, a line a K-line taken")
		name    = flag.String("name", "clean-follower", "its name, and the group it reads in")
		max     = flag.Int("max", 0, "stop once it has taken N K-lines in this run; 0, follow until stopped")
	)
	flag.Parse()
	if *topic == "" || *out == "" || *max < 0 || flag.NArg() > 0 {
		fail(errors.New("-topic TOPIC -out FILE [-brokers HOST:PORT] [-name NAME] [-max N]"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	bus := kafka.New(kafka.Config{Brokers: strings.Split(*brokers, ","), ClientID: *name})
	if err := follow(ctx, bus, *name, *topic, *out, *max); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "clean-follower: %v\n", err)
	os.Exit(1)
}

func follow(ctx context.Context, bus stream.Bus, name, topic, out string, max int) error {
	// where it takes up: its group's committed offset on the topic, and the topic's end as it stands now
	adm, err := bus.Admin(ctx)
	if err != nil {
		return err
	}
	spans, err := adm.Spans(ctx, topic)
	if err != nil {
		return err
	}
	groups, err := adm.Groups(ctx, topic)
	adm.Close(context.Background())
	if err != nil {
		return err
	}
	at, resume := int64(0), false
	for _, g := range groups {
		if g.Name == name {
			at, resume = g.Committed[0], len(g.Committed) > 0
		}
	}
	dates, err := done(out, resume)
	if err != nil {
		return err
	}
	mode := os.O_TRUNC // anew
	if resume {
		mode = os.O_APPEND
	}
	f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|mode, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if resume {
		fmt.Fprintf(os.Stderr, "clean-follower: %s: from offset %d, group %s's committed offset; %d K-lines in %s\n",
			topic, at, name, len(dates), out)
	} else {
		fmt.Fprintf(os.Stderr, "clean-follower: %s: from its start, no offset committed in group %s; %s written anew\n",
			topic, name, out)
	}
	end := spans[0].End
	if at >= end {
		fmt.Fprintf(os.Stderr, "clean-follower: at the topic's end, offset %d; following\n", end)
	}
	src, err := bus.Source(ctx, stream.SubscribeOptions{Group: name, Topics: []string{topic}, Start: stream.StartFromCommitted})
	if err != nil {
		return err
	}
	defer src.Close(context.Background())
	w := bufio.NewWriter(f)
	taken, copies := 0, 0
	for max == 0 || taken < max {
		recs, err := src.Fetch(ctx)
		if ctx.Err() != nil {
			break // stopped
		}
		if err != nil {
			return err
		}
		var last *stream.Record
		for i, r := range recs {
			m, err := clean.FromRecord(r)
			if err != nil {
				return err
			}
			k := m.KLine.Base()
			if dates[k.Date] {
				copies++
				fmt.Fprintf(os.Stderr, "clean-follower: offset %d: a copy of %s, dropped\n", r.Offset,
					time.UnixMilli(int64(k.Date)).UTC().Format(minute))
			} else {
				dates[k.Date] = true
				taken++
				fmt.Fprintf(w, "%d %s %s %s %s %s %s\n", k.Index, time.UnixMilli(int64(k.Date)).UTC().Format(minute),
					k.Open, k.High, k.Low, k.Close, k.Volume)
			}
			last = &recs[i]
			if r.Offset == end-1 {
				fmt.Fprintf(os.Stderr, "clean-follower: at the topic's end as it stood at the start, offset %d: %d taken, "+
					"%d copies dropped; following\n", end, taken, copies)
			}
			if taken == max {
				break
			}
		}
		if last == nil {
			continue
		}
		// what it has done, then its offset: the lines written and synced, then committed
		if err := w.Flush(); err != nil {
			return err
		}
		if err := f.Sync(); err != nil {
			return err
		}
		if err := src.Commit(context.WithoutCancel(ctx), *last); err != nil {
			return err
		}
		at = last.Offset + 1
	}
	fmt.Fprintf(os.Stderr, "clean-follower: stopped: %d taken, %d copies dropped; %d K-lines in %s; committed offset %d\n",
		taken, copies, len(dates), out, at)
	return nil
}

// done is the dates of the lines of the file the run before left, when it continues; none, when it starts anew. The
// file of a group that has committed must be there: it is what the group's offset says was done.
func done(out string, resume bool) (map[uint64]bool, error) {
	dates := map[uint64]bool{}
	if !resume {
		return dates, nil
	}
	f, err := os.Open(out)
	if err != nil {
		return nil, fmt.Errorf("its group has committed, so its file is what it has done: %w", err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for n := 1; s.Scan(); n++ {
		fields := strings.Fields(s.Text())
		if len(fields) != 7 {
			return nil, fmt.Errorf("%s:%d: not a line of its", out, n)
		}
		t, err := time.Parse(minute, fields[1])
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", out, n, err)
		}
		dates[uint64(t.UnixMilli())] = true
	}
	return dates, s.Err()
}
