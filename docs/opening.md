Opening for the next session, written 2026-11-11 by the session before (`docs/process.md`). It says where the work
stands, the next tasks, and what to read.

## Where the work stands

- The platform is built from the ground up on its formal definition, one product at a time (`docs/milestones.md`;
  `docs/decisions.md`, 2026-11-11). What stands: the definition taken (`qi-v2-platform-definition` 0.1.0, the input
  `definition`, the shell's `QI_DEFINITION`); the library (`lib/`, `lib/README.md`); `abstract-level` (`abstract/`,
  `abstract/README.md`), released 0.1.0: the flake's package `abstract-level`, the tag `abstract-level-0.1.0` on
  `09cc343`; chanlun 3.1.0's Go form, taken for `topic-schemas` (`2c954ca`: the input `chanlun`, the shell's
  `QI_CHANLUN_GO`, `go.work`'s replace).
- **This round**: `topic-schemas`' second task of three, the module `schemas/`, the schema of every topic. Stopped
  before any file of it was written, by the owner's answers to the session's questions (`docs/decisions.md`,
  2026-11-11, the last two entries of Decided). The brief has each schema encode through the contracts' own Go
  codecs, and the contracts give none for what it names: market 1.1.0's Go form has a codec for the ongoing K-line
  alone (`EncodeOngoingKLine`, `DecodeOngoingKLine`), none for `KLine`, and its vectors are ongoing K-lines;
  chanlun 3.1.0's has one for the run alone (`EncodeRun`, `DecodeRun`), none for its six records, and a record cannot
  go through it alone, the run's rules bounding every position by its merged bars; its vectors are runs. The owner:
  the module waits until market's Go form gives a codec for `KLine` and chanlun's one for each record, neither a
  text written here nor a K-line through the ongoing K-line's codec; a topic's schema has no key. No build this
  session (the owner's answer), so nothing was built or checked; this round changes `docs/decisions.md`,
  `docs/milestones.md` and this file alone.
- The stack (`stack-0.3.0`) and the test stack were not looked at or touched.
- `main` stood level with `origin/main` as `git` last saw it, before this round's commit; this round's commit is not
  pushed, and the push is the owner's. `.env` in place, ignored; `make install` run.

## The next tasks

1. The review of `abstract-level` 0.1.0 by a session that did not write it, still to be made: the task is done when
   each of its findings is fixed or dropped by the owner; a fix moves the tag `abstract-level-0.1.0` to the commit
   that holds it, the tag not being pushed. Read: `flake.nix` (`versions`, `packages`), the `Makefile`
   (`check-abstract-package`), `dev/scripts/verify.py`, `dev/taker/abstract-level/taker_test.go`,
   `abstract/README.md` (Taking it, Its check, Changes), `PRODUCTS.md`, `docs/decisions.md` (the entries on
   `abstract-level`).
2. In `qi-v2-contracts`, by the owner's brief there: market's Go form a codec for `KLine`, chanlun's one for each of
   its six records, each in a release; then each taken here, its lock moved by the owner's command.
3. Then `topic-schemas`' second task, as briefed, its schemas with no key. Read, for it: the brief; `docs/decisions.md`
   (the last two entries of Decided); `abstract/README.md`, `abstract/acquisition.go` (`Kind`),
   `abstract/definition_test.go` (how a README's table is held to the definition); the `Makefile` (`go.work`,
   `MODULES`, `check-abstract`, `check-abstract-package`); `dev/scripts/verify.py`; `PRODUCTS.md`; the contracts'
   READMEs at `QI_MARKET_GO` and `QI_CHANLUN_GO`. chanlun's vectors are in the input's source, `chanlun/vectors`,
   which no shell variable names yet.

## Standing rules learnt (until the owner approves them for `agents/rules.md`)

- **No `--remove-orphans` on the stack, and `-v` only on the test stack** (`make stack-test-down`), from the owner: the
  compose project `services` holds containers and volumes that are not the stack's (on this machine, other
  programs' containers and their volumes), which `--remove-orphans` removes, and `-v` removes the stack's
  volumes, which hold the data.
- **Every test and trial run on the test stack**, from the owner: the stack's data is kept, so nothing a test writes
  there can be taken out again; the test stack is wiped whenever a test is done; a run there keeps its state and its
  pages in a temporary folder of its own.
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
- **A session commits and does not push**, from the owner (2026-11-11): the push of `main`, and of a tag, is the
  owner's, whatever the shared rules say of a push after a commit.
- **No command that reaches the network without asking first**, from the owner's briefs: `git fetch`, `gh`, Nix
  without `--offline`; a Nix command that downloads is the owner's to run, and one that only evaluates is run with
  `--offline`. A Go target is run with `GOPROXY=off` in its environment, so that `make modules` fetches nothing.
- **`--offline` does not keep Nix from the network for an input the lock does not hold**, learnt by a session's
  fault: after `flake.nix` named the input `definition` and before `flake.lock` held it, `nix eval --offline` on the
  flake asked `api.github.com` for the tag and downloaded the input's source, no credential used. No Nix command is run
  on a flake while `flake.nix` names an input its lock lacks, or a reference it has not locked: the lock is written
  first, by the owner's command. The Makefile's `nix` commands run with no `--offline`, so an offline session runs
  them through a wrapper first on the path inside the shell.
- **A reviewer is given what it cannot read itself**, learnt by this round's review: the role reads files and runs
  nothing, `git` among them, so a review of a commit is given the commit's diff and the messages it is checked
  against as files in `.reports/`, named in its brief.

## Open, for the owner

- The row of `abstract-level` in `PRODUCTS.md`: the brief asks a new row; the row that stood, Form `library`, was
  restated as the flake package, so that one product has one row. Kept as it is unless the owner wants the library's
  row back beside it.
- In `docs/decisions.md`, Open: CI in the flake's shell. Decided there as the session's proposals until the owner
  decides: how the definition was taken, how the removal was built, how `abstract-level`'s module was built and its
  reviews settled, how its release is built, and how chanlun is taken (all 2026-11-11). The wording of `agents/rules.md` after the removal
  is the session's.
