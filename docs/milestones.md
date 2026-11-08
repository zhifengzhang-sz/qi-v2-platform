# The milestones

The project's milestones and where each stands now. Where the last session stopped: `docs/opening.md`.

The work they record from before this repository's start was done in the platform's earlier history, kept by the owner and not in this repository, which starts from that history's last tree (`docs/decisions.md`, the history begun anew); a commit a document names from before that start is of that history.

| # | Milestone | Stands |
|---|---|---|
| 1 | The stream and the warehouse in Go | closed: every program Go; the old platform, `go/`, removed, the core in its place (4) |
| 2 | The platform as a product | not started |
| 3 | The tooling to the practices of every qi-v2 project | the flake, the kit, the Makefile and the state documents in place; every input public, read with no login; no CI |
| 4 | The lean core and its additions | the core, naming no source, its first path and its first stores: every minute of SPY's session from Massive on `market.raw`, each first version, and Massive's revisions on `market.corrections`, both kept forever, read by a consumer and kept by two stores, TimescaleDB's and ClickHouse's, four series each; the first cleaning, the validation, SPY's clean topic kept as the record changes and made again past its tolerance, the stores' clean series written from it; `query`, the topics read in no group; the platform's library, Massive's reader and Massive's K-line in it; acquisition re-reading 30 days, each page kept once; `make start`, run on the stack; `make check`'s tests on the test stack; the tutorial on the core, three levels, every example run on the test stack; a user reads the topics, not the stores |

## 1 The stream and the warehouse in Go

Everything that moves data onto, along and off the stream, in Go (`README.md`, Operations on the stream).

**Stands.** Closed. Every program is Go, the TypeScript retired whole. The old platform, `go/` (the warehouse and its
data layer, ingest, replay, the engine on the stream, the record, the live feed, topic provisioning), is removed, ahead
of milestone 4's end, since it took a private project's executable; the core and its additions stand in its place (4).

**Left.** None of its own: what the core lacks is milestone 4's Left.

## 2 The platform as a product

The programs published as a flake package and as an image, taken by a taker without this folder.

**Stands.** Nothing published; the contract stated (`README.md`, The platform contract).

**Left.** The core's programs as a flake package and as one image by qi-v2's `releaseImage`.

## 3 The tooling to the practices of every qi-v2 project

The practices of `qi-v2-nix`'s `practices.md`: everything from outside a flake input, the locks checked, the agents'
rules from `agents/` by the kit, the state of the work in git.

**Stands.** `flake.nix` (qi-v2's shell with Go; the kit) and `flake.lock`, every input a public repository read with
no login; the `Makefile` at the root (`install`, `agents`, `verify`, `locks-check`, the role runs, the checks,
`modules`); `agents/` and the generated `AGENTS.md`; `docs/opening.md`, `milestones.md`, `decisions.md`, `designs/`,
`process.md`; `PRODUCTS.md`, checked by `make products-check`; the services stack an input, `qi-v2-services`' package
`stack` (`make stack-up`), and its test stack, for `make check`'s tests and trial runs (`make stack-test-up`, wiped by
`make stack-test-down`); `make start`, the platform's components kept running on a stack (`dev/scripts/run.py`), run
on both stacks; `make check` running each step once, the modules' work decided by Go's caches, the tests that reach a
stack keyed by the test stack's state (`go.toolchain.md`, 4), the modules they need fetched by `make modules`.

**Left.** CI in the flake's shell, none today (`docs/decisions.md`, Open).

## 4 The lean core and its additions

The platform as a lean core, every function an addition to it, a publisher or a consumer speaking only through the
stream by the raw market data schema (`core/README.md`; `docs/decisions.md`, 2026-11-06 to 2026-11-08).

**Stands.** The core (`core/`), naming no source: the stream; its two topics, `market.raw` and `market.corrections`,
provisioned alike, kept forever (retention unlimited, `CreateTime`); raw market data 0.3.0, each message a bar's first
version, the source's K-line (its base market 1.1.0's K-line, from the flake's `contracts`, its other fields the
source's, carried as written) and `read`, when its run read it, 0.2.0 and 0.1.0 read as before; market corrections
0.1.0, each a later version and the fields that differ; every bar of a series' grid on `market.raw`, a bar without one
from its source the zero K-line, its index its place in the grid; a copy dropped by a consumer; the topics' names. The
platform's library (`lib/`), a module on market and the standard library alone, for any app: its package `massive`,
Massive's K-line (the base K-line and Massive's `vw` and `n`, each optional, its base held to market's vectors),
Massive's page schema (checked against Massive's published spec), the fetcher that saves every page, once, named by its
content, the reader of a page. Its first path: Massive's reader, a source plugin the catalog's `massive-spy-1m` names
(`plugins/massive-reader`), a thin program over the library; acquisition (`publishers/acquisition`), its state the two
topics, read at each run and checked against the session's grid, each run re-reading the catalog's `reread` (30 days)
to the last minute the catalog's `delay` (20 h) old, publishing each new minute's first version and each revised
minute's correction, `-follow` running at its start and when the next session's last minute is `delay` old; the printer
(`consumers/printer`). SPY's 194,880 regular-session minutes of 2024-11-06 to 2026-11-06 on `market.raw`, 194,490 of them
twice, each copy the same value, every minute of the grid at its place; 417 corrections on `market.corrections`, Massive's
revisions since the stack's fetches (2026-11-04: 388, `vw`; 2026-11-06: 29, `volume` and `n`, `vw` in 25, `high` in 6,
`low` in 9). Acquisition's clean step after each series' run (`clean.go`): every minute's latest version through the
dataset's cleaning; the series' clean topic made by its first reset, its tail each minute after its last; the damage, a
minute whose `high` or `low` differs from its cleaned latest version; past the dataset's `tolerance` (10) a reset, the
topic made again under its name from the TimescaleDB store's corrected raw, checked against the record, each store's
clean series written from it. The validation, 0.1.0 (`plugins/validation`), the first cleaning, a program of its own
folder by a line protocol (`plugins.md`, 3): every price finite and above 0, `low ≤ min(open, close)`, `max(open,
close) ≤ high`, `volume ≥ 0`, its date after the last accepted, the zero K-line left out. Clean market data 0.1.0
(`core/clean`): one series' and one cleaning's topic, one partition, kept forever, its K-lines' index their place in the
clean series; SPY's, `market.clean.validation.v0-1-0.massive.SPY.M1`, 194,880 K-lines, none refused, the corrected raw's
(`eb2593d7…`). The core's stream: a source of no group, each group's members and offsets on a topic, a topic deleted
with its groups' offsets. `query` (`consumers/query`): the topics, series, bars, a minute's versions, the corrections,
the groups, read in no group, a clean topic among them. The stores, each a consumer in a module of its own under
`consumers/`, a database `massive` on its server:
TimescaleDB's, hypertables, and ClickHouse's, `ReplacingMergeTree`s read with `FINAL`, each reading Massive's own fields
through the library, `vw` and `n` NULL where a bar has none, of the contract's precision (`numeric(21, 6)`, `Decimal(21,
6)`); each keeps four series from both topics: the raw (194,880, `2625a8ae…`, its rows unchanged by the migrations since), the
corrections (417), the corrected raw, each minute's latest version (`eb2593d7…`, differing from the raw in exactly the 417
corrected minutes), the clean (SPY's clean topic as its last reset made it, `eb2593d7…`, without `read`, `0005`); each
series one md5 over its topic and both stores, made again from the topics alone, on the test stack from an empty
state. `make start`: the stack up, then acquisition and both stores kept running,
`make stack-down` pausing them and a start continuing each from where it stopped. The tutorial (`docs/tutorial/`), in
three levels, each its page and its programs, every example run on the test stack: reading the platform (`make start
STACK=test` with the tutorial's catalog, the topics by `query`, `clean-reader` reading the clean topic in no group on
franz-go and market alone); a consumer on the core's stream in the group of its name (`clean-follower`, committing
after what it has done, a copy dropped); a cleaning of one's own (`despike` 0.1.0), its clean topic made by
acquisition, corrections of one's own up to its tolerance and past it (`correct-high`, on the test stack alone), the
reset held off by a reader running and made once it is stopped. The platform contract: a user reads the topics, not
the stores.

**Left.**
- The charting consumer, the next step: one series' chart in a browser, drawn by visualization's `@qi/chart` 2.0.1,
  taken by lock, and updated as the series' K-lines come, of the K-lines and their volume alone; designed
  (`docs/designs/design-chart-consumer-2026-11-09.md`), its build to come.
- A live feed of the core's topics for browser clients, a gateway on the core: what of it the charting consumer leaves
  (its design, What is left).
- The other sources as plugins (Binance's, a CSV file's), and a source plugin's live stream (`plugins.md`, 5).
- Runs in the session: Massive's free plan gives no bar of the current trading day (`lib/README.md`); acquisition on
  `massive-spy-1m` first reads a minute 20 h after its end (`delay`).
- A clean topic's readers stopped and started again by their process manager at a reset (none exists yet; a reader
  running stops it), and a reset cut short by a crash made whole (`docs/decisions.md`, 2026-11-08, step 8.8).
- CI, none today, for the core, the library, the additions and the tutorial's programs (`docs/decisions.md`, Open, CI
  in the flake's shell).
- The library taken outside this repository: a version, and market's Go form without `go.work` (milestone 2).
