Opening for the next session, written 2026-11-11 by the session before (`docs/process.md`). It says where the work
stands, the next tasks, and what to read.

## Where the work stands

- The platform is built from the ground up on its formal definition, one product at a time (`docs/milestones.md`;
  `docs/decisions.md`, 2026-11-11). What stands: the definition taken (`qi-v2-platform-definition` 0.1.0, the input
  `definition`, the shell's `QI_DEFINITION`); the library (`lib/`, `lib/README.md`); and `abstract-level`'s Go module
  (`abstract/`, `abstract/README.md`), built and not released.
- **This round**: `abstract-level`'s Go module, the third of its four tasks. `abstract/`, a module of its own on the
  standard library alone, in the Go workspace (`MODULES` of the `Makefile`): the open parts as interfaces of one method
  each (`Reader`, `Cleaner`, `RawPublisher`, `CleanPublisher`, `CorrectionsPublisher`, `Consumer`, `Chart`, `Worker`),
  a part given reading ends that wait (`Cursor`, `ReadSet`) and writing ends that only append (`Appender`,
  `AppendSet`); built in, a topic's shape (`Topic`, `Record`, `TopicLogs`), the three kinds (`Kind`) and the wiring
  (`Wiring`, `Workspace`, `Acquisition.Run`, `Warehousing.Run`). Its README holds what the types do not hold, and the
  table from each declaration of the definition's seven files to its Go name, 163 rows, a structure's fields and an
  inductive's constructors among them. Its check is `make check-abstract`, in `make check` and in verify's table for
  `abstract/`, `flake.nix` and `flake.lock`; `check-lib` now checks the library alone, both by one recipe
  (`go-check`). How it is built where the brief left the choice: `docs/decisions.md`, 2026-11-11, the session's
  proposal.
- **Its review**, role `reviewer`, the Go against the Lean, declaration by declaration, and one recheck. Found and
  fixed: the series inside acquisition made the reader wait for the cleaner to read, so the raw topics depended on the
  cleaner (now each reader's untaken messages are held, and no part waits for another: `TestUnread`); a `ReadSet`
  converted into an `AppendSet` (two types now: `TestSetsApart`); one log could be given for two partitions
  (`ErrSameLog`, told where one pointer is given twice); a `Workspace` no wiring made, or a copy of one, got past the
  checks (`Run` refuses both). Found and stated, not changed: the `Database` values of several schemas are one
  database by their giver's keeping; `Acquisition.Run` and `Warehousing.Run` refuse a name twice where Lean's `run`
  takes any topics, a part finding a topic by its name. The recheck found the four fixed, and one place where the
  text said less than the code (which logs are told to be one; `Warehousing.Run` refusing a log given twice): the
  text now says it, and was not rechecked.
- **The review made again**, on `851a372`, its run logged: `agents.run/log` holds its lines (`reviewer-851a372`,
  start `RECORDED`, stop `PASS`, nothing changed), written by the kit's hook (`kit hook start`, `kit hook stop`),
  called by hand around the run, the session's kind of agent having no hook of its own; the record it stages names
  the kind `copilot`, which the kit writes for every hook's run, and the run was the role `reviewer` of this
  session's kind. One finding: one cursor given as both the source and the corrections was not refused, so the
  reader and the corrections' publisher would each take part of one log. Fixed: `ErrSameInput`, told where the
  cursor is one pointer or one channel. Three texts made exact with it.
- **The shell.** A shell entered before `flake.lock` held the definition has no `QI_DEFINITION`, and
  `make check-abstract` stops there with its name. This round's checks, and its commits, whose hook runs `make verify`,
  were run through the flake's shell entered afresh: `nix develop --offline -c env GOPROXY=off make …`.
- The stack (`stack-0.3.0`) and the test stack were not looked at or touched: no test of this round reaches a stack.
- The locks: `make locks-check` not run (it calls `gh api`).
- `main`: this round's commits are not pushed; the push is the owner's. `.env` in place, ignored; `make install` run;
  the owner's local file: `todo.md`.

## The next tasks

By a brief to come, `abstract-level`'s fourth task: its release (`agents/rules.md`, A release: one number, written
alike in the package's files and the flake package where it has one; an entry under Changes in its README; the
checker's pass; its row in `PRODUCTS.md`; the tag `abstract-level-<version>`, its push the owner's). Then each other
product of `docs/milestones.md`, in its order.

Read, for the release: `abstract/README.md`, `abstract/go.mod`, `PRODUCTS.md`, `flake.nix`, the `Makefile`,
`docs/impl/engineering/go.toolchain.md` (2).

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

## Open, for the owner

In `docs/decisions.md`, Open: CI in the flake's shell. Decided there as the session's proposal until the owner
decides: how the removal is built where the owner's answer left the choice, and how `abstract-level`'s module is built
where its brief left the choice (both 2026-11-11). The wording of `agents/rules.md` after the removal is the
session's.
