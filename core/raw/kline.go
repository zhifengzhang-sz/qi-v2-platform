package raw

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

// KLine is a source's K-line as the topic carries it (core/README.md): its canonical text, whose base fields the core
// reads as market 1.1.0's K-line (Base), and whose other fields are the source's, carried as written, for the source's
// reader to read. The zero KLine is none. It is a value: two K-lines of one text are equal.
type KLine struct {
	base market.KLine
	text string
}

// Base is the K-line's base fields, market's K-line.
func (k KLine) Base() market.KLine { return k.base }

// Text is the K-line's canonical text: one JSON object, no line break.
func (k KLine) Text() string { return k.text }

// Check is nil for a K-line read or made by this package, whose base keeps market's rules.
func (k KLine) Check() error {
	if k.text == "" {
		return errors.New("kline: none")
	}
	return k.base.Check()
}

// fields: the base's, market's K-line's
var fields = map[string]bool{"close": true, "date": true, "high": true, "index": true, "low": true, "open": true,
	"volume": true}

// own is the name of a source's own field: an ASCII letter, then letters and digits, so that it needs no escape and
// the names' order is ASCII's.
var own = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// digits is a number as market's JSON writes one (SPEC.md, The JSON): an optional -, then 0 or a digit not 0 and more
// digits, then optionally . and digits, the last not 0; no exponent.
var digits = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]*[1-9])?$`)

// number: whether a source's own field's value is a number market's JSON reads, in the one text market writes for it:
// a decimal of market's precision (at most 6 digits after the point and 15 from the first not 0) written as its digits
// are (ParseDecimal, AppendTo: -0 is 0, and is written 0), or a whole number from 0 to market's Bound. So that each
// value has one text, as each of the base's has, whatever the source reads it as.
func number(v []byte) bool {
	if !digits.Match(v) {
		return false
	}
	if d, err := market.ParseDecimal(string(v)); err == nil {
		return bytes.Equal(d.AppendTo(nil), v)
	}
	if bytes.ContainsAny(v, ".-") {
		return false
	}
	n, err := strconv.ParseUint(string(v), 10, 64)
	return err == nil && n <= market.Bound
}

type member struct {
	name  string
	value []byte
}

// DecodeKLine reads a source's K-line from its canonical text: one JSON object, its fields in the order of their names
// (ASCII), no spaces. Its base fields, market's K-line's seven, are read with every rule of market's JSON (market's
// own reader, DecodeOngoingKLine, reads them); each other field is the source's, its name a letter and then letters
// and digits, its value a number market's JSON reads, in the one text market writes for it (a decimal of market's
// precision, or a whole number within its bound: number), carried as it is written. Refused: anything market refuses
// of the base, asOf (the ongoing K-line's), another field not so written, and any text of the K-line but its canonical
// one.
func DecodeKLine(text []byte) (KLine, error) {
	ms, err := members(text)
	if err != nil {
		return KLine{}, fmt.Errorf("kline: %w", err)
	}
	k, err := assemble(ms)
	if err != nil {
		return KLine{}, err
	}
	if k.text != string(text) {
		return KLine{}, fmt.Errorf("kline: not in its canonical text, which is %s", k.text)
	}
	return k, nil
}

// FromBase is the K-line of market's base alone, no field of a source's own: its canonical text is market 1.1.0's
// K-line in market's canonical text, its seven fields in the order of their names, which a reader of market's K-line
// (@qi/market's klineSchema, strict) takes. A base that breaks market's rules is refused.
func FromBase(b market.KLine) (KLine, error) {
	if err := b.Check(); err != nil {
		return KLine{}, fmt.Errorf("kline: %w", err)
	}
	return assemble([]member{{"close", b.Close.AppendTo(nil)}, {"date", strconv.AppendUint(nil, b.Date, 10)},
		{"high", b.High.AppendTo(nil)}, {"index", strconv.AppendUint(nil, b.Index, 10)}, {"low", b.Low.AppendTo(nil)},
		{"open", b.Open.AppendTo(nil)}, {"volume", b.Volume.AppendTo(nil)}})
}

// WithIndex is the K-line at another place in its series: its index set, every other field as it is.
func WithIndex(k KLine, index uint64) (KLine, error) {
	if err := k.Check(); err != nil {
		return KLine{}, err
	}
	ms, err := members([]byte(k.text))
	if err != nil {
		return KLine{}, fmt.Errorf("kline: %w", err)
	}
	for i := range ms {
		if ms[i].name == "index" {
			ms[i].value = strconv.AppendUint(nil, index, 10)
		}
	}
	return assemble(ms)
}

// members: a JSON object's members, each name and its value's text, in the order written; the object alone, from its
// first byte to its last.
func members(text []byte) ([]member, error) {
	if !utf8.Valid(text) {
		return nil, errors.New("text not UTF-8")
	}
	if len(text) == 0 || text[0] != '{' {
		return nil, errors.New("not a JSON object beginning at its first byte")
	}
	dec := json.NewDecoder(bytes.NewReader(text))
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	var ms []member
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		ms = append(ms, member{name: t.(string), value: v}) // a name: the decoder holds the object's grammar
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if t, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("more after the object: %v %v", t, err)
	}
	return ms, nil
}

// assemble: the K-line of its members, read and written again: the base's read by market's reader, the others
// checked; its text the members in the order of their names, each base field as market writes it, each other as it
// is.
func assemble(ms []member) (KLine, error) {
	base := []byte(`{"asOf":` + strconv.FormatUint(market.Bound, 10)) // market's bound, which no date passes
	seen := map[string]bool{}
	for _, m := range ms {
		if seen[m.name] {
			return KLine{}, fmt.Errorf("kline: %q twice", m.name)
		}
		seen[m.name] = true
		switch {
		case fields[m.name]:
			base = append(append(base, `,"`+m.name+`":`...), m.value...)
		case m.name == "asOf":
			return KLine{}, errors.New("kline: asOf, the ongoing K-line's, not a K-line's")
		case !own.MatchString(m.name):
			return KLine{}, fmt.Errorf("kline: %q: a field's name, a letter and then letters and digits", m.name)
		case !number(m.value):
			return KLine{}, fmt.Errorf("kline: %s: %s, not market's text of a decimal of its precision or of a whole "+
				"number within its bound", m.name, m.value)
		}
	}
	o, err := market.DecodeOngoingKLine(append(base, '}'))
	if err != nil {
		return KLine{}, fmt.Errorf("kline: %w", fault(err))
	}
	k := KLine{base: market.KLine{Index: o.Index, Date: o.Date, Open: o.Open, High: o.High, Low: o.Low, Close: o.Close,
		Volume: o.Volume}}
	values := map[string][]byte{"index": strconv.AppendUint(nil, k.base.Index, 10),
		"date": strconv.AppendUint(nil, k.base.Date, 10), "open": k.base.Open.AppendTo(nil),
		"high": k.base.High.AppendTo(nil), "low": k.base.Low.AppendTo(nil), "close": k.base.Close.AppendTo(nil),
		"volume": k.base.Volume.AppendTo(nil)}
	sorted := slices.Clone(ms)
	slices.SortFunc(sorted, func(a, b member) int { return bytes.Compare([]byte(a.name), []byte(b.name)) })
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range sorted {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`"` + m.name + `":`)
		if v, ok := values[m.name]; ok {
			b.Write(v)
		} else {
			b.Write(m.value)
		}
	}
	b.WriteByte('}')
	k.text = b.String()
	return k, nil
}

// fault: market's fault, a broken rule given as the K-line's own (the ongoing K-line's check wraps its K-line's), a
// fault of the text as it is.
func fault(err error) error {
	if inner := errors.Unwrap(err); inner != nil {
		return inner
	}
	return err
}
