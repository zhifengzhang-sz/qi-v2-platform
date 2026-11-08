// provision makes the core's topics, market.raw and market.corrections (core/README.md), on the
// broker, alike: their partitions, every record kept, no compaction, each record's
// timestamp its publisher's, each set on the topic itself. Idempotent: on a
// topic already there it sets the schemas' configuration again, saying what
// it changed, and refuses one of another partition count, cleanup policy or
// timestamp type. A line a topic.
//
//	provision [-brokers localhost:19092] [-replicas 1]
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/kafka"
)

func main() {
	var (
		brokers  = flag.String("brokers", "localhost:19092", "the brokers, host:port, comma-separated (the local stack's Redpanda by default)")
		replicas = flag.Int("replicas", 1, "replication factor (1 on the local stack; 3 on a real cluster)")
	)
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if *replicas < 1 || *replicas > math.MaxInt16 {
		fatal(log, fmt.Errorf("-replicas %d: a replication factor from 1 to %d", *replicas, math.MaxInt16))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg := kafka.Config{Brokers: strings.Split(*brokers, ","), ClientID: "provision"}
	if err := kafka.WaitReady(ctx, cfg, 30*time.Second); err != nil {
		fatal(log, err)
	}
	adm, err := kafka.New(cfg).Admin(ctx)
	if err != nil {
		fatal(log, err)
	}
	defer adm.Close(context.Background())
	ps, err := raw.Provision(ctx, adm, int16(*replicas))
	for _, p := range ps {
		what := "exists"
		if p.Created {
			what = "created"
		}
		for _, k := range slices.Sorted(maps.Keys(p.Set)) {
			what += fmt.Sprintf("; %s was %q, set to the schema's", k, p.Set[k])
		}
		fmt.Printf("%s %d partitions, retention unlimited, timestamps %s, %s\n", p.Topic, raw.Partitions,
			raw.TimestampType, what)
	}
	if err != nil {
		fatal(log, err)
	}
}

func fatal(log *slog.Logger, err error) {
	log.Error("fatal", "err", err.Error())
	os.Exit(1)
}
