# The TimescaleDB writer

A consumer on the core (`../../core/README.md`), a module of its own on the core, the platform's library and market,
pgx its one other module: it reads the core's two topics, `market.raw` and `market.corrections`, and keeps their
Massive K-lines in a database of TimescaleDB, four series. It changes nothing else. It is this
folder, its entry in the root `Makefile`'s `CORE`, its database and its consumer group; removing those removes it.

## The program

```
timescaledb-writer [-brokers localhost:19092] [-name timescaledb-writer]
    [-url postgres://postgres:password@localhost:5432/postgres] [-database massive] [-to-end]
    [-clean TOPIC | -corrected KEY]
```

- **Its identity** is its name, `timescaledb-writer`, and its consumer group, the same.
- **Its store** is the database `-database` on the server `-url` names by a database on it (a password not in the URL
  is `PGPASSWORD`'s; the defaults are the local stack's). It is made from that database when it is not there, then
  migrated by the writer's own migrations (`migrations/`, recorded in `schema_migrations`).
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
  written. `-corrected KEY`, once its group (`-name`) has committed each partition of `market.raw` and
  `market.corrections` that holds records to its end as it stood when the run began (5 minutes at most, a line on
  stderr each second it waits, `stream.Admin.Groups`), the series' corrected raw on stdout, each row of
  `klines_corrected` of the series in the order of its date, Massive's K-line in its canonical text (`massive.Text`),
  one a line, the source's K-line as `market.raw` and `market.corrections` carry it.
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
| `corrections` | the corrections: each from `market.corrections`, by its minute and its `read`, with `corrected`, the fields it names (`text[]`) |
| `klines_corrected` | the corrected raw: each minute's latest version, the one of the greatest `read` (a first version without one the earliest) |
| `klines_clean` | the clean: a clean topic's series as it was made, written from it at each of acquisition's resets (`-clean`), never between them; empty until a clean topic is made |

| Column | Type | What |
|---|---|---|
| `instrument`, `timeframe` | `text` | the series, the message's |
| `index` | `bigint` | the K-line's place in its series |
| `date` | `bigint` | the bar's begin, milliseconds since 1970 UTC, as the contract has it |
| `open`, `high`, `low`, `close`, `volume` | `numeric(21,6)` | the contract's precision: 6 digits after the point, 15 before it |
| `vw` | `numeric(21,6)` | the volume-weighted average price, of that precision; NULL where the bar has none |
| `n` | `bigint` | the trades; NULL where the bar has none |
| `read` | `bigint` | when the run that published the version read it, milliseconds since 1970 UTC; NULL where its value carries none (0.1.0, 0.2.0); in `corrections`, never NULL; not in `klines_clean`, a clean value carrying none |

The primary key of `klines`, `klines_corrected` and `klines_clean` is `(instrument, timeframe, date)`, of
`corrections` `(instrument, timeframe, date, read)`; each a hypertable on `date`, chunks of 7 days of it (604,800,000).

- **A version once, a copy dropped** (the core's rule, `core/README.md`, Delivery). A version read again in the same
  fetch, of `market.raw` by its minute, of `market.corrections` by its minute and `read`, is dropped before the store
  sees it, the first taken; then the fetch's versions are compared with those stored: one not stored is inserted; one
  stored is a copy, dropped, and one stored with another K-line or read is counted and logged, never silent, the one
  stored kept. Then each minute of the fetch has its corrected raw the latest of its versions, stored or new, the one
  of the greatest `read`, the row stored kept on a tie; a row that changes is written. The three writes are one
  transaction, all or none. The primary keys enforce the uniqueness.
- **Its `ON CONFLICT`s and their reasons.** `klines` and `corrections`: `ON CONFLICT … DO NOTHING`, a row put
  meanwhile by another member of the group, in a rebalance, being of the same message, so that the one stored is kept
  and no first version or correction is replaced; `RETURNING` counts what was inserted, the rest counted as copies.
  `klines_corrected`: `ON CONFLICT (instrument, timeframe, date) DO UPDATE … WHERE coalesce(excluded.read, -1) >
  coalesce(k.read, -1)`, a minute's latest version being the one of the greatest `read`, so that a row is replaced by a
  version of a later read alone, whichever member writes last.
- **The precision.** market's decimal has at most 6 digits after the point and 15 from its first not 0, so up to 15
  before the point; `numeric(21,6)` holds every one, as it is, nothing rounded. The migration that made it so
  (`0002`) changed the columns' type alone, the scale the same: PostgreSQL rewrote no row.
- **`vw` and `n` optional.** Massive's spec requires of a row only `o`, `h`, `l`, `c`, `v` and `t`; a bar without `vw`
  or `n` is stored with it NULL, and a zero K-line, a minute without a bar, with both NULL. The migration that made them
  so (`0003`) dropped their `NOT NULL` alone: PostgreSQL rewrote no row.
- **The four series** (`0004`): `read` added to `klines`, a column that may be NULL and has no default, which changes
  the catalog alone, so that PostgreSQL rewrote no row; the three tables made, `klines_corrected` filled with `klines`'
  rows, since no correction existed when it was made.
- **The clean series** (`0005`): `read` dropped from `klines_clean`, clean market data 0.1.0 carrying none, the table
  empty then; a column dropped changes the catalog alone. `-clean` writes it in one transaction: the series' rows
  deleted, then every K-line of the topic inserted, then the series' rows counted, which must be the K-lines', so that
  the store holds the old series or the new; no `ON CONFLICT`: the primary key refuses a minute twice, the transaction
  with it.

## Tests

`consume_test.go`, the loop on the core's in-memory stream; `store_test.go`, the store on TimescaleDB, in a scratch
database of its own that it drops, by `QI_WAREHOUSE_TEST_URL` (`docs/impl/engineering/go.toolchain.md`, 4), the test stack's;
`without_test.go`, on it too, a K-line without `vw` and `n` read from a page by the library, published and stored
with both NULL. `consume_test.go` holds the core's cases: a copy taken once, of the same value and of another; a 0.2.0
and a 0.1.0 value read as first versions; a zero K-line corrected to a bar and back; a correction read before its first
version; `store_test.go` `0004` on a store holding bars, no row changed, and `0005` on one holding each series but the
clean, no row changed, the clean without `read`; the clean series written, then again, the series emptied first, another
series kept, a minute twice refused; the corrected raw read in the order of its dates. `clean_test.go`, on the in-memory
stream: `-clean`'s K-lines from a clean topic, a copy dropped and one of another value counted, the series emptied
first, an empty topic emptying it, a topic not a clean one or of another source refused; `-corrected`'s wait for its
group at the topics' ends.
