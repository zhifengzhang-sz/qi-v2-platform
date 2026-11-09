package main

import (
	"strconv"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

// text is a K-line's base, market 1.1.0's K-line, in market's canonical text: its seven fields in the order of their
// names, no spaces, each number as market writes it. It is the K-line @qi/market's klineSchema takes, which is strict: a
// clean value's K-line is the source's, its own fields (Massive's n and vw) beside the base's, and those are not
// market's K-line. Market's Go form 1.1.0 encodes an ongoing K-line but no K-line, so the text is written here; the
// test reads it back by the core's reader (raw.DecodeKLine) to the same base.
func text(k market.KLine) string {
	b := make([]byte, 0, 128)
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
	b = append(b, `,"open":`...)
	b = k.Open.AppendTo(b)
	b = append(b, `,"volume":`...)
	b = k.Volume.AppendTo(b)
	return string(append(b, '}'))
}
