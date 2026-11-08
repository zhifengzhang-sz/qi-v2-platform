Opening for the next session, written 2026-11-08 by the session before (`docs/process.md`). It says where the work
stands, the one next task, and what to read.

## Where the work stands

- The platform (`README.md`) is the core and its additions alone (`core/README.md`; the platform's library,
  `lib/README.md`; acquisition, the printer, `query`, the stores, Massive's reader, the validation; the tutorial,
  `docs/tutorial/`), every program Go. **This round** (`fd6f547`, then this closing commit; `docs/decisions.md`,
  2026-11-08, the owner's four entries on the project public, `go/` removed, the engine worker and the Ctrl-C rule, and
  the session's on how it is built): the project made ready to be public. The old platform, `go/`, removed ahead of the path's end,
  with everything only it used or that existed for it: `deps.lock` and the executable it named, CI's one job and its
  token (no CI is left), `examples/plugins/bar-returns`, `scripts/`, `test/conformance/`, `config/development.yaml`
  and `production.yaml`, `dev/scripts/locks-check.py`, `docs/owner-setup.md` (the owner's notes on private projects;
  in git's history), `docs/designs/flow.md`, the old platform's engineering documents; the Makefile without
  `check-go`, `generate`, `generate-check`, `e2e`, `backup`, and with `make modules` (the modules the workspace's
  packages and tests need, from the public Go module proxy, run first by every target that runs Go, the checks and
  builds keeping `GOPROXY=off`); the flake's shell without sqlc, PostgreSQL's clients and the host's engine executable,
  its inputs unchanged; `config/catalog.yaml` without the Binance datasets and `store`. No private project named in
  any tracked file: the documents restated, the past decisions keeping what was decided and why, the core's comments
  that cited `go/` stating the rule where it stands, two tests' derived-topic names generic (`engine.…`). The engine
  worker, and what it makes, are not the platform's (`core/README.md`, The topics' names). `agents/rules.md`: the
  repository public; the Ctrl-C rule, approved; the rules that named `go/` or a private project restated, the owner's
  to approve (`docs/decisions.md`, Open). `PRODUCTS.md`: the engine executable's row out; the contract the core's
  topics, the programs and their flags, the plugin protocols, a user reads the topics, not the stores.
- **The gate.** A case-ignored `git grep` of every tracked file for each private project's name, each short name
  standing for one, and each of their paths and products the tree named before (28 patterns; they are in
  `.reports/public.patterns.txt` while it lasts and in the round's report, never in a tracked file, where they would
  name the projects): empty but for one pattern's line in `agents/roles/reviewer.md`, the kit's class of AI model; the
  generated agent files the same; `deps.lock` and `owner-setup` only in `docs/decisions.md` and this opening, as this
  repository's own removed files. `consumers/query/README.md`'s output of `query topics` on the stack leaves out the
  old platform's two empty topics whose names abbreviate the private engine's API. `flake.lock`'s five inputs each downloaded over HTTPS with no credential (200). No
  credential in any tracked file: only the stacks' published local `password`. The checks, role `checker`, after the
  last change of `fd6f547`, from a fresh shell of the changed flake: `make modules` from an empty module cache (216 MB
  from the proxy), `make stack-test-up`, `make check` on the test stack PASS (95 top-level tests passed, 0 skipped, 0
  failed, 13 packages; `.reports/public.check2.log` while it lasts), `make products-check` valid, `make
  stack-test-down`. The review, role `reviewer`: two findings confirmed (the roles editor and writer naming `go/`'s
  targets; a search result stated without its exclusion), three plausible (two past decisions holding the private
  engine's version and design; `make modules` untested on an empty cache; the removed CI file unverifiable), all
  answered (`fd6f547`'s message). The closing commit's review: in its message.
- **The history, for the owner's choice before the project is made public** (not rewritten): of the 125 commits on
  `master` before this round, 36 name a private project in their messages (by its name or its product's), 31
  by a short name standing for one;
  every one holds a file that names one, from the first (`c9f3f3c`, 2026-04-07); so does `fd6f547` (this file, as it
  stood), not this closing commit. No blob of any ref holds a credential by its pattern (a private key, a GitHub, AWS,
  Slack or Google token, `apiKey=`, a bearer token, a secret's path): only the stacks' published local `password`;
  the Massive key's file was not read, so it is checked by pattern alone. Besides `master`: the tag
  `pre-reorg-2026-09-23` and the stash "step 8a, stopped 2026-11-06", both local.
- The stack at `stack-0.3.0`, up, not touched, as step 8.8 left it (then: `market.raw` 389,370 messages (p7),
  `market.corrections` 417, `market.clean.validation.v0-1-0.massive.SPY.M1` 194,880; the old platform's topics as
  they were); no component of the platform's running there. The test stack wiped.
- The locks: `make locks-check` not run (it calls `gh api`); `flake.lock` unchanged.
- `master` committed and pushed (`fd6f547`; this closing commit after it); the owner's local file: `todo.md`.

## The next task: the owner's choice

Making the repository public is the owner's (GitHub's setting), and so is its history: kept as it is, or written
anew before (above). Then milestone 4's Left (`docs/milestones.md`): a live feed of the core's topics, a gateway on
the core; the other sources as plugins and a source plugin's live stream; the runs in the session (Massive's free
plan, `delay`); a clean topic's readers stopped and
started again at a reset, and a reset cut short made whole; CI, none today, in the flake's shell; the library taken
outside this repository. Or the sessions' findings, each a step of its own. Of this round: the rules restated with
`go/`'s removal wait for the owner's approval; `plugins.md`'s sections are 1, 3 and 5, their numbers kept since many
files cite §3; `dev.environment.md`'s Known gaps holds no gap, and its test stack's line still names acquisition's
`-state`, which it no longer has; acquisition's test catalog keeps a Binance dataset and `store` (test data,
unchanged). Of step 8.9: the tutorial's programs have no tests of their own, the gate's run their test; the pages'
outputs hold the run's machine paths and times; a reset's reader stopped and started by hand; the gate's runner the
session's, outside the repository. Of step 8.8: a reader of a clean topic that runs stops a reset rather than being
stopped and started again; acquisition stopped in the middle of a reset deletes nothing; a store's clean series lags
its topic by the tail between resets; `make start`'s commands parted at spaces. Of step 8.7: `query`'s Kafka reads
its own, not the core's stream. Of step 8.6, standing: a topic name's series ambiguous when a name holds a dot,
refused a clean topic, allowed by the key; the grid of a timeframe other than M1 from the session's open; a publish
failing part way leaving corrections older than `reread` lost; a line of `massive` not Massive's K-line stopping both
writers for good; `make help` printing `Makefile` for each target; Massive's first row of a fetch by a rule not
known; Massive's pages kept without end; a real deployment's password in a writer's `-url`; ClickHouse's window in a
rebalance. The rounds before: `nixpkgs` through `qi-v2-nix`.

Read: `README.md`; `core/README.md`; `docs/milestones.md` (4); `docs/decisions.md`, 2026-11-08 and Open;
`agents/rules.md`.

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

## Open, for the owner

In `docs/decisions.md`, Open: CI in the flake's shell; the rules restated with `go/`'s removal; the branch;
`carrier.md`'s form. Decided, proposals until the owner decides: the session's entries of 2026-11-06 (no check of the
flake; the state's status and health and the stops of `make check`; `TestMigrations`' race fixed in the test, the test gone with `go/`; how
steps 8.1 to 8.4 are built where their briefs leave the choice), of 2026-11-07 (how step 8.5 is built) and of
2026-11-08 (how steps 8.6 to 8.9 are built; of 8.8, the cleaning named by the catalog and naming itself, the
protocol's `--after`, the validation reading the base fields as text, the tail from every minute after the clean
topic's last, the reset's source through the TimescaleDB writer's `-corrected` checked against the record, the stores'
clean series by their writers' `-clean`, the readers as groups stopping a reset, the core's stream's additions,
`klines_clean` without `read`, a failed reset deleting its topic; of 8.9, the tutorial's programs a list of their own
by their `go.mod`, `check-tutorial`, not built by `make build`; the catalogs as templates and the user's folder by
`mktemp -d`; `clean-reader`'s whole-value check; the follower's file as what it has done; `despike`'s 2% and the
tolerance 3; `correct-high`; acquisition run by hand on the advanced level; how `go/`'s removal is built: `make
modules`, CI removed whole, the documents removed and restated, the catalog's and the flake's changes; and, the
owner's to judge, of 8.8, a reader running not stopped, a crash mid-reset).
