package raw

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

const (
	// CorrectionsTopic is the market corrections topic, the core's: each later version of a minute on market.raw.
	CorrectionsTopic = "market.corrections"
	// CorrectionsSchema and CorrectionsVersion name the schema in every value of it.
	CorrectionsSchema  = "market.corrections"
	CorrectionsVersion = "0.1.0"
)

// Topics are the core's topics, each made and configured alike (Provision).
var Topics = []string{Topic, CorrectionsTopic}

// Correction is one value of market.corrections: a later version of a minute on market.raw, as a run read it, and the
// fields in which it differs from the minute's version before it.
type Correction struct {
	Message // the version: a value of raw market data 0.3.0's fields, its read given
	// Corrected is the names of the fields in which the version differs from the one before it, in the order of their
	// names (ASCII), each once: a field of either K-line, its value another or none in the other (Differ).
	Corrected []string
}

// CorrectionIdentity is what a consumer of market.corrections drops a copy by: the key, the bar's date and the read.
type CorrectionIdentity struct {
	Identity
	Read uint64
}

// Identity is the correction's key, its bar's date and its read.
func (c Correction) Identity() CorrectionIdentity {
	return CorrectionIdentity{Identity: c.Message.Identity(), Read: c.Read}
}

// correctable: a field a correction may name: a K-line's field but date, by which the correction is of its bar (the
// base's six others, or a source's own, a letter and then letters and digits, but asOf, the ongoing K-line's).
func correctable(n string) bool {
	return (fields[n] && n != "date") || (!fields[n] && own.MatchString(n) && n != "asOf")
}

// Check is the schema's rules on a correction: its message's (Message.Check), its read given, and the fields it names
// one or more, each one a correction may name, in the order of their names, each once.
func (c Correction) Check() error {
	if err := c.Message.Check(); err != nil {
		return err
	}
	if c.Read == 0 {
		return errors.New("raw: a correction without its read")
	}
	if len(c.Corrected) == 0 {
		return errors.New("raw: a correction naming no field")
	}
	for i, n := range c.Corrected {
		if !correctable(n) {
			return fmt.Errorf("raw: corrected %q: a K-line's field but date", n)
		}
		if i > 0 && c.Corrected[i-1] >= n {
			return fmt.Errorf("raw: corrected %q after %q: the fields in the order of their names, each once", n,
				c.Corrected[i-1])
		}
	}
	return nil
}

// Encode is the correction's canonical text, once its rules are checked: one JSON object of its fields in the order of
// their names, no spaces, corrected a list of names, the K-line in its canonical text.
func (c Correction) Encode() ([]byte, error) {
	if err := c.Check(); err != nil {
		return nil, err
	}
	return encode(c.Message, c.Corrected, CorrectionsSchema, CorrectionsVersion), nil
}

// DecodeCorrection reads a value of market.corrections: this schema, of this version, every rule checked, and nothing
// but its canonical text.
func DecodeCorrection(text []byte) (Correction, error) {
	v, err := decode(text)
	if err != nil {
		return Correction{}, err
	}
	if v.schema != CorrectionsSchema || v.version != CorrectionsVersion || v.corrected == nil || !v.read {
		return Correction{}, fmt.Errorf("raw: schema %q version %q: this reader reads %s %s, its corrected and its "+
			"read given", v.schema, v.version, CorrectionsSchema, CorrectionsVersion)
	}
	c := Correction{Message: v.m, Corrected: *v.corrected}
	if err := c.Check(); err != nil {
		return Correction{}, err
	}
	if err := canonical(text, c.Message, c.Corrected, v.schema, v.version); err != nil {
		return Correction{}, err
	}
	return c, nil
}

// Record is the correction as a record of market.corrections: its key, its instrument the partition key, its
// canonical text the value, the bar's date the timestamp, as market.raw's.
func (c Correction) Record() (stream.Record, error) {
	v, err := c.Encode()
	if err != nil {
		return stream.Record{}, err
	}
	return stream.Record{Topic: CorrectionsTopic, Key: []byte(c.Key()), PartitionKey: []byte(c.Instrument), Value: v,
		Time: time.UnixMilli(int64(c.KLine.Base().Date))}, nil
}

// CorrectionFromRecord reads a record of market.corrections: its value decoded, its key the value's, its timestamp
// the bar's date.
func CorrectionFromRecord(r stream.Record) (Correction, error) {
	c, err := DecodeCorrection(r.Value)
	if err != nil {
		return Correction{}, fmt.Errorf("%s/%d@%d: %w", r.Topic, r.Partition, r.Offset, err)
	}
	if err := keyed(r, c.Message); err != nil {
		return Correction{}, err
	}
	return c, nil
}

// Differ is the names of the fields in which two K-lines differ, in the order of their names: a field of one and not
// the other, or of both with another value. None when their texts are one. Each is in its canonical text, so two
// values of a field are the same when their texts are.
func Differ(a, b KLine) ([]string, error) {
	if err := a.Check(); err != nil {
		return nil, err
	}
	if err := b.Check(); err != nil {
		return nil, err
	}
	of := func(k KLine) (map[string][]byte, error) {
		ms, err := members([]byte(k.text))
		if err != nil {
			return nil, err
		}
		out := make(map[string][]byte, len(ms))
		for _, m := range ms {
			out[m.name] = m.value
		}
		return out, nil
	}
	fa, err := of(a)
	if err != nil {
		return nil, err
	}
	fb, err := of(b)
	if err != nil {
		return nil, err
	}
	var out []string
	for n, va := range fa {
		if vb, ok := fb[n]; !ok || !bytes.Equal(va, vb) {
			out = append(out, n)
		}
	}
	for n := range fb {
		if _, ok := fa[n]; !ok {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out, nil
}

// Zero is the zero K-line at a place of a series and a date: the base's fields alone, open, high, low, close and
// volume 0, no field of a source's own; market 1.1.0's K-line, a price that is 0 keeping its rule (0 ≤ 0 ≤ 0). It is a
// minute of a series' grid for which its source gives no bar (core/README.md).
func Zero(index, date uint64) (KLine, error) {
	return DecodeKLine([]byte(`{"close":0,"date":` + strconv.FormatUint(date, 10) + `,"high":0,"index":` +
		strconv.FormatUint(index, 10) + `,"low":0,"open":0,"volume":0}`))
}
