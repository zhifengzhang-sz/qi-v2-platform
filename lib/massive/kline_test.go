package massive

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

func dec(t *testing.T, s string) market.Decimal {
	t.Helper()
	d, err := market.ParseDecimal(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// first is page1.json's first row as Massive's K-line, every field set: six's first line.
func first(t *testing.T) KLine {
	return KLine{Date: 1792742400000, Open: dec(t, "764.7"), High: dec(t, "764.8"), Low: dec(t, "763"),
		Close: dec(t, "763.64"), Volume: dec(t, "10391.875729"), Vw: dec(t, "763.8517"), HasVw: true, N: 976, HasN: true}
}

func TestKLine(t *testing.T) {
	k := first(t)
	for _, c := range []struct {
		name        string
		hasVw, hasN bool
		text        string
	}{
		{"both", true, true, six[0]},
		{"no vw", false, true, `{"close":763.64,"date":1792742400000,"high":764.8,"index":0,"low":763,"n":976,"open":764.7,"volume":10391.875729}`},
		{"no n", true, false, `{"close":763.64,"date":1792742400000,"high":764.8,"index":0,"low":763,"open":764.7,"volume":10391.875729,"vw":763.8517}`},
		{"neither", false, false, `{"close":763.64,"date":1792742400000,"high":764.8,"index":0,"low":763,"open":764.7,"volume":10391.875729}`},
	} {
		k := k
		k.HasVw, k.HasN = c.hasVw, c.hasN
		if !c.hasVw {
			k.Vw = market.Decimal{}
		}
		if !c.hasN {
			k.N = 0
		}
		s, err := Text(k)
		if err != nil || s != c.text+"\n" {
			t.Errorf("%s: text %q, %v; want %s", c.name, s, err, c.text)
		}
		got, err := ReadKLine([]byte(c.text))
		if err != nil || got != k {
			t.Errorf("%s: read %+v, %v", c.name, got, err)
		}
		if got.Base() != (market.KLine{Date: k.Date, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close, Volume: k.Volume}) {
			t.Errorf("%s: base %+v", c.name, got.Base())
		}
	}
	// A field not set holds 0, so that a K-line has one value for each text; n within market's bound.
	for name, k := range map[string]KLine{
		"vw not set, not 0": {Date: k.Date, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close, Vw: k.Vw},
		"n not set, not 0":  {Date: k.Date, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close, N: 1},
		"n past the bound":  {Date: k.Date, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close, N: market.Bound + 1, HasN: true},
		"low above open":    {Date: k.Date, Open: k.Open, High: k.High, Low: k.High, Close: k.Close},
	} {
		if s, err := Text(k); err == nil {
			t.Errorf("%s: written, %s", name, s)
		}
	}
}

func TestReadKLineRefused(t *testing.T) {
	text := six[0]
	for name, s := range map[string]string{
		"a field that is no field": strings.Replace(text, `"n":976,`, `"n":976,"otc":1,`, 1),
		"asOf":                     `{"asOf":1792742400000,` + text[1:],
		"n twice":                  strings.Replace(text, `"n":976,`, `"n":976,"n":976,`, 1),
		"vw null":                  strings.Replace(text, `"vw":763.8517`, `"vw":null`, 1),
		"vw a text":                strings.Replace(text, `"vw":763.8517`, `"vw":"763.8517"`, 1),
		"vw with 7 digits":         strings.Replace(text, `"vw":763.8517`, `"vw":763.8517001`, 1),
		"vw ending in 0":           strings.Replace(text, `"vw":763.8517`, `"vw":763.85170`, 1),
		"n negative":               strings.Replace(text, `"n":976`, `"n":-976`, 1),
		"n a fraction":             strings.Replace(text, `"n":976`, `"n":976.5`, 1),
		"n with an exponent":       strings.Replace(text, `"n":976`, `"n":9.76e2`, 1),
		"n past the bound":         strings.Replace(text, `"n":976`, `"n":9007199254740992`, 1),
		"n of 17 digits":           strings.Replace(text, `"n":976`, `"n":10000000000000000`, 1),
		"n a leading 0":            strings.Replace(text, `"n":976`, `"n":0976`, 1),
		"volume missing":           strings.Replace(text, `,"volume":10391.875729`, ``, 1),
		"an object in vw":          strings.Replace(text, `"vw":763.8517`, `"vw":{}`, 1),
		"more after the object":    text + `{}`,
		"not an object":            `[` + text + `]`,
		"empty":                    ``,
	} {
		if k, err := ReadKLine([]byte(s)); err == nil {
			t.Errorf("%s: read as %+v", name, k)
		} else if k != (KLine{}) {
			t.Errorf("%s: %v, and a K-line %+v", name, err, k)
		}
	}
	// JSON's spaces and another order are read; -0 is 0.
	spaced := `{ "volume" : 10391.875729, "close": 763.64, "date": 1792742400000, "high": 764.8, "index": -0, "low": 763, "n": 976, "open": 764.7, "vw": 763.8517 }` + "\n"
	if k, err := ReadKLine([]byte(spaced)); err != nil || k != first(t) {
		t.Errorf("spaced: %+v, %v", k, err)
	}
}

// asOf is an ongoing K-line's asOf member, with its comma and the spaces around it: what an ongoing K-line has and
// its K-line has not.
var asOf = regexp.MustCompile(`\s*,\s*"asOf"\s*:\s*[^,}]*|"asOf"\s*:\s*[^,}]*,\s*`)

// TestVectors holds the base of Massive's K-line to market's vectors (QI_MARKET_VECTORS, the shell's: market's
// vectors/ in the flake input's source), each an ongoing K-line of the base, read as a K-line with its asOf taken out:
// kline.json read with neither vw nor n, its base market's own reading of it, and written back as its text; each
// reject vector (reject/, the file named by what it breaks) refused, but those that break only asOf (as-of-*), which
// a K-line has not: each of them, its asOf taken out, is kline.json's K-line, read so.
func TestVectors(t *testing.T) {
	dir := os.Getenv("QI_MARKET_VECTORS")
	if dir == "" {
		t.Fatal("QI_MARKET_VECTORS is not set: market's vectors, the shell's (flake.nix)")
	}
	ongoing, err := os.ReadFile(filepath.Join(dir, "ongoing", "kline.json"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := market.DecodeOngoingKLine(ongoing)
	if err != nil {
		t.Fatal(err)
	}
	base := market.KLine{Index: o.Index, Date: o.Date, Open: o.Open, High: o.High, Low: o.Low, Close: o.Close,
		Volume: o.Volume}
	text := asOf.ReplaceAll(bytes.TrimSuffix(ongoing, []byte("\n")), nil)
	k, err := ReadKLine(text)
	if err != nil || k.HasVw || k.HasN || k.Base() != base {
		t.Fatalf("kline.json: %+v, %v; want %+v", k, err, base)
	}
	if s, err := Text(k); err != nil || s != string(text)+"\n" {
		t.Fatalf("kline.json: written %q, %v; want %s", s, err, text)
	}
	rejects, err := filepath.Glob(filepath.Join(dir, "ongoing", "reject", "*.json"))
	if err != nil || len(rejects) == 0 {
		t.Fatalf("no reject vector in %s: %v", dir, err)
	}
	refused, read := 0, 0
	for _, p := range rejects {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(p)
		k, err := ReadKLine(asOf.ReplaceAll(b, nil))
		if strings.HasPrefix(name, "as-of-") {
			if err != nil || k.Base() != base || k.HasVw || k.HasN {
				t.Errorf("%s, its asOf taken out: %+v, %v", name, k, err)
			}
			read++
			continue
		}
		if err == nil {
			t.Errorf("%s: read as %+v", name, k)
		}
		refused++
	}
	if refused == 0 || read == 0 {
		t.Fatalf("%d refused, %d of asOf read", refused, read)
	}
	t.Logf("market's vectors: kline.json read and written; of %d reject vectors, %d refused, %d breaking asOf alone read as kline.json's K-line", len(rejects), refused, read)
}

// TestPageWithout: a page whose rows have neither vw nor n is valid by the schema, read, and its lines leave both out.
func TestPageWithout(t *testing.T) {
	page := regexp.MustCompile(`"vw":[0-9.]+,|,"n":[0-9]+`).ReplaceAll(saved(t, "page1.json"), nil)
	if bytes.Contains(page, []byte(`"vw"`)) || bytes.Contains(page, []byte(`"n":`)) {
		t.Fatalf("page: %s", page)
	}
	if err := CheckPage(page); err != nil {
		t.Fatal(err)
	}
	p, err := ReadPage(page)
	if err != nil || len(p.Bars) != 3 {
		t.Fatalf("%+v, %v", p, err)
	}
	b := NewBars(Request{Instrument: "SPY", Timeframe: "M1", Spans: [][2]uint64{{1792742400000, 1792742700000}}},
		time.UnixMilli(1792742700000+60_000))
	var got []string
	if err := b.Page(p, func(s string) error { got = append(got, s); return nil }); err != nil {
		t.Fatal(err)
	}
	for i, s := range got {
		want := regexp.MustCompile(`,"vw":[0-9.]+|,"n":[0-9]+`).ReplaceAllString(six[i], "") + "\n"
		if s != want {
			t.Errorf("line %d: %q, want %q", i, s, want)
		}
		k, err := ReadKLine([]byte(s))
		if err != nil || k.HasVw || k.HasN || k.Vw != (market.Decimal{}) || k.N != 0 {
			t.Errorf("line %d read: %+v, %v", i, k, err)
		}
	}
	// One row with vw alone, one with n alone.
	page = bytes.Replace(saved(t, "page1.json"), []byte(`"vw":763.8517,`), nil, 1)
	page = bytes.Replace(page, []byte(`,"n":703`), nil, 1)
	if p, err = ReadPage(page); err != nil || p.Bars[0].HasVw || !p.Bars[0].HasN || !p.Bars[1].HasVw || p.Bars[1].HasN {
		t.Fatalf("%+v, %v", p, err)
	}
}
