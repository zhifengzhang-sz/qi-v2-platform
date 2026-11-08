package clean

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/mem"
)

var validation = Cleaning{Name: "validation", Version: "0.1.0"}

func kline(t *testing.T, text string) raw.KLine {
	t.Helper()
	k, err := raw.DecodeKLine([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

const klineText = `{"close":600.5,"date":1764167400000,"high":601,"index":0,"low":599,"n":3,"open":600,"volume":10,"vw":600.25}`

func message(t *testing.T) Message {
	return Message{Cleaning: validation, Origin: raw.Live, Source: "massive", Instrument: "SPY", Timeframe: "M1",
		Publisher: "acquisition", KLine: kline(t, klineText)}
}

func TestTopic(t *testing.T) {
	got, err := Topic(validation, "massive", "SPY", "M1")
	if err != nil || got != "market.clean.validation.v0-1-0.massive.SPY.M1" {
		t.Fatalf("Topic: %q, %v", got, err)
	}
	if got, err := Topic(Cleaning{Name: "validation", Version: "12.0.3"}, "massive", "SPY", "M1"); err != nil ||
		got != "market.clean.validation.v12-0-3.massive.SPY.M1" {
		t.Fatalf("Topic of 12.0.3: %q, %v", got, err)
	}
	// A name of the series holding a dot: refused, with that reason.
	for _, c := range []struct{ source, instrument, timeframe, what string }{
		{"massive", "BRK.B", "M1", `instrument "BRK.B" holds a dot`},
		{"mass.ive", "SPY", "M1", `source "mass.ive" holds a dot`},
		{"massive", "SPY", "M.1", `timeframe "M.1" holds a dot`},
	} {
		got, err := Topic(validation, c.source, c.instrument, c.timeframe)
		if err == nil || !strings.Contains(err.Error(), c.what) || !strings.Contains(err.Error(), "until such names are settled") {
			t.Errorf("%s:%s:%s: %q, %v", c.source, c.instrument, c.timeframe, got, err)
		}
	}
	for _, c := range []Cleaning{{"Validation", "0.1.0"}, {"valid.ation", "0.1.0"}, {"", "0.1.0"},
		{"validation", "0.1"}, {"validation", "v0.1.0"}, {"validation", "0.01.0"}} {
		if got, err := Topic(c, "massive", "SPY", "M1"); err == nil {
			t.Errorf("cleaning %+v: %q", c, got)
		}
	}
	if got, err := Topic(validation, "massive", "SP:Y", "M1"); err == nil {
		t.Errorf("an instrument not a name: %q", got)
	}
	if got, err := Topic(validation, "massive", strings.Repeat("S", 220), "M1"); err == nil {
		t.Errorf("a topic name past 249 characters: %q", got)
	}
}

func TestDerived(t *testing.T) {
	topic := "market.clean.validation.v0-1-0.massive.SPY.M1"
	all := []string{"market.raw", "market.corrections", topic, "engine.go.v0-2-0.validation.v0-1-0.massive.SPY.M1",
		"market.clean.validation.v0-1-0.massive.QQQ.M1", "engine.go.v0-2-0.validation.v0-2-0.massive.SPY.M1",
		"market.clean.other.v1-0-0.validation.v0-1-0.massive.SPY.M1"}
	got := Derived(topic, all)
	want := []string{"engine.go.v0-2-0.validation.v0-1-0.massive.SPY.M1",
		"market.clean.other.v1-0-0.validation.v0-1-0.massive.SPY.M1"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("Derived: %v, want %v", got, want)
	}
}

func TestCanonical(t *testing.T) {
	m := message(t)
	b, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"cleaning":{"name":"validation","version":"0.1.0"},"instrument":"SPY","kline":` + klineText +
		`,"origin":"live","publisher":"acquisition","schema":"market.clean","source":"massive","timeframe":"M1","version":"0.1.0"}`
	if string(b) != want {
		t.Fatalf("Encode:\n%s\nwant\n%s", b, want)
	}
	got, err := Decode(b)
	if err != nil || got != m {
		t.Fatalf("Decode: %+v, %v", got, err)
	}
	for name, text := range map[string]string{
		"a space":             strings.Replace(want, `"instrument":"SPY"`, `"instrument": "SPY"`, 1),
		"another version":     strings.TrimSuffix(want, `"version":"0.1.0"}`) + `"version":"0.2.0"}`,
		"another schema":      strings.Replace(want, `"market.clean"`, `"market.raw"`, 1),
		"no cleaning":         strings.Replace(want, `"cleaning":{"name":"validation","version":"0.1.0"},`, ``, 1),
		"a field unknown":     strings.Replace(want, `"origin"`, `"read":1,"origin"`, 1),
		"an origin unknown":   strings.Replace(want, `"live"`, `"made"`, 1),
		"a K-line broken":     strings.Replace(want, `"low":599`, `"low":602`, 1),
		"more after it":       want + `{}`,
		"a dotted instrument": strings.Replace(want, `"instrument":"SPY"`, `"instrument":"S.PY"`, 1),
	} {
		if _, err := Decode([]byte(text)); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
}

func TestRecord(t *testing.T) {
	ctx := context.Background()
	bus := mem.New()
	adm, _ := bus.Admin(ctx)
	topic := "market.clean.validation.v0-1-0.massive.SPY.M1"
	p, err := Provision(ctx, adm, topic, 1)
	if err != nil || !p.Created {
		t.Fatalf("Provision: %+v, %v", p, err)
	}
	if n, _ := adm.Partitions(ctx, topic); n[topic] != 1 {
		t.Fatalf("partitions %v", n)
	}
	c, _ := adm.Configs(ctx, topic)
	for k, v := range map[string]string{"retention.ms": "-1", "retention.bytes": "-1", "cleanup.policy": "delete",
		"message.timestamp.type": "CreateTime"} {
		if c[k] != v {
			t.Errorf("%s %q, want %q", k, c[k], v)
		}
	}
	if p, err := Provision(ctx, adm, topic, 1); err != nil || p.Created {
		t.Fatalf("Provision again: %+v, %v", p, err)
	}
	m := message(t)
	r, err := m.Record()
	if err != nil || r.Topic != topic || string(r.Key) != "massive:SPY:M1" || r.Time.UnixMilli() != 1764167400000 {
		t.Fatalf("Record: %+v, %v", r, err)
	}
	sink, _ := bus.Sink(ctx)
	if err := sink.Publish(ctx, r); err != nil {
		t.Fatal(err)
	}
	src, _ := bus.Source(ctx, stream.SubscribeOptions{Topics: []string{topic}})
	got, err := src.Fetch(ctx)
	if err != nil || len(got) != 1 || got[0].Partition != 0 {
		t.Fatalf("fetch: %+v, %v", got, err)
	}
	if back, err := FromRecord(got[0]); err != nil || back != m {
		t.Fatalf("FromRecord: %+v, %v", back, err)
	}
	for name, f := range map[string]func(r stream.Record) stream.Record{
		"another topic": func(r stream.Record) stream.Record {
			r.Topic = "market.clean.validation.v0-1-0.massive.QQQ.M1"
			return r
		},
		"another key":       func(r stream.Record) stream.Record { r.Key = []byte("massive:QQQ:M1"); return r },
		"another timestamp": func(r stream.Record) stream.Record { r.Time = r.Time.Add(time.Millisecond); return r },
	} {
		if _, err := FromRecord(f(got[0])); err == nil {
			t.Errorf("%s: read", name)
		}
	}
}
