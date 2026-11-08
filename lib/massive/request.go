package massive

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

// timeframes: each timeframe Massive is asked for, as the endpoint's multiplier and timespan, and its length.
var timeframes = map[string]struct {
	path string
	step time.Duration
}{
	"M1": {"1/minute", time.Minute}, "M5": {"5/minute", 5 * time.Minute}, "M15": {"15/minute", 15 * time.Minute},
	"M30": {"30/minute", 30 * time.Minute}, "H1": {"1/hour", time.Hour},
}

// Timeframes gives the timeframes Massive is asked for, shortest first.
func Timeframes() []string {
	out := make([]string, 0, len(timeframes))
	for tf := range timeframes {
		out = append(out, tf)
	}
	slices.SortFunc(out, func(a, b string) int { return int(timeframes[a].step - timeframes[b].step) })
	return out
}

// Request is what Massive is asked for: a series, and its spans of the bars' begin times, milliseconds since 1970
// (UTC), both ends included, ascending, each after the one before.
type Request struct {
	Instrument, Timeframe string
	Spans                 [][2]uint64
}

// Check is nil for a request Massive can be asked: an instrument of no '/', '?', '#', '%', ':' or space; a timeframe
// of Timeframes; a span or more, each FROM ≤ TO within market's bound, after the one before.
func (r Request) Check() error {
	if r.Instrument == "" || strings.ContainsAny(r.Instrument, "/?#%: ") {
		return fmt.Errorf("series %q: INSTRUMENT:TIMEFRAME", r.Instrument+":"+r.Timeframe)
	}
	if _, ok := timeframes[r.Timeframe]; !ok {
		return fmt.Errorf("timeframe %q: %s", r.Timeframe, strings.Join(Timeframes(), ", "))
	}
	if len(r.Spans) == 0 {
		return fmt.Errorf("%s:%s: no span", r.Instrument, r.Timeframe)
	}
	for i, sp := range r.Spans {
		if sp[0] > sp[1] || sp[1] > market.Bound {
			return fmt.Errorf("span %d %d: FROM ≤ TO, milliseconds since 1970", sp[0], sp[1])
		}
		if i > 0 && sp[0] <= r.Spans[i-1][1] {
			return fmt.Errorf("span %d %d: not after the span before it", sp[0], sp[1])
		}
	}
	return nil
}

// From is the first span's begin, To the last one's end.
func (r Request) From() uint64 { return r.Spans[0][0] }
func (r Request) To() uint64   { return r.Spans[len(r.Spans)-1][1] }

// Counts is what a request's pages gave.
type Counts struct {
	// Pages and Rows: the pages read and their rows; Lines, the bars given as lines; Open, the bars not closed yet;
	// Outside, the bars outside every span.
	Pages, Rows, Lines, Open, Outside int
}

// Bars takes a request's pages as they come, in order, and gives the request's bars as lines: each bar in a span
// and closed by the time it was made with, ascending, Massive's K-line in its canonical text (Text), its index its
// line's place from 0. Massive is asked once for the whole of the spans, from the first FROM to the last TO: a bar it
// gives outside every span is counted and not given, and one not closed yet is counted too.
type Bars struct {
	r      Request
	step   uint64 // the timeframe's length, ms
	closed uint64 // the time by which a bar is closed, ms
	at     int    // the span the rows have reached
	last   int64  // the last row's date
	index  uint64
	Counts Counts
}

// NewBars gives the bars of a request checked by Request.Check, a bar closed when its end is not after now.
func NewBars(r Request, now time.Time) *Bars {
	return &Bars{r: r, step: uint64(timeframes[r.Timeframe].step.Milliseconds()), closed: uint64(now.UnixMilli()),
		last: -1}
}

// Page takes the request's next page and gives each of its bars in a span and closed, as a line, to line. Rows not
// ascending, across pages too, stop it.
func (b *Bars) Page(p Page, line func(string) error) error {
	b.Counts.Pages++
	for _, k := range p.Bars {
		b.Counts.Rows++
		if int64(k.Date) <= b.last {
			return fmt.Errorf("row t=%d after t=%d: Massive's rows not ascending", k.Date, b.last)
		}
		b.last = int64(k.Date)
		switch {
		case !b.in(k.Date):
			b.Counts.Outside++
			continue
		case k.Date+b.step > b.closed:
			b.Counts.Open++
			continue
		}
		k.Index = b.index
		s, err := Text(k)
		if err != nil {
			return fmt.Errorf("row t=%d: %w", k.Date, err)
		}
		if err := line(s); err != nil {
			return err
		}
		b.index++
		b.Counts.Lines++
	}
	return nil
}

func (b *Bars) in(date uint64) bool {
	for b.at < len(b.r.Spans) && date > b.r.Spans[b.at][1] {
		b.at++
	}
	return b.at < len(b.r.Spans) && date >= b.r.Spans[b.at][0]
}
