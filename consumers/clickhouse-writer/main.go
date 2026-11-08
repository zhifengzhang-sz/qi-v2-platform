// clickhouse-writer, a consumer on the core (README.md): it reads the core's two topics, market.raw and
// market.corrections, and keeps their Massive K-lines in a database of ClickHouse, four series read with FINAL: the
// raw, each minute's first version; the corrections; the corrected raw, each minute's latest version; the clean,
// written from a clean topic when it is made.
//
//	clickhouse-writer [-brokers localhost:19092] [-name clickhouse-writer]
//	    [-url http://default:password@localhost:8123] [-database massive] [-to-end] [-clean TOPIC]
//
// Its consumer group is its name. It makes its database when it is not there and migrates it, then reads from its
// group's committed offsets, or from the topics' start when the raw holds no bar, and keeps running until it is
// stopped; -to-end reads the topics from their start to their end as they stood when it began, and stops. It commits
// what it has written. On stderr: where it takes up, what it writes, and at the end what it read and wrote.
//
// Acquisition's reset runs it in a mode of its own (clean.go), reading in no group and committing nothing, then
// stopping: -clean TOPIC, the store's clean series of the clean topic's series emptied and written from the topic, every
// K-line of it.
package main

import (
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
		brokers  = flag.String("brokers", "localhost:19092", "the brokers, host:port, comma-separated (the local stack's Redpanda by default)")
		name     = flag.String("name", "clickhouse-writer", "this component's name, its consumer group")
		url      = flag.String("url", "http://default:password@localhost:8123", "the ClickHouse server, by its HTTP interface's URL (the local stack's by default)")
		database = flag.String("database", "massive", "the store's database, made when it is not there")
		toEnd    = flag.Bool("to-end", false, "read the topics from their start to their end as they stand now, then stop")
		topic    = flag.String("clean", "", "write the store's clean series of the clean topic's series from the topic, every K-line of it, then stop")
	)
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	o := options{name: *name, toEnd: *toEnd}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	st, err := open(ctx, *url, *database)
	if err != nil {
		fatal(log, err)
	}
	defer st.Close(context.Background())
	cfg := kafka.Config{Brokers: strings.Split(*brokers, ","), ClientID: o.name}
	if err := kafka.WaitReady(ctx, cfg, 30*time.Second); err != nil {
		fatal(log, err)
	}
	if *topic != "" {
		r, err := cleanFrom(ctx, kafka.New(cfg), st, *topic, log)
		fmt.Fprintf(os.Stderr, "%s: -clean %s\n", o.name, r)
		if err != nil {
			fatal(log, err)
		}
		return
	}
	r, err := consume(ctx, kafka.New(cfg), st, o, log)
	fmt.Fprintf(os.Stderr, "%s: %s\n", o.name, r)
	if err != nil && !(errors.Is(err, context.Canceled) && !o.toEnd) {
		fatal(log, err)
	}
}

func fatal(log *slog.Logger, err error) {
	log.Error("fatal", "err", err.Error())
	os.Exit(1)
}
