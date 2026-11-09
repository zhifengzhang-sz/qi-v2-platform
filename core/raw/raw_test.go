package raw

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/mem"
)

func dec(t *testing.T, s string) market.Decimal {
	t.Helper()
	d, err := market.ParseDecimal(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// base is SPY's minute of 2026-10-23 04:00 New York time, at index 7, as market's K-line.
func base(t *testing.T) market.KLine {
	return market.KLine{Index: 7, Date: 1792742400000, Open: dec(t, "764.7"), High: dec(t, "764.8"), Low: dec(t, "763"),
		Close: dec(t, "763.64"), Volume: dec(t, "10391.875729")}
}

// baseText is its canonical text; ownText the same bar's K-line of a source with two fields of its own, n and vw,
// whose names sort among the base's.
const (
	baseText = `{"close":763.64,"date":1792742400000,"high":764.8,"index":7,"low":763,"open":764.7,"volume":10391.875729}`
	ownText  = `{"close":763.64,"date":1792742400000,"high":764.8,"index":7,"low":763,"n":976,"open":764.7,"volume":10391.875729,"vw":763.8517}`
)

// read is when the run read it: 2026-10-24 12:00 New York time.
const read = 1792857600000

const ownMessage = `{"instrument":"SPY","kline":` + ownText + `,"origin":"live","publisher":"acquisition","read":1792857600000,"schema":"market.raw","source":"src","timeframe":"M1","version":"0.3.0"}`

func kline(t *testing.T, text string) KLine {
	t.Helper()
	k, err := DecodeKLine([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func message(t *testing.T) Message {
	return Message{Origin: Live, Source: "src", Instrument: "SPY", Timeframe: "M1", Publisher: "acquisition",
		KLine: kline(t, ownText), Read: read}
}

func TestKLineText(t *testing.T) {
	for _, text := range []string{baseText, ownText} {
		k, err := DecodeKLine([]byte(text))
		if err != nil || k.Text() != text || k.Base() != base(t) || k.Check() != nil {
			t.Fatalf("%s: %+v, %v", text, k, err)
		}
	}
	// Another index, every other field as it is; a source's own fields carried as written.
	k, err := WithIndex(kline(t, ownText), 194490)
	if err != nil || k.Text() != strings.Replace(ownText, `"index":7`, `"index":194490`, 1) || k.Base().Index != 194490 {
		t.Fatalf("with index: %+v, %v", k, err)
	}
	if k != kline(t, k.Text()) {
		t.Fatalf("with index: not the K-line of its text")
	}
	if _, err := WithIndex(KLine{}, 1); err == nil {
		t.Errorf("no K-line: given an index")
	}
	if _, err := WithIndex(kline(t, baseText), market.Bound+1); err == nil {
		t.Errorf("an index past market's bound: given")
	}
	// The base alone: market's K-line's text, of a K-line read with fields of its own too; a broken base refused.
	for _, text := range []string{baseText, ownText} {
		if k, err := FromBase(kline(t, text).Base()); err != nil || k.Text() != baseText || k.Base() != base(t) ||
			k != kline(t, baseText) {
			t.Fatalf("from the base of %s: %+v, %v", text, k, err)
		}
	}
	broken := base(t)
	broken.Low = dec(t, "765")
	if k, err := FromBase(broken); err == nil || k != (KLine{}) {
		t.Errorf("a low above the high: %+v, %v", k, err)
	}
	// Other fields of any name and number, in the order of their names.
	for _, text := range []string{
		`{"a":1,` + baseText[1:],
		`{"close":763.64,"date":1792742400000,"high":764.8,"index":7,"low":763,"open":764.7,"trades":-0.5,"volume":10391.875729}`,
		strings.TrimSuffix(baseText, "}") + `,"zz9":9007199254740991}`,
		strings.TrimSuffix(baseText, "}") + `,"zz9":-999999999999999}`,
		strings.TrimSuffix(baseText, "}") + `,"zz9":0.000001}`,
	} {
		if k, err := DecodeKLine([]byte(text)); err != nil || k.Text() != text || k.Base() != base(t) {
			t.Errorf("%s: %+v, %v", text, k, err)
		}
	}
}

func TestKLineRefused(t *testing.T) {
	for name, text := range map[string]string{
		"a space":                   strings.Replace(ownText, `"close":`, `"close": `, 1),
		"a space before":            " " + ownText,
		"a line break after":        ownText + "\n",
		"fields reordered":          `{"date":1792742400000,"close":763.64,"high":764.8,"index":7,"low":763,"n":976,"open":764.7,"volume":10391.875729,"vw":763.8517}`,
		"an own field out of order": `{"close":763.64,"date":1792742400000,"high":764.8,"index":7,"low":763,"open":764.7,"n":976,"volume":10391.875729}`,
		"a trailing zero":           strings.Replace(ownText, "763.64", "763.640", 1),
		"an own trailing zero":      strings.Replace(ownText, "763.8517", "763.85170", 1),
		"an own exponent":           strings.Replace(ownText, `"n":976`, `"n":9.76e2`, 1),
		"an own leading 0":          strings.Replace(ownText, `"n":976`, `"n":0976`, 1),
		"an own -0":                 strings.Replace(ownText, `"n":976`, `"n":-0`, 1),
		"an own past the bound":     strings.Replace(ownText, `"n":976`, `"n":9007199254740992`, 1),
		"an own of 20 digits":       strings.Replace(ownText, `"n":976`, `"n":12345678901234567890`, 1),
		"an own of 7 decimals":      strings.Replace(ownText, "763.8517", "763.8517001", 1),
		"an own of 16 figures":      strings.Replace(ownText, "763.8517", "1234567890.123456", 1),
		"an own negative whole":     strings.Replace(ownText, `"n":976`, `"n":-9007199254740991`, 1),
		"an own text":               strings.Replace(ownText, `"n":976`, `"n":"976"`, 1),
		"an own null":               strings.Replace(ownText, `"n":976`, `"n":null`, 1),
		"an own boolean":            strings.Replace(ownText, `"n":976`, `"n":true`, 1),
		"an own object":             strings.Replace(ownText, `"n":976`, `"n":{}`, 1),
		"an own array":              strings.Replace(ownText, `"n":976`, `"n":[976]`, 1),
		"an own name of a digit":    strings.Replace(ownText, `"n":976`, `"9":976`, 1),
		"an own name escaped":       strings.Replace(ownText, `"n":976`, `"\u006e":976`, 1),
		"an own name with _":        strings.Replace(ownText, `"open":764.7`, `"o_x":1,"open":764.7`, 1),
		"a base name escaped":       strings.Replace(ownText, `"high":`, `"\u0068igh":`, 1),
		"asOf":                      `{"asOf":1792742400000,` + ownText[1:],
		"a field twice":             strings.Replace(ownText, `"n":976,`, `"n":976,"n":976,`, 1),
		"a base field twice":        strings.Replace(ownText, `"high":764.8,`, `"high":764.8,"high":764.8,`, 1),
		"a base field missing":      strings.Replace(ownText, `"volume":10391.875729,`, ``, 1),
		"a base field null":         strings.Replace(ownText, `"close":763.64`, `"close":null`, 1),
		"a base field a text":       strings.Replace(ownText, `"close":763.64`, `"close":"763.64"`, 1),
		"low above open":            strings.Replace(ownText, `"low":763`, `"low":764.75`, 1),
		"a negative volume":         strings.Replace(ownText, "10391.875729", "-1", 1),
		"seven decimals":            strings.Replace(ownText, "10391.875729", "10391.8757291", 1),
		"an index past the bound":   strings.Replace(ownText, `"index":7`, `"index":9007199254740992`, 1),
		"not an object":             `[` + ownText + `]`,
		"empty":                     ``,
		"after the object":          ownText + `{}`,
		"text not UTF-8":            strings.Replace(ownText, `"n"`, "\"n\xff\"", 1),
	} {
		if k, err := DecodeKLine([]byte(text)); err == nil {
			t.Errorf("%s: read as %+v", name, k)
		} else if k != (KLine{}) {
			t.Errorf("%s: %v, and a K-line", name, err)
		}
	}
	if err := (KLine{}).Check(); err == nil {
		t.Errorf("no K-line: checked")
	}
}

func TestMessage(t *testing.T) {
	m := message(t)
	b, err := m.Encode()
	if err != nil || string(b) != ownMessage {
		t.Fatalf("encode: %s, %v\nwant %s", b, err, ownMessage)
	}
	got, err := Decode(b)
	if err != nil || got != m {
		t.Fatalf("decode: %+v, %v", got, err)
	}
	if m.Key() != "src:SPY:M1" || m.Identity() != (Identity{Key: "src:SPY:M1", Date: 1792742400000}) {
		t.Fatalf("key %s, identity %+v", m.Key(), m.Identity())
	}
	r, err := m.Record()
	if err != nil {
		t.Fatal(err)
	}
	if r.Topic != "market.raw" || string(r.Key) != "src:SPY:M1" || string(r.PartitionKey) != "SPY" ||
		string(r.Value) != ownMessage || r.Time.UnixMilli() != 1792742400000 {
		t.Fatalf("record %+v", r)
	}
	if back, err := FromRecord(r); err != nil || back != m {
		t.Fatalf("from record: %+v, %v", back, err)
	}
	other := r
	other.Key = []byte("src:QQQ:M1")
	if _, err := FromRecord(other); err == nil {
		t.Errorf("a record whose key is not its value's: read")
	}
	other = r
	other.Time = time.UnixMilli(1792742400001)
	if _, err := FromRecord(other); err == nil {
		t.Errorf("a record whose timestamp is not its bar's date: read")
	}
	// Any source carries its own K-line, the base's alone among them.
	m.Source, m.KLine = "other", kline(t, baseText)
	if b, err := m.Encode(); err != nil || !strings.Contains(string(b), `"kline":`+baseText+`,`) {
		t.Fatalf("the base's K-line: %s, %v", b, err)
	}
}

// TestVersionsBefore: a value of 0.1.0 or 0.2.0, the versions before, as the topic holds them, is read as before, a
// first version with no read: its K-line the same, and the message the one a value of 0.3.0 of the same fields gives
// but its read.
func TestVersionsBefore(t *testing.T) {
	for _, version := range []string{Version010, Version020} {
		for _, c := range []struct{ source, text string }{{"src", ownText}, {"other", baseText}} {
			before := `{"instrument":"SPY","kline":` + c.text + `,"origin":"live","publisher":"acquisition","schema":"market.raw","source":"` + c.source + `","timeframe":"M1","version":"` + version + `"}`
			m, err := Decode([]byte(before))
			if err != nil || m.KLine.Text() != c.text || m.Source != c.source || m.Read != 0 {
				t.Fatalf("%s: %+v, %v", version, m, err)
			}
			if _, err := m.Encode(); err == nil {
				t.Fatalf("%s: written again with no read", version)
			}
			m.Read = read
			again, err := m.Encode()
			want := strings.Replace(strings.Replace(before, `"version":"`+version+`"`, `"version":"0.3.0"`, 1),
				`"schema":"market.raw"`, `"read":1792857600000,"schema":"market.raw"`, 1)
			if err != nil || string(again) != want {
				t.Fatalf("%s written again, with a read: %s, %v", version, again, err)
			}
			if _, err := FromRecord(stream.Record{Topic: Topic, Key: []byte(c.source + ":SPY:M1"), Value: []byte(before),
				Time: time.UnixMilli(1792742400000)}); err != nil {
				t.Fatalf("%s from a record: %v", version, err)
			}
			if _, err := Decode([]byte(strings.Replace(before, `"origin":"live"`, `"origin": "live"`, 1))); err == nil {
				t.Fatalf("%s not in its canonical text: read", version)
			}
			if _, err := Decode([]byte(strings.Replace(before, `"schema":`, `"read":1792857600000,"schema":`, 1))); err == nil {
				t.Fatalf("%s with a read: read", version)
			}
		}
	}
}

func TestMessageRefused(t *testing.T) {
	for name, text := range map[string]string{
		"a space":            strings.Replace(ownMessage, `"origin":"live"`, `"origin": "live"`, 1),
		"another version":    strings.Replace(ownMessage, `"version":"0.3.0"`, `"version":"0.4.0"`, 1),
		"no read":            strings.Replace(ownMessage, `"read":1792857600000,`, ``, 1),
		"a read of 0":        strings.Replace(ownMessage, `"read":1792857600000`, `"read":0`, 1),
		"a read of 01":       strings.Replace(ownMessage, `"read":1792857600000`, `"read":01792857600000`, 1),
		"a read null":        strings.Replace(ownMessage, `"read":1792857600000`, `"read":null`, 1),
		"a read negative":    strings.Replace(ownMessage, `"read":1792857600000`, `"read":-1`, 1),
		"a read a decimal":   strings.Replace(ownMessage, `"read":1792857600000`, `"read":1792857600000.5`, 1),
		"a read a string":    strings.Replace(ownMessage, `"read":1792857600000`, `"read":"1792857600000"`, 1),
		"a read past bound":  strings.Replace(ownMessage, `"read":1792857600000`, `"read":9007199254740992`, 1),
		"a correction's":     `{"corrected":["close"],` + ownMessage[1:],
		"its schema's":       strings.Replace(strings.Replace(ownMessage, `"schema":"market.raw"`, `"schema":"market.corrections"`, 1), `"version":"0.3.0"`, `"version":"0.1.0"`, 1),
		"another schema":     strings.Replace(ownMessage, `"schema":"market.raw"`, `"schema":"raw"`, 1),
		"an origin unknown":  strings.Replace(ownMessage, `"origin":"live"`, `"origin":"backfill"`, 1),
		"a field unknown":    strings.Replace(ownMessage, `"origin":"live",`, `"origin":"live","dataset":"x",`, 1),
		"a field missing":    strings.Replace(ownMessage, `"publisher":"acquisition",`, ``, 1),
		"a field twice":      strings.Replace(ownMessage, `"origin":"live",`, `"origin":"live","origin":"live",`, 1),
		"a name's case":      strings.Replace(ownMessage, `"source":`, `"Source":`, 1),
		"an escape":          strings.Replace(ownMessage, `"SPY"`, "\"\\u0053PY\"", 1),
		"a colon in a name":  strings.Replace(ownMessage, `"SPY"`, `"sim:SPY"`, 1),
		"fields reordered":   `{"kline":` + ownText + `,"instrument":"SPY","origin":"live","publisher":"acquisition","read":1792857600000,"schema":"market.raw","source":"src","timeframe":"M1","version":"0.3.0"}`,
		"after the object":   ownMessage + `{}`,
		"a kline not canon.": strings.Replace(ownMessage, `"close":763.64`, `"close":763.640`, 1),
		"a kline of asOf":    strings.Replace(ownMessage, `"kline":{`, `"kline":{"asOf":1792742400000,`, 1),
		"no kline":           strings.Replace(ownMessage, `"kline":`+ownText+`,`, ``, 1),
	} {
		if m, err := Decode([]byte(text)); err == nil {
			t.Errorf("%s: read as %+v", name, m)
		}
	}
	m := message(t)
	m.Publisher = ""
	if _, err := m.Encode(); err == nil {
		t.Errorf("a message without a publisher: written")
	}
	m = message(t)
	m.KLine = KLine{}
	if _, err := m.Encode(); err == nil {
		t.Errorf("a message without a K-line: written")
	}
	m = message(t)
	m.Read = 0
	if _, err := m.Encode(); err == nil {
		t.Errorf("a message without its read: written")
	}
	m.Read = market.Bound + 1
	if _, err := m.Encode(); err == nil {
		t.Errorf("a read past market's bound: written")
	}
}

func TestProvision(t *testing.T) {
	ctx := context.Background()
	bus := mem.New()
	adm, _ := bus.Admin(ctx)
	for i, want := range []bool{true, false} {
		ps, err := Provision(ctx, adm, 1)
		if err != nil || len(ps) != 2 || ps[0].Topic != Topic || ps[1].Topic != CorrectionsTopic {
			t.Fatalf("provision %d: %+v, %v", i, ps, err)
		}
		for _, p := range ps {
			if p.Created != want || p.Set != nil {
				t.Fatalf("provision %d: %+v", i, p)
			}
		}
	}
	// Both topics alike: their partitions, every record kept, never compacted, each record's timestamp its publisher's.
	for _, topic := range []string{Topic, CorrectionsTopic} {
		if n, _ := adm.Partitions(ctx, topic); n[topic] != Partitions {
			t.Fatalf("%s: partitions %v", topic, n)
		}
		if c, _ := adm.Configs(ctx, topic); c["retention.ms"] != "-1" || c["retention.bytes"] != "-1" ||
			c["cleanup.policy"] != "delete" || c["message.timestamp.type"] != "CreateTime" {
			t.Fatalf("%s: configs %v", topic, c)
		}
	}
	// A topic of a finite retention is set to keep every record, and
	// says what it had; the other made.
	for _, topic := range []string{Topic, CorrectionsTopic} {
		finite := mem.New()
		fadm, _ := finite.Admin(ctx)
		_ = fadm.Ensure(ctx, stream.TopicSpec{Name: topic, Partitions: Partitions, RetentionMS: 30 * 24 * 3600 * 1000})
		_ = fadm.Alter(ctx, topic, map[string]string{"retention.bytes": "1073741824"})
		ps, err := Provision(ctx, fadm, 1)
		if err != nil || len(ps) != 2 {
			t.Fatalf("%s of 30 days and 1 GiB: %+v, %v", topic, ps, err)
		}
		for _, p := range ps {
			if p.Topic != topic {
				if !p.Created {
					t.Fatalf("%s of 30 days and 1 GiB: %s not made", topic, p.Topic)
				}
				continue
			}
			if p.Created || p.Set["retention.ms"] != "2592000000" || p.Set["retention.bytes"] != "1073741824" ||
				len(p.Set) != 2 {
				t.Fatalf("%s of 30 days and 1 GiB: %+v", topic, p)
			}
		}
		if c, _ := fadm.Configs(ctx, topic); c["retention.ms"] != "-1" || c["retention.bytes"] != "-1" {
			t.Fatalf("%s of 30 days and 1 GiB, set: %v", topic, c)
		}
	}
	for _, topic := range []string{Topic, CorrectionsTopic} {
		for name, spec := range map[string]stream.TopicSpec{
			"2 partitions":  {Name: topic, Partitions: 2},
			"compacted":     {Name: topic, Partitions: Partitions, CleanupPolicy: "compact"},
			"LogAppendTime": {Name: topic, Partitions: Partitions, TimestampType: "LogAppendTime"},
		} {
			other := mem.New()
			oadm, _ := other.Admin(ctx)
			_ = oadm.Ensure(ctx, spec)
			if _, err := Provision(ctx, oadm, 1); err == nil {
				t.Fatalf("%s of %s: provisioned", topic, name)
			}
			if c, _ := oadm.Configs(ctx, topic); c["retention.ms"] == "-1" {
				t.Fatalf("%s of %s: refused, and its retention set", topic, name)
			}
		}
	}
	for _, topic := range []string{Topic, CorrectionsTopic} {
		if s := Spec(topic, 1); s.Name != topic || s.RetentionMS != -1 || s.CleanupPolicy != "delete" ||
			s.TimestampType != "CreateTime" || s.Partitions != 16 {
			t.Fatalf("spec %+v", s)
		}
	}
}

// A message's partition is its instrument's, whatever its source and
// timeframe: the same for src:SPY:M1 and other:SPY:H1.
func TestPartitionByInstrument(t *testing.T) {
	ctx := context.Background()
	bus := mem.New()
	adm, _ := bus.Admin(ctx)
	if _, err := Provision(ctx, adm, 1); err != nil {
		t.Fatal(err)
	}
	sink, _ := bus.Sink(ctx)
	a := message(t)
	b := message(t)
	b.Source, b.Timeframe, b.KLine = "other", "H1", kline(t, baseText)
	ra, _ := a.Record()
	rb, _ := b.Record()
	if err := sink.Publish(ctx, ra, rb); err != nil {
		t.Fatal(err)
	}
	src, _ := bus.Source(ctx, stream.SubscribeOptions{Group: "t", Topics: []string{Topic}})
	recs, err := src.Fetch(ctx)
	if err != nil || len(recs) != 2 {
		t.Fatalf("fetch: %d, %v", len(recs), err)
	}
	want := stream.PartitionOf([]byte("SPY"), Partitions)
	for _, r := range recs {
		if r.Partition != want {
			t.Fatalf("%s: partition %d, SPY's %d", r.Key, r.Partition, want)
		}
		if _, err := FromRecord(r); err != nil {
			t.Fatal(err)
		}
	}
}
