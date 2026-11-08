// Package clean is the clean market data schema, version 0.1.0 (core/README.md): a cleaning's topic of one series,
// market.clean.<cleaning>.<version>.<source>.<instrument>.<timeframe>, each value one K-line of the series as the
// cleaning gave it, the source's, its index its place in the clean series from 0, in the order of their dates; one
// partition, keyed by its series, every record kept, never compacted, the record's timestamp the bar's date; delivery
// at least once, a copy dropped by its key and date, the first taken. A topic made again keeps its name and is read
// from its start; no field marks it made again.
package clean

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

const (
	// Schema and Version name the schema in every value.
	Schema  = "market.clean"
	Version = "0.1.0"
	// Kind begins every clean topic's name.
	Kind = "market.clean"
	// Partitions is a clean topic's partition count: one, its series'.
	Partitions = 1
	// longest is the longest name of a topic Kafka takes.
	longest = 249
)

// Cleaning is a cleaning and its version, as its program names them (docs/impl/engineering/plugins.md, 3).
type Cleaning struct {
	Name    string // lower-case letters, digits and '-', from a letter
	Version string // MAJOR.MINOR.PATCH
}

var (
	cleaningName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	semver       = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
)

// Check is a cleaning's rules: its name lower-case letters, digits and '-', from a letter, and its version
// MAJOR.MINOR.PATCH, so that neither holds a dot of a topic's name but the version's, written as hyphens there.
func (c Cleaning) Check() error {
	if !cleaningName.MatchString(c.Name) {
		return fmt.Errorf("clean: cleaning %q: lower-case letters, digits and '-', from a letter", c.Name)
	}
	if !semver.MatchString(c.Version) {
		return fmt.Errorf("clean: cleaning %s: version %q: MAJOR.MINOR.PATCH", c.Name, c.Version)
	}
	return nil
}

// Topic is the series' clean topic of the cleaning (core/README.md, The topics' names):
// market.clean.<cleaning>.<version>.<source>.<instrument>.<timeframe>, its version written with hyphens (v0-1-0 for
// 0.1.0). A name of the series holding a dot is refused, with that reason, until such names are settled: a topic's
// name parts its names at its dots, so that the series of market.clean.validation.v0-1-0.a.b.c.M1 could be a.b:c or
// a:b.c.
func Topic(c Cleaning, source, instrument, timeframe string) (string, error) {
	if err := c.Check(); err != nil {
		return "", err
	}
	for _, n := range []struct{ what, v string }{{"source", source}, {"instrument", instrument},
		{"timeframe", timeframe}} {
		if strings.Contains(n.v, ".") {
			return "", fmt.Errorf("clean: %s %q holds a dot, which parts a topic's names: no clean topic for its "+
				"series until such names are settled", n.what, n.v)
		}
	}
	m := raw.Message{Origin: raw.Live, Source: source, Instrument: instrument, Timeframe: timeframe, Publisher: "p"}
	if err := names(m); err != nil {
		return "", err
	}
	t := strings.Join([]string{Kind, c.Name, "v" + strings.ReplaceAll(c.Version, ".", "-"), source, instrument,
		timeframe}, ".")
	if len(t) > longest {
		return "", fmt.Errorf("clean: %s: %d characters, Kafka's topic names %d at most", t, len(t), longest)
	}
	return t, nil
}

// Derived is, of the topics named, those derived from the clean topic: each named by everything that derived it and
// by its series (core/README.md, The topics' names), so ending in the clean topic's name after market.clean
// (.validation.v0-1-0.massive.SPY.M1); the clean topic itself not among them.
func Derived(topic string, topics []string) []string {
	tail := strings.TrimPrefix(topic, Kind)
	var out []string
	for _, t := range topics {
		if t != topic && strings.HasSuffix(t, tail) {
			out = append(out, t)
		}
	}
	return out
}

// Spec is a clean topic as provisioning makes it: one partition, every record kept, deleted never compacted, each
// record's timestamp its publisher's, as the core's topics are (raw.Spec) but its partitions.
func Spec(topic string, replicas int16) stream.TopicSpec {
	s := raw.Spec(topic, replicas)
	s.Partitions = Partitions
	return s
}

// Provision makes the clean topic when it is not there and sets the schema's configuration on it (raw.ProvisionTopic).
func Provision(ctx context.Context, adm stream.Admin, topic string, replicas int16) (raw.Provisioned, error) {
	return raw.ProvisionTopic(ctx, adm, Spec(topic, replicas))
}

// Message is one value of a clean topic: a K-line of its series as the cleaning gave it.
type Message struct {
	Cleaning   Cleaning
	Origin     string // the K-line's, as market.raw has it: live, replay or simulated
	Source     string
	Instrument string
	Timeframe  string
	Publisher  string    // the publishing component's name
	KLine      raw.KLine // the cleaned K-line, the source's, its index its place in the clean series
}

// Key is the message's key, its series': <source>:<instrument>:<timeframe>.
func (m Message) Key() string { return m.Source + ":" + m.Instrument + ":" + m.Timeframe }

// Identity is what a consumer of a clean topic drops a copy by: the key and the bar's date.
func (m Message) Identity() raw.Identity {
	return raw.Identity{Key: m.Key(), Date: m.KLine.Base().Date}
}

// Topic is the message's clean topic.
func (m Message) Topic() (string, error) {
	return Topic(m.Cleaning, m.Source, m.Instrument, m.Timeframe)
}

// names: the raw market data schema's rules on a message's origin, its names and, when it has one, its K-line.
func names(m raw.Message) error {
	if m.KLine.Text() == "" {
		k, err := raw.Zero(0, 0)
		if err != nil {
			return err
		}
		m.KLine = k
	}
	return m.Check()
}

// Check is the schema's rules on a message: its cleaning's, its origin one of the three, its names well formed as
// raw market data's, none holding a dot (Topic), and a K-line, its base keeping market's rules.
func (m Message) Check() error {
	if _, err := m.Topic(); err != nil {
		return err
	}
	if err := (raw.Message{Origin: m.Origin, Source: m.Source, Instrument: m.Instrument, Timeframe: m.Timeframe,
		Publisher: m.Publisher, KLine: m.KLine}).Check(); err != nil {
		return fmt.Errorf("clean: %w", err)
	}
	return nil
}

// Encode is the message's canonical text, once its rules are checked: one JSON object of its fields in the order of
// their names, no spaces, the cleaning an object of its name and version, the K-line in its canonical text.
func (m Message) Encode() ([]byte, error) {
	if err := m.Check(); err != nil {
		return nil, err
	}
	return encode(m), nil
}

func encode(m Message) []byte {
	var b bytes.Buffer
	str := func(field, v string) { b.WriteString(`"` + field + `":"` + v + `"`) }
	b.WriteString(`{"cleaning":{`)
	str("name", m.Cleaning.Name)
	b.WriteByte(',')
	str("version", m.Cleaning.Version)
	b.WriteString(`},`)
	str("instrument", m.Instrument)
	b.WriteString(`,"kline":` + m.KLine.Text() + `,`)
	str("origin", m.Origin)
	b.WriteByte(',')
	str("publisher", m.Publisher)
	b.WriteByte(',')
	str("schema", Schema)
	b.WriteByte(',')
	str("source", m.Source)
	b.WriteByte(',')
	str("timeframe", m.Timeframe)
	b.WriteByte(',')
	str("version", Version)
	b.WriteByte('}')
	return b.Bytes()
}

// Decode reads a value of a clean topic: this schema, of this version, every rule checked, and nothing but its
// canonical text.
func Decode(text []byte) (Message, error) {
	var v struct {
		Cleaning *struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"cleaning"`
		Instrument string          `json:"instrument"`
		KLine      json.RawMessage `json:"kline"`
		Origin     string          `json:"origin"`
		Publisher  string          `json:"publisher"`
		Schema     string          `json:"schema"`
		Source     string          `json:"source"`
		Timeframe  string          `json:"timeframe"`
		Version    string          `json:"version"`
	}
	dec := json.NewDecoder(bytes.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return Message{}, fmt.Errorf("clean: not a value of %s: %w", Schema, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Message{}, errors.New("clean: more after the value's object")
	}
	if v.Schema != Schema || v.Version != Version || v.Cleaning == nil {
		return Message{}, fmt.Errorf("clean: schema %q version %q: this reader reads %s %s, its cleaning given",
			v.Schema, v.Version, Schema, Version)
	}
	k, err := raw.DecodeKLine(v.KLine)
	if err != nil {
		return Message{}, fmt.Errorf("clean: %w", err)
	}
	m := Message{Cleaning: Cleaning{Name: v.Cleaning.Name, Version: v.Cleaning.Version}, Origin: v.Origin,
		Source: v.Source, Instrument: v.Instrument, Timeframe: v.Timeframe, Publisher: v.Publisher, KLine: k}
	if err := m.Check(); err != nil {
		return Message{}, err
	}
	if again := encode(m); !bytes.Equal(again, text) {
		return Message{}, fmt.Errorf("clean: not in its canonical text, which is %s", again)
	}
	return m, nil
}

// Record is the message as a record of its clean topic: its key, its canonical text the value, the bar's date the
// timestamp; the topic's one partition its partition.
func (m Message) Record() (stream.Record, error) {
	v, err := m.Encode()
	if err != nil {
		return stream.Record{}, err
	}
	t, err := m.Topic()
	if err != nil {
		return stream.Record{}, err
	}
	return stream.Record{Topic: t, Key: []byte(m.Key()), PartitionKey: []byte(m.Instrument), Value: v,
		Time: time.UnixMilli(int64(m.KLine.Base().Date))}, nil
}

// FromRecord reads a record of a clean topic: its value decoded, the topic the value's, its key the value's, its
// timestamp the bar's date.
func FromRecord(r stream.Record) (Message, error) {
	m, err := Decode(r.Value)
	if err != nil {
		return Message{}, fmt.Errorf("%s/%d@%d: %w", r.Topic, r.Partition, r.Offset, err)
	}
	if t, _ := m.Topic(); t != r.Topic {
		return Message{}, fmt.Errorf("%s/%d@%d: a value of %s", r.Topic, r.Partition, r.Offset, t)
	}
	if string(r.Key) != m.Key() {
		return Message{}, fmt.Errorf("%s/%d@%d: key %q, its value's %q", r.Topic, r.Partition, r.Offset, r.Key, m.Key())
	}
	if d := m.KLine.Base().Date; r.Time.UnixMilli() != int64(d) {
		return Message{}, fmt.Errorf("%s/%d@%d: timestamp %d, its bar's date %d", r.Topic, r.Partition, r.Offset,
			r.Time.UnixMilli(), d)
	}
	return m, nil
}
