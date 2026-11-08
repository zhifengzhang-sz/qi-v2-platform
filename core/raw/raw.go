// Package raw is the raw market data schema, version 0.3.0, and the market corrections schema, version 0.1.0
// (core/README.md): the topics market.raw, each minute's first version, and market.corrections, each later version, for
// every source and instrument; their key, their partition, every record kept; a value, one JSON object in canonical form
// holding the source's K-line in its canonical text, its base fields market 1.1.0's K-line, its other fields the
// source's, and when the run that published it read it; a correction's value the same and the names of the fields that
// differ from the version before; the record's timestamp the bar's date; delivery at least once, a copy dropped. A value
// of 0.1.0 or 0.2.0 of the raw market data, the versions before, is read as before, with no read.
package raw

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

const (
	// Topic is the raw market data topic, the core's: each minute's first version.
	Topic = "market.raw"
	// Schema and Version name the schema in every value written; Version020 and Version010, the versions before, are
	// read too, each a value of this version's but its version and its read, which they carry not.
	Schema     = "market.raw"
	Version    = "0.3.0"
	Version020 = "0.2.0"
	Version010 = "0.1.0"
	// Partitions is the topics' partition count; a message's partition
	// is Kafka's default partitioner's for its instrument
	// (stream.PartitionOf).
	Partitions = 16
	// Unlimited is the topics' retention, by time (retention.ms) and by
	// size (retention.bytes): every record kept, forever.
	Unlimited = "-1"
	// TimestampType is the topics' message.timestamp.type: a record's
	// timestamp the one its publisher gives, the bar's date.
	TimestampType = "CreateTime"
)

// The origins of a message's bar.
const (
	Live      = "live"      // from its source, as acquisition read it
	Replay    = "replay"    // from a store, published again
	Simulated = "simulated" // made, not read from a market
)

// name is a source, an instrument, a timeframe or a publisher: letters,
// digits, '.', '_' and '-', not empty, beginning with a letter or a
// digit; so a key's three names split at its colons, and a value's names
// need no escape in JSON.
var name = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Message is one value of market.raw: a minute's first version.
type Message struct {
	Origin     string // Live, Replay or Simulated
	Source     string // where the bar's data came from, never its reader
	Instrument string
	Timeframe  string
	Publisher  string // the publishing component's name
	KLine      KLine  // the source's K-line
	// Read is when the run that published it read it, milliseconds since 1970 UTC, from 1 to market's Bound; 0 in a
	// value of 0.1.0 or 0.2.0, which carries none.
	Read uint64
}

// Identity is what a consumer of market.raw drops a copy by: the key and the bar's date.
type Identity struct {
	Key  string
	Date uint64
}

// Key is the message's key: <source>:<instrument>:<timeframe>.
func (m Message) Key() string { return m.Source + ":" + m.Instrument + ":" + m.Timeframe }

// Identity is the message's key and its bar's date.
func (m Message) Identity() Identity { return Identity{Key: m.Key(), Date: m.KLine.Base().Date} }

// Check is the schema's rules on a message: its origin one of the three,
// its names well formed, a K-line, its base keeping market's rules, and a
// read within market's bound (0, none, in a value of a version before).
func (m Message) Check() error {
	switch m.Origin {
	case Live, Replay, Simulated:
	default:
		return fmt.Errorf("raw: origin %q: live, replay or simulated", m.Origin)
	}
	for _, f := range []struct{ field, v string }{
		{"source", m.Source}, {"instrument", m.Instrument}, {"timeframe", m.Timeframe}, {"publisher", m.Publisher},
	} {
		if !name.MatchString(f.v) {
			return fmt.Errorf("raw: %s %q: letters, digits, '.', '_' and '-', from a letter or a digit", f.field, f.v)
		}
	}
	if err := m.KLine.Check(); err != nil {
		return fmt.Errorf("raw: %w", err)
	}
	if m.Read > market.Bound {
		return fmt.Errorf("raw: read %d: past market's bound, %d", m.Read, uint64(market.Bound))
	}
	return nil
}

// Encode is the message's canonical text, once its rules are checked: one
// JSON object of its fields in the order of their names, no spaces, the
// K-line in its canonical text; its version this schema's, its read given.
func (m Message) Encode() ([]byte, error) {
	if err := m.Check(); err != nil {
		return nil, err
	}
	if m.Read == 0 {
		return nil, errors.New("raw: no read: a value of " + Version + " carries when its run read it")
	}
	return encode(m, nil, Schema, Version), nil
}

// hasRead: whether a value of the schema and version carries its read.
func hasRead(schema, version string) bool {
	return (schema == Schema && version == Version) || (schema == CorrectionsSchema && version == CorrectionsVersion)
}

// encode: a value's canonical text, of a message checked: its fields in the order of their names (corrected first, a
// correction's alone), no spaces; read where its schema and version carry it.
func encode(m Message, corrected []string, schema, version string) []byte {
	var b bytes.Buffer
	str := func(field, v string) { b.WriteString(`"` + field + `":"` + v + `"`) }
	b.WriteByte('{')
	if corrected != nil {
		b.WriteString(`"corrected":[`)
		for i, n := range corrected {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`"` + n + `"`)
		}
		b.WriteString(`],`)
	}
	str("instrument", m.Instrument)
	b.WriteString(`,"kline":`)
	b.WriteString(m.KLine.Text())
	b.WriteByte(',')
	str("origin", m.Origin)
	b.WriteByte(',')
	str("publisher", m.Publisher)
	b.WriteByte(',')
	if hasRead(schema, version) {
		b.WriteString(`"read":` + strconv.FormatUint(m.Read, 10) + `,`)
	}
	str("schema", schema)
	b.WriteByte(',')
	str("source", m.Source)
	b.WriteByte(',')
	str("timeframe", m.Timeframe)
	b.WriteByte(',')
	str("version", version)
	b.WriteByte('}')
	return b.Bytes()
}

// value: a value of either schema as written, its fields read and its K-line decoded, nothing checked of its version.
type value struct {
	m               Message
	corrected       *[]string
	schema, version string
	read            bool // whether it carries read
}

// decode reads a value of either schema: one JSON object of the fields a value may carry, nothing after it, its
// source well formed and its K-line the source's (DecodeKLine); its read, when it carries one, a whole number.
func decode(text []byte) (value, error) {
	var v struct {
		Corrected  *[]string       `json:"corrected"`
		Instrument string          `json:"instrument"`
		KLine      json.RawMessage `json:"kline"`
		Origin     string          `json:"origin"`
		Publisher  string          `json:"publisher"`
		Read       json.RawMessage `json:"read"`
		Schema     string          `json:"schema"`
		Source     string          `json:"source"`
		Timeframe  string          `json:"timeframe"`
		Version    string          `json:"version"`
	}
	dec := json.NewDecoder(bytes.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return value{}, fmt.Errorf("raw: not a value of %s or %s: %w", Schema, CorrectionsSchema, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return value{}, errors.New("raw: more after the value's object")
	}
	if !name.MatchString(v.Source) {
		return value{}, fmt.Errorf("raw: source %q: letters, digits, '.', '_' and '-', from a letter or a digit", v.Source)
	}
	k, err := DecodeKLine(v.KLine)
	if err != nil {
		return value{}, fmt.Errorf("raw: %w", err)
	}
	out := value{m: Message{Origin: v.Origin, Source: v.Source, Instrument: v.Instrument, Timeframe: v.Timeframe,
		Publisher: v.Publisher, KLine: k}, corrected: v.Corrected, schema: v.Schema, version: v.Version,
		read: v.Read != nil}
	if v.Read != nil {
		r, err := strconv.ParseUint(string(v.Read), 10, 64)
		if err != nil {
			return value{}, fmt.Errorf("raw: read %s: a whole number, milliseconds since 1970", v.Read)
		}
		out.m.Read = r
	}
	return out, nil
}

// canonical: whether the value is the one text it has, the message's encode by its schema and version.
func canonical(text []byte, m Message, corrected []string, schema, version string) error {
	if again := encode(m, corrected, schema, version); !bytes.Equal(again, text) {
		return fmt.Errorf("raw: not in its canonical text, which is %s", again)
	}
	return nil
}

// Decode reads a value of market.raw: this schema, of this version, 0.2.0 or 0.1.0, every rule checked, and nothing
// but its canonical text. A value of this version carries its read, from 1; one of 0.2.0 or 0.1.0 carries none, and is
// read as one of this version with no read (Read 0): its K-line, of 0.1.0 one of market 1.0.0 in market's canonical
// text, is its source's K-line in its canonical text, its fields but the base's the source's.
func Decode(text []byte) (Message, error) {
	v, err := decode(text)
	if err != nil {
		return Message{}, err
	}
	if v.schema != Schema || (v.version != Version && v.version != Version020 && v.version != Version010) ||
		v.corrected != nil {
		return Message{}, fmt.Errorf("raw: schema %q version %q: this reader reads %s %s, %s and %s", v.schema,
			v.version, Schema, Version, Version020, Version010)
	}
	if v.read != hasRead(v.schema, v.version) || (v.read && v.m.Read == 0) {
		return Message{}, fmt.Errorf("raw: %s %s: a read from 1 in a value of %s alone", Schema, v.version, Version)
	}
	if err := v.m.Check(); err != nil {
		return Message{}, err
	}
	if err := canonical(text, v.m, nil, v.schema, v.version); err != nil {
		return Message{}, err
	}
	return v.m, nil
}

// Record is the message as a record of market.raw: its key, its
// instrument the partition key, its canonical text the value, the bar's
// date the timestamp.
func (m Message) Record() (stream.Record, error) {
	v, err := m.Encode()
	if err != nil {
		return stream.Record{}, err
	}
	return stream.Record{Topic: Topic, Key: []byte(m.Key()), PartitionKey: []byte(m.Instrument), Value: v,
		Time: time.UnixMilli(int64(m.KLine.Base().Date))}, nil
}

// FromRecord reads a record of market.raw: its value decoded, its key the
// value's, its timestamp the bar's date.
func FromRecord(r stream.Record) (Message, error) {
	m, err := Decode(r.Value)
	if err != nil {
		return Message{}, fmt.Errorf("%s/%d@%d: %w", r.Topic, r.Partition, r.Offset, err)
	}
	if err := keyed(r, m); err != nil {
		return Message{}, err
	}
	return m, nil
}

// keyed: whether a record's key and timestamp are its message's.
func keyed(r stream.Record, m Message) error {
	if string(r.Key) != m.Key() {
		return fmt.Errorf("%s/%d@%d: key %q, its value's %q", r.Topic, r.Partition, r.Offset, r.Key, m.Key())
	}
	if d := m.KLine.Base().Date; r.Time.UnixMilli() != int64(d) {
		return fmt.Errorf("%s/%d@%d: timestamp %d, its bar's date %d", r.Topic, r.Partition, r.Offset,
			r.Time.UnixMilli(), d)
	}
	return nil
}
