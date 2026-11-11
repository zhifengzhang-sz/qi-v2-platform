package abstract

import "context"

// A Log holds one log: a sequence of messages that only grows, at its end. It is what a topic's partition is
// (Topic.Logs), given from outside this package: Kafka's by a binding, memory's in a test. It has no end, and no
// means to change or remove a message it holds.
//
// A log has one writer. A working space gives a part the log's writing end alone (Appender) or one of its reading
// ends alone (Cursor), never the log.
type Log[T any] interface {
	Appender[T]
	// Read gives a cursor on the log, at its first message. Each call gives a cursor of its own.
	Read() Cursor[T]
}

// An Appender is the writing end of a log: a part that is given one can only append to it.
type Appender[T any] interface {
	// Append puts m at the end of the log. It returns the context's error when ctx is done before m is appended.
	Append(ctx context.Context, m T) error
}

// A Cursor is a reading end of a log: its messages in the log's order, from the first, each once. A message's
// position in its log is the number of messages the cursor gave before it.
type Cursor[T any] interface {
	// Next gives the next message, waiting until the log holds it. It returns the context's error when ctx is done
	// first; it never says that the log has ended, a log having no end.
	Next(ctx context.Context) (T, error)
}

// appendOnly is a writing end and nothing more: what it wraps cannot be reached through it.
type appendOnly[T any] struct{ to Appender[T] }

func (a appendOnly[T]) Append(ctx context.Context, m T) error { return a.to.Append(ctx, m) }

// readOnly is a reading end and nothing more.
type readOnly[T any] struct{ from Cursor[T] }

func (r readOnly[T]) Next(ctx context.Context) (T, error) { return r.from.Next(ctx) }
