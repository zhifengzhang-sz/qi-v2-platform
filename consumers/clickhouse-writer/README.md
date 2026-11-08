# The ClickHouse writer

A consumer on the core (`../../core/README.md`), a module of its own on the core, the platform's library and market
alone, ClickHouse reached through its HTTP interface with the standard library: it reads the core's two topics,
`market.raw` and `market.corrections`, and keeps their Massive K-lines in a database of ClickHouse, four series. It is a program under the core's rules, not ClickHouse's Kafka engine, and it changes nothing else. It is
this folder, its entry in the root `Makefile`'s `CORE`, its database and its consumer group; removing those removes it.

## The program

```
clickhouse-writer [-brokers localhost:19092] [-name clickhouse-writer]
    [-url http://default:password@localhost:8123] [-database massive] [-to-end] [-clean TOPIC]
```

- **Its identity** is its name, `clickhouse-writer`, and its consumer group, the same.
- **Its store** is the database `-database` on the server `-url` names (the local stack's by default). It is made when
  it is not there, then migrated by the writer's own migrations (`migrations/`, recorded in `schema_migrations`), each
  statement one that may run again, since ClickHouse runs a migration's statements and its row one by one.
- **What it reads.** Both topics, from its group's committed offsets, or from their start when the raw holds no bar,
  so a store dropped is made again from the topics alone by running the writer; `-to-end`: from the topics' start to
  their end as they stood when the run began, then it stops. Without `-to-end` it keeps running, each message taken as
  it comes, until it is stopped (`make start` runs it so, `../../docs/impl/engineering/dev.environment.md`, 3). It
  commits what it has written, after the write: at least once.
- **Each message** of `market.raw` is read as raw market data 0.3.0, or 0.2.0 or 0.1.0 as before (`raw.FromRecord`),
  each of `market.corrections` as market corrections 0.1.0 (`raw.CorrectionFromRecord`), and one that is not stops it;
  the source `massive`'s are kept, each K-line read as Massive's by the library (`massive.ReadKLine`,
  `../../lib/README.md`), one that is not Massive's stopping it too; the others passed over.
- **Its modes for acquisition's reset** (`clean.go`; `../../docs/impl/engineering/plugins.md`, 3), each a run of its
  own beside the writer that follows the topics, reading in no group and committing nothing, then stopping: `-clean
  TOPIC`, the store's clean series of the clean topic's series (its name's last three names; the source `massive`'s
  alone) emptied, then written with every K-line of the topic, from its start to its end as it stood when the run began,
  each read as clean market data 0.1.0 (`clean.FromRecord`) and as Massive's K-line (`massive.ReadKLine`), a copy dropped
  by its date, the first taken, one of another value logged; on stderr, the K-lines read, the copies and the rows
  written.
- **On stderr**: at its start, each topic's offsets and where it reads from, then the offset it takes up at in each
  partition, its first message there; after each write, what it wrote; at the end, the messages read and of another
  source; of each topic, the versions stored and the copies dropped, of them those of another value, each logged; the
  rows of the corrected raw written.

## The store

Four series (`migrations/0001_klines.sql`, its precision `0002_precision.sql`, `vw` and `n` optional
`0003_optional.sql`, the four series `0004_series.sql`, the clean without `read` `0005_clean.sql`), each a table of
Massive's K-line's fields under their names (market 1.1.0's base K-line and Massive's `vw` and `n`, `lib/massive`),
the series' and, but the clean, `read`:

| Table | The series |
|---|---|
| `klines` | the raw: each minute's first version, from `market.raw` |
| `corrections` | the corrections: each from `market.corrections`, by its minute and its `read`, with `corrected`, the fields it names (`Array(String)`) |
| `klines_corrected` | the corrected raw: each minute's latest version, the one of the greatest `read` (a first version without one the earliest) |
| `klines_clean` | the clean: a clean topic's series as it was made, written from it at each of acquisition's resets (`-clean`), never between them; empty until a clean topic is made |

| Column | Type | What |
|---|---|---|
| `instrument`, `timeframe` | `String` | the series, the message's |
| `index` | `Int64` | the K-line's place in its series |
| `date` | `Int64` | the bar's begin, milliseconds since 1970 UTC, as the contract has it |
| `open`, `high`, `low`, `close`, `volume` | `Decimal(21, 6)` | the contract's precision: 6 digits after the point, 15 before it |
| `vw` | `Nullable(Decimal(21, 6))` | the volume-weighted average price, of that precision; NULL where the bar has none |
| `n` | `Nullable(Int64)` | the trades; NULL where the bar has none |
| `read` | `Nullable(Int64)` | when the run that published the version read it, milliseconds since 1970 UTC; NULL where its value carries none (0.1.0, 0.2.0); in `corrections`, `Int64`; not in `klines_clean`, a clean value carrying none |

Each table's engine is `ReplacingMergeTree`, ordered by `(instrument, timeframe, date)`, `corrections` by
`(instrument, timeframe, date, read)`, and each is read with `FINAL`, each key once: of the rows of one key, `FINAL`
and the merges keep the one inserted last.

- **A version once, a copy dropped** (the core's rule, `core/README.md`, Delivery). A version read again in the same
  fetch, of `market.raw` by its minute, of `market.corrections` by its minute and `read`, is dropped before the store
  sees it, the first taken; then the fetch's versions are compared with those stored (read with `FINAL` over each
  series' span of dates): one not stored is inserted; one stored is a copy, dropped, and one stored with another K-line
  or read is counted and logged, never silent, the one stored kept. Then each minute of the fetch has its corrected raw
  the latest of its versions, stored or new, the one of the greatest `read`, the row stored kept on a tie; a row that
  changes is inserted, and wins as the row inserted last. Each table's rows go in one `INSERT`, all or none, the raw's,
  then the corrections', then the corrected raw's, a fetch being far below ClickHouse's block of 1,048,576 rows; the
  three are not one transaction, and a write cut short is made again by the fetch read again, its offsets not
  committed, each minute's corrected raw taken again from its versions. ClickHouse enforces no uniqueness: two rows of
  one key stand until a merge where two members of the group wrote one key at once, in a rebalance, or where a row of
  the corrected raw was replaced; `FINAL` reads each key once, its row inserted last. In that rebalance alone, a
  minute's corrected raw can be left, unlogged, at an earlier version, when the member that read it inserts last; one
  member, as on the local stack, cannot.
- **The precision.** market's decimal has at most 6 digits after the point and 15 from its first not 0, so up to 15
  before the point; `Decimal(21, 6)` holds every one, as it is, nothing rounded. The migration that made it so
  (`0002`) rewrote each part's columns, by a mutation it waits for, every value the same.
- **`vw` and `n` optional.** Massive's spec requires of a row only `o`, `h`, `l`, `c`, `v` and `t`; a bar without `vw`
  or `n` is stored with it NULL (`\N` in the rows the writer sends and reads), two NULLs the same, and a zero K-line, a
  minute without a bar, with both NULL. The migration that made them so (`0003`) made the two columns `Nullable`, a
  mutation it waits for rewriting each part's two columns, every value the same.
- **The four series** (`0004`): `read` added to `klines`, which changes the table's metadata alone, ClickHouse
  rewriting no part; the three tables made, `klines_corrected` filled with `klines`' rows read with `FINAL`, since no
  correction existed when it was made. Each statement may run again: the `INSERT`, run again before the migration's
  row is written, inserts the same rows, which `FINAL` reads once.
- **The clean series** (`0005`): `read` dropped from `klines_clean`, clean market data 0.1.0 carrying none, the table
  empty then, by a statement that waits for it and may run again (`DROP COLUMN IF EXISTS`). `-clean` writes it in
  three steps, not one transaction: the series' rows deleted by a mutation it waits for (`mutations_sync = 2`), then
  every K-line of the topic in one `INSERT`, then the series' rows counted with `FINAL`, which must be the K-lines'.
  Cut short, the store holds the series empty or in part, the run failing and acquisition's reset with it, which then
  deletes the clean topic it made, so that its next run makes it again and runs `-clean` again, writing it whole.

## Tests

`consume_test.go`, the loop on the core's in-memory stream; `store_test.go`, the store on ClickHouse, in a scratch
database of its own that it drops, by `QI_CLICKHOUSE_TEST_URL` (`docs/impl/engineering/go.toolchain.md`, 4), the test stack's;
`without_test.go`, on it too, a K-line without `vw` and `n` read from a page by the library, published and stored
with both NULL. `consume_test.go` holds the core's cases: a copy taken once, of the same value and of another; a 0.2.0
and a 0.1.0 value read as first versions; a zero K-line corrected to a bar and back; a correction read before its first
version; `store_test.go` `0004` on a store holding bars, no part rewritten, and `0005` on one holding each series but
the clean, no part rewritten, the clean without `read`, the statement run again changing nothing; the clean series
written, then again, the series emptied first, another series kept. `clean_test.go`, on the in-memory stream: `-clean`'s
K-lines from a clean topic, a copy dropped and one of another value counted, the series emptied first, an empty topic
emptying it, a topic not a clean one or of another source refused.
