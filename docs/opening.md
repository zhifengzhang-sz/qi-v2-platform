Opening for the next session, written 2026-11-11 by the session before (`docs/process.md`). It says where the work
stands, the next tasks, and what to read.

## Where the work stands

- The platform is built from the ground up on its formal definition, one product at a time (`docs/milestones.md`;
  `docs/decisions.md`, 2026-11-11). What stands: the definition taken (`qi-v2-platform-definition` 0.1.0, the input
  `definition`, the shell's `QI_DEFINITION`); the library (`lib/`, `lib/README.md`); `abstract-level` (`abstract/`,
  `abstract/README.md`), released 0.1.0: the flake's package `abstract-level`, the tag `abstract-level-0.1.0` on the
  commit of this opening, neither pushed: the push of `main` and of the tag waits for the owner's yes.
- **This round**: `abstract-level`'s fourth task, its release (`docs/decisions.md`, 2026-11-11: the owner's brief, and
  the session's proposal on how it is built). The package (`flake.nix`, `versions`, `packages`): `go.mod`, the
  module's Go files but its tests, its README and `LICENSE`, at its root. Its number, 0.1.0, in the flake, in
  `abstract/README.md` (Taking it, Changes) and in its row of `PRODUCTS.md`, the row that stood restated (Form
  `flake package`, Version `0.1.0`, Release `abstract-level-0.1.0`), not a second row. Its check,
  `make check-abstract-package`: the package built by `nix build --offline`, its files held to exactly what it is
  stated to hold, each the work tree's; then a module in a temporary folder, the test of
  `dev/taker/abstract-level/` beside a `go.mod` the target writes, replacing the module's path with the package's
  store path, `gofmt`, `go vet`, `go test` with `GOWORK=off` and `GOPROXY=off`, the test wiring a working space from
  what the package exports and running it. In verify's table for `abstract/`, `flake.nix`, `flake.lock`, `LICENSE`
  and the taker's test; not in `make check`. `docs/decisions.md`, as the session's proposals, the choices of tasks 1
  and 3 no entry held (how the definition was taken; how the module's two reviews were settled) and this round's.
- **Its review**, role `reviewer`, on `7570d70`, its run logged (`reviewer-7570d70`, start `RECORDED`, stop `PASS`,
  nothing changed). Three findings: the package's check passed whatever the package held (fixed: its files held to
  the list, each compared, a trial on three wrong copies failing each); the row of `PRODUCTS.md` restated where the
  brief asks a new row (not taken: one product has one row, its form now the flake package; the owner's to decide,
  Open below); the decision's entry said the package is made as `market-go` is, `LICENSE` included, where
  `market-go` holds none (fixed: the layout `market-go`'s, the `LICENSE` as the definition's package holds its own).
  Wording fixed: `doc.go` in the README's files, the reason no `go.sum` line is needed, the Shapes of the entry on
  the definition, the taker's module not one of `MODULES`; not taken: `make check`'s "every check", which stood
  before and is not this change's. The reviewer, having no `git`, read the work tree and not the commits' messages:
  the entries on tasks 1 to 3 rest on the session's reading of `0fa98c2`, `e2b1b14`, `851a372` and `abfe7bb`.
- **The checks**, role `checker`, every nix offline and `GOPROXY=off`, through the flake's shell: `make locks-check`
  with the owner's yes (it calls `gh api`), every lock current; `make verify VERIFY=--all` passing before the first
  commit and again after the fixes, each run logged; after the last commit, `git status --porcelain`,
  `git rev-parse HEAD` and the full `make verify` in one command, as the round's report gives it.
- The stack (`stack-0.3.0`) and the test stack were not looked at or touched: no test of this round reaches a stack.
- `main`: this round's commits and the tag are not pushed; the push is the owner's. `.env` in place, ignored;
  `make install` run.

## The next tasks

First, the review of this release by a session that did not write it: the task is done when each of its findings is
fixed or dropped by the owner. A fix moves the tag `abstract-level-0.1.0` to the commit that holds it, the tag not
being pushed. Then the owner's push of `main` and of the tag. Then, by a brief to come, the next product of
`docs/milestones.md`, `library`.

Read, for the review: `flake.nix` (`versions`, `packages`), the `Makefile` (`check-abstract-package`),
`dev/scripts/verify.py`, `dev/taker/abstract-level/taker_test.go`, `abstract/README.md` (Taking it, Its check,
Changes), `PRODUCTS.md`, `docs/decisions.md` (the last four entries of Decided).

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
  reviews settled, and how its release is built (all 2026-11-11). The wording of `agents/rules.md` after the removal
  is the session's.
