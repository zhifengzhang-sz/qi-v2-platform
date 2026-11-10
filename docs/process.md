# How the project is worked: the machine, the files, the sessions

One page for what is otherwise spread out: what a machine needs, every kind of file and where it comes from, where
each rule lives, and how a session starts and ends. Each fact keeps its one home; this page points to it.

## The machine

Once a machine: Nix with flakes; a Docker engine (on WSL, Docker Desktop with its WSL integration turned on for the
distribution); for a push and `make locks-check`, GitHub's access (one browser login with `gh`, an SSH key added to
the account), a clone over HTTPS needing none. Then the clone, `nix develop`, `make install`. The shell gives every
tool the targets call (`flake.nix`; the tools and their versions: `docs/impl/engineering/dev.environment.md`, 2). The
services stack, `qi-v2-services`' Redpanda, TimescaleDB and ClickHouse, is an input, its package `stack` pinned by
`flake.lock`, and it holds two stacks (`docs/impl/engineering/dev.environment.md`, 3): the stack (`make stack-up`),
whose data is kept and which no test writes; and the test stack (`make stack-test-up`), beside it, every port the
stack's plus 10000, on which every test and every trial run is made, and which is wiped (`make stack-test-down`)
whenever a test is done. Nothing is taken by hand: every input is a public repository, read with no login.

## The files, by kind

| Kind | Where | In git | Made by | On a new machine |
|---|---|---|---|---|
| the work: the Go modules, the specifications | `lib/`, `docs/impl/` | yes | the sessions | the clone |
| the state of the work | `docs/opening.md` (the next session's opening), `docs/milestones.md`, `docs/designs/`, `docs/decisions.md` | yes | each session, at its end | the clone |
| the rules for agents, their source | `agents/` (`rules.md`, `roles/`, `stages/`) | yes | the sessions, the owner approving | the clone |
| the rules for agents, generated | `AGENTS.md`, `CLAUDE.md`, `.claude/agents/`, `.github/agents/`, `.github/hooks/kit.json` | no | `make agents` (in `make install`), by the kit | `make install` |
| the project's machinery | `Makefile`, `dev/hooks/` (the pre-commit), `dev/scripts/` (verify), `.envrc` (direnv) | yes | the sessions | the clone |
| the Go workspace | `go.work` (and Go's `go.work.sum`): the Go modules and market's Go form at its store path (`docs/impl/engineering/go.toolchain.md`, 2) | no | `make go.work`, and the shell on entry | `nix develop` |
| the tools | `/nix/store`, pinned by `flake.nix` and `flake.lock`; the common flakes in the public `qi-v2-nix` | the pins | `nix develop` | `nix develop` |
| the kit | `dev/agents` → `/nix/store`, pinned by `flake.lock` | the pin | `make install` | `make install` |
| what the project owns, gives and takes | `PRODUCTS.md` (`make products-check`) | yes | the sessions, in the commit of the change it describes | the clone |
| the dependencies' versions | each module's `go.mod` and `go.sum` (the Go modules) | yes | the sessions; `go mod` | the clone |
| build output | Go's caches under `~/.cache/platform/`, its module cache filled by `make modules` | no | the builds | the first build |
| Massive's pages | `~/.local/share/qi/massive/pages/`: each page Massive sent but its `request_id`, a series a folder, a page a file named by its content and kept once, a fetch a line of the folder's `fetches` (`lib/README.md`) | no | the library's fetcher | not carried: fetched again |
| reports | `.reports/`: long review output; nothing a later session relies on | no | the roles | made again |
| the owner's notes | `todo.md` | no (local) | the owner | not carried |
| an agent's own memory | `~/.claude/projects/…` | no | Claude Code | not used (`AGENTS.md`) |

A file a later session needs is tracked; a file made again by a target is not. A document in `.reports/` that a
tracked file or the opening comes to rely on moves to `docs/designs/`.

## Where each rule lives

| Rules | Home |
|---|---|
| the standing rules of the work, for every agent and the session | `agents/rules.md` (its generated copy `AGENTS.md`) |
| each role's | `agents/roles/<role>.md` |
| the code's conventions | `docs/impl/engineering/go.toolchain.md`, 5 |
| what the platform is, what stands, what it depends on | `README.md`; what it is, in full: the definition (`QI_DEFINITION`, the input `definition`) |
| the engineering specifications | `docs/impl/engineering/readme.md`, the index |
| the tools and the setup | `flake.nix`; `docs/impl/engineering/dev.environment.md` |
| the decisions | `docs/decisions.md`: each once, with who made it, when, and why, the why in facts and logic, never a quotation in its place; the documents state what stands |
| rules the owner gives in a session | the opening's Standing rules learnt, until the owner approves their wording for `agents/rules.md` |

## A session

1. **Start.** On a new machine, The machine above; else `git pull`. Then `nix develop` and, after a change of
   `flake.lock` or `agents/`, `make install`. `AGENTS.md` (read by every agent) sends the session to
   `docs/opening.md`, then to the files it names. Then `make locks-check`: a lock is outdated when its source has
   something newer than what it names (an input's branch past `flake.lock`'s commit). The session reports each outdated lock to the owner with what updates it, and updates
   none without the owner's say.
2. **Work.** The opening's one task. Before the first build of the session, the owner is asked. Checks run in the
   role `checker` (`make check`: `check-lib`, `agents-check`); every test that reaches a stack, and every trial run,
   is on the test stack. No fallbacks: a missing tool or input stops the run with its name.
3. **End.** `docs/opening.md` rewritten for the next session (where it stopped, the one task, the standing rules
   learnt), `docs/milestones.md` brought up to date, every document a later session needs in `docs/designs/`;
   committed by explicit paths, after the checks pass, and pushed.

## Another project

It takes this project's products through what is published, never by a path into this folder: each product's row in
`PRODUCTS.md` names its form and its release.
