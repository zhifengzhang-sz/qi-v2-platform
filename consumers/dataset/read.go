package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // a zone resolves without the host's zoneinfo

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// spec is what a data set is made of: its clean topic, its zone and range, its product, version and image.
type spec struct {
	topic, zone          string
	loc                  *time.Location
	from, to             string // the range's first and last sessions' dates in the zone, YYYY-MM-DD
	name, version, image string
}

var (
	// product: an image's name as a registry takes one, lower-case letters and digits parted by one '.', '_' or '-'
	product = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)
	semver  = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
)

const day = "2006-01-02"

// parse is a run's spec from its flags, each value checked: a clean topic's name; a zone Go knows; two dates, the first
// not after the last; the product an image's name; the version MAJOR.MINOR.PATCH; the image a repository of a registry
// ending in the product.
func parse(topic, zone, from, to, name, version, image string) (spec, error) {
	if !strings.HasPrefix(topic, clean.Kind+".") {
		return spec{}, fmt.Errorf("-topic %q: a clean topic, %s.<cleaning>.<version>.<source>.<instrument>.<timeframe>",
			topic, clean.Kind)
	}
	loc, err := time.LoadLocation(zone)
	if err != nil || zone == "Local" {
		return spec{}, fmt.Errorf("-zone %q: a zone of the time zone database (America/New_York)", zone)
	}
	for _, d := range []struct{ flag, v string }{{"from", from}, {"to", to}} {
		if t, err := time.Parse(day, d.v); err != nil || t.Format(day) != d.v {
			return spec{}, fmt.Errorf("-%s %q: a date, YYYY-MM-DD", d.flag, d.v)
		}
	}
	if from > to {
		return spec{}, fmt.Errorf("-from %s after -to %s", from, to)
	}
	if !product.MatchString(name) {
		return spec{}, fmt.Errorf("-name %q: an image's name, lower-case letters and digits parted by '.', '_' or '-'", name)
	}
	if !semver.MatchString(version) {
		return spec{}, fmt.Errorf("-version %q: MAJOR.MINOR.PATCH", version)
	}
	if reg, path, ok := strings.Cut(image, "/"); !ok || reg == "" || !strings.HasSuffix("/"+path, "/"+name) {
		return spec{}, fmt.Errorf("-image %q: a registry's repository ending in the product, as ghcr.io/OWNER/%s", image,
			name)
	}
	return spec{topic: topic, zone: zone, loc: loc, from: from, to: to, name: name, version: version, image: image}, nil
}

// tally is what a read has taken: the K-lines, the copies dropped and, of those, the ones of another value.
type tally struct{ klines, copies, others int }

// read is the topic's K-lines, each its base, from its start to its end as it stood when the read began: its span
// first (one partition, a clean topic's, holding at least one record), then its records in no group, each value read
// as clean market data 0.1.0 (clean.FromRecord); its offset after the last one's; a copy, a K-line of a date taken (a
// clean topic's key is its series', so its date alone), dropped, the first taken, one of another value counted; else
// the next K-line, its index the last one's and 1 (from 0) and its date after the last one's. Anything else stops it,
// named by its topic, partition and offset.
func read(ctx context.Context, bus stream.Bus, topic string) ([]market.KLine, tally, error) {
	var t tally
	adm, err := bus.Admin(ctx)
	if err != nil {
		return nil, t, err
	}
	spans, err := adm.Spans(ctx, topic)
	adm.Close(context.Background())
	if err != nil {
		return nil, t, err
	}
	s, ok := spans[0]
	if len(spans) != clean.Partitions || !ok {
		return nil, t, fmt.Errorf("%s: %d partitions, a clean topic's %d", topic, len(spans), clean.Partitions)
	}
	if s.Start >= s.End {
		return nil, t, fmt.Errorf("%s: no record (offsets %d to %d)", topic, s.Start, s.End)
	}
	src, err := bus.Source(ctx, stream.SubscribeOptions{Topics: []string{topic}})
	if err != nil {
		return nil, t, err
	}
	defer src.Close(context.Background())
	var ks []market.KLine
	last := int64(-1)
	dates := map[uint64][sha256.Size]byte{}
	for last < s.End-1 {
		recs, err := src.Fetch(ctx)
		if err != nil {
			return nil, t, err
		}
		for _, r := range recs {
			if r.Offset >= s.End {
				return ks, t, nil // written after the read began
			}
			at := fmt.Sprintf("%s/%d@%d", topic, r.Partition, r.Offset)
			if r.Offset <= last {
				return nil, t, fmt.Errorf("%s: after offset %d: the offsets of a partition ascend", at, last)
			}
			last = r.Offset
			m, err := clean.FromRecord(r)
			if err != nil {
				return nil, t, err
			}
			k := m.KLine.Base()
			sum := sha256.Sum256(r.Value)
			if was, ok := dates[k.Date]; ok {
				t.copies++
				if was != sum {
					t.others++
				}
				continue
			}
			switch n := uint64(len(ks)); {
			case k.Index != n:
				return nil, t, fmt.Errorf("%s: index %d, the next %d: not a value of %s %s", at, k.Index, n,
					clean.Schema, clean.Version)
			case n > 0 && k.Date <= ks[n-1].Date:
				return nil, t, fmt.Errorf("%s: date %d, not after the last, %d: not a value of %s %s", at, k.Date,
					ks[n-1].Date, clean.Schema, clean.Version)
			}
			dates[k.Date] = sum
			ks = append(ks, k)
			t.klines++
		}
	}
	return ks, t, nil
}

// session is a K-line's session's date in the zone.
func session(k market.KLine, loc *time.Location) string {
	return time.UnixMilli(int64(k.Date)).In(loc).Format(day)
}

// pick is the K-lines of the range, the sessions from s.from to s.to, both included, their dates in the zone; the
// topic covering it, its first K-line's session on or before s.from and its last's on or after s.to, and at least one
// K-line in it, else it stops.
func pick(ks []market.KLine, s spec) ([]market.KLine, error) {
	first, last := session(ks[0], s.loc), session(ks[len(ks)-1], s.loc)
	if first > s.from || last < s.to {
		return nil, fmt.Errorf("%s: its sessions %s to %s in %s, which do not cover the range %s to %s", s.topic, first,
			last, s.zone, s.from, s.to)
	}
	var in []market.KLine
	for _, k := range ks {
		if d := session(k, s.loc); s.from <= d && d <= s.to {
			in = append(in, k)
		}
	}
	if len(in) == 0 {
		return nil, fmt.Errorf("%s: no K-line in the sessions %s to %s in %s", s.topic, s.from, s.to, s.zone)
	}
	return in, nil
}
