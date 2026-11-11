// Package abstract is the platform's definition in Go, as far as code can hold it: the open parts of the working
// space's components as interfaces, and what the definition fixes built in (README.md, which holds the table from
// each declaration of the definition to its name here). The definition is qi-v2-platform-definition, in Lean, on
// Kahn process networks; this package takes the standard library alone.
//
// # A part
//
// A part is one of the definition's processes: acquisition's reader, cleaner and three publishers (Reader, Cleaner,
// RawPublisher, CleanPublisher, CorrectionsPublisher), the warehouse consumer (Consumer), visualization's chart
// (Chart), the engine's worker (Worker). Each is an interface of one method, and a call of the method is the process
// at work: it is given the reading ends of what it reads (a Cursor, or a ReadSet of topics) and the writing ends of
// what it writes (an Appender, or an AppendSet of topics), and nothing else. It reads by waiting on a cursor and
// writes by appending; it returns when its context is done, with the context's error, or on an error of its own. A
// part that returns nil has stopped: what it has appended stands, and it reads no more.
//
// What the types hold a part to: through what it is given it reaches nothing else, and it can only append. What
// they cannot, and the definition requires (Kahn's condition), is the implementation's to keep:
//
//   - what a part appends depends on what it has read, and on nothing else: not on the time, not on which of two
//     cursors has a message first. A part that reads several cursors waits on one at a time, in an order what it
//     has read decides; reading them as their messages arrive is a merge by arrival, which is no process;
//   - a part never takes back what it appended, which the writing end gives it no means to do, and never waits for
//     the end of what it reads: a log has no end, and a context that is done is not one;
//   - what a part keeps between messages it keeps in the call: the warehouse consumer is called once for each
//     partition, and each call is the consumer on that partition alone.
//
// # What is built in
//
// A topic's shape (Topic, Record, TopicLogs: a name, at least one partition, one log of records a partition, append
// only); the three kinds of data acquisition publishes (Kind); and the wiring of a working space (Wiring, Workspace):
// acquisition alone reads the source and the corrections, the clean series is made from the raw, the corrections
// are applied to nothing, each publisher writes the topics of its kind alone, warehousing reads every topic of
// acquisition's, a partition at a time, into the topic's table, the engine is optional and given clean topics
// only, visualization is given its clean topics and all of the engine's, and two topics of one name are refused when
// the working space is made.
package abstract
