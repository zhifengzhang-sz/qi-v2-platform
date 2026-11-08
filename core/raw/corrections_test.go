package raw

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// correctedText is ownText's bar as read again: its high, n and vw revised.
const correctedText = `{"close":763.64,"date":1792742400000,"high":764.9,"index":7,"low":763,"n":977,"open":764.7,"volume":10391.875729,"vw":763.8519}`

const correctionValue = `{"corrected":["high","n","vw"],"instrument":"SPY","kline":` + correctedText + `,"origin":"live","publisher":"acquisition","read":1792944000000,"schema":"market.corrections","source":"src","timeframe":"M1","version":"0.1.0"}`

func correction(t *testing.T) Correction {
	m := message(t)
	m.KLine, m.Read = kline(t, correctedText), 1792944000000
	return Correction{Message: m, Corrected: []string{"high", "n", "vw"}}
}

func TestCorrection(t *testing.T) {
	c := correction(t)
	b, err := c.Encode()
	if err != nil || string(b) != correctionValue {
		t.Fatalf("encode: %s, %v\nwant %s", b, err, correctionValue)
	}
	got, err := DecodeCorrection(b)
	if err != nil || got.Message != c.Message || !slices.Equal(got.Corrected, c.Corrected) {
		t.Fatalf("decode: %+v, %v", got, err)
	}
	if id := c.Identity(); id != (CorrectionIdentity{Identity: Identity{Key: "src:SPY:M1", Date: 1792742400000},
		Read: 1792944000000}) {
		t.Fatalf("identity %+v", id)
	}
	r, err := c.Record()
	if err != nil || r.Topic != "market.corrections" || string(r.Key) != "src:SPY:M1" || string(r.PartitionKey) != "SPY" ||
		string(r.Value) != correctionValue || r.Time.UnixMilli() != 1792742400000 {
		t.Fatalf("record %+v, %v", r, err)
	}
	if back, err := CorrectionFromRecord(r); err != nil || back.Message != c.Message {
		t.Fatalf("from record: %+v, %v", back, err)
	}
	other := r
	other.Time = time.UnixMilli(1792742400001)
	if _, err := CorrectionFromRecord(other); err == nil {
		t.Errorf("a record whose timestamp is not its bar's date: read")
	}
	// A correction is not a first version, nor the reverse.
	if _, err := Decode(b); err == nil {
		t.Errorf("a correction read as a value of market.raw")
	}
	if _, err := CorrectionFromRecord(stream.Record{Topic: CorrectionsTopic, Key: []byte("src:SPY:M1"),
		Value: []byte(ownMessage), Time: time.UnixMilli(1792742400000)}); err == nil {
		t.Errorf("a value of market.raw read as a correction")
	}
	// To the zero K-line: fields of the source's own gone are named too.
	z, err := Zero(7, 1792742400000)
	if err != nil {
		t.Fatal(err)
	}
	c.KLine, c.Corrected = z, []string{"close", "high", "low", "n", "open", "volume", "vw"}
	if _, err := c.Encode(); err != nil {
		t.Fatalf("to the zero K-line: %v", err)
	}
}

func TestCorrectionRefused(t *testing.T) {
	for name, text := range map[string]string{
		"no corrected":       strings.Replace(correctionValue, `"corrected":["high","n","vw"],`, ``, 1),
		"corrected empty":    strings.Replace(correctionValue, `["high","n","vw"]`, `[]`, 1),
		"corrected null":     strings.Replace(correctionValue, `["high","n","vw"]`, `null`, 1),
		"not in order":       strings.Replace(correctionValue, `["high","n","vw"]`, `["n","high","vw"]`, 1),
		"a name twice":       strings.Replace(correctionValue, `["high","n","vw"]`, `["high","high","n","vw"]`, 1),
		"date":               strings.Replace(correctionValue, `["high","n","vw"]`, `["date","high","n","vw"]`, 1),
		"asOf":               strings.Replace(correctionValue, `["high","n","vw"]`, `["asOf","high","n","vw"]`, 1),
		"a name not a field": strings.Replace(correctionValue, `["high","n","vw"]`, `["high","n","v-w"]`, 1),
		"a space":            strings.Replace(correctionValue, `["high","n","vw"]`, `["high", "n","vw"]`, 1),
		"an escape":          strings.Replace(correctionValue, `"high"`, "\"\\u0068igh\"", 1),
		"no read":            strings.Replace(correctionValue, `"read":1792944000000,`, ``, 1),
		"a read of 0":        strings.Replace(correctionValue, `"read":1792944000000`, `"read":0`, 1),
		"another version":    strings.Replace(correctionValue, `"version":"0.1.0"`, `"version":"0.3.0"`, 1),
		"market.raw's":       strings.Replace(correctionValue, `"schema":"market.corrections"`, `"schema":"market.raw"`, 1),
		"fields reordered":   `{"instrument":"SPY","corrected":["high","n","vw"],` + correctionValue[len(`{"corrected":["high","n","vw"],"instrument":"SPY",`):],
	} {
		if c, err := DecodeCorrection([]byte(text)); err == nil {
			t.Errorf("%s: read as %+v", name, c)
		}
	}
	c := correction(t)
	c.Corrected = nil
	if _, err := c.Encode(); err == nil {
		t.Errorf("a correction naming no field: written")
	}
	c = correction(t)
	c.Read = 0
	if _, err := c.Encode(); err == nil {
		t.Errorf("a correction without its read: written")
	}
}

func TestDiffer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want []string
	}{
		{ownText, ownText, nil},
		{ownText, correctedText, []string{"high", "n", "vw"}},
		{correctedText, ownText, []string{"high", "n", "vw"}},
		{ownText, baseText, []string{"n", "vw"}},
		{baseText, ownText, []string{"n", "vw"}},
		{baseText, `{"close":0,"date":1792742400000,"high":0,"index":7,"low":0,"open":0,"volume":0}`,
			[]string{"close", "high", "low", "open", "volume"}},
		{ownText, `{"a":1,` + ownText[1:], []string{"a"}},
	} {
		got, err := Differ(kline(t, c.a), kline(t, c.b))
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%s against %s: %v, %v; want %v", c.a, c.b, got, err, c.want)
		}
	}
	if _, err := Differ(KLine{}, kline(t, ownText)); err == nil {
		t.Errorf("no K-line: compared")
	}
}

func TestZero(t *testing.T) {
	z, err := Zero(194880, 1794061800000)
	want := `{"close":0,"date":1794061800000,"high":0,"index":194880,"low":0,"open":0,"volume":0}`
	if err != nil || z.Text() != want || z.Base().Index != 194880 || z.Base().Date != 1794061800000 ||
		z.Base().Open.Sign() != 0 || z.Check() != nil {
		t.Fatalf("zero: %s, %v", z.Text(), err)
	}
}
