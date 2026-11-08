# The core

The platform's lean core, a Go module of its own (`core/`, `github.com/zhifengzhang-sz/qi-v2-platform/core`). It holds
only what the publishers and consumers on it need, and every function is an addition to it, a publisher or a consumer
speaking only through the stream by the schema below.

| Package | What it is |
|---|---|
| `stream` | the stream's interface: a keyed sink whose partition is chosen by a partition key, an ordered source with offset commit, a source of no group, which reads every partition from its start and commits nothing, topic provisioning, a topic deleted with every group's offsets on it, each partition's span of offsets, a topic's configuration, read and set, and each group's members and committed offsets on a topic |
| `stream/kafka`, `stream/mem` | its Kafka implementation (franz-go, against Redpanda) and its in-memory one |
| `stream/streamtest` | their contract test, which both pass |
| `raw` | the raw market data schema, version 0.3.0, the market corrections schema, version 0.1.0, and the provisioning of their topics, and of any topic of the core's schemas by its spec |
| `clean` | the clean market data schema, version 0.1.0: a cleaning's topic of one series, its name, its provisioning and its values |
| `cmd/provision` | `provision [-brokers localhost:19092] [-replicas 1]`: makes `market.raw` and `market.corrections`, alike, and sets the schemas' configuration on each topic itself, made or already there (`retention.ms` and `retention.bytes` -1, `cleanup.policy` `delete`, `message.timestamp.type` `CreateTime`), none left to the cluster's defaults; refuses a topic whose partitions, cleanup policy or timestamp type are not the schemas'; a line a topic |

## The raw market data schema, version 0.3.0

- **The topic** is `market.raw`, the core's, for every source and instrument: each bar's first version, as the run that
  published it read it. Its key is `<source>:<instrument>:<timeframe>`, each name of letters, digits, `.`, `_` and
  `-`, from a letter or a digit. It is partitioned by the instrument alone: 16 partitions, a message's partition the
  one Kafka's default partitioner gives its instrument (murmur2 of its bytes, the sign bit cleared, mod 16). It keeps
  every record, forever (`retention.ms` and `retention.bytes` -1), and is never compacted: it is the raw record, from
  whose start any store, one added later too, fills itself; it is never made again.
- **The value** is one JSON object in canonical form, its fields in the order of their names, no spaces: `instrument`;
  `kline`, the source's K-line in its canonical text; `origin`, `live`, `replay` or `simulated`; `publisher`; `read`,
  when the run that published it read it, milliseconds since 1970 UTC, a whole number from 1 to market's bound;
  `schema`, `market.raw`; `source`, where the bar's data came from, never its reader; `timeframe`; `version`,
  `0.3.0`. The record's timestamp is the bar's `date` (`message.timestamp.type` `CreateTime`), never the wall clock,
  so that it means market time.
- **The K-line** (`raw.KLine`) is the source's: one JSON object, its fields in the order of their names (ASCII), no
  spaces (market's canonical text, market 1.1.0's SPEC.md, The JSON). Its base fields, `index`, `date`, `open`, `high`,
  `low`, `close` and `volume`, are market 1.1.0's K-line, which the core reads with every rule of market's JSON
  (`raw.DecodeKLine`, by market's own reader) and gives as `Base`; each other field is the source's, its name a letter
  and then letters and digits, its value a number market's JSON reads, in the one text market writes for it (a decimal
  of market's precision, or a whole number from 0 to market's bound), carried as it is written, its meaning the
  source's: the source's reader reads it (each in the platform's library, `../lib/README.md`). Not a K-line: `asOf`, the
  ongoing K-line's.
- **Every bar of a series' grid** is on `market.raw`, from the series' first: its publisher's grid, every bar of the
  series' session (acquisition's, its catalog dataset's, `../docs/impl/engineering/plugins.md`, 3). A bar its source
  gives none for, a minute without an eligible trade, is the **zero K-line** (`raw.Zero`): the base's fields alone,
  `open`, `high`, `low`, `close` and `volume` 0, no field of the source's own, a K-line of market 1.1.0 (a price may be
  0, and `0 ≤ 0 ≤ 0` keeps its rule). A bar's `index` is its place in the grid from the series' first, and never moves.
- **Delivery** is at least once: a consumer drops a message whose key and bar's `date` it has taken already, so that
  two copies of a value are one, the first taken; a later version of a bar is a correction, on `market.corrections`.
- **Each component's identity** is its name, and its consumer group when it reads in one: a publisher writes its name as
  `publisher`, and a consumer that reads in a group reads in the group of its name. A reader in no group (`query`; a
  store's writer writing its clean series, `-clean`; acquisition reading a clean topic) joins none and commits nothing.
- **Versions 0.2.0 and 0.1.0**, the versions before, are read as before (`raw.Decode`), each a first version with no
  `read` (`Read` 0): a value of 0.2.0 is one of 0.3.0 but its `version` and its `read`, which it carries not; one of
  0.1.0 is one of 0.2.0 but its `version`, its K-line one of market 1.0.0 in market's canonical text, which is its
  source's K-line in its canonical text. A value is written as 0.3.0.

## The market corrections schema, version 0.1.0

- **The topic** is `market.corrections`, the core's, for every source and instrument: each later version of a bar on
  `market.raw` that differs from the bar's version before it, as a run read it again. It is made and configured as
  `market.raw` is (`provision`): its key, its partitioning by the instrument, 16 partitions, every record kept
  forever, never compacted, the record's timestamp the bar's `date` (`CreateTime`); it is never made again.
- **The value** (`raw.Correction`) is raw market data 0.3.0's value, its `read` given, and `corrected`, the names of
  the fields in which its K-line differs from the bar's version before it (its last correction, else its first
  version on `market.raw`), in the order of their names (ASCII), each once: a field of either K-line whose value is
  another, or which the other has not (`raw.Differ`); never `date`, by which the correction is of its bar. One JSON
  object in canonical form, its fields in the order of their names, no spaces: `corrected`, a list of names;
  `instrument`; `kline`; `origin`; `publisher`; `read`; `schema`, `market.corrections`; `source`; `timeframe`;
  `version`, `0.1.0`.
- **A bar's latest version** is its correction of the greatest `read`, else its first version: from the two topics
  alone, any result can be made again from what was known at any time, each version's `read`.
- **Delivery** is at least once: a consumer drops a message whose key, bar's `date` and `read` it has taken already.

## The clean market data schema, version 0.1.0

- **A clean topic** is one series' and one cleaning's: each K-line of the series a cleaning gave, in the order of their
  dates, its `index` its place in the clean series from 0. Its name is
  `market.clean.<cleaning>.<version>.<source>.<instrument>.<timeframe>` (The topics' names), a series a name of which
  holds a dot refused one, with that reason, until such names are settled. It has one partition; its key is its series'
  (`<source>:<instrument>:<timeframe>`); it keeps every record, forever (`retention.ms` and `retention.bytes` -1), and is
  never compacted; the record's timestamp is the bar's `date` (`CreateTime`); it is provisioned when it is first made
  (`clean.Provision`), as the core's topics are but its partition. It is made again under its name, from its start: a
  reader of it, started again after it is made again, reads it from its start, as if nothing had happened, the topic
  deleted with every group's offsets on it; no field marks it made again. Its publisher is acquisition, which keeps it
  as the record changes (`../docs/impl/engineering/plugins.md`, 3).
- **The value** (`clean.Message`) is one JSON object in canonical form, its fields in the order of their names, no
  spaces: `cleaning`, an object of the cleaning's `name` (lower-case letters, digits and `-`, from a letter) and
  `version` (`MAJOR.MINOR.PATCH`), in that order; `instrument`; `kline`, the cleaned K-line, the source's, in its
  canonical text, as raw market data's (`raw.KLine`), its `index` its place in the clean series; `origin`, the
  minute's, as `market.raw` has it; `publisher`; `schema`, `market.clean`; `source`; `timeframe`; `version`, `0.1.0`.
  It carries no `read`: it is the K-line of the minute's latest version when the cleaning was given it. The record's
  timestamp is the bar's `date`.
- **Delivery** is at least once: a consumer drops a message whose key and bar's `date` it has taken already, the first
  taken.

## The topics' names

`market.raw` and `market.corrections` are each one topic for every series, keyed by its series, and are never made
again. Every other topic is one series', named by its kind, by everything that derived it, each with its version, and
by its series, its key's three names (`<source>.<instrument>.<timeframe>`), none holding a dot:

| Kind | Name | Example |
|---|---|---|
| the raw market data | `market.raw` | `market.raw` |
| the market corrections | `market.corrections` | `market.corrections` |
| a cleaning's | `market.clean.<cleaning>.<version>.<source>.<instrument>.<timeframe>` | `market.clean.validation.v0-1-0.<source>.SPY.M1` |
| an engine's, over a cleaning | `<engine>.<implementation>.<version>.<cleaning>.<version>.<source>.<instrument>.<timeframe>` | `<engine>.go.v0-2-0.validation.v0-1-0.<source>.SPY.M1` |

A version is written with hyphens, `v0-2-0` for 0.2.0, so that a name's dots part its names alone. A topic made again
keeps its name. A topic derived from a clean topic ends in the clean topic's name after `market.clean`
(`.validation.v0-1-0.<source>.SPY.M1`), and is deleted with it when it is made again (`clean.Derived`). Each is made
when the path that needs it comes: of the core's topics, `market.raw`, `market.corrections` and the validation's clean
topic of each series acquisition cleans exist today. An engine that reads a clean topic, and the topics it makes, are
not the platform's: the engine is a consumer's, and its topic is named so that the core deletes it with the clean topic
it was made from.
