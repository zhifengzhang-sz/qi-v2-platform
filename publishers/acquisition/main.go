// acquisition, a publisher on the core (core/README.md): it runs a catalog dataset's source plugin
// (docs/impl/engineering/plugins.md, 3) for each of its series and publishes every bar of the series' grid, the
// dataset's session, on the core's topics: each bar's first version on market.raw, as raw market data 0.3.0, origin
// live, a bar the plugin gives no line for the zero K-line; each later version that differs from the bar's latest on
// market.corrections, as market corrections 0.1.0, naming the fields that differ. It knows no source: the dataset names
// its source and its plugin; the K-line the source's messages carry is the source's, read by the core
// (raw.DecodeKLine). Its state is the record, the two topics themselves, read from their start at each run: a bar's
// index is its place in the series' grid from the series' first bar, and never moves. After each series' run, the
// dataset's cleaning plugin keeps the series' clean topic (clean.go): its tail, the damage, a reset.
//
//	acquisition -dataset NAME -corrected COMMAND -store COMMAND [-store COMMAND]… [-catalog config/catalog.yaml]
//	            [-brokers localhost:19092] [-name acquisition] [-lines FILE] [-follow]
//
// -corrected is the command of the store whose corrected raw a reset reads, the TimescaleDB store's writer
// (`COMMAND -corrected KEY`); each -store, the command of a store whose clean series a reset writes, each store's
// writer (`COMMAND -clean TOPIC`); a command its words parted by spaces.
//
// A run asks for the span from the later of the series' first bar and the dataset's reread before now (from the bar
// after the series' last on market.raw, when that is earlier; a series with none: the history's first bar) to the last
// bar of the grid whose end is the dataset's delay before now, the plugin run as `COMMAND… --backfill
// INSTRUMENT:TIMEFRAME FROM TO [FROM TO]…`, each session day's [open, close) cut to the span (one span for a dataset
// without a session), its stderr passed through; with no bar in the span, the plugin is not run. Its bars are taken
// once it has read the whole of the spans, its exit 0, and published then. -lines keeps the lines the plugin prints.
// The record not the grid's (a bar of the grid missing before the series' last, a bar not of the grid, an index not
// its place), a line that is not the source's K-line in market's canonical text, a bar not after the one before it,
// outside the spans asked or not of the grid, or the plugin's exit other than 0 stops the run, nothing of the series
// published. A line a series a run, on stdout: the record as it found it, the span asked, what it published.
// -follow keeps it running: a run at its start, then one whenever the last bar of the next session becomes the
// dataset's delay old (once a session day), until it is stopped (SIGINT, SIGTERM); a run that fails is a line on stdout,
// and the next comes at its time, asking from the bar after the series' last on market.raw. A second line a series a
// run: the clean topic, the tail published, the damage, a reset made.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/kafka"
)

func main() {
	var (
		catalog = flag.String("catalog", "config/catalog.yaml", "the catalog")
		name    = flag.String("dataset", "", "the catalog's dataset to acquire")
		brokers = flag.String("brokers", "localhost:19092", "the brokers, host:port, comma-separated (the local stack's Redpanda by default)")
		me      = flag.String("name", "acquisition", "this component's name: the messages' publisher, and the group it reads the record in")
		keep    = flag.String("lines", "", "a file to write the plugin's lines to, as read")
		follow  = flag.Bool("follow", false, "keep running: a run at the start, then one whenever the last bar of the next session becomes the dataset's delay old")
		from    = flag.String("corrected", "", "the command of the store whose corrected raw a reset reads, the TimescaleDB store's writer (COMMAND -corrected KEY)")
		stores  commands
	)
	flag.Var(&stores, "store", "the command of a store whose clean series a reset writes (COMMAND -clean TOPIC); once a store")
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if *name == "" {
		fatal(log, fmt.Errorf("-dataset: the catalog's dataset to acquire"))
	}
	d, err := loadDataset(*catalog, *name)
	if err != nil {
		fatal(log, err)
	}
	if strings.TrimSpace(*from) == "" || len(stores) == 0 {
		fatal(log, fmt.Errorf("-corrected and -store: the stores a reset reads and writes"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg := kafka.Config{Brokers: strings.Split(*brokers, ","), ClientID: *me}
	if err := kafka.WaitReady(ctx, cfg, 30*time.Second); err != nil {
		fatal(log, err)
	}
	bus := kafka.New(cfg)
	a := &acquirer{bus: bus, dataset: d, publisher: *me, now: time.Now, out: os.Stdout,
		source: command{argv: strings.Fields(*from), stderr: os.Stderr}}
	for _, s := range stores {
		a.stores = append(a.stores, command{argv: s, stderr: os.Stderr})
	}
	var kept *bufio.Writer
	var keptFile *os.File
	if *keep != "" {
		if keptFile, err = os.Create(*keep); err != nil {
			fatal(log, err)
		}
		kept = bufio.NewWriter(keptFile)
		a.lines = kept
	}
	if *follow {
		err = a.follow(ctx)
		if errors.Is(err, context.Canceled) {
			err = nil // stopped
		}
	} else {
		err = a.run(ctx)
	}
	if kept != nil {
		if ferr := kept.Flush(); ferr != nil {
			fatal(log, fmt.Errorf("-lines: %w", ferr))
		}
		if ferr := keptFile.Close(); ferr != nil {
			fatal(log, fmt.Errorf("-lines: %w", ferr))
		}
	}
	if err != nil {
		fatal(log, err)
	}
}

// commands is a flag given once a command, its words parted by spaces.
type commands [][]string

func (c *commands) String() string { return fmt.Sprint(*c) }

func (c *commands) Set(v string) error {
	f := strings.Fields(v)
	if len(f) == 0 {
		return fmt.Errorf("a command, its words parted by spaces")
	}
	*c = append(*c, f)
	return nil
}

func fatal(log *slog.Logger, err error) {
	log.Error("fatal", "err", err.Error())
	os.Exit(1)
}
