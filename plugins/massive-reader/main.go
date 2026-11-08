// massive-reader is Massive's reader, a source plugin (docs/impl/engineering/plugins.md, 3), a thin program over the
// platform's library (lib/massive, which holds every rule of reading Massive): it reads Massive's aggregates (formerly
// Polygon.io's) for a series and its spans and prints its closed bars in them, one a line, ascending, each Massive's
// K-line (massive.KLine) in its canonical text, every field of Massive's row kept. A line's index is its place among
// the lines printed, from 0; the series' own index is its publisher's.
//
//	massive-reader [-key FILE] [-interval 12s] [-pages DIR] --backfill INSTRUMENT:TIMEFRAME FROM TO [FROM TO]…
//
// A span is FROM TO, milliseconds since 1970 (UTC), the bars' begin times, both included; the spans ascending, each
// after the one before. The timeframes: M1, M5, M15, M30, H1. Massive is asked once for the whole of them
// (massive.Fetcher), each page saved under -pages (by default ~/.local/share/qi/massive/pages) before it is read, a
// page named by its content and kept once; a page or a row that is not what the library reads (massive.ReadPage) stops the run. The key is read by the library
// from -key (default ~/.config/qi/massive_key). On stderr, one line at the end: the rows Massive gave, the lines
// printed, the rows not closed yet and those outside the spans, and where the pages are, those saved and those kept
// already. Exit 0 once every page is
// read; 1, with `massive-reader: ` and the fault on stderr.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/lib/massive"
)

func main() {
	var (
		backfill = flag.Bool("backfill", false, "print the bars of INSTRUMENT:TIMEFRAME whose begin time is in a span [FROM, TO], ms")
		keyFile  = flag.String("key", massive.KeyFile(), "the key's file, one line")
		interval = flag.Duration("interval", massive.Interval, "the least time between two calls (the free plan: five a minute)")
		pages    = flag.String("pages", massive.PagesDir(), "where the pages are saved, a series a folder, a page a file named by its content")
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: massive-reader [-key FILE] [-interval 12s] [-pages DIR] --backfill INSTRUMENT:TIMEFRAME FROM TO [FROM TO]…")
		flag.PrintDefaults()
	}
	flag.Parse()
	if !*backfill {
		fail(errors.New("--backfill INSTRUMENT:TIMEFRAME FROM TO [FROM TO]…: the only mode (no live stream)"))
	}
	r, err := parseSpan(flag.Args())
	if err != nil {
		fail(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	f := &massive.Fetcher{KeyFile: *keyFile, Interval: *interval, Dir: *pages, Log: os.Stderr}
	out := bufio.NewWriter(os.Stdout)
	c, got, err := run(ctx, f, r, time.Now(), out)
	if ferr := out.Flush(); err == nil {
		err = ferr
	}
	saved := ""
	if got.Dir != "" {
		saved = fmt.Sprintf("; the pages in %s, %d saved, %d kept already", got.Dir, got.Saved, len(got.Pages)-got.Saved)
	}
	fmt.Fprintf(os.Stderr, "massive-reader: %s:%s, %d spans in [%d, %d]: %d rows given by Massive in %d pages; %d printed, %d not closed, %d outside the spans%s\n",
		r.Instrument, r.Timeframe, len(r.Spans), r.From(), r.To(), c.Rows, c.Pages, c.Lines, c.Open, c.Outside, saved)
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "massive-reader: %v\n", err)
	os.Exit(1)
}

// parseSpan reads the arguments of --backfill as a request the library checks.
func parseSpan(args []string) (massive.Request, error) {
	if len(args) < 3 || len(args)%2 != 1 {
		return massive.Request{}, fmt.Errorf("--backfill INSTRUMENT:TIMEFRAME FROM TO [FROM TO]…: %d arguments", len(args))
	}
	instrument, timeframe, ok := strings.Cut(args[0], ":")
	if !ok {
		return massive.Request{}, fmt.Errorf("series %q: INSTRUMENT:TIMEFRAME", args[0])
	}
	r := massive.Request{Instrument: instrument, Timeframe: timeframe}
	for i := 1; i < len(args); i += 2 {
		from, err1 := strconv.ParseUint(args[i], 10, 64)
		to, err2 := strconv.ParseUint(args[i+1], 10, 64)
		if err1 != nil || err2 != nil {
			return massive.Request{}, fmt.Errorf("span %s %s: FROM ≤ TO, milliseconds since 1970", args[i], args[i+1])
		}
		r.Spans = append(r.Spans, [2]uint64{from, to})
	}
	return r, r.Check()
}

// run fetches the request's pages and prints its bars, a page's lines flushed once the page is read.
func run(ctx context.Context, f *massive.Fetcher, r massive.Request, now time.Time, out *bufio.Writer) (massive.Counts, massive.Fetched, error) {
	bars := massive.NewBars(r, now)
	got, err := f.Fetch(ctx, r, func(p massive.Page) error {
		if err := bars.Page(p, func(line string) error { _, err := out.WriteString(line); return err }); err != nil {
			return err
		}
		return out.Flush()
	})
	return bars.Counts, got, err
}
