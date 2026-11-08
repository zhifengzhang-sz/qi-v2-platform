Opening for the next session, written 2026-11-08 by the session before (`docs/process.md`). It says where the work
stands, the one next task, and what to read.

A commit this opening names from before 2026-11-08's new start is of the platform's earlier history, kept by the owner and not in this repository (`docs/decisions.md`, 2026-11-08, the history begun anew); each hash is left as it was.

## Where the work stands

- The platform (`README.md`) is the core and its additions alone (`core/README.md`; the platform's library,
  `lib/README.md`; acquisition, the printer, `query`, the stores, Massive's reader, the validation; the tutorial,
  `docs/tutorial/`), every program Go.
- **This round** (this repository's two commits; `docs/decisions.md`, 2026-11-08: the history begun anew, the rules
  restated with `go/`'s removal approved, `make modules` accepted, the branch `main`): the platform's history begun
  anew, this repository public from its start. Its first commit, on GitHub's `dcb8ef9` (its `LICENSE` alone), is the
  tree of the earlier history's last commit, `b1c325b`, as it is (the tree `fc174e58`), its `LICENSE` the platform's.
  Its second: the decisions; Open's entries on the restated rules and on the branch closed; the branch `main` in
  `agents/rules.md`'s repository line (the files `make agents` writes from it ignored, written again) and in
  `PRODUCTS.md`'s Release; a
  line in `docs/decisions.md`, `docs/milestones.md` and this opening on the earlier history's commits; this opening.
  No code, no pin, no rule's wording but the branch's changed. The earlier history, with its tag and its stash, is
  kept by the owner, private.
- **The round before** (`fd6f547`, `b1c325b`; `docs/decisions.md`, 2026-11-08, the owner's entries on the project
  public, `go/` removed, the engine worker and the Ctrl-C rule, and the session's on how it is built): the project
  made ready to be public. The old platform, `go/`, removed ahead of the path's end, with everything only it used or
  that existed for it: `deps.lock` and the executable it named, CI's one job and its token (no CI is left),
  `examples/plugins/bar-returns`, `scripts/`, `test/conformance/`, `config/development.yaml` and `production.yaml`,
  `dev/scripts/locks-check.py`, `docs/owner-setup.md` (the owner's notes on private projects; in the earlier
  history), `docs/designs/flow.md`, the old platform's engineering documents; the Makefile without `check-go`,
  `generate`, `generate-check`, `e2e`, `backup`, and with `make modules` (the modules the workspace's packages and
  tests need, from the public Go module proxy, run first by every target that runs Go, the checks and builds keeping
  `GOPROXY=off`); the flake's shell without sqlc, PostgreSQL's clients and the host's engine executable, its inputs
  unchanged; `config/catalog.yaml` without the Binance datasets and `store`. No private project named in any tracked
  file: the documents restated, the past decisions keeping what was decided and why, the core's comments that cited
  `go/` stating the rule where it stands, two tests' derived-topic names generic (`engine.…`). The engine worker, and
  what it makes, are not the platform's (`core/README.md`, The topics' names). `agents/rules.md`: the repository
  public; the Ctrl-C rule; the rules that named `go/` or a private project restated, approved this round.
  `PRODUCTS.md`: the engine executable's row out; the contract the core's topics, the programs and their flags, the
  plugin protocols, a user reads the topics, not the stores. `consumers/query/README.md`'s output of `query topics`
  on the stack leaves out the old platform's two empty topics whose names abbreviate the private engine's API. Its
  checks, role `checker`, from a fresh shell: `make modules` from an empty module cache, `make check` on the test
  stack PASS (95 top-level tests passed, 0 skipped, 0 failed, 13 packages), `make products-check` valid.
- **The gate of this round.** The trees: the first commit's tree `b1c325b`'s; the second's diff against
  it the files of the second commit above and nothing else. The search: the round before's, a case-ignored `git grep
  -I -P` of every tracked file for each of its 28 patterns (each private project's name, each short name standing
  for one, each of their paths and products the tree named before) and one of this round's, and the same over every
  message of this repository's history; the patterns are in `.reports/public.patterns.txt` while it lasts, never in
  a tracked file, where they would name what they search for. Empty in the first commit's message, the second's
  searched before the push; in the tracked files empty but
  for one pattern's line in `agents/roles/reviewer.md`, the kit's class of AI model, the generated agent files the
  same, and `deps.lock` and `owner-setup` in `docs/decisions.md` and this opening, as the earlier history's own
  removed files. The checks, role `checker`, after the last change: `make verify`, in the second commit's message.
  The push, once every line of the gate before it has passed: `main` alone to `origin` (no tag, no other branch).
- The stack at `stack-0.3.0`, up, not touched, as step 8.8 left it (then: `market.raw` 389,370 messages (p7),
  `market.corrections` 417, `market.clean.validation.v0-1-0.massive.SPY.M1` 194,880; the old platform's topics as
  they were); no component of the platform's running there. The test stack down, not used this round.
- The locks: `make locks-check` not run (it calls `gh api`); `flake.lock` unchanged.
- `main`: this repository's two commits, pushed to `origin` once the gate has passed; `.env` in place, ignored;
  `make install` run; the owner's local file: `todo.md`.

## The next task

Milestone 4's Left (`docs/milestones.md`): a live feed of the core's topics, a gateway on the core; the other
sources as plugins and a source plugin's live stream; the runs in the session (Massive's free plan, `delay`); a clean
topic's readers stopped and started again at a reset, and a reset cut short made whole; CI, none today, in the
flake's shell; the library taken outside this repository. Or the sessions' findings, each a step of its own. Of the
round before: `plugins.md`'s sections are 1, 3 and 5, their numbers kept since many files cite §3;
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

In `docs/decisions.md`, Open: CI in the flake's shell; `carrier.md`'s form. Decided, proposals until the owner
decides: the session's entries of 2026-11-06 (no check of the flake; the state's status and health and the stops of
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
owner's to judge, of 8.8, a reader running not stopped, a crash mid-reset).
