# The charting consumer: one series' chart in a browser (2026-11-09)

The charting consumer is an addition to the core: a chart of one series in a browser, drawn by `@qi/chart` and updated
as the series' K-lines come, of the K-lines and their volume alone. This is its design, made before its build; nothing
of it is built yet. It rests on: the core's topics and the clean market data schema (`core/README.md`); a reader of the
core's topics as the tutorial shows one (`docs/tutorial/`, the basic and medium levels) and the reset as it stands
(`docs/tutorial/advanced/README.md`, 4; `docs/impl/engineering/plugins.md`, 3); `@qi/chart` 2.0.1, `qi-v2-visualization`
at the tag `qi-chart-2.0.1`: its README (the package's `README.md`, `dev/ts/lib/chart/README.md` in its source), its
source, and its online design (`docs/designs/design-chart-online-2026-11-08.md` there); market 1.1.0 (`qi-v2-contracts`,
`market/SPEC.md`).

## What this design guarantees

1. **The chart is the clean topic's.** Every K-line a page draws is a K-line of the series' clean topic, in the topic's
   order, its base fields as the topic holds them, from the first of its window on none left out but a copy, none
   changed; and a page's chart after every update is the library's batch chart of the K-lines it holds (the library's
   own guarantee, its online design's 2).
2. **A reader of the core's topics alone.** It reads one clean topic, in the group of its name: no store, no topic
   written, no offset committed.
3. **No reset crossed.** While it runs, its group holds its series' reset off, as any reader of a clean topic in a group
   does; a topic made again under it all the same (At a reset, the one way) stops it with a fault; started again, it
   reads the topic from its start, and each page draws its chart whole again from it. A K-line of a topic made again is
   never stepped onto a chart of the topic before.
4. **The library as released.** `@qi/chart` is taken by its tag and used as its README says; the drawing is all the
   library's; nothing of it is changed or copied into this repository.
5. **An update's cost bounded.** A page holds at most twice the window's K-lines; at that, it is drawn whole again from
   the last window.
6. **Nothing left running.** Ctrl-C or SIGTERM reaches the program, which leaves its group and ends every page's feed;
   a check's browser is stopped with its run, and found gone.
7. **No login, nothing private.** Every part is a public repository's or nixpkgs', read with no login; a page loads
   nothing but what the program serves.

## The first path

One series, `massive:SPY:M1`, its clean topic `market.clean.validation.v0-1-0.massive.SPY.M1` as the tutorial's catalog
makes it on the test stack (`docs/tutorial/basic/README.md`, 1): the program `chart` reads it, and a browser at its
address draws its last 3,900 K-lines (ten regular sessions) with their volume, then each K-line acquisition adds to the
topic, as it comes. On the real data K-lines come once a session day: acquisition first reads a session's minutes once
its last is 20 h old (the catalog's `delay`; Massive's free plan gives no bar of the current day, `lib/README.md`),
publishes them, and adds them to the clean topic's tail (`plugins.md`, 3), 390 for a regular session, one after another.

## The consumer

**Its program.** `chart`, a Go module of its own folder, `consumers/chart`, in the `Makefile`'s `CORE` (so in the
workspace, `check-core` and `make build`), on the core's `stream`, `stream/kafka` and `clean` and the standard library,
no other module. Its page's sources are in `consumers/chart/page/`, built by the flake (The feed and the page, Its
build). `make build` builds `bin/chart` and links the page's build at `bin/chart-page`.

```
chart -topic TOPIC -page DIR [-brokers localhost:19092] [-listen localhost:8090] [-name chart] [-window 3900]
```

- `-topic`: the clean topic, as `query topics` lists it; its series is the topic's.
- `-page`: the page's folder (`bin/chart-page`); each of its files is checked there at start, a missing one stopping
  it, named.
- `-brokers`: as the core's programs, the stack's by default, the test stack's `localhost:29092`.
- `-listen`: where it serves the page and its feed; on the test stack, `localhost:18090`.
- `-name`: its name and its group (`core/README.md`, a component's identity).
- `-window`: the K-lines a page is drawn whole with (The window).

**How it is run.** By hand, in the foreground: `bin/chart -topic … -page bin/chart-page`. Ctrl-C or SIGTERM reaches it;
it closes its listener, ends each page's feed and closes its source, so that its group has no member, and exits 0 with
a line. Not by `make start`: a component kept running there would hold its series' every reset off while the stack is
up (At a reset).

**What it reads.** At start, the topic's end as it stands (the admin's `Spans`; no such topic stops it, named). Then
the topic from its start, in the group of its name (`stream.StartFromBeginning`, which drops the group's committed
offsets; it commits none), each value as clean market data 0.1.0 (`clean.FromRecord`), and of each:

- its offset is after the last one's, else it stops, naming the offset;
- a copy, a K-line of a date it has taken (a clean topic's key is its series', so its date alone), is dropped, the
  first taken, those of another value counted, each date's text kept as a digest for the whole series
  (`core/README.md`, the clean schema's delivery);
- else it is the next K-line: its index the last one's and 1, its date after the last one's (the clean series numbers
  its K-lines from 0 in the order of their dates); anything else stops it, naming the offset, as a value not of the
  schema;
- a fault of the fetch stops it, named.

It keeps the last window's K-lines and, once it has read to the end as it stood at start, serves; its line gives the
K-lines read, the copies dropped and its address. Then it follows the topic, each new K-line given to every page.

It reads the whole topic at each start (194,100 values for SPY), not from its window's first offset: a copy makes an
offset more than a place, so that offset is known only by reading, and the index and date checked from the first
K-line are what show the topic is the schema's. It reads on the core's stream, as the platform's additions do (the
stores, the tutorial's medium level), not on franz-go alone as the basic level's reader, which stands for a reader
outside this repository.

## The feed and the page

**What serves them.** The program, by the standard library's HTTP server, at `-listen`:

| Path | What |
|---|---|
| `/` | the page, `index.html` |
| `/chart.js` | the page's script bundled with `@qi/chart`, `@qi/market` and what they import |
| `/tv.js` | the library's tv engine, `engine/tv.js` of the package `qi-chart` |
| `/lightweight-charts.js`, `/LICENSE.lightweight-charts` | Lightweight Charts 5.2.1's standalone build, which the tv engine draws with, and its licence: visualization's package `lightweight-charts` |
| `/feed` | the feed: server-sent events |

No other server: nothing of Node or Bun runs, and a page loads nothing from anywhere else.

**The feed**, the least the path needs: one series, its pages on the machine, the batch and then each K-line, from the
program to a page, as server-sent events (`text/event-stream`). Its events:

- `start`: `{"title": "…", "klines": ["…", …]}`, the series' last window of K-lines, in order, each its text: the page
  draws its chart whole from them. It is the first event of every connection, and is sent again when a page's chart
  holds twice the window's K-lines (The window).
- `kline`: one K-line's text, the next: the page takes it by the online component.

Each K-line is market 1.1.0's K-line, its seven fields (`index`, `date`, `open`, `high`, `low`, `close`, `volume`), in
market's canonical text (its fields in the order of their names, no spaces, each number as market writes it): the
K-line the library takes, `@qi/market`'s `klineSchema`, which is strict, a record with a source's own fields (Massive's
`n`, `vw`) not being market's K-line (`market/SPEC.md`). The program writes it from the base of the clean value's
K-line (`raw.KLine.Base`), since market's Go form 1.1.0 encodes an ongoing K-line but no K-line; its test reads the text
back by the core's reader to the same base.

Why server-sent events: the feed is one way; the program needs the standard library alone (a WebSocket would need a
module, `docs/impl/engineering/go.toolchain.md`, 5); the browser's `EventSource` reconnects by itself. A reconnect is a
connection like the first, its first event a `start`, so the program keeps nothing of a page but its connection and the
number of K-lines it has sent it, and a page whose feed falls behind (its buffer full) is closed and reconnects. The feed is the consumer's own, between its
program and its page, which are built together: no part of the platform contract.

**The page** is the consumer's, as the library's README leaves a page to its caller (Using it, The page) and its online
design runs the online component in the caller's page, bundled with the caller's script (The two components):

- `index.html`: an element `#chart` and a line for a fault; Lightweight Charts' script, then `chart.js`.
- `main.ts`, the README's example of the two components used together. At a `start`: each K-line read by `@qi/market`'s
  `decodeAs(klineSchema, text)`; the chart's components, the title (the series and its cleaning), the scale, the
  K-lines as its x-axis, no structure data (each of its six lists empty), no add-ons and no options (the library's
  defaults: the K-lines and the volume shown, the price distribution none chosen, the structure's layers empty);
  `tvChart(c, "all")`, whose `chart` and `layers` are set as `CHART` and `LAYERS` with `PALETTE`; the first `start` then
  loads `tv.js`, which draws them and gives the page `UPDATE`, `REDRAW` and `CHART_UI`, and a later one calls
  `REDRAW()`, then `CHART_UI.latest()`, since `REDRAW` keeps the view by its places, which after a window sent again lie
  past the K-lines; `online(c)`, the online chart. At a `kline`: `decodeAs`, `complete(o, k)`, `UPDATE(tvTail(step))`,
  the step's chart kept. A fault, any of them failing (`decodeAs`'s answer not `ok`, a `Result`'s failure), is shown on
  the page and on the console, the chart left as it was, and nothing more is stepped until the next `start`. The
  K-lines' texts the page has taken are kept, for the checks to read.
- **The palette.** The tv engine reads `PALETTE`, a colour, light and dark, of each palette name its chart uses, and
  stops, naming it, at a name missing (`engine/shared.ts`, `colours`). The package holds none (the chart app's
  `palette.json` is the app's, no product), and its README names none of the names a chart's defaults and the engine
  use (`cl-up`, `cl-down`, `cl-bg`, `cl-ink`, `cl-muted` among them, in its source): the page's palette holds those of a
  chart of K-lines and volume, held to the library by the browser check, where a missing one is a fault.

**Its build.** A package of the flake, `chart-page`, built by Nix from the page's sources and visualization's packages,
with Bun and TypeScript from the flake's nixpkgs (the nixpkgs visualization builds its engines' scripts with): `main.ts`
type-checked as the README says a taker's TypeScript takes the sources (`moduleResolution` `Bundler`,
`allowImportingTsExtensions`, `noEmit`; `strict` and `exactOptionalPropertyTypes`), against the package's
`node_modules`; then bundled by `bun build --target=browser --format=iife --minify` into `chart.js`; beside it
`index.html`, the package's `engine/tv.js`, and Lightweight Charts' standalone build and its licence. Neither Bun nor
TypeScript is in the shell, and no TypeScript runs but in a page. It is the program's page, not a product
(`PRODUCTS.md`, Products; the core's programs published are milestone 2). The page's sources are tracked files, since
the flake builds from git's tree.

## A record of the clean topic as the library's K-line

The library takes market's K-lines and makes each a bar (`src/locate.ts`, `located`): its place its index less the
first's, the indexes one after another; its date a whole second; each price a whole number at the chart's scale, read
exactly from its digits, the scale a power of ten up to 1,000,000; its volume as it is. Of a clean topic's K-line:

- its index is its place in the clean series from 0, one after another (the program checks it), so a window from any
  K-line holds;
- its date is a minute's begin, a whole second;
- the scale is 1,000,000, which holds every price market writes (6 digits after the point at most; SPY's have up to 4),
  up to 9,007,199,254 (2⁵³ − 1 at the scale);
- its volume, Massive's a decimal (`715150.169187`), is taken as it is.

**At start-up**, the batch: a `start`'s K-lines, the components above, `tvChart`. **At each update**, the online
component: a `kline`, `complete`. Every K-line of the path is complete: the clean topic holds K-lines alone, each the
cleaned K-line of a minute's latest version once it is read, and the core's topics carry no ongoing K-line
(`core/README.md`, The K-line: `asOf` is not a K-line's); with no bar of the current day from Massive, nothing on the
path is forming, and `forming` is not used.

**The window.** At each update the library makes the page data of every K-line again (its README, The online component:
about 110 ms for 4,000 K-lines with four add-ons, in headless Chromium), so a page's update costs as much as its chart
holds. The window, 3,900 K-lines by default, ten regular sessions, keeps it near the library's measure. A page's chart
grows by each K-line; when it holds twice the window, the program sends it a `start` again, of the last window, and the
page is drawn whole again (`REDRAW`, the view then on the newest K-lines): no page holds more than 7,800. A session's 390 K-lines come in one run, and the
page takes them one after another.

## At a reset

A reset deletes the series' clean topic with every group's offsets on it and makes it again under its name; no field
marks it made again (`core/README.md`, The clean market data schema). Acquisition makes no reset while a group of the
topic has a member: its run fails, naming the group, and the next tries again (`publishers/acquisition/clean.go`,
`reset`; `docs/tutorial/advanced/README.md`, 4); under `-follow` the next run is at its time, a session day later.

The consumer reads in the group of its name, so while it runs it holds its series' reset off, as every reader of a
clean topic in a group does, and its pages see no reset. To let a reset through, it is stopped (Ctrl-C); the next run of
acquisition makes the reset; started again, it reads the topic made again from its start, and each page's
`EventSource`, cut off when it stopped, reconnects and is given a `start`: its chart drawn whole again, of the topic
made again.

Why not in no group, which would hold no reset off: nothing the core gives tells a reader that runs that its topic was
made again. Read in no group, it goes on at its offset; on a topic made again no shorter than the one before, the K-line
at that offset continues the index and the date, and would be stepped onto a chart whose earlier K-lines the reset has
changed, with no fault (guarantee 1 lost). Telling it needs a mark of a reset that the core has not, a change of the
core ahead of the path (What is left).

**The one way a topic is made again under it.** Acquisition checks the groups of a clean topic that is there: a reset
that fails once its topic is made deletes the topic with no check of its groups, and the next run, finding no topic,
makes it as the series' first reset, checking only the groups of the topics derived from it (`clean.go`, `reset`). A
chart started between the two (its topic there, made by the reset that then failed) has its topic deleted and made
again under it. Its client stops it then: franz-go 1.22.0 never takes up a topic made again under a new topic ID, and
its consumer of the old one fails with `UNKNOWN_TOPIC_ID` (`pkg/kgo/source.go`, `cursor`; `metadata.go`), which the
core's `Fetch` returns, a fetch's fault stopping the chart. That rests on the broker giving each topic its ID: the
module's test on the test stack makes its topic again under a reader that runs and holds that the reader stops, and
if Redpanda does not, the build stops there and the design comes back. The gap is acquisition's, for any reader of a
clean topic (What is left).

What this gives up: while the chart runs, its series' damage past its tolerance is not mended, neither its clean topic
nor the stores' clean series, and once it is stopped the reset waits for acquisition's next run, a session day later
under `make start`, or a run by hand. A chart that runs through a reset is milestone 4's step on a clean topic's readers
stopped and started again by their process manager.

## What the platform takes

**`@qi/chart` 2.0.1**, `qi-v2-visualization` at the tag `qi-chart-2.0.1` (its commit `351531cf…`), a flake input read
over HTTPS with no login. Of it the consumer uses:

- of the package `qi-chart`: `@qi/chart`'s batch (`tvChart`) and online component (`online`, `complete`, `tvTail`), its
  tv engine's script (`engine/tv.js`) and its handles (`UPDATE`, `REDRAW`); `@qi/market`'s `decodeAs` and
  `klineSchema`; and in the bundle what they import: `@qi/base`, `@qi/core`, `zod`, `@qi/indicator` (which the
  library's price distribution and its schemas import), and the schemas of the structure contract's TypeScript form, a
  module of the package, which the library's x-axis takes beside market's K-line;
- the package `lightweight-charts`, 5.2.1, its standalone build (Apache-2.0, its licence served with it).

Not used: `forming`, the d3fc engine and the package `d3fc`, `tikzChart`, marks, add-ons, the price distribution, the
structure input, the chart app.

**What it brings into `flake.lock`.** Of the input's own inputs, `qi-v2` and `kit` follow this flake's, the same commit
(`d49a2a87…`), and with them `nixpkgs` (`fa137313…`); `market` follows this flake's `contracts`, the same commit
(`3354d2b1…`, `market-1.1.0`). New in the lock, three nodes, by their inputs' names: `visualization`,
`qi-v2-visualization` (`351531cf…`); a second node of `contracts`, `qi-v2-contracts` at the structure contract's tag of
1.1.0 (`c4842562…`), from which the package takes that contract's TypeScript form; `qi-v2-ts`, at `qi-v2-ts-0.1.0`
(`3c55f72e…`), `@qi/base` and `@qi/core`. So the structure contract at 1.1.0 is in the lock, its tag's name among the
lock's text, and its schemas in the page's bundle, though no structure is drawn, where that contract's release is 3.1.0
(which the tree of this flake's `contracts` pin holds, unused); whether it is kept is open (`docs/decisions.md`, Open). Chromium, for the
browser check, and Bun and TypeScript, for the page's build, are nixpkgs' at this flake's pin: nothing more in the lock.

**What changes:**

| File | Change |
|---|---|
| `flake.nix` | the input `visualization`, `github:zhifengzhang-sz/qi-v2-visualization?ref=qi-chart-2.0.1`, its `qi-v2` and `kit` following this flake's and its `market` this flake's `contracts`; the output `packages.x86_64-linux.chart-page` (Its build); the shell unchanged |
| `flake.lock` | the three nodes above; nothing else changed |
| `PRODUCTS.md` | Takes: `visualization`, from `qi-v2-visualization`, its packages `qi-chart` (`@qi/chart` 2.0.1) and `lightweight-charts`, pinned by `flake.lock`; Products unchanged |
| `Makefile` | `consumers/chart` in `CORE`; `build` links the page's build at `bin/chart-page` (`nix build .#chart-page`); `check-chart`, in `check` (How it is checked) |
| `dev/scripts/verify.py` | `check-chart` for a change of `consumers/chart/`, `flake.nix` or `flake.lock` |
| the shell | unchanged: no Bun, TypeScript or Chromium in it |

With the build, the documents that state what stands are brought to it: `README.md` (the contract's programs, the
operations on the stream, the dependencies), `dev.environment.md` (the tools, the chart's address), `go.toolchain.md`
(4, the browser check's variables), `plugins.md` (3, a clean topic's readers); and `agents/rules.md`'s lines on the
dependencies and on a browser page, once restated (`docs/decisions.md`, Open).

## How it is checked

**What `make check` gains.**

- `consumers/chart` in `check-core`: gofmt, vet and its tests. On the in-memory stream (`stream/mem`): a K-line's text,
  the base's alone, read back by the core's reader; the feed, by an HTTP test server and a reader of its events: a
  `start` of the last window, each K-line after, a copy dropped, a break of the offset, the index or the date stopping
  it, a `start` again at twice the window, a reconnect given a `start`. On the test stack (`QI_KAFKA_BROKERS`, kept by
  `QI_STACK_STATE`, `go.toolchain.md`, 4): the same from a clean topic of its own, deleted after; its group listed with
  a member while it serves (`stream.Admin`'s `Groups`), which is what holds a reset off; and its topic deleted and made
  again under it as it runs, which stops it with a fault (At a reset, the one way).
- `check-chart`, a target of its own in `check`: the page in headless Chromium. Chromium is the flake's nixpkgs' (`nix
  build --inputs-from . nixpkgs#chromium`, as visualization's checks take it) and the page the flake's `chart-page`,
  each given by a variable (`QI_CHROMIUM`, `QI_CHART_PAGE`) to a test of the module, which skips, saying so, without
  them. The test drives Chromium by its DevTools protocol over `--remote-debugging-pipe`, with the standard library alone
  (a process and two pipes), and serves the feed from the in-memory stream with K-lines of the contract at Massive's
  precision: the page loads with no exception and no fault; its K-lines' texts are the feed's, the batch's and each
  step's; a `start` again, at twice the window and as after a reset, draws it whole again, the K-lines in view
  (`CHART_UI.here()`) its newest; a screenshot is kept in the run's folder. Chromium
  is stopped with the test and found gone.

**Before the path is called done**, on the test stack, on the real data, by the session, every run on the test stack
and its files in a folder of its own (`mktemp -d`), as the tutorial's:

1. The test stack filled by `make start STACK=test` with the tutorial's catalog but its `delay` some sessions longer
   than 20 h, so that its record ends some sessions short; then acquisition stopped by its pid, as the tutorial's
   advanced level does, the writers left running.
2. `bin/chart` on the clean topic (`-brokers localhost:29092 -listen localhost:18090`), and the browser check's driver on
   its page: drawn whole, its K-lines the topic's last window, as the topic read in no group gives them
   (`clean-reader`'s way).
3. Acquisition run once by hand with the catalog's `delay` of 20 h: the sessions held back published and added to the
   clean topic's tail; the page takes each K-line as it comes, and at the end holds the topic's last K-lines; each
   update's cost and a screenshot reported.
4. The reset: `correct-high` on 11 minutes older than the 30 days re-read, past the validation's tolerance (10);
   acquisition's run fails, naming the group `chart`; the chart stopped, nothing of it left running; the next run makes
   the reset; the chart started again with a window that holds the corrected minutes; the page, reconnected, drawn whole,
   its K-lines the topic made again's, the corrected `high`s among them.
5. `make stack-test-down`; nothing of the chart or of Chromium left running.

Every input is public, read with no login; the Massive key is read by the library's fetcher alone, as on every run.

## What the library lacks for the path

Nothing the path needs is missing from the package. Two things it carries or leaves unsaid are for visualization:

- **The palette.** The package holds no palette, and its README names no palette name that a chart's defaults or its
  engines use: the page's palette is written from the library's source and held to it by the browser check.
- **The structure contract at 1.1.0.** A chart of market's K-lines alone takes the structure contract at 1.1.0 into the
  lock and the bundle, the x-axis's schema taking that contract's K-line too, where its release is 3.1.0: a release
  whose market path takes none of it, or takes its current release, would keep the older version out.

Not needed by the path: an update in O(1), which the library's online design leaves to a later version (What an update
costs), would lift the window's bound.

## What is left

Each a step of its own:

- **More series**: a chart of each, or a page choosing among the clean topics; a chart of `market.raw` or
  `market.corrections`.
- **A forming K-line**, once a source gives one: a source plugin's live stream (`plugins.md`, 5) and a topic of ongoing
  K-lines, which the core has not.
- **A reset with the chart running**: a clean topic's readers stopped and started again by their process manager
  (milestone 4), or a mark of a reset that a reader running can read; then the chart under `make start`.
- **Acquisition's first reset after one that failed** checking the groups of the topic it makes, as a reset of a topic
  that is there does (At a reset, the one way), for every reader of a clean topic.
- **The rest of milestone 4's live feed**, a gateway on the core: any of the core's topics, its protocol in the platform
  contract, a client choosing its topic and series, from a time or an offset, corrections and resets carried, clients
  beyond the machine's own pages, many at once; the charting consumer then a client of it.
- **The rest of a chart**: add-ons (indicators), marks, the price distribution, the d3fc engine, the page's controls (a toolbar,
  the legend, the settings), times in the series' zone and its prices' digits in the legend (the library's defaults:
  UTC, and 2); the structure, once computed and published.
- **The page as a product**, with the core's programs (milestone 2); `check-chart` in CI (`docs/decisions.md`, Open,
  CI).
