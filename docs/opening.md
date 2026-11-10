Opening for the next session, written 2026-11-11 by the session before (`docs/process.md`). It says where the work
stands, the next tasks, and what to read.

## Where the work stands

- The platform is built from the ground up on its formal definition, one product at a time (`docs/milestones.md`;
  `docs/decisions.md`, 2026-11-11). What stands: the definition taken (`qi-v2-platform-definition` 0.1.0, the input
  `definition`, the shell's `QI_DEFINITION`), and the library (`lib/`, `lib/README.md`).
- **This round** (the commit after `0fa98c2`, the removal): the build before cleared away, as the owner agreed to the
  session's list. The tag `old-build` is on `0fa98c2`, the commit before it. Removed, 129 files: the core, acquisition,
  the plugins and the catalog, the stores, the charting consumer, the printer, `query`, the data sets (their lock,
  README, install and script) and their maker, the tutorial, the four designs, the plugins' and the carrier's
  documents, `dev/scripts/run.py`, `dev/stack-state.mk`. Cut to what is left: `flake.nix` (the input `visualization`
  out, and `qi-v2-ts` with it; the packages `datasets` and `chart-page` and `lib.dataset` out), `flake.lock` (those two
  nodes dropped, every other node as before, written offline), the `Makefile` (`check` is `check-lib` and
  `agents-check`; the five `stack-*` targets kept, no longer stopping any program first), `dev/scripts/verify.py`,
  `.gitignore`. Restated to what stands: `README.md`, `PRODUCTS.md`, `agents/rules.md` and the checker's role,
  `docs/` (this file, the milestones, the decisions begun anew, the process, the two engineering documents and their
  index, the designs' index), and three places of the library's own files that pointed at a removed thing, no code
  changed. The data sets' release tags (`datasets-1.0.0`, `qi-v2-platform-klines-massive-spy-1m-1.0.0`) and the
  image on GHCR stay. Not in git: `bin/`, the old programs, deleted; `.reports/` left as it was.
- **The round before** (`0fa98c2`): the definition taken, `flake.lock` written by the owner's `nix flake lock`.
- The stack (`stack-0.3.0`) was not looked at or touched by this round; it holds the topics and the stores' databases
  the build before wrote, which nothing here reads or writes. The test stack was not used. Outside the repository and
  untouched: Massive's pages (`~/.local/share/qi/massive/pages/`), the data set's folder
  (`~/.local/share/qi/datasets/`), the old runs' logs (`~/.local/state/qi/run/`).
- The locks: `make locks-check` not run (it calls `gh api`).
- `main`: the removal's commit and the tag `old-build` are not pushed; their push is the owner's. `.env` in place,
  ignored; `make install` run; the owner's local file: `todo.md`.

## The next tasks

By briefs to come, `abstract-level`'s other tasks: the Go module of the definition's interfaces, checked against the
definition at `QI_DEFINITION`; then its release. Then each other product of `docs/milestones.md`, in its order.

Read, for `abstract-level`: the definition's files at `QI_DEFINITION` (`Kafka.lean`, `Acquisition.lean`,
`Warehousing.lean`, `Visualization.lean`, `MarketModelEngine.lean`, `Workspace.lean`, `Wiring.lean`, its `README.md`);
`docs/impl/engineering/go.toolchain.md` (2, 5); `Makefile`, `dev/scripts/verify.py`, `flake.nix`, `PRODUCTS.md`.

## Standing rules learnt (until the owner approves them for `agents/rules.md`)

- **No `--remove-orphans` on the stack, and `-v` only on the test stack** (`make stack-test-down`), from the owner: the
  compose project `services` holds containers and volumes that are not the stack's (on this machine `fff-postgres`,
  `qicore-qdrant`, `qicore-chromadb` and their volumes), which `--remove-orphans` removes, and `-v` removes the stack's
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
- **`--offline` does not keep Nix from the network for an input the lock does not hold**, learnt by a session's
  fault: after `flake.nix` named the input `definition` and before `flake.lock` held it, `nix eval --offline` on the
  flake asked `api.github.com` for the tag and downloaded the input's source, no credential used. No Nix command is run
  on a flake while `flake.nix` names an input its lock lacks, or a reference it has not locked: the lock is written
  first, by the owner's command. The Makefile's `nix` commands run with no `--offline`, so an offline session runs
  them through a wrapper first on the path inside the shell.

## Open, for the owner

In `docs/decisions.md`, Open: CI in the flake's shell. Decided there as the session's proposal until the owner
decides: how the removal is built where the owner's answer left the choice (2026-11-11). The wording of
`agents/rules.md` after the removal is the session's: the owner reads it in the removal's commit.
