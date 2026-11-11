# Development Environment

The tools, and the shared infrastructure that `qi-v2-services` publishes. Verified on WSL2 with Docker Engine 29.9.3
and Compose 5.7.0 (section 2).

## 1. Repository layout

This repository needs no other checkout. The infrastructure stack is `qi-v2-services`' published package `stack`,
market's Go form and vectors are `qi-v2-contracts`' package `market-go` and its source, chanlun's Go form is
`qi-v2-contracts`' package `chanlun-go`, at chanlun's own release, and the platform's formal definition is
`qi-v2-platform-definition`'s package of that name, each an input of `flake.nix` pinned by `flake.lock`, a public
repository read with no login.

## 2. Tools

For the Go tools and the daily commands, see [go.toolchain.md](go.toolchain.md).

The tools come from the flake (`flake.nix`, pinned by `flake.lock`): `nix develop` gives them, and `make` runs only
in that shell (`make help` lists the targets). This table is the one home of the tools and their versions, as
nixpkgs gives them at `flake.lock`'s pin; the last row is not the flake's.

| Tool | Version | Note |
|---|---|---|
| Go | 1.26.10 | the flake's, with gopls; its caches under `~/.cache/platform/`; `GOTOOLCHAIN=local`, so go never fetches another toolchain; every `go.mod` says `go 1.26.0` |
| Python | 3.13.17 | the shell's, its standard library alone: the Makefile's scripts (`dev/scripts/`) |
| Docker + Compose | 29.9.3 / 5.7.0 | the host's, not pinned: the versions the stack was last started on (section 3); the stack's program runs `docker compose` with the `docker` on the path |

The shell names four store paths of the flake's inputs: `QI_MARKET_GO`, market's Go form, and `QI_CHANLUN_GO`, chanlun's
Go form, which `go.work` takes; `QI_MARKET_VECTORS`, market's vectors, which the library's tests read; `QI_DEFINITION`,
the definition's package, its Lean's files at its root, which the test of `abstract/`'s table reads
(`abstract/README.md`).

## 3. Infrastructure

The stack is `qi-v2-services`' published package `stack`, tag `stack-0.3.0`, an input of `flake.nix` pinned by
`flake.lock`: every image pinned (TimescaleDB `2.30.1-pg17`, Redpanda `v26.2.6`, Console `v3.12.0`, ClickHouse
`26.8.34.13`, Redis `8.8.3-alpine`), the data in its named volumes (`services_timescaledb_data`,
`services_redpanda_data`, `services_clickhouse_data`, `services_redis_data`). The package holds two stacks, each for
one thing:

- **The stack** (`qicore-*`, the ports below) holds data that is kept. No test writes it. It is never brought down
  with `-v`, which removes its volumes, nor with `--remove-orphans`, which removes containers of the Compose project
  `services` that are not the stack's.
- **The test stack** (`qitest-*`, the Compose project `services-test`, its own network and volumes, every port the
  stack's plus 10000: Redpanda at `localhost:29092`, TimescaleDB at `localhost:15432`, ClickHouse at
  `http://localhost:18123`; Redpanda, Console, TimescaleDB and ClickHouse on the stack's images, users and passwords,
  no Redis) is for tests and for a run that tries a change: every test that reaches a stack, and every trial run. It
  touches nothing of the stack, and is wiped whenever a test is done.

The Makefile is the one way to start either:

```sh
make stack-up                        # the stack: Redpanda, Console, TimescaleDB, ClickHouse, Redis
make stack-down                      # stops it; the volumes stay
make stack-test-up                   # the test stack, beside it
make stack-test-stop                 # stops the test stack; its volumes stay
make stack-test-down                 # wipes the test stack: its containers, network and volumes (down -v, the one target that runs it)
```

| Service | Host endpoint |
|---|---|
| Redpanda broker | `localhost:19092` |
| Schema registry | `localhost:18081` |
| Redpanda Console | `http://localhost:8080` |
| TimescaleDB | `localhost:5432`, `postgres`/`password` |
| ClickHouse | `http://localhost:8123` (HTTP), `localhost:9000` (native), `default`/`password` |

`make stack-up` pulls the images the machine does not have. Under Compose 5.7.0, over a VPN, its pull of several
images at once has hung for over 20 minutes, where `docker pull` of each image by its tag, one at a time, worked;
`make stack-up` then starts from the local images. The images and their tags:
`nix run --inputs-from . services#stack -- config --images`.

## 4. Tests

```sh
make check                           # the library, the definition's interfaces: gofmt, vet, tests; the agents' spec (go.toolchain.md, 4)
```

No test reaches a stack today.

## 5. Continuous integration

There is none.
