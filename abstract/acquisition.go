package abstract

import (
	"context"
	"errors"
	"fmt"
)

// Kind is one of the three kinds of data acquisition publishes, each to topics of its own kind.
type Kind int

const (
	// Raw: the raw series, the vendor's corrected raw series and then its raw series.
	Raw Kind = iota
	// Clean: the clean series, made from the raw series.
	Clean
	// Corrections: the vendor's corrections, published as they come and applied to nothing.
	Corrections
)

// kinds are the three, in their order.
var kinds = [...]Kind{Raw, Clean, Corrections}

func (k Kind) String() string {
	switch k {
	case Raw:
		return "raw"
	case Clean:
		return "clean"
	case Corrections:
		return "corrections"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// AllTopics is acquisition's topics, of every kind, each with its logs.
type AllTopics struct {
	Raw         []TopicLogs
	Clean       []TopicLogs
	Corrections []TopicLogs
}

// Of gives the topics of one kind; none for a kind that is not one of the three.
func (t AllTopics) Of(k Kind) []TopicLogs {
	switch k {
	case Raw:
		return t.Raw
	case Clean:
		return t.Clean
	case Corrections:
		return t.Corrections
	}
	return nil
}

// Inputs is what acquisition reads, and nothing else in a working space does: its source, the vendor's corrected raw
// series first and then its raw series, of messages Src; and the vendor's corrections stream, of messages Corr. They
// are two logs, each read whole by its own part: one cursor given as both is refused (ErrSameInput), where it is
// given as one pointer or one channel.
type Inputs[Src, Corr any] struct {
	Source      Cursor[Src]
	Corrections Cursor[Corr]
}

func (in Inputs[Src, Corr]) check() error {
	if in.Source == nil {
		return errors.New("no source to read")
	}
	if in.Corrections == nil {
		return errors.New("no corrections stream to read")
	}
	if source, corrections := any(in.Source), any(in.Corrections); told(source) && source == corrections {
		return ErrSameInput
	}
	return nil
}

// A Reader is acquisition's reader: it reads the source and gives the raw series, of k-lines Kline.
type Reader[Src, Kline any] interface {
	Read(ctx context.Context, source Cursor[Src], raw Appender[Kline]) error
}

// A Cleaner is acquisition's cleaner: it reads the raw series and gives the clean series.
type Cleaner[Kline any] interface {
	Clean(ctx context.Context, raw Cursor[Kline], clean Appender[Kline]) error
}

// A RawPublisher is the publisher's part for the raw series: it reads the raw series and writes the raw topics,
// which are the topics it is given.
type RawPublisher[Kline any] interface {
	PublishRaw(ctx context.Context, raw Cursor[Kline], topics AppendSet) error
}

// A CleanPublisher is the publisher's part for the clean series: it reads the clean series and writes the clean
// topics, which are the topics it is given.
type CleanPublisher[Kline any] interface {
	PublishClean(ctx context.Context, clean Cursor[Kline], topics AppendSet) error
}

// A CorrectionsPublisher is the publisher's part for the vendor's corrections: it reads the corrections stream and
// writes the corrections topics, which are the topics it is given.
type CorrectionsPublisher[Corr any] interface {
	PublishCorrections(ctx context.Context, corrections Cursor[Corr], topics AppendSet) error
}

// Acquisition is the acquisition of a k-line contract: its reader, its cleaner and its publisher, in three parts,
// one a kind. Src is what the reader reads, Kline the contract's k-line (raw, corrected raw and clean k-lines are
// all this), Corr a message of the vendor's corrections stream.
type Acquisition[Src, Kline, Corr any] struct {
	Reader             Reader[Src, Kline]
	Cleaner            Cleaner[Kline]
	PublishRaw         RawPublisher[Kline]
	PublishClean       CleanPublisher[Kline]
	PublishCorrections CorrectionsPublisher[Corr]
}

func (a Acquisition[Src, Kline, Corr]) check() error {
	switch {
	case a.Reader == nil:
		return errors.New("acquisition: no reader")
	case a.Cleaner == nil:
		return errors.New("acquisition: no cleaner")
	case a.PublishRaw == nil:
		return errors.New("acquisition: no publisher of the raw series")
	case a.PublishClean == nil:
		return errors.New("acquisition: no publisher of the clean series")
	case a.PublishCorrections == nil:
		return errors.New("acquisition: no publisher of the corrections")
	}
	return nil
}

// appendSets gives, for each kind, the topics of that kind to append to: no two of a kind of one name, and no log
// in two partitions, of one kind or of two.
func (t AllTopics) appendSets() ([len(kinds)]AppendSet, error) {
	var sets [len(kinds)]AppendSet
	seen := logsSeen{}
	for _, k := range kinds {
		s, err := newTopicSet(t.Of(k))
		if err != nil {
			return sets, fmt.Errorf("acquisition: the %v topics: %w", k, err)
		}
		for _, l := range s.topics {
			if err := seen.add(l); err != nil {
				return sets, fmt.Errorf("acquisition: %w", err)
			}
		}
		sets[k] = AppendSet{appendTo: s}
	}
	return sets, nil
}

// Run is acquisition as one process, from the source and the corrections to every topic: the reader gives the raw
// series from the source; the cleaner gives the clean series from the raw series; each part of the publisher is
// given one series and the topics of its own kind alone, the raw series for the raw topics, the clean series for
// the clean topics, the corrections stream for the corrections topics. The corrections reach their publisher and
// nothing else. A part finds a topic by its name, so two topics of a kind of one name are refused (ErrSameName),
// and so is a log given for two partitions (ErrSameLog). It returns when every part has returned, with the first
// error.
func (a Acquisition[Src, Kline, Corr]) Run(ctx context.Context, in Inputs[Src, Corr], topics AllTopics) error {
	if err := a.check(); err != nil {
		return fmt.Errorf("abstract: %w", err)
	}
	if err := in.check(); err != nil {
		return fmt.Errorf("abstract: acquisition: %w", err)
	}
	sets, err := topics.appendSets()
	if err != nil {
		return fmt.Errorf("abstract: %w", err)
	}
	if err := a.run(ctx, in, sets); err != nil {
		return fmt.Errorf("abstract: acquisition: %w", err)
	}
	return nil
}

func (a Acquisition[Src, Kline, Corr]) run(ctx context.Context, in Inputs[Src, Corr], topics [len(kinds)]AppendSet) error {
	const toCleaner, toPublisher = 0, 1
	raw := newSeries[Kline](2)
	clean := newSeries[Kline](1)
	g := newGroup(ctx)
	g.run("the reader", func(ctx context.Context) error {
		return a.Reader.Read(ctx, readOnly[Src]{from: in.Source}, appendOnly[Kline]{to: raw})
	})
	g.run("the cleaner", func(ctx context.Context) error {
		defer raw.leave(toCleaner)
		return a.Cleaner.Clean(ctx, raw.cursor(toCleaner), appendOnly[Kline]{to: clean})
	})
	g.run("the publisher of the raw series", func(ctx context.Context) error {
		defer raw.leave(toPublisher)
		return a.PublishRaw.PublishRaw(ctx, raw.cursor(toPublisher), topics[Raw])
	})
	g.run("the publisher of the clean series", func(ctx context.Context) error {
		defer clean.leave(0)
		return a.PublishClean.PublishClean(ctx, clean.cursor(0), topics[Clean])
	})
	g.run("the publisher of the corrections", func(ctx context.Context) error {
		return a.PublishCorrections.PublishCorrections(ctx, readOnly[Corr]{from: in.Corrections}, topics[Corrections])
	})
	return g.wait()
}
