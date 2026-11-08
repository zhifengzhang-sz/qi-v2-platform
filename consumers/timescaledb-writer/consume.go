package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/lib/massive"
)

// options is a run's.
type options struct {
	name  string // the component's name, its consumer group
	toEnd bool   // read the topics from their start to their end as they stood when the run began, then stop
}

// taken is what became of a topic's versions of Massive: stored; dropped, a copy of a version taken already, in
// the fetch or the store; of the copies, those of another K-line or read.
type taken struct{ stored, copies, differing int }

// report is what a run read and wrote.
type report struct {
	read, other      int   // messages read from both topics; of them, of a source other than massive
	raw, corrections taken // of market.raw and of market.corrections
	corrected        int   // rows of the corrected raw written, new or replacing
}

func (r report) String() string {
	return fmt.Sprintf("%d messages read from %s and %s, %d of another source; %s: %d first versions stored, %d copies "+
		"dropped (%d of another value); %s: %d corrections stored, %d copies dropped (%d of another value); the "+
		"corrected raw: %d rows written", r.read, raw.Topic, raw.CorrectionsTopic, r.other, raw.Topic, r.raw.stored,
		r.raw.copies, r.raw.differing, raw.CorrectionsTopic, r.corrections.stored, r.corrections.copies,
		r.corrections.differing, r.corrected)
}

// minute is a bar's place in the store: its series and its date.
type minute struct {
	instrument, timeframe string
	date                  uint64
}

// version is a bar's version: Massive's K-line, read by the library's reader of Massive (lib/massive), and when the
// run that published it read it (0: none, a first version of 0.1.0 or 0.2.0).
type version struct {
	k    massive.KLine
	read uint64
}

// correction is a version on market.corrections, and the fields it names.
type correction struct {
	version
	corrected []string
}

func (a correction) same(b correction) bool {
	if a.version != b.version || len(a.corrected) != len(b.corrected) {
		return false
	}
	for i := range a.corrected {
		if a.corrected[i] != b.corrected[i] {
			return false
		}
	}
	return true
}

// stamp is a correction's identity: its minute and its read.
type stamp struct {
	minute
	read uint64
}

// held is what the store holds of a fetch's minutes: their raw, their corrections of the fetch's stamps, and their
// corrected raw.
type held struct {
	raw         map[minute]version
	corrections map[stamp]correction
	corrected   map[minute]version
}

// rows is what a write puts in the store: rows of the raw and of the corrections, inserted; rows of the corrected
// raw, each inserted or replacing its minute's.
type rows struct {
	raw         []row
	corrections []crow
	corrected   []row
}

type row struct {
	m minute
	v version
}

type crow struct {
	s stamp
	c correction
}

// put is what a write did: the rows inserted of the raw and the corrections (a row there already, put meanwhile by
// another member of the group, not inserted), and the corrected raw's rows put.
type put struct{ raw, corrections, corrected int }

// store is the database the writer keeps: four series, the raw, the corrections, the corrected raw and the clean
// (README.md, The store).
type store interface {
	// Empty: whether the raw holds no bar.
	Empty(ctx context.Context) (bool, error)
	// Held: what the store holds of the minutes and the stamps.
	Held(ctx context.Context, minutes []minute, stamps []stamp) (held, error)
	// Write puts the rows, at once where the store can.
	Write(ctx context.Context, w rows) (put, error)
	// Clean empties the clean series of the instrument and timeframe and writes the rows, and gives the rows of the
	// series it holds after.
	Clean(ctx context.Context, instrument, timeframe string, rs []row) (int, error)
}

// consume reads both topics as o says and keeps Massive's versions in st: the raw, each minute's first version from
// market.raw, a copy dropped; the corrections from market.corrections, each by its minute and read, a copy dropped;
// the corrected raw, each minute's latest version, the one of the greatest read. It reads from its group's committed
// offsets, or from the topics' start when the raw holds no bar or o.toEnd; it commits what it has written. A message
// that is not raw market data (0.3.0, or 0.2.0 or 0.1.0, read as before) on market.raw, or market corrections 0.1.0
// on market.corrections, or one of the source massive whose K-line is not Massive's (massive.ReadKLine), stops it.
// Every price and quantity of market's decimal fits the store (its migration 0002); vw and n, Massive's own, each
// optional, are NULL where a bar has none (0003); the four series by 0004.
func consume(ctx context.Context, bus stream.Bus, st store, o options, log *slog.Logger) (report, error) {
	var r report
	empty, err := st.Empty(ctx)
	if err != nil {
		return r, err
	}
	start := stream.StartFromCommitted
	if empty || o.toEnd {
		start = stream.StartFromBeginning
	}
	adm, err := bus.Admin(ctx)
	if err != nil {
		return r, err
	}
	spans := map[string]map[int32]stream.Span{}
	for _, t := range raw.Topics {
		if spans[t], err = adm.Spans(ctx, t); err != nil {
			adm.Close(ctx)
			return r, err
		}
		for p, s := range spans[t] {
			if s.End > s.Start {
				log.Info("the topic", "topic", t, "partition", p, "start", s.Start, "end", s.End)
			}
		}
	}
	adm.Close(ctx)
	log.Info("reading", "topics", raw.Topics, "group", o.name, "from_start", start == stream.StartFromBeginning,
		"store_empty", empty, "to_end", o.toEnd)
	src, err := bus.Source(ctx, stream.SubscribeOptions{Group: o.name, Topics: raw.Topics, Start: start})
	if err != nil {
		return r, err
	}
	defer src.Close(context.Background())
	next := map[string]map[int32]int64{raw.Topic: {}, raw.CorrectionsTopic: {}}
	// done: every partition of both read to its end as it stood, or holding nothing then
	done := func() bool {
		for t, ps := range spans {
			for p, s := range ps {
				if max(next[t][p], s.Start) < s.End {
					return false
				}
			}
		}
		return true
	}
	for !(o.toEnd && done()) {
		recs, err := src.Fetch(ctx)
		if err != nil {
			return r, err
		}
		var taken []stream.Record
		var raws []row
		var corrs []crow
		rawAt, corrAt := map[minute]int{}, map[stamp]int{}
		for _, rec := range recs {
			if o.toEnd && rec.Offset >= spans[rec.Topic][rec.Partition].End {
				continue // published after the run began: neither written nor committed
			}
			if _, ok := next[rec.Topic][rec.Partition]; !ok {
				log.Info("took up", "topic", rec.Topic, "partition", rec.Partition, "offset", rec.Offset)
			}
			next[rec.Topic][rec.Partition] = rec.Offset + 1
			var m raw.Message
			var fields []string
			switch rec.Topic {
			case raw.Topic:
				if m, err = raw.FromRecord(rec); err != nil {
					return r, err
				}
			case raw.CorrectionsTopic:
				c, err := raw.CorrectionFromRecord(rec)
				if err != nil {
					return r, err
				}
				m, fields = c.Message, c.Corrected
			default:
				return r, fmt.Errorf("%s/%d@%d: a topic not asked for", rec.Topic, rec.Partition, rec.Offset)
			}
			r.read++
			taken = append(taken, rec)
			if m.Source != massive.Source {
				r.other++
				continue
			}
			k, err := massive.ReadKLine([]byte(m.KLine.Text()))
			if err != nil {
				return r, fmt.Errorf("%s/%d@%d: %w", rec.Topic, rec.Partition, rec.Offset, err)
			}
			mi, v := minute{instrument: m.Instrument, timeframe: m.Timeframe, date: k.Date}, version{k: k, read: m.Read}
			if rec.Topic == raw.Topic {
				if i, ok := rawAt[mi]; ok { // a copy in the fetch: the first taken
					r.raw.copies++
					if raws[i].v != v {
						r.raw.differing++
						log.Warn("a copy of another value dropped", "topic", rec.Topic, "key", m.Key(), "date", k.Date,
							"offset", rec.Offset)
					}
					continue
				}
				rawAt[mi] = len(raws)
				raws = append(raws, row{m: mi, v: v})
				continue
			}
			s, c := stamp{minute: mi, read: m.Read}, correction{version: v, corrected: fields}
			if i, ok := corrAt[s]; ok {
				r.corrections.copies++
				if !corrs[i].c.same(c) {
					r.corrections.differing++
					log.Warn("a copy of another value dropped", "topic", rec.Topic, "key", m.Key(), "date", k.Date,
						"read", m.Read, "offset", rec.Offset)
				}
				continue
			}
			corrAt[s] = len(corrs)
			corrs = append(corrs, crow{s: s, c: c})
		}
		if len(raws)+len(corrs) > 0 {
			if err := write(ctx, st, raws, corrs, &r, log); err != nil {
				return r, err
			}
		}
		if err := src.Commit(ctx, taken...); err != nil {
			return r, err
		}
	}
	return r, nil
}

// write keeps a fetch's versions: each of the raw and of the corrections not held already, a copy dropped; then, for
// each minute of the fetch, its corrected raw the latest of the versions it has, held or new, the one of the greatest
// read, the row held kept on a tie.
func write(ctx context.Context, st store, raws []row, corrs []crow, r *report, log *slog.Logger) error {
	var minutes []minute
	seen := map[minute]bool{}
	for _, x := range raws {
		if !seen[x.m] {
			seen[x.m] = true
			minutes = append(minutes, x.m)
		}
	}
	stamps := make([]stamp, 0, len(corrs))
	for _, x := range corrs {
		stamps = append(stamps, x.s)
		if !seen[x.s.minute] {
			seen[x.s.minute] = true
			minutes = append(minutes, x.s.minute)
		}
	}
	h, err := st.Held(ctx, minutes, stamps)
	if err != nil {
		return err
	}
	var w rows
	for _, x := range raws {
		if was, ok := h.raw[x.m]; ok {
			r.raw.copies++
			if was != x.v {
				r.raw.differing++
				log.Warn("a copy of another value dropped, the first version stored kept", "instrument", x.m.instrument,
					"timeframe", x.m.timeframe, "date", x.m.date)
			}
			continue
		}
		w.raw = append(w.raw, x)
		h.raw[x.m] = x.v
	}
	of := map[minute][]version{}
	for _, x := range corrs {
		if was, ok := h.corrections[x.s]; ok {
			r.corrections.copies++
			if !was.same(x.c) {
				r.corrections.differing++
				log.Warn("a copy of another value dropped, the correction stored kept", "instrument", x.s.instrument,
					"timeframe", x.s.timeframe, "date", x.s.date, "read", x.s.read)
			}
			of[x.s.minute] = append(of[x.s.minute], was.version)
			continue
		}
		w.corrections = append(w.corrections, x)
		of[x.s.minute] = append(of[x.s.minute], x.c.version)
	}
	for _, mi := range minutes {
		was, stored := h.corrected[mi]
		best, have := was, stored
		cands := of[mi]
		if v, ok := h.raw[mi]; ok {
			cands = append(cands, v)
		}
		for _, v := range cands {
			if !have || v.read > best.read {
				best, have = v, true
			}
		}
		if have && (!stored || best != was) {
			w.corrected = append(w.corrected, row{m: mi, v: best})
		}
	}
	p, err := st.Write(ctx, w)
	if err != nil {
		return err
	}
	r.raw.stored += p.raw
	r.raw.copies += len(w.raw) - p.raw
	r.corrections.stored += p.corrections
	r.corrections.copies += len(w.corrections) - p.corrections
	r.corrected += p.corrected
	if len(w.raw)+len(w.corrections)+len(w.corrected) > 0 {
		log.Info("wrote", "raw", p.raw, "corrections", p.corrections, "corrected", p.corrected)
	}
	return nil
}
