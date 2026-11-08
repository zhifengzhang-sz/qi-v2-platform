package main

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/mem"
)

func dec(t *testing.T, s string) market.Decimal {
	d, err := market.ParseDecimal(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func msg(t *testing.T, instrument string, i int, close string) raw.Message {
	k, err := raw.DecodeKLine([]byte(`{"close":` + dec(t, close).String() + `,"date":` +
		strconv.FormatUint(1764167400000+uint64(i)*60_000, 10) + `,"high":601,"index":` + strconv.Itoa(i) +
		`,"low":599,"n":3,"open":600,"volume":10,"vw":600.5}`))
	if err != nil {
		t.Fatal(err)
	}
	return raw.Message{Origin: raw.Live, Source: "massive", Instrument: instrument, Timeframe: "M1", Publisher: "acquisition", KLine: k,
		Read: 1764280800000}
}

func text(t *testing.T, ms ...raw.Message) []string {
	var out []string
	for _, m := range ms {
		b, err := m.Encode()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	return out
}

func bus(t *testing.T, ms ...raw.Message) *mem.Bus {
	ctx := context.Background()
	b := mem.New()
	adm, _ := b.Admin(ctx)
	if _, err := raw.Provision(ctx, adm, 1); err != nil {
		t.Fatal(err)
	}
	sink, _ := b.Sink(ctx)
	for _, m := range ms {
		r, err := m.Record()
		if err != nil {
			t.Fatal(err)
		}
		if err := sink.Publish(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	return b
}

func TestToEnd(t *testing.T) {
	// Two runs of SPY's three bars, the second with bar 1's close revised, and one QQQ bar.
	first := []raw.Message{msg(t, "SPY", 0, "600.1"), msg(t, "SPY", 1, "600.2"), msg(t, "SPY", 2, "600.3")}
	second := []raw.Message{msg(t, "SPY", 0, "600.1"), msg(t, "SPY", 1, "600.25"), msg(t, "SPY", 2, "600.3")}
	qqq := msg(t, "QQQ", 0, "600")
	b := bus(t, append(append(append([]raw.Message(nil), first...), qqq), second...)...)
	ctx := context.Background()

	var out bytes.Buffer
	r, err := consume(ctx, b, options{name: "printer", fromBeginning: true, toEnd: true}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if r.read != 7 || r.written != 7 || r.repeats != 3 || r.changed != 1 || len(r.keys) != 2 {
		t.Fatalf("report %+v", r)
	}
	// Each message as published, every field kept: per key in the order published.
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	var spy []string
	for _, l := range lines {
		if strings.Contains(l, `"instrument":"SPY"`) {
			spy = append(spy, l)
		}
	}
	if want := text(t, append(append([]raw.Message(nil), first...), second...)...); strings.Join(spy, "\n") != strings.Join(want, "\n") {
		t.Fatalf("SPY's lines:\n%s\nwant:\n%s", strings.Join(spy, "\n"), strings.Join(want, "\n"))
	}

	out.Reset()
	r, err = consume(ctx, b, options{name: "printer", fromBeginning: true, toEnd: true, dedup: true}, &out)
	if err != nil {
		t.Fatal(err)
	}
	// Each bar once, its first copy (the schema's delivery): bar 1's close of the first run, the second's dropped.
	want := strings.Join(text(t, qqq, first[0], first[1], first[2]), "\n") + "\n"
	if out.String() != want || r.written != 4 || r.read != 7 {
		t.Fatalf("deduplicated:\n%s\nwant:\n%s\n%+v", out.String(), want, r)
	}
}

func TestEmptyAndRefused(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	if r, err := consume(ctx, bus(t), options{name: "printer", fromBeginning: true, toEnd: true}, &out); err != nil || r.read != 0 {
		t.Fatalf("an empty topic: %+v, %v", r, err)
	}
	b := bus(t, msg(t, "SPY", 0, "600.1"))
	sink, _ := b.Sink(ctx)
	if err := sink.Publish(ctx, stream.Record{Topic: raw.Topic, Key: []byte("massive:SPY:M1"), PartitionKey: []byte("SPY"),
		Value: []byte(`{"not":"raw"}`), Time: time.UnixMilli(1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := consume(ctx, b, options{name: "printer", fromBeginning: true, toEnd: true}, &out); err == nil {
		t.Fatalf("a message not of the schema: read")
	}
}

// Without -to-end it reads on, from its group's committed offset, until stopped.
func TestStream(t *testing.T) {
	b := bus(t, msg(t, "SPY", 0, "600.1"), msg(t, "SPY", 1, "600.2"))
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	r, err := consume(ctx, b, options{name: "printer"}, &out)
	if err == nil || r.written != 2 || strings.Count(out.String(), "\n") != 2 {
		t.Fatalf("%+v, %v, %q", r, err, out.String())
	}
	// Its group committed both: a second run reads nothing more.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if r, _ := consume(ctx2, b, options{name: "printer"}, &out); r.read != 0 {
		t.Fatalf("after the commit: read %d", r.read)
	}
}
