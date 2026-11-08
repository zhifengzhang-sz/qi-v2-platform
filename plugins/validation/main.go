// validation, version 0.1.0, the first cleaning: acquisition's cleaning plugin (docs/impl/engineering/plugins.md, 3),
// the rules below, on a series' K-lines.
//
//	validation --clean [--after DATE]
//
// At its start it prints its name and version, one line, {"cleaning":"validation","version":"0.1.0"}. Then it reads
// K-lines on stdin, one a line, a series' in the order of their dates, each the source's K-line in its canonical
// text, and answers each with one line, in order: {"kline":<the K-line>}, the K-line as it came, when it keeps every
// rule; else {"refused":"<reason>"}, the first rule it breaks. The rules, in order: every price, open, high, low and
// close, a finite decimal of market's and above 0; low ≤ min(open, close); max(open, close) ≤ high; volume ≥ 0; its
// date after the date of the last K-line it accepted, or after DATE (--after: milliseconds since 1970, the last the
// series' clean topic holds). So the zero K-line, a minute without a bar, every price 0, is refused. An answer is
// written out whenever no more of its input is waiting. Exit 0 at the end of its input, every answer written; 1, with
// `validation: ` and the fault on stderr, on a line that is not a K-line's JSON object (a base field missing, a date
// or an index not a whole number).
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

// The cleaning's name and version (the core's clean market data names its topic by them, core/README.md).
const (
	name    = "validation"
	version = "0.1.0"
)

func main() {
	var (
		run   = flag.Bool("clean", false, "judge the K-lines on stdin, one a line, a series' in the order of their dates")
		after = flag.String("after", "", "the date, milliseconds since 1970, after which the first K-line must be: the last the series' clean topic holds")
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: validation --clean [--after DATE]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if !*run || flag.NArg() > 0 {
		fail(errors.New("--clean [--after DATE]: the only mode"))
	}
	var from *uint64
	if *after != "" {
		d, err := strconv.ParseUint(*after, 10, 64)
		if err != nil {
			fail(fmt.Errorf("--after %q: a date, milliseconds since 1970", *after))
		}
		from = &d
	}
	if err := clean(os.Stdin, os.Stdout, from); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "validation: %v\n", err)
	os.Exit(1)
}

// clean is the protocol: its name and version, then an answer to each line of in, in order, on out.
func clean(in io.Reader, out io.Writer, after *uint64) error {
	r := bufio.NewReaderSize(in, 1<<16)
	w := bufio.NewWriter(out)
	if _, err := fmt.Fprintf(w, `{"cleaning":%q,"version":%q}`+"\n", name, version); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	v := judge{}
	if after != nil {
		v.last, v.any = *after, true
	}
	for n := 1; ; n++ {
		line, err := r.ReadBytes('\n')
		if len(line) == 0 && err == io.EOF {
			break
		}
		if err != nil && err != io.EOF {
			return err
		}
		line = bytes.TrimSuffix(line, []byte("\n"))
		k, err := read(line)
		if err != nil {
			return fmt.Errorf("line %d: %w", n, err)
		}
		if reason := v.rules(k); reason != "" {
			b, _ := json.Marshal(struct {
				Refused string `json:"refused"`
			}{reason})
			_, err = fmt.Fprintf(w, "%s\n", b)
		} else {
			_, err = fmt.Fprintf(w, `{"kline":%s}`+"\n", line)
		}
		if err != nil {
			return err
		}
		if r.Buffered() == 0 { // no more input waiting: the answers so far written out
			if err := w.Flush(); err != nil {
				return err
			}
		}
	}
	return w.Flush()
}

// kline is a K-line's base fields as the validation reads them: its date, and each price and the volume as the text
// the line gives it, read as market's decimal where it is one.
type kline struct {
	date   uint64
	fields [5]field // open, high, low, close, volume
}

type field struct {
	name, text string
	d          market.Decimal
	ok         bool // a finite decimal of market's
}

// read: a line's base fields, a JSON object's; a base field missing, or its index or date not a whole number, a fault.
func read(line []byte) (kline, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(line, &m); err != nil {
		return kline{}, fmt.Errorf("not a K-line's JSON object: %w", err)
	}
	var k kline
	for _, n := range []string{"index", "date"} {
		v, ok := m[n]
		if !ok {
			return kline{}, fmt.Errorf("no %s, a field of the base", n)
		}
		x, err := strconv.ParseUint(string(v), 10, 64)
		if err != nil {
			return kline{}, fmt.Errorf("%s %s: a whole number", n, v)
		}
		if n == "date" {
			k.date = x
		}
	}
	for i, n := range []string{"open", "high", "low", "close", "volume"} {
		v, ok := m[n]
		if !ok {
			return kline{}, fmt.Errorf("no %s, a field of the base", n)
		}
		f := field{name: n, text: string(v)}
		if d, err := market.ParseDecimal(f.text); err == nil {
			f.d, f.ok = d, true
		}
		k.fields[i] = f
	}
	return k, nil
}

// judge is the validation of one series in order: the date of the last K-line it accepted, when any.
type judge struct {
	last uint64
	any  bool
}

// rules: the first rule the K-line breaks, as its reason; "" for none, the K-line accepted.
func (j *judge) rules(k kline) string {
	open, high, low, cls, vol := k.fields[0], k.fields[1], k.fields[2], k.fields[3], k.fields[4]
	for _, p := range []field{open, high, low, cls} {
		if !p.ok {
			return fmt.Sprintf("%s %s: a price not a finite decimal of market's", p.name, p.text)
		}
		if p.d.Sign() <= 0 {
			return fmt.Sprintf("%s %s: a price not above 0", p.name, p.text)
		}
	}
	lo, hi := open, cls
	if cls.d.Cmp(open.d) < 0 {
		lo, hi = cls, open
	}
	if low.d.Cmp(lo.d) > 0 {
		return fmt.Sprintf("low %s above min(open, close) %s", low.text, lo.text)
	}
	if hi.d.Cmp(high.d) > 0 {
		return fmt.Sprintf("max(open, close) %s above high %s", hi.text, high.text)
	}
	if !vol.ok {
		return fmt.Sprintf("volume %s: not a finite decimal of market's", vol.text)
	}
	if vol.d.Sign() < 0 {
		return fmt.Sprintf("volume %s below 0", vol.text)
	}
	if j.any && k.date <= j.last {
		return fmt.Sprintf("date %d not after %d, the last accepted", k.date, j.last)
	}
	j.last, j.any = k.date, true
	return ""
}
