// query, a consumer on the core (README.md): the common questions of the core's two topics answered from the stream,
// knowing their schemas (core/raw) and each series' zone (the catalog), so that no one needs a partition, an offset or
// a millisecond to read the record. It reads only, in no group: no topic written, no group joined or made, no offset
// committed.
//
//	query topics      [-stack stack|test]
//	query series      [-stack stack|test] [-catalog config/catalog.yaml] [-topic market.raw|market.corrections|CLEAN]
//	query bars        [-series KEY] [-first N | -last N | -from T -to T] [-topic market.raw|market.corrections|CLEAN]
//	                  [-stack stack|test] [-catalog config/catalog.yaml]
//	query minute      -series KEY -at T [-stack stack|test] [-catalog config/catalog.yaml]
//	query corrections -series KEY [-stack stack|test] [-catalog config/catalog.yaml]
//	query groups      [-stack stack|test]
//
// -stack is the stack read, the stack's Redpanda (localhost:19092) by default, or the test stack's (localhost:29092).
// KEY is a series' key, <source>:<instrument>:<timeframe>; a series is read from its instrument's partition alone. A
// series' zone is its dataset's session.zone in the catalog, UTC where none names one. T is a minute in the series'
// zone (2026-11-04T09:30), or with an offset or Z (2026-11-04T14:30Z), the form each minute is shown in. CLEAN is a
// clean topic, one series' (market.clean.<cleaning>.<version>.<source>.<instrument>.<timeframe>), read from its one
// partition, its KEY its name's series, -series given or not.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
)

// stacks are the brokers of each stack (docs/impl/engineering/dev.environment.md, 3).
var stacks = map[string]string{"stack": "localhost:19092", "test": "localhost:29092"}

const usage = `query reads the core's topics, read only, in no group:
  query topics      [-stack stack|test]
  query series      [-stack stack|test] [-catalog config/catalog.yaml] [-topic market.raw|market.corrections|CLEAN]
  query bars        [-series KEY] [-first N | -last N | -from T -to T] [-topic market.raw|market.corrections|CLEAN]
                    [-stack stack|test] [-catalog config/catalog.yaml]
  query minute      -series KEY -at T [-stack stack|test] [-catalog config/catalog.yaml]
  query corrections -series KEY [-stack stack|test] [-catalog config/catalog.yaml]
  query groups      [-stack stack|test]
KEY: <source>:<instrument>:<timeframe>, as massive:SPY:M1. T: a minute in the series' zone (2026-11-04T09:30),
or with an offset or Z (2026-11-04T14:30Z). CLEAN: a clean topic, as market.clean.validation.v0-1-0.massive.SPY.M1, its KEY its name's (-series not needed).
query <subcommand> -h: its flags.
`

// opener opens the record on the brokers.
type opener func(ctx context.Context, brokers []string) (record, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	open := func(ctx context.Context, brokers []string) (record, error) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return openKafka(ctx, brokers)
	}
	err := run(ctx, os.Args[1:], open, os.Stdout, os.Stderr)
	switch {
	case errors.Is(err, flag.ErrHelp):
	case errors.Is(err, errUsage):
		os.Exit(2)
	case err != nil:
		fmt.Fprintln(os.Stderr, "query:", strings.TrimPrefix(err.Error(), "query: "))
		os.Exit(1)
	}
}

// errUsage is a call not of the usage, said on stderr already.
var errUsage = errors.New("usage")

// run runs the subcommand args name with its flags, its output on out.
func run(ctx context.Context, args []string, open opener, out, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errUsage
	}
	cmd := args[0]
	fs := flag.NewFlagSet("query "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	stack := fs.String("stack", "stack", "the stack read: stack (localhost:19092) or test (localhost:29092)")
	var catalogPath, topic, key, at, from, to *string
	var first, last *int
	zoned := func() {
		catalogPath = fs.String("catalog", "config/catalog.yaml", "the catalog, whose datasets give each series' zone")
	}
	keyed := func() { key = fs.String("series", "", "the series' key, <source>:<instrument>:<timeframe>") }
	topical := func() {
		topic = fs.String("topic", raw.Topic, "the topic: "+raw.Topic+", "+raw.CorrectionsTopic+" or a clean topic, "+
			clean.Kind+".<cleaning>.<version>.<source>.<instrument>.<timeframe>")
	}
	switch cmd {
	case "topics", "groups":
	case "series":
		zoned()
		topical()
	case "bars":
		zoned()
		keyed()
		topical()
		first = fs.Int("first", 0, "the first N minutes (on "+raw.CorrectionsTopic+", corrections)")
		last = fs.Int("last", 0, "the last N minutes (on "+raw.CorrectionsTopic+", corrections)")
		from = fs.String("from", "", "the minutes from T, T included")
		to = fs.String("to", "", "the minutes to T, T included")
	case "minute":
		zoned()
		keyed()
		at = fs.String("at", "", "the minute T")
	case "corrections":
		zoned()
		keyed()
	case "-h", "-help", "--help", "help":
		fmt.Fprint(out, usage)
		return nil
	default:
		fmt.Fprintf(stderr, "query: no subcommand %q\n%s", cmd, usage)
		return errUsage
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return errUsage
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("query %s: %q: no argument but its flags", cmd, fs.Args())
	}
	brokers, ok := stacks[*stack]
	if !ok {
		return fmt.Errorf("query %s: -stack %q: stack or test", cmd, *stack)
	}
	if topic != nil && *topic != raw.Topic && *topic != raw.CorrectionsTopic {
		if _, err := cleanSeries(*topic); err != nil {
			return fmt.Errorf("query %s: -topic %q: %s, %s or a clean topic (%v)", cmd, *topic, raw.Topic,
				raw.CorrectionsTopic, err)
		}
	}
	var cat *catalog
	if catalogPath != nil {
		var err error
		if cat, err = loadCatalog(*catalogPath); err != nil {
			return err
		}
	}
	var s series
	var loc *time.Location
	if key != nil {
		if *key == "" && topic != nil && isClean(*topic) {
			*key, _ = cleanSeries(*topic) // a clean topic is one series': its name's
		}
		if *key == "" {
			return fmt.Errorf("query %s: no -series, the series' key", cmd)
		}
		var err error
		if s, err = parseSeries(*key); err != nil {
			return err
		}
		if loc, err = cat.zone(s); err != nil {
			return err
		}
		if topic != nil && isClean(*topic) {
			if of, _ := cleanSeries(*topic); of != s.key {
				return fmt.Errorf("query %s: -series %s: the clean topic %s is %s's", cmd, s.key, *topic, of)
			}
		}
	}
	var w window
	if cmd == "bars" {
		var err error
		if w, err = windowOf(*first, *last, *from, *to, loc); err != nil {
			return fmt.Errorf("query bars: %w", err)
		}
	}
	var t time.Time
	if cmd == "minute" {
		if *at == "" {
			return errors.New("query minute: no -at, the minute T")
		}
		var err error
		if t, err = parseMinute(*at, loc); err != nil {
			return err
		}
	}
	rec, err := open(ctx, []string{brokers})
	if err != nil {
		return err
	}
	defer rec.Close()
	switch cmd {
	case "topics":
		return topics(ctx, rec, out)
	case "series":
		return seriesOn(ctx, rec, cat, *topic, out)
	case "bars":
		return bars(ctx, rec, s, loc, *topic, w, out)
	case "minute":
		return minuteOf(ctx, rec, s, loc, t, out)
	case "corrections":
		return correctionsBy(ctx, rec, s, loc, out)
	}
	return groupsOf(ctx, rec, out)
}

// windowOf is bars' window from its flags: -first N, -last N, or -from T and -to T, one or both, at most one of the
// three; N from 1, T in the series' zone, -from not after -to.
func windowOf(first, last int, from, to string, loc *time.Location) (window, error) {
	given := 0
	for _, g := range []bool{first != 0, last != 0, from != "" || to != ""} {
		if g {
			given++
		}
	}
	if given > 1 {
		return window{}, errors.New("-first, -last, or -from and -to: one of them")
	}
	if first < 0 || last < 0 {
		return window{}, errors.New("-first and -last take a number of minutes from 1")
	}
	w := window{first: first, last: last}
	for _, f := range []struct {
		v  string
		to **uint64
	}{{from, &w.from}, {to, &w.to}} {
		if f.v == "" {
			continue
		}
		t, err := parseMinute(f.v, loc)
		if err != nil {
			return window{}, err
		}
		d := uint64(t.UnixMilli())
		*f.to = &d
	}
	if w.from != nil && w.to != nil && *w.from > *w.to {
		return window{}, fmt.Errorf("-from %s after -to %s", from, to)
	}
	return w, nil
}
