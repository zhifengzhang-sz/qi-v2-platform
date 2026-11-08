// despike, version 0.1.0, the tutorial's cleaning (docs/tutorial/advanced/README.md): a cleaning plugin of
// acquisition's, by the line protocol of docs/impl/engineering/plugins.md, 3, on market's Go form and the standard
// library alone; it imports nothing of the platform.
//
//	despike --clean [--after DATE]
//
// At its start it prints its name and version, one line, {"cleaning":"despike","version":"0.1.0"}. Then it reads
// K-lines on stdin, one a line, a series' in the order of their dates, each the source's K-line in its canonical text,
// and answers each with one line, in order: {"kline":<the K-line>}, the K-line as it came, when it keeps both rules;
// else {"refused":"<reason>"}, the first rule it breaks. Its rules, in order: the zero K-line, a minute without a bar
// (open, high, low, close and volume 0), is refused; a spike, a K-line whose range, high − low, is more than 2% of its
// low, is refused. --after, the date of the last K-line the series' clean topic holds, is taken and not needed: each
// K-line is judged alone. An answer is written out whenever no more of its input is waiting. Exit 0 at the end of its
// input, every answer written; 1, with `despike: ` and the fault on stderr, on a line that is not a K-line's JSON
// object of market's decimals.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

// The cleaning's name and version, by which acquisition names its clean topic (core/README.md, The topics' names).
const (
	name    = "despike"
	version = "0.1.0"
)

// spike is the largest range a K-line keeps, as a part of its low: 2%.
var spike = big.NewRat(2, 100)

func main() {
	run := flag.Bool("clean", false, "judge the K-lines on stdin, one a line, a series' in the order of their dates")
	after := flag.String("after", "", "the date, milliseconds since 1970, of the last K-line the series' clean topic holds (not needed)")
	flag.Parse()
	if !*run || flag.NArg() > 0 {
		fail(errors.New("--clean [--after DATE]: the only mode"))
	}
	if _, err := strconv.ParseUint(*after, 10, 64); *after != "" && err != nil {
		fail(fmt.Errorf("--after %q: a date, milliseconds since 1970", *after))
	}
	if err := clean(os.Stdin, os.Stdout); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "despike: %v\n", err)
	os.Exit(1)
}

// clean is the protocol: its name and version, then an answer to each line of in, in order, on out.
func clean(in io.Reader, out io.Writer) error {
	r := bufio.NewReaderSize(in, 1<<16)
	w := bufio.NewWriter(out)
	fmt.Fprintf(w, `{"cleaning":%q,"version":%q}`+"\n", name, version)
	if err := w.Flush(); err != nil {
		return err
	}
	for n := 1; ; n++ {
		line, err := r.ReadBytes('\n')
		if len(line) == 0 && err == io.EOF {
			return w.Flush()
		}
		if err != nil && err != io.EOF {
			return err
		}
		line = bytes.TrimSuffix(line, []byte("\n"))
		reason, err := rules(line)
		if err != nil {
			return fmt.Errorf("line %d: %w", n, err)
		}
		if reason != "" {
			b, _ := json.Marshal(struct {
				Refused string `json:"refused"`
			}{reason})
			fmt.Fprintf(w, "%s\n", b)
		} else {
			fmt.Fprintf(w, `{"kline":%s}`+"\n", line)
		}
		if r.Buffered() == 0 { // no more input waiting: the answers so far written out
			if err := w.Flush(); err != nil {
				return err
			}
		}
	}
}

// rules: the first rule the K-line breaks, as its reason; "" for none, the K-line kept. Its base's prices and volume
// are read as market's decimals, exactly, and a range is reckoned in exact fractions (math/big), never in floats.
func rules(line []byte) (string, error) {
	var f map[string]json.RawMessage
	if err := json.Unmarshal(line, &f); err != nil {
		return "", fmt.Errorf("not a K-line's JSON object: %w", err)
	}
	var open, high, low, cls, vol market.Decimal
	for _, d := range []struct {
		name string
		d    *market.Decimal
	}{{"open", &open}, {"high", &high}, {"low", &low}, {"close", &cls}, {"volume", &vol}} {
		v, err := market.ParseDecimal(string(f[d.name]))
		if err != nil {
			return "", fmt.Errorf("%s %s: %w", d.name, f[d.name], err)
		}
		*d.d = v
	}
	if open.Sign() == 0 && high.Sign() == 0 && low.Sign() == 0 && cls.Sign() == 0 && vol.Sign() == 0 {
		return "the zero K-line: no bar this minute", nil
	}
	h, l := exact(high), exact(low)
	r := new(big.Rat).Sub(h, l)
	if r.Cmp(new(big.Rat).Mul(l, spike)) > 0 {
		return fmt.Sprintf("a spike: range %s (high %s - low %s) more than 2%% of low", text(r), high, low), nil
	}
	return "", nil
}

// exact is a decimal as an exact fraction.
func exact(d market.Decimal) *big.Rat {
	r, _ := new(big.Rat).SetString(d.String()) // market's digits, which big.Rat reads
	return r
}

// text is a fraction of at most 6 digits after the point, as market writes a decimal: no 0 after its last digit.
func text(r *big.Rat) string {
	s := r.FloatString(6)
	return strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
}
