// clean-reader, the tutorial's first reader (docs/tutorial/basic/README.md): a clean topic read from its start to its
// end, in no group, on franz-go and market's Go form alone. It takes no package of the platform's: it relies on the
// platform contract and the contracts, the clean market data schema 0.1.0 (core/README.md) and market 1.1.0's K-line,
// as a reader outside this repository does.
//
//	clean-reader -topic TOPIC [-brokers localhost:29092]
//
// A line a K-line on stdout, in the topic's order: its index, its date (UTC), open, high, low, close and volume, as
// the K-line's canonical text writes them. A copy, a value whose key and date it has taken already, is dropped, the
// first taken, as the schema's delivery says. At the end, on stderr, the K-lines and the copies counted. It joins no
// group and commits nothing. Exit 1, the fault on stderr, on a value that is not clean market data 0.1.0.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

// value is a value of a clean topic, clean market data 0.1.0: one JSON object, the cleaned K-line in its canonical
// text.
type value struct {
	Cleaning struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"cleaning"`
	Instrument string          `json:"instrument"`
	KLine      json.RawMessage `json:"kline"`
	Origin     string          `json:"origin"`
	Publisher  string          `json:"publisher"`
	Schema     string          `json:"schema"`
	Source     string          `json:"source"`
	Timeframe  string          `json:"timeframe"`
	Version    string          `json:"version"`
}

func main() {
	brokers := flag.String("brokers", "localhost:29092", "the brokers, host:port, comma-separated (the test stack's)")
	topic := flag.String("topic", "", "the clean topic, market.clean.<cleaning>.<version>.<source>.<instrument>.<timeframe>")
	flag.Parse()
	if *topic == "" || flag.NArg() > 0 {
		fail(errors.New("-topic TOPIC [-brokers HOST:PORT]"))
	}
	if err := read(context.Background(), strings.Split(*brokers, ","), *topic); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "clean-reader: %v\n", err)
	os.Exit(1)
}

// read prints the topic's K-lines from its start to its end as it stood when the read began: a clean topic has one
// partition, 0.
func read(ctx context.Context, brokers []string, topic string) error {
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...),
		kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{topic: {0: kgo.NewOffset().AtStart()}}))
	if err != nil {
		return err
	}
	defer cl.Close()
	ends, err := kadm.NewClient(cl).ListEndOffsets(ctx, topic)
	if err != nil {
		return err
	}
	end, ok := ends.Lookup(topic, 0)
	if !ok || end.Err != nil {
		return fmt.Errorf("%s: its partition 0: %v", topic, end.Err)
	}
	type taken struct {
		key  string
		date uint64
	}
	seen := map[taken]bool{}
	out := bufio.NewWriter(os.Stdout)
	klines, copies := 0, 0
	for next := int64(0); next < end.Offset; {
		fs := cl.PollFetches(ctx)
		if err := fs.Err(); err != nil {
			return err
		}
		for _, r := range fs.Records() {
			if r.Offset >= end.Offset {
				break
			}
			next = r.Offset + 1
			key, k, err := decode(r)
			if err != nil {
				return fmt.Errorf("%s/%d@%d: %w", r.Topic, r.Partition, r.Offset, err)
			}
			if seen[taken{key, k.Date}] {
				copies++
				continue
			}
			seen[taken{key, k.Date}] = true
			klines++
			fmt.Fprintf(out, "%d %s %s %s %s %s %s\n", k.Index, time.UnixMilli(int64(k.Date)).UTC().Format("2006-01-02T15:04Z07:00"),
				k.Open, k.High, k.Low, k.Close, k.Volume)
		}
	}
	if err := out.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "clean-reader: %s: %d messages, %d K-lines, %d copies dropped\n", topic, end.Offset, klines,
		copies)
	return nil
}

// decode reads a value as clean market data 0.1.0, as core/README.md states it: one JSON object of its fields alone;
// its schema and version; its cleaning's name (lower-case letters, digits and '-', from a letter) and version
// (MAJOR.MINOR.PATCH); its origin live, replay or simulated; its names of letters, digits, '.', '_' and '-', from a
// letter or a digit, its series' three holding no dot; the record's topic the value's,
// market.clean.<cleaning>.<version>.<source>.<instrument>.<timeframe>, its version with hyphens; its key its series'
// (<source>:<instrument>:<timeframe>); its K-line the source's (kline); the record's timestamp the K-line's date; and
// the value in its canonical text, its fields in the order of their names, no spaces, each number as market writes it.
func decode(r *kgo.Record) (string, market.KLine, error) {
	var v value
	dec := json.NewDecoder(bytes.NewReader(r.Value))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return "", market.KLine{}, fmt.Errorf("not a value of market.clean: %w", err)
	}
	if v.Schema != "market.clean" || v.Version != "0.1.0" {
		return "", market.KLine{}, fmt.Errorf("schema %q version %q: this reader reads market.clean 0.1.0", v.Schema, v.Version)
	}
	if !cleaningName.MatchString(v.Cleaning.Name) || !semver.MatchString(v.Cleaning.Version) {
		return "", market.KLine{}, fmt.Errorf("cleaning %q %q: a name and a version", v.Cleaning.Name, v.Cleaning.Version)
	}
	if v.Origin != "live" && v.Origin != "replay" && v.Origin != "simulated" {
		return "", market.KLine{}, fmt.Errorf("origin %q: live, replay or simulated", v.Origin)
	}
	for _, n := range []string{v.Source, v.Instrument, v.Timeframe, v.Publisher} {
		if !name.MatchString(n) {
			return "", market.KLine{}, fmt.Errorf("name %q: letters, digits, '.', '_' and '-', from a letter or a digit", n)
		}
	}
	for _, n := range []string{v.Source, v.Instrument, v.Timeframe} {
		if strings.Contains(n, ".") {
			return "", market.KLine{}, fmt.Errorf("name %q holds a dot, which parts a topic's names", n)
		}
	}
	topic := strings.Join([]string{"market.clean", v.Cleaning.Name, "v" + strings.ReplaceAll(v.Cleaning.Version, ".", "-"),
		v.Source, v.Instrument, v.Timeframe}, ".")
	if r.Topic != topic {
		return "", market.KLine{}, fmt.Errorf("a value of %s", topic)
	}
	series := v.Source + ":" + v.Instrument + ":" + v.Timeframe
	if string(r.Key) != series {
		return "", market.KLine{}, fmt.Errorf("key %q, its value's %q", r.Key, series)
	}
	k, text, err := kline(v.KLine)
	if err != nil {
		return "", market.KLine{}, err
	}
	if r.Timestamp.UnixMilli() != int64(k.Date) {
		return "", market.KLine{}, fmt.Errorf("timestamp %d, its K-line's date %d", r.Timestamp.UnixMilli(), k.Date)
	}
	str := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	canonical := `{"cleaning":{"name":` + str(v.Cleaning.Name) + `,"version":` + str(v.Cleaning.Version) + `},"instrument":` +
		str(v.Instrument) + `,"kline":` + text + `,"origin":` + str(v.Origin) + `,"publisher":` + str(v.Publisher) +
		`,"schema":"market.clean","source":` + str(v.Source) + `,"timeframe":` + str(v.Timeframe) + `,"version":"0.1.0"}`
	if string(r.Value) != canonical {
		return "", market.KLine{}, fmt.Errorf("not in its canonical text, which is %s", canonical)
	}
	return series, k, nil
}

var (
	own          = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
	digits       = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]*[1-9])?$`)
	cleaningName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	semver       = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	name         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// kline reads the source's K-line (core/README.md, The K-line): its base market 1.1.0's K-line, every rule of it
// checked, index and date whole numbers, the prices and the volume market's decimals, read exactly from their digits;
// each other field the source's, its name a letter and then letters and digits, never asOf (the ongoing K-line's), its
// value a number in the one text market writes for it, a decimal of market's or a whole number to market's bound. It
// gives the base and the canonical text, its fields in the order of their names, the base's as market writes them. What
// a source's own field means (Massive's vw and n) is its reader's, and not read here.
func kline(text []byte) (market.KLine, string, error) {
	var f map[string]json.RawMessage
	if err := json.Unmarshal(text, &f); err != nil {
		return market.KLine{}, "", fmt.Errorf("kline: %w", err)
	}
	var k market.KLine
	for _, w := range []struct {
		name string
		n    *uint64
	}{{"index", &k.Index}, {"date", &k.Date}} {
		n, err := strconv.ParseUint(string(f[w.name]), 10, 64)
		if err != nil {
			return market.KLine{}, "", fmt.Errorf("kline: %s %s: a whole number", w.name, f[w.name])
		}
		*w.n = n
		f[w.name] = json.RawMessage(strconv.FormatUint(n, 10))
	}
	for _, d := range []struct {
		name string
		d    *market.Decimal
	}{{"open", &k.Open}, {"high", &k.High}, {"low", &k.Low}, {"close", &k.Close}, {"volume", &k.Volume}} {
		v, err := market.ParseDecimal(string(f[d.name]))
		if err != nil {
			return market.KLine{}, "", fmt.Errorf("kline: %s %s: %w", d.name, f[d.name], err)
		}
		*d.d = v
		f[d.name] = json.RawMessage(v.String())
	}
	if err := k.Check(); err != nil {
		return market.KLine{}, "", fmt.Errorf("kline: %w", err)
	}
	for n, v := range f {
		switch n {
		case "index", "date", "open", "high", "low", "close", "volume":
		case "asOf":
			return market.KLine{}, "", fmt.Errorf("kline: asOf, the ongoing K-line's, not a K-line's")
		default:
			if !own.MatchString(n) || !number(string(v)) {
				return market.KLine{}, "", fmt.Errorf("kline: %s %s: a source's field, a name and a number as market writes it", n, v)
			}
		}
	}
	b, err := json.Marshal(f) // a map's names in their order, as the canonical text has them
	return k, string(b), err
}

// number: a value in the one text market writes for it, a decimal of its precision as its digits are written, or a
// whole number from 0 to its bound.
func number(v string) bool {
	if !digits.MatchString(v) {
		return false
	}
	if d, err := market.ParseDecimal(v); err == nil {
		return d.String() == v
	}
	if strings.ContainsAny(v, ".-") {
		return false
	}
	n, err := strconv.ParseUint(v, 10, 64)
	return err == nil && n <= market.Bound
}
