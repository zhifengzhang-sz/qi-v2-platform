# The rules of the platform's work

qi-v2-platform: the streaming architecture and the operations that run on the stream (`README.md`). These are its
standing rules, for an agent of any kind; the format of this file is the kit's (`agents/README.md`).

## The session

**Read first.**

1. `docs/opening.md`: where the last session stopped, what is open for the owner, the next task, and the files that task needs. Read those files, and no others, before starting.
2. `docs/milestones.md`: what the work is for.
3. `docs/process.md`: how a session starts and ends, and where each rule lives.

Read the others only when the task touches them: `README.md` (the system context, the operations on the stream, the platform's contract); `docs/impl/engineering/readme.md`, the index of the engineering specifications (the core, the plugins and the catalog, the Go toolchain, the development environment, the carrier).

**The rules.**

- **The owner's local file**: `todo.md`, never committed.
- **The repository.** This folder is on `master`; its remote is `origin`, `zhifengzhang-sz/qi-v2-platform` on GitHub, public; pushed after a commit. Being public, it takes nothing that cannot be read with no login, and names no private project in any file: what anyone reads here is all there is.
- **The dependencies are two, and known** (`README.md`, Dependencies and layout), each public: `qi-v2-services`, the stack, its published package `stack` pinned by `flake.lock` and run by `make stack-up` and `make stack-down`, never from a checkout; `qi-v2-contracts`, the market data contract, its package `market-go` and market's vectors pinned by `flake.lock`. The tools are `qi-v2-nix`'s, the inputs of `flake.nix`, and its `practices.md` is the standard the tooling follows. Nothing of who consumes the platform: no consumer named in code, documents or tests; no document cited outside this repository, its dependencies and `qi-v2-nix`.

## Every role

- **The platform is the platform.** A capability belongs here only if more than one consumer could use it, and it is built as part of the contract, a topic, a program and its flags, a plugin protocol (`README.md`, The platform contract); a consumer's code never. The core names no source; an engine that reads the core's topics, and what it makes, is not the platform's.
- **No conflict is resolved silently**: every uniqueness the code relies on is enforced by the database or checked by a query that fails the run, and every `ON CONFLICT` of a store on the core is listed with its reason in the store's README.
- **Every program is Go**: the core, the library and the additions, a module each (`docs/impl/engineering/go.toolchain.md`, 2); the TypeScript apps are retired, and a browser page is a consumer's, not the platform's. No Confluent component. The code's conventions: `docs/impl/engineering/go.toolchain.md`, 5.
- **A run's Ctrl-C reaches the program itself, and nothing of a step is left running after it**: the signal is sent to the program's own process, and after each step a check finds nothing of the step still running.
- **The project's secrets**: `.env`; the Massive key, `~/.config/qi/massive_key`, read by the library's fetcher alone (`lib/README.md`). The local stacks' `password` is none.
