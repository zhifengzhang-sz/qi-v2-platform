package massive

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

// Page is one page of Massive's aggregates, read.
type Page struct {
	// Status is the page's status: OK, or DELAYED for the plans whose recent data comes late.
	Status string
	// NextURL is the next page's address, "" on the last.
	NextURL string
	// Bars is each of the page's rows as Massive's K-line (KLine), in the page's order, every field of the row in
	// it; Index is 0, a line's to give.
	Bars []KLine
}

// ReadPage reads a page: checked against the schema (CheckPage), its status OK or DELAYED, each row read as Massive's
// K-line (KLine), every field of the row kept: t as date, the bar's begin time; o, h, l, c, v; vw and n, each set
// when the row has it; prices and volumes as market's decimals, read exactly from Massive's digits. Each name is read as it is written,
// its case too. A page that breaks the schema, and a row that is not such a K-line (a field it has no place for, otc
// among them; a number past market's bounds or with more than six digits after the point; a broken rule), refuse the
// page, whole: nothing is dropped or changed to fit. The page's attributes but status, next_url and results are not
// read (the saved page keeps them).
func ReadPage(b []byte) (Page, error) {
	v, err := checked(b)
	if err != nil {
		return Page{}, fmt.Errorf("page: not Massive's aggregates by its schema: %w", err)
	}
	// The schema holds: the page an object, status a string, next_url a string if there, results an array of objects
	// if there.
	m := v.(map[string]any)
	p := Page{Status: m["status"].(string)}
	if p.Status != "OK" && p.Status != "DELAYED" {
		return Page{}, fmt.Errorf("page: status %q, not OK or DELAYED", p.Status)
	}
	p.NextURL, _ = m["next_url"].(string)
	rows, _ := m["results"].([]any)
	p.Bars = make([]KLine, 0, len(rows))
	for _, r := range rows {
		k, err := kline(r.(map[string]any))
		if err != nil {
			return Page{}, fmt.Errorf("page: %w", err)
		}
		p.Bars = append(p.Bars, k)
	}
	return p, nil
}

// fields: a row's fields Massive's K-line has a place for.
var fields = []string{"c", "h", "l", "n", "o", "t", "v", "vw"}

// kline reads a row as Massive's K-line, every field of the row into it, vw and n when the row has them, refusing a
// row with a field it has not.
func kline(w map[string]any) (KLine, error) {
	raw, _ := json.Marshal(w) // the row, for an error: its fields by name, each number its digits
	names := make([]string, 0, len(w))
	for name := range w {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if !slices.Contains(fields, name) {
			return KLine{}, fmt.Errorf("row %s: not Massive's K-line: %q, a field it has no place for", raw, name)
		}
	}
	var k KLine
	var err error
	// at: a field's digits; a field of the base missing is a fault, vw or n missing is not set
	at := func(field string, optional bool) (string, bool) {
		v, there := w[field]
		n, ok := v.(json.Number)
		if !ok && (there || !optional) {
			err = errors.Join(err, fmt.Errorf("no %s", field))
		}
		return n.String(), ok
	}
	t, _ := at("t", false)
	o, _ := at("o", false)
	h, _ := at("h", false)
	l, _ := at("l", false)
	c, _ := at("c", false)
	v, _ := at("v", false)
	vw, hasVw := at("vw", true)
	n, hasN := at("n", true)
	if err != nil {
		return k, fmt.Errorf("row %s: %w", raw, err)
	}
	k.Date, err = count(t)
	k.HasVw, k.HasN = hasVw, hasN
	for _, f := range []struct {
		name string
		s    string
		to   *market.Decimal
		set  bool
	}{{"o", o, &k.Open, true}, {"h", h, &k.High, true}, {"l", l, &k.Low, true}, {"c", c, &k.Close, true},
		{"v", v, &k.Volume, true}, {"vw", vw, &k.Vw, hasVw}} {
		if err != nil || !f.set {
			continue
		}
		if *f.to, err = decimal(f.s); err != nil {
			err = fmt.Errorf("%s: %w", f.name, err)
		}
	}
	if err == nil && hasN {
		k.N, err = count(n)
	}
	if err == nil {
		err = k.Check()
	}
	if err != nil {
		return KLine{}, fmt.Errorf("row %s: %w", raw, err)
	}
	return k, nil
}

// decimal is a JSON number as market's decimal, exactly: refused when it has more than six digits after the point
// or is past market's bounds, never rounded.
func decimal(s string) (market.Decimal, error) {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return market.Decimal{}, fmt.Errorf("%q: not a number", s)
	}
	units := new(big.Rat).Mul(r, big.NewRat(1_000_000, 1))
	if !units.IsInt() {
		return market.Decimal{}, fmt.Errorf("%s: more than six digits after the point", s)
	}
	if !units.Num().IsInt64() {
		return market.Decimal{}, fmt.Errorf("%s: past market's bounds", s)
	}
	return market.DecimalOf(units.Num().Int64(), 6)
}

// count is a JSON number of a row as a whole number from 0 to market's bound.
func count(s string) (uint64, error) {
	r, ok := new(big.Rat).SetString(s)
	if !ok || !r.IsInt() || r.Sign() < 0 || !r.Num().IsUint64() || r.Num().Uint64() > market.Bound {
		return 0, fmt.Errorf("%q: not a whole number from 0 to %d", s, uint64(market.Bound))
	}
	return r.Num().Uint64(), nil
}
