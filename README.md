# qi-v2-platform

**The streaming architecture and the operations that run on the stream.**

Market data are events on Kafka topics; this repository is what moves them onto, along and off that stream. What the
platform is, is defined in Lean by [`qi-v2-platform-definition`](https://github.com/zhifengzhang-sz/qi-v2-platform-definition):
a working space of four components, acquisition, warehousing, visualization and the market model engine (optional),
wired by the topics, acquisition alone reading the vendor's logs. This repository is that working space in Go, built on
the definition from the ground up, one product at a time (`docs/milestones.md`); the market model engine is not the
platform's.

## What stands

**The platform's library** ([`lib/README.md`](lib/README.md)), a Go module of its own on market's Go form and the
standard library alone: what reads a source, as packages any app can use, the platform's or a user's. Its first
package is Massive's reader: Massive's page schema, the fetcher that keeps every page, and the reader of a page.

What the project owns, gives and takes: [`PRODUCTS.md`](PRODUCTS.md). The build before this one is at the tag
`old-build`.

## Language

Every program is Go. No Confluent component anywhere.

## Running it

```sh
# once a machine: Nix with flakes, a Docker engine (docs/process.md, The machine)
nix develop                        # the tools (flake.nix; docs/impl/engineering/dev.environment.md, 2)
make install                       # git's hooks, the kit, the agents' files
make help                          # the targets
make check                         # the library: gofmt, vet, tests; the agents' spec

# infrastructure: Redpanda, Console, TimescaleDB, ClickHouse, Redis
make stack-up                      # qi-v2-services' package `stack`, at flake.lock's pin; make stack-down stops it
make stack-test-up                 # the test stack beside it, for tests; make stack-test-down wipes it
```

The state of the work and how a session goes: `docs/opening.md`, `docs/milestones.md`, `docs/decisions.md`,
`docs/process.md`. The rules for the agents that work on it: `agents/rules.md` (`AGENTS.md` and `CLAUDE.md` are
generated from it by `make agents`).

## Dependencies and layout

What this repository needs, and nothing about who uses it; each is a public repository, read with no login:

| Dependency | What the platform takes from it | How |
|---|---|---|
| [`qi-v2-platform-definition`](https://github.com/zhifengzhang-sz/qi-v2-platform-definition) | the platform's formal definition 0.1.0, in Lean: the base, Kafka's topic, the four components and their wiring | its package `qi-v2-platform-definition`, the Lean as source, an input of `flake.nix` at the tag `qi-v2-platform-definition-0.1.0` pinned by `flake.lock`, its `qi-v2` and `kit` following this flake's; the shell names its store path, `QI_DEFINITION` |
| [`qi-v2-services`](https://github.com/zhifengzhang-sz/qi-v2-services) | the infrastructure: Redpanda (broker, registry, Console), TimescaleDB, ClickHouse, Redis | its published package `stack`, an input of `flake.nix` at the tag `stack-0.3.0` pinned by `flake.lock`: the stack, `make stack-up`, `make stack-down`, and the test stack, `make stack-test-up`, `make stack-test-stop`, `make stack-test-down` |
| [`qi-v2-contracts`](https://github.com/zhifengzhang-sz/qi-v2-contracts) | the market data contract, `market` 1.1.0: its Go form, the base K-line of the library's K-line; its vectors, to which the library holds its K-lines' base | its package `market-go` and its source's `market/vectors`, an input of `flake.nix` at the tag `market-1.1.0` pinned by `flake.lock`, which `go.work` (written by `make go.work` and the shell) puts in place of the module the library's `go.mod` requires at `v0.0.0` |
| [`qi-v2-nix`](https://github.com/zhifengzhang-sz/qi-v2-nix) | the tools from one nixpkgs (Go in the shell), the pure shell, the kit for the agents' files | the inputs of `flake.nix`, pinned by `flake.lock`; `nix develop`; the tools and their versions: [`dev.environment.md`](docs/impl/engineering/dev.environment.md), 2 |

```
flake.nix, flake.lock      the tools, as inputs pinned by the lock (nix develop); Makefile, the targets, run in that shell
PRODUCTS.md                what the platform owns, gives and takes, an index (make products-check)
agents/                    the rules and roles of the agents that work here; AGENTS.md and CLAUDE.md are generated from it
docs/opening.md …          the state of the work: opening, milestones, decisions, designs, process
dev/                       hooks (the pre-commit) and scripts (verify)
docs/impl/engineering/     the Go toolchain, the development environment
lib/                       the platform's library, a Go module on market and the standard library alone: Massive's reader
go.work                    the Go workspace (make go.work), not in git
```

## License

Apache License 2.0 (`LICENSE`). Copyright 2026 Zhifeng Zhang.
