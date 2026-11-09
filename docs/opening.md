Opening for the next session, written 2026-11-09 by the session before (`docs/process.md`). It says where the work
stands, the next tasks, and what to read.

A commit this opening names from before 2026-11-08's new start is of the platform's earlier history, kept by the owner and not in this repository (`docs/decisions.md`, 2026-11-08, the history begun anew); each hash is left as it was.

## Where the work stands

- The platform (`README.md`) is the core and its additions alone (`core/README.md`; the platform's library,
  `lib/README.md`; acquisition, the printer, `query`, the stores, Massive's reader, the validation, the charting
  consumer, the data sets' maker; the tutorial, `docs/tutorial/`), every program Go, the charting consumer's page drawn
  by `@qi/chart`; its products the contract, the library and the K-line data sets (`PRODUCTS.md`).
- **This round** (the commit tagged `qi-v2-platform-klines-massive-spy-1m-1.0.0`; `docs/decisions.md`, 2026-11-09 on
  the K-line data set, and Open): the K-line data sets designed and built
  (`docs/designs/design-klines-dataset-2026-11-09.md`, `datasets/README.md`), and the first released,
  `qi-v2-platform-klines-massive-spy-1m` 1.0.0: SPY's clean topic, `market.clean.validation.v0-1-0.massive.SPY.M1`,
  whole as the stack holds it, the sessions 2024-11-06 to 2026-11-06 in New York, 502, every one whole (497 of 390
  minutes, 5 early closes of 210, as the catalog's calendar has them), 194,880 K-lines, 25 files a month each of market
  1.1.0's K-lines, 21,837,727 bytes, the image's layers 3.9 MB, its digest `sha256:5741bcb8…` in its lock,
  `datasets/qi-v2-platform-klines-massive-spy-1m.lock`. The program `dataset` (`consumers/dataset`, in `CORE`), the core's
  `raw.FromBase`; `make dataset`, `dataset-check`, `dataset-push` (the owner's), `dataset-remote-check`, and
  `check-datasets` in `make check` (`dev/scripts/dataset.py`); crane 0.21.6 the flake's nixpkgs', not in the shell. The
  checks: the data set made twice from the stack to the same bytes, every file and the layout; read back from its
  image through a registry of crane's own on 127.0.0.1, its digest the lock's and every file the lock's, the registry
  stopped after; `make check`, role `checker`, passing, and `make products-check`. One review, role `reviewer`, four
  findings settled before the commit, not rechecked: the range's reason added to the proposal; the push's login and the
  check's registry removed at SIGTERM or SIGHUP too; `README.md`'s contract row saying "once the owner has set it
  public"; `dataset-remote-check` held to the tag's lock. The owner's: the brief (the data set ahead of the app's build);
  the session's first build, asked and allowed. The session's: the design's choices, a proposal; Massive's terms on
  redistribution found, Open. Every Nix command offline; the stack read only, its clean topic in no group; the test
  stack not used; no credential used, no registry logged in to; nothing on GHCR yet.
- **The round before** (`ab928cd`, `55c1e18`; `docs/decisions.md`, 2026-11-09 on the charting consumer's build): the
  charting consumer built as designed (`consumers/chart`; `docs/designs/design-chart-consumer-2026-11-09.md`), on
  `@qi/chart` 3.0.0, `qi-v2-visualization` at `qi-chart-3.0.0`, its page the flake's package `chart-page`, checked in
  headless Chromium by `make check-chart`, in `make check`. 3.0.0 held to each part of 2.0.1 the design uses and to the
  two things asked of it; Redpanda gives topic IDs, a chart whose topic is made again under it stopping with
  `UNKNOWN_TOPIC_ID` about 47 s after (`TestStackRemade`). The gate on the test stack, on the real data, in a folder of
  its own: SPY's clean topic of 192,540 K-lines, the page's 3,900 its last; a run by hand's 1,170 stepped one by one,
  9.1 to 26.7 ms each (median 12.3); the reset's run failing, naming the group `chart`; the chart stopped, the reset
  made, the chart started again with a window of 8,580, the page drawn whole again, the 11 corrected `high`s among its
  K-lines; its outputs in `.reports/chart-gate/` (ignored). The owner's: the brief, the session's design proposal taken
  with `qi-chart-3.0.0` in place of the next release; TypeScript 5.9.3 fetched from a binary cache. The session's: the
  build's choices, a proposal (the page's palette every name of the README's table, its TypeScript at ES2023 among
  them); `agents/rules.md` restated as decided, `make agents`. One review, role `reviewer`, two findings and a point,
  settled before the commit, not rechecked; `make check` twice, role `checker`, each passing. Every Nix command offline; the test
  stack used and wiped; the stack not touched.
- **The rounds before** (`040e610`, `259d04a`, `ae30fed`; `3546680`, `7e4d7ba`; `docs/decisions.md`, 2026-11-08 and 2026-11-09):
  the platform as an app designed, no code (`docs/designs/design-platform-image-2026-11-09.md`, a researcher's notes
  beside it): one Compose app on GHCR, `ghcr.io/zhifengzhang-sz/qi-v2-platform`, running the first path's five programs
  from one image with the Redpanda and TimescaleDB they need, the programs as the flake package `platform`, the release
  0.1.0, its push the owner's; and the charting consumer designed, the design the one the round after built.
- **The round before those** (`71a6391`, `0d22bc0`): the platform's history begun anew, this repository public from its start,
  its first commit the tree of the earlier history's last (`b1c325b`), as it is; the decisions of 2026-11-08 on it, the
  restated rules, `make modules` and the branch `main`.
- The stack at `stack-0.3.0`, up, written by nothing since step 8.8 (`market.raw` 389,370 messages (p7),
  `market.corrections` 417, `market.clean.validation.v0-1-0.massive.SPY.M1` 194,880, as this round read them; the old
  platform's topics as they were); no component of the platform's running there; this round read its clean topic in
  no group. The test stack down, not used by this round.
- The locks: `make locks-check` not run (it calls `gh api`); `flake.lock` unchanged by this round. The data set's
  folder, `~/.local/share/qi/datasets/qi-v2-platform-klines-massive-spy-1m/1.0.0/` (its files, its image's layout, its
  lock as made), which `make dataset-push` pushes.
- `main`: pushed to `origin`, with the tag; `.env` in place, ignored; `make install` run; the owner's local file:
  `todo.md`.

## The next tasks

First the owner's: the data set's image pushed to GHCR (`make dataset-push`, at the tag, the token asked for and its
login removed after), and set public on its package's page once the owner has judged Massive's terms for it
(`docs/decisions.md`, Open); then, by a brief, `make dataset-remote-check`, with no login, and its result recorded, in
work of its own. Then, by a brief to come: the app's build.

**The app's build**, by a brief to come from its design (`docs/designs/design-platform-image-2026-11-09.md`). The
brief names what it allows: the Massive key's use by `make app-test`, as `make start STACK=test` uses it; the first
build of its session asked of the owner. Its first steps hold what the design rests on but could not see run, one not
holding bringing the design back: `buildGoModule` building the workspace, and the package's closure the package alone
(`nix path-info -r`); each layout built again to the same bytes (`nix build --rebuild`); Compose running the app's
hand-built form from a registry (`-f oci://…@sha256:…`, `--insecure-registry` for the check's); the pages' volume made
from the image's `/work`, owned by the user 1000; Docker Hub's `golang:1.26.10`; BuildKit's `ADD` of a Git URL at a
commit. With the build, `agents/rules.md` on the tools and the dependencies restated as the owner decides
(`docs/decisions.md`, Open). Then the release 0.1.0, by a brief of its own: its commit and tags pushed by the session,
its push to GHCR the owner's, whether GHCR serves the app held first by that push and the check after it.

Then milestone 4's other Left (`docs/milestones.md`): the rest of the live feed, a gateway on the core; the other
sources as plugins and a source plugin's live stream; the runs in the session (Massive's free plan, `delay`); a clean
topic's readers stopped and started again at a reset, a reader that stops and starts itself among them, and a reset cut
short made whole; CI, none today, in the flake's
shell; the library taken outside this repository; what the charting consumer leaves (its design, What is left); what
the data sets' design leaves (its What is left). The sessions' findings, each a step of its own. Of the K-line data
set's build: whether a range's ends are whole sessions is held by the release against the catalog's calendar, not by
the program, which knows no calendar; the read back is by crane alone, no Docker pull of the image tried; the image's
digest depends on the flake's Go (its gzip), so a newer Go gives the same files under another digest, and a released
version is pushed from its built folder, never made again on another Go; a GHCR limit on anonymous pulls is not known
(the app's notes, G2). Of the charting consumer's build: `@qi/chart`'s README names no TypeScript
level, and its sources need ES2023 (`toSorted`), a line the owner may take to visualization; the page's first drawing
opens on every K-line of its window (the engine's `all`, the address naming none), 3,900 at about a third of a pixel
each, and a `start` again on the newest 400 (`latest`); the Makefile's `nix` commands run with no `--offline`, so an
offline session runs them through a wrapper first on the path inside the shell, never on the path the shell is entered
from, whose first `nix` the shell's hook links as its host tool (`~/.cache/platform/host-tools/nix`; once linked to the
wrapper, a loop, set right by entering the shell again from the host's path); `TestLive` counts Chromium's console
lines as faults, a feed refused while the chart is stopped among them. Of the charting consumer's design: acquisition's
first reset after one that failed checks no group of the topic it makes, so a reader started between the two has its
topic made again under it. Of the app's design: the catalog's dataset, its
session's calendar among it, copied once more inline in `app/compose.yaml`, as in the tutorial's two catalogs, no
check holding the copies to `config/catalog.yaml`; the workspace now holds `consumers/chart`, a module the app's
image does not run, and the flake the input `visualization`, which the package `platform` does not take. Of `go/`'s removal: `plugins.md`'s sections are 1, 3 and 5, their numbers kept since many files cite §3;
`dev.environment.md`'s Known gaps holds no gap, and its test stack's line still names acquisition's `-state`, which
it no longer has; acquisition's test catalog keeps a Binance dataset and `store` (test data, unchanged). Of step
8.9: the tutorial's programs have no tests of their own, the gate's run their test; the pages' outputs hold the
run's machine paths and times; a reset's reader stopped and started by hand; the gate's runner the session's,
outside the repository. Of step 8.8: a reader of a clean topic that runs stops a reset rather than being stopped and
started again; acquisition stopped in the middle of a reset deletes nothing; a store's clean series lags its topic
by the tail between resets; `make start`'s commands parted at spaces. Of step 8.7: `query`'s Kafka reads its own,
not the core's stream. Of step 8.6, standing: a topic name's series ambiguous when a name holds a dot, refused a
clean topic, allowed by the key; the grid of a timeframe other than M1 from the session's open; a publish failing
part way leaving corrections older than `reread` lost; a line of `massive` not Massive's K-line stopping both
writers for good; `make help` printing `Makefile` for each target; Massive's first row of a fetch by a rule not
known; Massive's pages kept without end; a real deployment's password in a writer's `-url`; ClickHouse's window in a
rebalance. The rounds before: `nixpkgs` through `qi-v2-nix`.

Read, for the app: its design and the notes beside it; `docs/decisions.md`, 2026-11-09 and Open; `agents/rules.md`;
`flake.nix`, `flake.lock`, `Makefile`, `dev/scripts/run.py`, `dev/scripts/verify.py`, `PRODUCTS.md`; qi-v2's
`qi-v2/flake.nix` (`releaseImage`) and the package `stack`'s Compose files, at the store paths `flake.lock` pins;
`docs/tutorial/basic/README.md` and `clean-reader/`; nixpkgs' `pkgs/build-support/go/module.nix`.

## Standing rules learnt (until the owner approves them for `agents/rules.md`)

- **No `--remove-orphans` on the stack, and `-v` only on the test stack** (`make stack-test-down`), from the owner: the
  compose project `services` holds containers and volumes that are not the stack's (on this machine `fff-postgres`,
  `qicore-qdrant`, `qicore-chromadb` and their volumes), which `--remove-orphans` removes, and `-v` removes the stack's
  volumes, which hold the data.
- **Every test and trial run on the test stack**, from the owner: `market.raw` on the stack keeps every message forever,
  so nothing a test writes there can be taken out again; the test stack is wiped whenever a test is done; a run there
  keeps its state and its pages in a temporary folder of its own.
- **A wait for a source is one wait in the background**, from the owner: when a source gives nothing until a time
  (Massive's free plan, a day's bars after midnight New York), the run waits for that time by one wait, never by runs
  that fail.
- **No credential of the owner's without asking first** (gh's token, a registry login, a token to `nix` through
  `NIX_CONFIG` or the environment, an SSH key for anything but the push a brief allows, a `git fetch` or `git ls-remote`
  of `origin` among them, the GitHub tools of an MCP server), whatever the reason, a rate limit included; GitHub's
  public repositories are read over HTTPS; `make locks-check` calls `gh api`, so it is asked before it runs. The Massive
  key is read only by the library's fetcher, never printed or logged; a refused fetch stops the run.
- **Nothing pushed to a registry but by the owner**, from the owner: a push to GHCR, or any registry, is the owner's,
  with a login of its own, asked each time; a session builds and checks, and logs in to no registry.

## Open, for the owner

In `docs/decisions.md`, Open: Massive's market data terms and a public data set (its knowledge base: redistributing its
market data needs one of its business products; the image is not to be set public until the owner has judged it); CI
in the flake's shell; `carrier.md`'s form; the rules on the dependencies and the tools, for the Dockerfile and the app.
Decided, proposals until the owner decides: the session's entry of 2026-11-09 on how the K-line data sets are designed
and built (the clean series, the range, a file a month of market's K-lines, the versions, the names, the image as a
layout, the lock's place, the program and the targets); the session's entries of 2026-11-06 (no check of the flake; the state's status and health and the stops of
`make check`; `TestMigrations`' race fixed in the test, the test gone with `go/`; how steps 8.1 to 8.4 are built
where their briefs leave the choice), of 2026-11-07 (how step 8.5 is built) and of 2026-11-08 (how steps 8.6 to 8.9
are built; of 8.8, the cleaning named by the catalog and naming itself, the protocol's `--after`, the validation
reading the base fields as text, the tail from every minute after the clean topic's last, the reset's source through
the TimescaleDB writer's `-corrected` checked against the record, the stores' clean series by their writers'
`-clean`, the readers as groups stopping a reset, the core's stream's additions, `klines_clean` without `read`, a
failed reset deleting its topic; of 8.9, the tutorial's programs a list of their own by their `go.mod`,
`check-tutorial`, not built by `make build`; the catalogs as templates and the user's folder by `mktemp -d`;
`clean-reader`'s whole-value check; the follower's file as what it has done; `despike`'s 2% and the tolerance 3;
`correct-high`; acquisition run by hand on the advanced level; how `go/`'s removal is built, but `make modules`,
accepted: CI removed whole, the documents removed and restated, the catalog's and the flake's changes; and, the
owner's to judge, of 8.8, a reader running not stopped, a crash mid-reset), and of 2026-11-09 (how the charting
consumer is built where its brief and its design leave the choice; how the platform as an app is designed). The
owner took the session's proposal of how the charting consumer is designed (2026-11-09).
