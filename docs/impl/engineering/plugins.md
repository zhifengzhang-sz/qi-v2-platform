# Plugins: extending the platform without changing it

Draft v0.1, 2026-09-25, for review. The platform is open-ended: anyone
who uses it can add a consumer or a publisher of their own, in any
language, without a change to the platform's code. This document says
what a plugin is, where one plugs in, the rules it follows, what exists
today and what is proposed. The hands-on version, on the core, is the tutorial,
[`../../tutorial/README.md`](../../tutorial/README.md).

## 1. What a plugin is

A plugin is a program the platform runs or reads from, speaking one of
the platform's plug protocols. It is not linked into the platform and
does not import its code; it lives wherever its author keeps it. The
platform gives it what is hard to get right — ordered delivery per
series, replay, offsets, provisioning, validation — and the plugin
supplies only its computation.

| Plug point | The plugin | The platform gives it | Status |
|---|---|---|---|
| **Data source** (§3) — a reader of a source's bars | an executable a catalog dataset names (`command`), which prints a series' bars for its spans, one a line, the source's K-line (the base K-line of market 1.1.0 and the source's own fields) | acquisition (`publishers/acquisition`): the dataset's series and history, the spans of its session, each series' index, the publish on `market.raw` (`core/README.md`) | **works today** for `massive-spy-1m` (`plugins/massive-reader`, over the platform's library, `lib/`) |
| **Cleaning** (§3) — a judge of a series' K-lines | an executable a catalog dataset names (`cleaning`), which answers each K-line given it, in order, with the K-line it gives or its refusal and the reason | acquisition: the series' clean topic, one series' (`core/README.md`, clean market data), its tail, its damage and its reset, each store's clean series | **works today** for `massive-spy-1m`: the validation, 0.1.0 (`plugins/validation`) |
| **Stream reader** | any Kafka client | `market.raw`, `market.corrections` and a clean topic | **works today** (a plain consumer, in no group or in the group of its name: the tutorial, `../../tutorial/README.md`, and `consumers/printer`, `consumers/query`) |

A plugin never writes a platform-owned topic (`market.raw`,
`market.corrections`, `market.clean.*`): the core's topics are written
only through acquisition.

## 3. Data sources

A catalog dataset names its source plugin, the reader of its source,
by `command`; its `source` is where the bars' data comes from, never
the reader:

```yaml
- name: massive-spy-1m
  source: massive
  command: [../bin/massive-reader]   # a relative path resolves against the catalog
  instruments: [SPY]
  timeframes: [M1]
  history: 2y
  reread: 30d     # how far back each run asks again
  delay: 20h      # how old a bar must be before it is first read
  cleaning: [../bin/validation]   # its cleaning plugin, the validation
  tolerance: 10   # the damaged minutes a clean topic keeps before it is made again
  session: { zone: America/New_York, open: "09:30", close: "16:00", … }
```

- **The catalog** (`config/catalog.yaml`, its paths relative to its
  folder) holds the datasets, each named once: its `source`; its
  `command`; its series, each of its `instruments` at each of its
  `timeframes` (`M1`, `M5`, `M15`, `M30`, `H1`, `H4`, `D1`, `W1`); its
  `history`, its first bar's distance behind now (a number and `m`,
  `h`, `d` or `y`, a year 365 days) or a date; its `session`, the bars
  whose begin time is in `[open, close)` of a session day in `zone`,
  Monday to Friday, except the `closed` dates, an `early` date closing
  at its own time, a calendar held as data in the catalog, not as code;
  and `reread`, `delay`, `cleaning` and `tolerance`, below. Acquisition
  reads the dataset it is given, any other key not read; `query` reads
  each dataset's series and its session's zone.

- **The plugin.** `command… --backfill INSTRUMENT:TIMEFRAME FROM TO
  [FROM TO]…`, one span or more (milliseconds since 1970, the bars'
  begin times, both ends included; ascending, each after the one
  before), prints the series' closed bars in the spans, ascending, one a
  line, each the source's K-line in its canonical text
  (`core/README.md`: market 1.1.0's base K-line and the source's own
  fields, Massive's `vw` and `n` for `massive`, each optional,
  `lib/README.md`), its `index` its line's place from 0; on stderr
  what it read; exit 0 once it has read the whole of the spans,
  non-zero on a fault. It is not linked into the platform and imports
  nothing of it but the platform's library (`lib/README.md`), which
  takes market and the standard library alone, as any app may.
- **Acquisition** (`publishers/acquisition`;
  `acquisition -dataset NAME -corrected COMMAND -store COMMAND [-store COMMAND]… [-lines FILE] [-follow]`, each
  command its words parted by spaces) publishes every
  bar of each series' **grid** on the core's topics (`core/README.md`):
  the grid is every bar of the dataset's session, a timeframe apart
  from each session day's open while before its close (the catalog's
  rule, above; every multiple of the timeframe for a
  dataset without one), and a bar's `index` is its place in the grid
  from the series' first bar on `market.raw`, so that it never moves;
  a bar the source gives none for is the zero K-line. Its state is
  the record, the two topics themselves, read from their start at each
  run (a consumer of its own group, `-name`, which commits nothing):
  each bar's first version on `market.raw`, a copy dropped, and its
  latest correction on `market.corrections`; the record is checked
  against the grid, every bar from the series' first to its last at its
  place, and one that is not stops the run. Each run asks the plugin
  for one span, cut into each session day's `[open, close)`: from the
  later of the series' first bar and the dataset's `reread` before now,
  or from the bar after the series' last on `market.raw` when that is
  earlier, so that no bar of the grid is left out (a series with none:
  from the history's first bar of the grid), to the last bar of the
  grid whose end is the dataset's `delay` before now, so that it never
  asks for a day the source does not serve yet. A timeframe's length is
  the catalog's (`M1` to `W1`), a dataset of another refused; `reread`
  and `delay`, each a duration as the catalog writes one, are required.
  Once the plugin has read the whole of the span (its exit 0), each
  bar of the grid in it, the plugin's line or the zero K-line, its
  index set: after the series' last on `market.raw`, is published
  there, its first version, origin `live`, publisher its name, `read`
  the time the span was read whole; already there and differing from
  its latest version in any field, is published on
  `market.corrections`, naming the fields that differ; else nothing.
  A line that is not the source's K-line in its canonical text, a bar
  not after the one before it, outside the spans asked or not of the
  grid, or the plugin's exit other than 0 stops the run, nothing of the
  series published. It knows no source. `-lines` keeps the lines as
  read. `-follow` keeps it running: a run at its start, then one
  whenever the last bar of the next session becomes `delay` old (for
  `massive-spy-1m`, once a trading day, at 12:00 New York, at 09:00
  the day after an early close), the times from the catalog's session
  and `delay` alone, the clock read again each minute at most; a run
  that fails is a line of its output, and the next comes at its time,
  asking from the bar after the series' last, so that nothing it left
  out is lost. A start continues from the record: from the bar after
  the series' last on `market.raw`, with its re-read.
- **Massive's reader** (`plugins/massive-reader`), a thin program over
  the library's package `massive` (`lib/README.md`), which holds every
  rule of reading Massive: Massive's aggregates, unadjusted, asked once
  for the whole of the spans (Massive answers whole days from the
  first: its rows outside the spans are counted, not printed), each
  page saved as Massive sent it but its `request_id` (`-pages`, by
  default `~/.local/share/qi/massive/pages`), named by its content
  and kept once, and checked against
  Massive's page schema, every field of its row kept (`t` as `date`;
  `o`, `h`, `l`, `c`, `v`; `vw` and `n`, each when the row has it, as
  Massive's spec allows), the numbers read exactly from Massive's
  digits into market's decimals; a page that breaks the schema, a row
  with a field the K-line has not (`otc`), a field of the base
  missing, more than six digits after the point or a broken rule stops
  it, nothing changed to fit; its calls spaced 12 s (the free plan); the
  key from `~/.config/qi/massive_key`, sent in the Authorization header
  alone.
- **A cleaning** (`cleaning`, a command as `command` is, its program
  resolved against the catalog's folder) is the program acquisition runs
  to clean a series: `command… --clean [--after DATE]`. At its start it
  prints one line naming itself, `{"cleaning":NAME,"version":VERSION}`
  (lower-case letters, digits and `-`, from a letter; `MAJOR.MINOR.PATCH`),
  by which the series' clean topic is named (`core/README.md`, The
  topics' names). Then it reads K-lines on stdin, one a line, a series' in
  the order of their dates, each the source's K-line in its canonical
  text, and answers each with exactly one line, in order: `{"kline":K}`,
  the K-line it gives, the input cleaned, of its input's date, in its
  canonical text, its `index` set by its publisher; or
  `{"refused":"REASON"}`. `--after DATE` (milliseconds since 1970) is the
  date of the last K-line the series' clean topic holds, as a cleaning
  judging the series in order needs it of the run before. It writes its
  answers out whenever no more of its input waits; exit 0 at the end of
  its input, every answer written, non-zero on a fault (a line it cannot
  read). It imports nothing of the platform; it is its folder, its entry
  in the root `Makefile`'s `CORE` (the tutorial's, found by its `go.mod`,
  `TUTORIAL`) and its dataset's `cleaning`, and removing those removes it.
  Acquisition checks the first line, one answer a K-line, each K-line
  given of its input's date and in its canonical text, and the exit 0,
  else the run stops.
- **The validation, 0.1.0** (`plugins/validation`, on market's Go form
  and the standard library alone), the first cleaning: it judges each K-line in order by its base fields and
  refuses the first rule it breaks, with its reason: every price,
  `open`, `high`, `low` and `close`, a finite decimal of market's and
  above 0; `low ≤ min(open, close)`; `max(open, close) ≤ high`;
  `volume ≥ 0`; its `date` after the last it accepted (or after
  `--after`). A K-line keeping every rule it gives as it came. So the
  zero K-line, a minute without a bar, is left out.
- **The clean topic** is kept by acquisition after each series' run,
  once the series' versions are published (`clean.go`), on the dataset's
  `tolerance` (a number of minutes from 0, required with `cleaning`):
  - **The latest versions cleaned**: every minute of the series on
    `market.raw`, its latest version (its correction of the greatest
    `read`, else its first version), given to the cleaning in order;
    what it gives is the series' cleaned latest versions.
  - **Its first reset**: a series with no clean topic has it made by a
    reset (below).
  - **The tail**: else, the clean topic read from its start to its end,
    in no group, each K-line at its place, a copy dropped; each minute
    after its last on `market.raw`, its first version, given to the
    cleaning (`--after` its last), each K-line it gives published on
    the clean topic after the last, numbered after it. A minute refused
    at the end of the clean series is given again with the next, and
    a minute published on `market.raw` by a run that stopped before its
    tail is given at the next run.
  - **The damage**: the minutes whose `high` or `low` in the clean topic
    differs from the cleaned latest version, or that one holds and the
    other not. A minute corrected and corrected back counts nothing,
    one corrected twice once, a correction of a field but `high` and
    `low` nothing; a zero K-line corrected to a bar counts, the cleaned
    latest versions holding a minute the clean topic does not.
  - **The reset**, when the damage is more than the tolerance: the
    TimescaleDB store's corrected raw of the series read once its writer
    has read `market.raw` and `market.corrections` to their ends
    (`-corrected`: the writer's `-corrected KEY`, which waits for its
    group at the topics' ends as they stood, 5 minutes at most), every
    minute to the series' last, and checked against the record, each
    minute's latest version, every field, the run stopping where one
    differs; the clean topic's readers, and those of each topic derived
    from it, stopped, a group with a member reading one stopping the
    reset; the clean topic and each topic derived from it deleted, with
    every group's offsets on them, so that a reader started again reads
    from its start; the clean topic provisioned under its name; the
    corrected raw given to the cleaning, each K-line it gives published,
    numbered from 0; each store's clean series emptied and written from
    the topic (`-store`: each writer's `-clean TOPIC`, its README under
    `consumers/`); the topic read again, the damage after it 0, else
    the run stops. A step that fails once the topic is made deletes it,
    the run failing, so that the next run makes it again whole, as the
    series' first reset, every store's clean series with it; acquisition
    stopped in the middle of a reset (killed, the machine down) deletes
    nothing, and a topic it left made is taken as it is, each store's
    clean series as that reset left it. The stores' clean series is
    written only so: between resets the clean topic grows by its tail,
    and the stores' clean series is the topic as its last reset made it.
  - **Its line**, a second line a series a run: the clean topic, its
    K-lines before the run, the tail (given, published and their index,
    refused, each minute and its reason), the clean series after it,
    the latest versions refused, the damage and its minutes; for a
    reset, when, how many minutes damaged and which, the topics derived
    from it deleted, the minutes read from the store, those published
    and refused, the stores written, the damage after it. A series whose
    name holds a dot is refused a clean topic, with that reason, its line
    saying so, its raw published.
- **A live stream** (`--stream`) is not part of the protocol yet.

## 5. Proposed, for review

1. **The source plugin's live stream** (§3): `--stream`, closed bars as
   they close.
2. Where plugins live: a user's wherever its author keeps it; the
   platform's own under `plugins/`.

Decisions asked of the reviewer: the live stream's protocol (1).
