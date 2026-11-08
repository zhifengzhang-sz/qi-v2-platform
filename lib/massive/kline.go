package massive

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

// Source is the source whose K-lines are Massive's: a message's source on the stream (core/README.md) whose K-line
// this package reads.
const Source = "massive"

// KLine is Massive's K-line: the base K-line of market 1.1.0, its fields and its rule (Base), and Massive's own two
// fields, vw and n, each optional, as Massive's spec has its row (o, h, l, c, v and t required, vw and n not). It is
// the library's own record, as market leaves a source's K-line to its taker (market's SPEC.md, K-line), and a value
// as market's records are: its fields numbers and decimals, none a pointer, so that == compares two K-lines.
type KLine struct {
	Index  uint64         // its place in its series, from 0
	Date   uint64         // its begin time, milliseconds since 1970 UTC: Massive's t
	Open   market.Decimal // Massive's o
	High   market.Decimal // Massive's h
	Low    market.Decimal // Massive's l
	Close  market.Decimal // Massive's c
	Volume market.Decimal // Massive's v
	Vw     market.Decimal // Massive's vw, the volume-weighted average price, a price; when HasVw, else 0
	HasVw  bool
	N      uint64 // Massive's n, the number of trades, a whole number from 0 to market's Bound; when HasN, else 0
	HasN   bool
}

// Base is the K-line's base, market's K-line: its fields but vw and n.
func (k KLine) Base() market.KLine {
	return market.KLine{Index: k.Index, Date: k.Date, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close,
		Volume: k.Volume}
}

// Check is the K-line's rules: the base's (market's KLine.Check); n a whole number within market's Bound; a field
// not set 0, so that a K-line has one value for each text.
func (k KLine) Check() error {
	if err := k.Base().Check(); err != nil {
		return err
	}
	if k.N > market.Bound {
		return fmt.Errorf("n: %d: a number past %d", k.N, uint64(market.Bound))
	}
	if !k.HasVw && k.Vw != (market.Decimal{}) {
		return fmt.Errorf("vw: %s, though not set", k.Vw)
	}
	if !k.HasN && k.N != 0 {
		return fmt.Errorf("n: %d, though not set", k.N)
	}
	return nil
}

// Text is the K-line's canonical text, once its rules are checked, by the base's rules (market's SPEC.md, The JSON):
// its fields in the order of their names, a field not set left out, no spaces, a number in its digits; then a line
// break. Written with market's means for a record of a taker's own: a decimal's digits by AppendTo.
func Text(k KLine) (string, error) {
	if err := k.Check(); err != nil {
		return "", err
	}
	b := make([]byte, 0, 192)
	b = append(b, `{"close":`...)
	b = k.Close.AppendTo(b)
	b = append(b, `,"date":`...)
	b = strconv.AppendUint(b, k.Date, 10)
	b = append(b, `,"high":`...)
	b = k.High.AppendTo(b)
	b = append(b, `,"index":`...)
	b = strconv.AppendUint(b, k.Index, 10)
	b = append(b, `,"low":`...)
	b = k.Low.AppendTo(b)
	if k.HasN {
		b = append(b, `,"n":`...)
		b = strconv.AppendUint(b, k.N, 10)
	}
	b = append(b, `,"open":`...)
	b = k.Open.AppendTo(b)
	b = append(b, `,"volume":`...)
	b = k.Volume.AppendTo(b)
	if k.HasVw {
		b = append(b, `,"vw":`...)
		b = k.Vw.AppendTo(b)
	}
	b = append(b, "}\n"...)
	return string(b), nil
}

// ReadKLine reads Massive's K-line from its JSON, by the rules of market's JSON (SPEC.md, The JSON), as market reads
// its own records: one object of the K-line's fields, the base's seven each once, vw and n once if there, nothing
// else; a number in its digits, a whole number (index, date, n) from 0 to market's Bound, a decimal (open, high, low,
// close, volume, vw) read by market's ParseDecimal; refused, besides, a name that is no field, a name twice, a field
// of the base missing, null, a value of another kind, text not UTF-8, anything after the object, and a broken rule.
// Its spaces and its order are JSON's to allow: the canonical text is Text's. On a fault, the zero KLine.
func ReadKLine(text []byte) (KLine, error) {
	k, err := readKLine(text)
	if err != nil {
		return KLine{}, fmt.Errorf("massive: kline: %w", err)
	}
	return k, nil
}

func readKLine(text []byte) (KLine, error) {
	var k KLine
	if !utf8.Valid(text) {
		return k, errors.New("text not UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(text))
	dec.UseNumber()
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return k, fmt.Errorf("not a JSON object: %v", what(t, err))
	}
	seen := map[string]bool{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return k, err
		}
		name := t.(string) // a member's name, encoding/json's decoder holding the object's grammar
		if seen[name] {
			return k, fmt.Errorf("%q twice", name)
		}
		seen[name] = true
		t, err = dec.Token()
		if err != nil {
			return k, err
		}
		n, ok := t.(json.Number)
		if !ok {
			return k, fmt.Errorf("%s: a number, not %v", name, what(t, nil))
		}
		s := n.String()
		switch name {
		case "index":
			k.Index, err = whole(s)
		case "date":
			k.Date, err = whole(s)
		case "open":
			k.Open, err = market.ParseDecimal(s)
		case "high":
			k.High, err = market.ParseDecimal(s)
		case "low":
			k.Low, err = market.ParseDecimal(s)
		case "close":
			k.Close, err = market.ParseDecimal(s)
		case "volume":
			k.Volume, err = market.ParseDecimal(s)
		case "vw":
			k.Vw, err = market.ParseDecimal(s)
			k.HasVw = true
		case "n":
			k.N, err = whole(s)
			k.HasN = true
		default:
			return k, fmt.Errorf("%q: a name that is no field of Massive's K-line", name)
		}
		if err != nil {
			return k, fmt.Errorf("%s: %s: %w", name, s, err)
		}
	}
	if _, err := dec.Token(); err != nil {
		return k, err
	}
	if t, err := dec.Token(); err != io.EOF {
		return k, fmt.Errorf("more after the object: %v", what(t, err))
	}
	for _, f := range []string{"close", "date", "high", "index", "low", "open", "volume"} {
		if !seen[f] {
			return k, fmt.Errorf("%s missing", f)
		}
	}
	if err := k.Check(); err != nil {
		return KLine{}, err
	}
	return k, nil
}

// whole is a whole number written in digits as market writes one (SPEC.md, The JSON): 0 or a digit not 0 and more
// digits, -0 being 0, no fraction or exponent; from 0 to market's Bound.
func whole(s string) (uint64, error) {
	t := s
	if len(t) > 1 && t[0] == '-' {
		t = t[1:]
	}
	for i := 0; i < len(t); i++ {
		if t[i] < '0' || '9' < t[i] {
			return 0, errors.New("a whole number in digits, no fraction or exponent")
		}
	}
	if t == "" || (len(t) > 1 && t[0] == '0') {
		return 0, errors.New("a whole number in digits, no 0 before its first digit")
	}
	if len(t) > 16 {
		return 0, fmt.Errorf("a number past %d", uint64(market.Bound))
	}
	n, _ := strconv.ParseUint(t, 10, 64) // at most 16 digits
	switch {
	case n > market.Bound:
		return 0, fmt.Errorf("a number past %d", uint64(market.Bound))
	case s[0] == '-' && n != 0:
		return 0, errors.New("a whole number not negative")
	}
	return n, nil
}

// what names a token for an error: null, a text, a boolean, an object or an array, else the token or the fault.
func what(t json.Token, err error) string {
	switch v := t.(type) {
	case nil:
		if err != nil {
			return err.Error()
		}
		return "null"
	case string:
		return "a text"
	case bool:
		return "a boolean"
	case json.Delim:
		if v == '{' {
			return "an object"
		}
		if v == '[' {
			return "an array"
		}
		return string(v)
	}
	return fmt.Sprint(t)
}
