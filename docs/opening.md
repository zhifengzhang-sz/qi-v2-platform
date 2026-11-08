Opening for the next session, written 2026-11-09 by the session before (`docs/process.md`). It says where the work
stands, the next tasks, and what to read.

A commit this opening names from before 2026-11-08's new start is of the platform's earlier history, kept by the owner and not in this repository (`docs/decisions.md`, 2026-11-08, the history begun anew); each hash is left as it was.

## Where the work stands

- The platform (`README.md`) is the core and its additions alone (`core/README.md`; the platform's library,
  `lib/README.md`; acquisition, the printer, `query`, the stores, Massive's reader, the validation; the tutorial,
  `docs/tutorial/`), every program Go.
- **This round** (two commits after `7e4d7ba`, `040e610` and the one after; `docs/decisions.md`, 2026-11-09 on the
  platform as images, and Open): the platform as an app designed, no code,
  `docs/designs/design-platform-image-2026-11-09.md`, with a researcher's sourced notes beside it
  (`notes-compose-publish-2026-11-09.md`): one Compose app on GHCR, `ghcr.io/zhifengzhang-sz/qi-v2-platform`, running
  the first path's five programs from one image, `ghcr.io/zhifengzhang-sz/qi-v2-platform-programs`, with the Redpanda
  and TimescaleDB they need, its own network and volumes, no host port; a user needs Docker and a Massive key and
  reads the app's document alone, nothing of the dev side; a component of the user's a container on its network, built
  by a Dockerfile, the tutorial's reader showing it; the programs as the flake package `platform` for another qi-v2
  project; the release 0.1.0, a tag for each product, the digests computed before the push from OCI layouts built in
  Nix, the push the owner's by oras (`make release-push`), `make release-check` after it with no login. The owner's:
  the brief; in the session, one image, not one for each program; a Dockerfile for external use and the flake for use
  between projects (the session's proposal declined); Nix no prerequisite of the image's use; GHCR, not Docker Hub; the
  app and the dev side apart, the app holding the stack's two services it needs (superseding the stack taken from
  `qi-v2-services`' Compose files, the first commit's). The session's: how it is designed, a proposal; Open's entry on
  the rules for the Dockerfile and the app's images. Two reviews, role `reviewer`: the first commit's (twelve
  findings and two smaller), the second for what the first design gives that the revised one does not (thirteen and
  three smaller), each settled before its commit, not rechecked; one researcher's run. No code, no lock, no flake
  change; the stack and the test stack not touched (Docker read only: its containers, networks, version, plugins);
  nothing pushed to a registry.
- **The round before** (`3546680`, `7e4d7ba`; `docs/decisions.md`, 2026-11-08 and 2026-11-09 on the charting
  consumer): the charting consumer designed, no code,
  `docs/designs/design-chart-consumer-2026-11-09.md`: one series' chart in a browser, drawn by `@qi/chart` (made from
  2.0.1, `qi-v2-visualization` at the tag `qi-chart-2.0.1`, read over HTTPS with no login) and updated as the series'
  K-lines come, of the K-lines and their volume alone. The owner's: the decision of 2026-11-08 on `@qi/chart`,
  recorded; the design's brief; the page the consumer's, asked in the session; then its answers on the design: the
  platform takes visualization's next release, not 2.0.1, nothing of the structure contract older than 3.1.0 in it,
  the build waiting for it; the palette asked of visualization in that release, no other; a running chart holding its
  series' reset off, a reader that stops and starts itself at a reset a step of its own; the rules restated at the
  build as proposed. The session's: how it is designed, a proposal, its reset part decided. Its review, role
  `reviewer`, once, its findings settled before the first commit; the second, the owner's answers recorded and the
  design, `docs/milestones.md` (4, Left) and this opening brought to them, not reviewed. No code, no lock, no flake
  change; the stack and the test stack not touched.
- **The round before those** (`71a6391`, `0d22bc0`): the platform's history begun anew, this repository public from its start,
  its first commit the tree of the earlier history's last (`b1c325b`), as it is; the decisions of 2026-11-08 on it, the
  restated rules, `make modules` and the branch `main`.
- The stack at `stack-0.3.0`, up, not touched, as step 8.8 left it (then: `market.raw` 389,370 messages (p7),
  `market.corrections` 417, `market.clean.validation.v0-1-0.massive.SPY.M1` 194,880; the old platform's topics as
  they were); no component of the platform's running there. The test stack down, not used these two rounds.
- The locks: `make locks-check` not run (it calls `gh api`); `flake.lock` unchanged.
- `main`: pushed to `origin`; `.env` in place, ignored; `make install` run; the owner's local file: `todo.md`.

## The next tasks

Two, each by a brief to come; which goes first is the owner's to say, and the chart's build waits on visualization.

**The charting consumer's build**, by a brief to come from its design (`docs/designs/design-chart-consumer-2026-11-09.md`),
once visualization's next release is out: the build waits for it. That release, asked of visualization, holds nothing
of the structure contract older than 3.1.0 (2.0.1 takes it at 1.1.0 into the lock and the page) and gives the palette
(2.0.1 holds none and its README names none of the names a chart uses). The brief names its tag and what it brings
into `flake.lock`, from the release's own; the build's first step holds to it each part of 2.0.1 the design uses (the
design, What the platform takes), a part not kept bringing the design back; then the module's test on the test stack
holding that a reader stops when its topic is made again under it (the design, At a reset, the one way), and if
Redpanda gives no topic IDs, the build stops and the design comes back. With the build, `agents/rules.md`'s lines on
the dependencies and on a browser page restated as decided (`docs/decisions.md`, 2026-11-09).

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
shell; the library taken outside this repository. The sessions' findings, each a step of its own. Of the charting
consumer's design: acquisition's first reset after one that failed checks no group of the topic it makes, so a reader
started between the two has its topic made again under it. Of the app's design: the catalog's dataset, its
session's calendar among it, copied once more inline in `app/compose.yaml`, as in the tutorial's two catalogs, no
check holding the copies to `config/catalog.yaml`. Of `go/`'s removal: `plugins.md`'s sections are 1, 3 and 5, their numbers kept since many files cite §3;
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

Read, for the chart: the design; `core/README.md`; `docs/tutorial/medium/README.md` and `docs/tutorial/advanced/README.md` (4);
`publishers/acquisition/clean.go` (`reset`); `core/stream/stream.go`; `docs/decisions.md`, 2026-11-08 and 2026-11-09,
and Open; `agents/rules.md`; `@qi/chart`'s README, the package's `README.md` (`dev/ts/lib/chart/README.md` at the next
release's tag, over HTTPS), and its `flake.lock`.

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

In `docs/decisions.md`, Open: CI in the flake's shell; `carrier.md`'s form; the rules on the dependencies and the
tools, for the Dockerfile and the app. Decided, proposals until the owner decides: the session's entries of 2026-11-06 (no check of the flake; the state's status and health and the stops of
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
consumer is designed, but its reset held off while it runs, decided; how the platform as an app is designed).
