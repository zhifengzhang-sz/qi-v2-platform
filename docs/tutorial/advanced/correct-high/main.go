// correct-high, the tutorial's test tool (docs/tutorial/advanced/README.md): corrections written on the test stack's
// market.corrections, each a minute's latest version with its high 0.01 more, so that a clean topic is damaged and a
// cleaning's tolerance and its reset can be tried. It writes the test stack alone: it refuses any broker but the test
// stack's, localhost:29092, since on the stack market.corrections is acquisition's alone and keeps every message
// forever.
//
//	correct-high -at T [-minutes N] [-series massive:SPY:M1] [-brokers localhost:29092]
//
// T is the first minute, with its offset or Z (2026-09-15T10:00-04:00, as query shows a minute), each next minute a
// minute after it, N in all (1 by default); each must be on market.raw. Each minute's latest version, its correction
// of the greatest read, else its first version, is read from market.raw and market.corrections, from their start to
// their end, in no group; its correction is that K-line, its high 0.01 more, read now, corrected ["high"], publisher
// correct-high, published on market.corrections as market corrections 0.1.0 (core/README.md). A line a correction on
// stdout.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/kafka"
)

// testStack is the one broker it writes: the test stack's (docs/impl/engineering/dev.environment.md, 3).
const testStack = "localhost:29092"

const minute = "2006-01-02T15:04Z07:00"

func main() {
	var (
		brokers = flag.String("brokers", testStack, "the test stack's broker, the only one it writes")
		series  = flag.String("series", "massive:SPY:M1", "the series, <source>:<instrument>:<timeframe>")
		at      = flag.String("at", "", "the first minute, with its offset or Z (2026-09-15T10:00-04:00)")
		minutes = flag.Int("minutes", 1, "how many minutes, each a minute after the one before")
	)
	flag.Parse()
	if *brokers != testStack {
		fail(fmt.Errorf("-brokers %s: only the test stack's, %s: on the stack, market.corrections is acquisition's "+
			"alone and keeps every message forever", *brokers, testStack))
	}
	first, err := time.Parse(minute, *at)
	if err != nil || *minutes < 1 || flag.NArg() > 0 {
		fail(errors.New("-at T (2026-09-15T10:00-04:00) [-minutes N] [-series KEY]"))
	}
	ctx := context.Background()
	if err := correct(ctx, kafka.New(kafka.Config{Brokers: []string{testStack}, ClientID: "correct-high"}), *series,
		first, *minutes); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "correct-high: %v\n", err)
	os.Exit(1)
}

func correct(ctx context.Context, bus stream.Bus, series string, first time.Time, n int) error {
	firsts, latest, err := versions(ctx, bus, series)
	if err != nil {
		return err
	}
	read := uint64(time.Now().UnixMilli())
	var recs []stream.Record
	var lines []string
	for i := range n {
		t := first.Add(time.Duration(i) * time.Minute)
		date := uint64(t.UnixMilli())
		m, ok := firsts[date]
		if !ok {
			return fmt.Errorf("%s at %s: no such minute on market.raw", series, t.Format(minute))
		}
		k := m.KLine
		if c, ok := latest[date]; ok {
			k = c.KLine
		}
		higher, err := withHigh(k)
		if err != nil {
			return err
		}
		corrected, err := raw.Differ(k, higher)
		if err != nil {
			return err
		}
		m.KLine, m.Publisher, m.Read = higher, "correct-high", read
		r, err := raw.Correction{Message: m, Corrected: corrected}.Record()
		if err != nil {
			return err
		}
		recs = append(recs, r)
		lines = append(lines, fmt.Sprintf("%s at %s: high %s, now %s, corrected %v", series, t.Format(minute),
			k.Base().High, higher.Base().High, corrected))
	}
	sink, err := bus.Sink(ctx)
	if err != nil {
		return err
	}
	defer sink.Close(context.Background())
	if err := sink.Publish(ctx, recs...); err != nil {
		return err
	}
	for _, l := range lines {
		fmt.Printf("%s; read %s, on market.corrections\n", l, time.UnixMilli(int64(read)).UTC().Format("2006-01-02T15:04:05.000Z"))
	}
	return nil
}

// versions reads the series on market.raw and market.corrections from their start to their end as they stood when
// the read began, in no group: each minute's first version, its first copy, and its correction of the greatest read.
func versions(ctx context.Context, bus stream.Bus, series string) (map[uint64]raw.Message, map[uint64]raw.Correction, error) {
	adm, err := bus.Admin(ctx)
	if err != nil {
		return nil, nil, err
	}
	left := map[string]map[int32]int64{} // the next offset each partition must reach
	for _, t := range raw.Topics {
		spans, err := adm.Spans(ctx, t)
		if err != nil {
			adm.Close(context.Background())
			return nil, nil, err
		}
		left[t] = map[int32]int64{}
		for p, s := range spans {
			if s.End > s.Start {
				left[t][p] = s.End
			}
		}
		if len(left[t]) == 0 {
			delete(left, t)
		}
	}
	adm.Close(context.Background())
	firsts, latest := map[uint64]raw.Message{}, map[uint64]raw.Correction{}
	src, err := bus.Source(ctx, stream.SubscribeOptions{Topics: raw.Topics}) // no group: every partition from its start
	if err != nil {
		return nil, nil, err
	}
	defer src.Close(context.Background())
	for len(left) > 0 {
		recs, err := src.Fetch(ctx)
		if err != nil {
			return nil, nil, err
		}
		for _, r := range recs {
			end, ok := left[r.Topic][r.Partition]
			if !ok || r.Offset >= end {
				continue
			}
			if r.Offset == end-1 {
				delete(left[r.Topic], r.Partition)
				if len(left[r.Topic]) == 0 {
					delete(left, r.Topic)
				}
			}
			if string(r.Key) != series {
				continue
			}
			if r.Topic == raw.Topic {
				m, err := raw.FromRecord(r)
				if err != nil {
					return nil, nil, err
				}
				if _, ok := firsts[m.KLine.Base().Date]; !ok {
					firsts[m.KLine.Base().Date] = m
				}
				continue
			}
			c, err := raw.CorrectionFromRecord(r)
			if err != nil {
				return nil, nil, err
			}
			if was, ok := latest[c.KLine.Base().Date]; !ok || c.Read > was.Read {
				latest[c.KLine.Base().Date] = c
			}
		}
	}
	return firsts, latest, nil
}

// withHigh is the K-line with its high 0.01 more, every other field as it is: its canonical text, its high replaced,
// read again by the core (raw.DecodeKLine), which holds it to market's rules and its canonical text.
func withHigh(k raw.KLine) (raw.KLine, error) {
	var f map[string]json.RawMessage
	if err := json.Unmarshal([]byte(k.Text()), &f); err != nil {
		return raw.KLine{}, err
	}
	h, _ := new(big.Rat).SetString(k.Base().High.String())
	h.Add(h, big.NewRat(1, 100))
	d, err := market.ParseDecimal(strings.TrimSuffix(strings.TrimRight(h.FloatString(6), "0"), "."))
	if err != nil {
		return raw.KLine{}, err
	}
	f["high"] = json.RawMessage(d.String())
	text, err := json.Marshal(f) // a map's names in their order, as the canonical text has them
	if err != nil {
		return raw.KLine{}, err
	}
	return raw.DecodeKLine(text)
}
