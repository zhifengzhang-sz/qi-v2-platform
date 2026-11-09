// dataset, a consumer on the core (docs/designs/design-klines-dataset-2026-11-09.md): a K-line data set made from one
// clean topic, read in no group: its files, one a month of its sessions' dates in the zone, each line market 1.1.0's
// K-line; its image, one layer a file, as an OCI image layout; and its lock, the image's tag and digest and every
// file's sha256. It writes no topic, joins no group and commits no offset.
//
//	dataset -topic TOPIC -zone ZONE -from DATE -to DATE -name PRODUCT -version VERSION -image REPOSITORY -out DIR
//	        [-brokers localhost:19092]
//
// It reads the topic's end as it stands, then the topic from its start to that end, each value as clean market data
// 0.1.0: a copy, a K-line of a date it has taken, dropped, the first taken; else the next K-line, its index the last
// one's and 1 and its date after the last one's; anything else stops it, named. It keeps the K-lines of the sessions
// from -from to -to, both included, their dates in -zone, the topic covering them; then it makes DIR, which must not
// exist, and writes files/, oci/ and PRODUCT.lock in it, the same bytes each time from the same topic. It exits 0 with
// a line; 2 on a flag missing or not known; 1 on any other fault, a flag's value refused among them. Ctrl-C or SIGTERM
// stops it, 1.
package main

import (
	"context"
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

const usage = "-topic TOPIC -zone ZONE -from YYYY-MM-DD -to YYYY-MM-DD -name PRODUCT -version MAJOR.MINOR.PATCH " +
	"-image REPOSITORY -out DIR [-brokers HOST:PORT]"

func main() {
	var (
		topic   = flag.String("topic", "", "the clean topic, as query topics lists it")
		zone    = flag.String("zone", "", "the series' zone, whose dates part its sessions and its months (America/New_York)")
		from    = flag.String("from", "", "the range's first session, its date in the zone, YYYY-MM-DD")
		to      = flag.String("to", "", "the range's last session, its date in the zone, YYYY-MM-DD")
		name    = flag.String("name", "", "the data set: the product, the image's name")
		version = flag.String("version", "", "its version, MAJOR.MINOR.PATCH, the image's tag")
		image   = flag.String("image", "", "the image's repository, ending in the product (ghcr.io/OWNER/PRODUCT)")
		out     = flag.String("out", "", "the folder it makes and writes, which must not exist: files/, oci/, PRODUCT.lock")
		brokers = flag.String("brokers", "localhost:19092", "the brokers, host:port, comma-separated (the local stack's Redpanda by default)")
	)
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if *topic == "" || *zone == "" || *from == "" || *to == "" || *name == "" || *version == "" || *image == "" ||
		*out == "" || flag.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "dataset", usage)
		os.Exit(2)
	}
	s, err := parse(*topic, *zone, *from, *to, *name, *version, *image)
	if err != nil {
		fatal(log, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg := kafka.Config{Brokers: strings.Split(*brokers, ","), ClientID: "dataset"}
	if err := kafka.WaitReady(ctx, cfg, 30*time.Second); err != nil {
		fatal(log, err)
	}
	bus := kafka.New(cfg)
	defer bus.Close(context.Background())
	if err := run(ctx, bus, s, *out, os.Stdout); err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("stopped: %w", err)
		}
		fatal(log, err)
	}
}

func fatal(log *slog.Logger, err error) {
	log.Error("fatal", "err", err.Error())
	os.Exit(1)
}
