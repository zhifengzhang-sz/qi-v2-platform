// chart, the charting consumer, an addition to the core (docs/designs/design-chart-consumer-2026-11-09.md): one series'
// chart in a browser, drawn by @qi/chart and updated as the series' K-lines come, of the K-lines and their volume
// alone. It reads one clean topic in the group of its name, writes no topic and commits no offset, and serves the page
// and its feed.
//
//	chart -topic TOPIC -page DIR [-brokers localhost:19092] [-listen localhost:8090] [-name chart] [-window 3900]
//
// At its start it reads the topic's end as it stands, then the topic from its start, in the group of its name, each
// value as clean market data 0.1.0: a copy, a K-line of a date it has taken, dropped, the first taken; else the next
// K-line, its index the last one's and 1 and its date after the last one's; anything else stops it, named. Once it has
// read to that end it serves, at -listen, the page (DIR's files, each checked at its start) and the feed, server-sent
// events: a start of the last window of K-lines, each its text, market 1.1.0's K-line in market's canonical text; then
// each K-line as it comes; a start again when a page's chart holds twice the window. While it runs its group holds its
// series' reset off; a topic made again under it stops it with a fault. Ctrl-C or SIGTERM closes its listener, ends
// each page's feed and closes its source, so that its group has no member, and it exits 0 with a line.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/kafka"
)

func main() {
	var (
		topic   = flag.String("topic", "", "the clean topic, as query topics lists it; its series is the topic's")
		page    = flag.String("page", "", "the page's folder (bin/chart-page)")
		brokers = flag.String("brokers", "localhost:19092", "the brokers, host:port, comma-separated (the local stack's Redpanda by default; the test stack's localhost:29092)")
		listen  = flag.String("listen", "localhost:8090", "where it serves the page and its feed")
		name    = flag.String("name", "chart", "this component's name, its consumer group")
		window  = flag.Int("window", 3900, "the K-lines a page is drawn whole with")
	)
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if *topic == "" || *page == "" || *window < 1 || flag.NArg() > 0 {
		fatal(log, errors.New("-topic TOPIC -page DIR [-brokers HOST:PORT] [-listen HOST:PORT] [-name NAME] [-window N, N ≥ 1]"))
	}
	files, err := load(*page)
	if err != nil {
		fatal(log, err)
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fatal(log, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); stop() }() // a second signal ends it at once
	cfg := kafka.Config{Brokers: strings.Split(*brokers, ","), ClientID: *name}
	if err := kafka.WaitReady(ctx, cfg, 30*time.Second); err != nil {
		ln.Close()
		fatal(log, err)
	}
	o := options{topic: *topic, name: *name, window: *window}
	if err := run(ctx, kafka.New(cfg), o, files, ln, os.Stderr); err != nil {
		fatal(log, err)
	}
}

func fatal(log *slog.Logger, err error) {
	log.Error("fatal", "err", err.Error())
	os.Exit(1)
}
