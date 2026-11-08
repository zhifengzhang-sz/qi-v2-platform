# Development Environment

How to run the platform locally against the shared
infrastructure that `qi-v2-services` publishes. Verified on WSL2 with
Docker Engine 29.9.3 and Compose 5.7.0 (section 2).

## 1. Repository layout

This repository needs no other checkout. The infrastructure stack is
`qi-v2-services`' published package `stack`, and market's Go form and
vectors are `qi-v2-contracts`' package `market-go` and its source, each an
input of `flake.nix` pinned by `flake.lock`, a public repository read with
no login.

## 2. Tools

For what each Go tool replaces on the TypeScript side and the daily
commands, see [go.toolchain.md](go.toolchain.md).

The tools come from the flake (`flake.nix`, pinned by `flake.lock`): `nix develop` gives them, and `make` runs only
in that shell (`make help` lists the targets). This table is the one home of the tools and their versions, as
nixpkgs gives them at `flake.lock`'s pin; the last row is not the flake's.

| Tool | Version | Note |
|---|---|---|
| Go | 1.26.10 | the flake's, with gopls; its caches under `~/.cache/platform/`; `GOTOOLCHAIN=local`, so go never fetches another toolchain; every `go.mod` says `go 1.26.0` |
| Docker + Compose | 29.9.3 / 5.7.0 | the host's, not pinned: the versions the stack was last started on (section 3); the stack's program runs `docker compose` with the `docker` on the path |

## 3. Infrastructure

The stack is `qi-v2-services`' published package `stack`, tag `stack-0.3.0`,
an input of `flake.nix` pinned by `flake.lock`: every image pinned (TimescaleDB
`2.30.1-pg17`, Redpanda `v26.2.6`, Console `v3.12.0`, ClickHouse `26.8.34.13`,
Redis `8.8.3-alpine`), the data in its named volumes (`services_timescaledb_data`,
`services_redpanda_data`, `services_clickhouse_data`, `services_redis_data`). The
package holds two stacks, each for one thing:

- **The stack** (`qicore-*`, the ports below) holds the platform's data: the
  core's topics, `market.raw` among them, which keeps every message forever, so that nothing
  written to it can be taken out again, and the stores on the core. Nothing but
  the platform's own runs writes it; no test does. It is never brought down with
  `-v`, which removes its volumes, nor with `--remove-orphans`, which removes
  containers of the Compose project `services` that are not the stack's.
- **The test stack** (`qitest-*`, the Compose project `services-test`, its own
  network and volumes, every port the stack's plus 10000: Redpanda at
  `localhost:29092`, TimescaleDB at `localhost:15432`, ClickHouse at
  `http://localhost:18123`; Redpanda, Console, TimescaleDB and ClickHouse on the
  stack's images, users and passwords, no Redis) is for tests and for a run that
  tries a change: every test, the stack's tests of `make check` among them
  (`go.toolchain.md`, 4), and every trial run, with a state and a folder of its
  own where a program keeps one outside the stack (acquisition's `-state`, the
  plugin's `-pages`). It touches nothing of the stack, and is wiped whenever a
  test is done.

The Makefile is the one way to start either:

```sh
make stack-up                        # the stack: Redpanda, Console, TimescaleDB, ClickHouse, Redis (Redis not used by the platform)
make stack-down                      # stops the platform's components on it, then it; the volumes stay
make stack-test-up                   # the test stack, beside it
make stack-test-stop                 # stops the test stack's components, then it; its volumes stay
make stack-test-down                 # wipes the test stack, its components stopped first: its containers, network and volumes (down -v, the one target that runs it)
make start                           # the stack up, then the platform's components on it (STACK=test CATALOG=…: the test stack's)
```

**The platform running.** `make start` brings the stack up, builds the core's
programs, provisions the core's topics, then starts acquisition (`-follow`,
`plugins.md`, 3) and both stores' writers (their READMEs under `consumers/`),
each a process of its own in a session of its own, which keeps running while
the stack is up (`dev/scripts/run.py`): its log and its pid in
`~/.local/state/qi/run/<stack>/`, each start a line of each log, then the
component's own. Acquisition is given each writer's program for its reset
(`-corrected`, the TimescaleDB writer's, whose corrected raw it reads;
`-store`, both writers', whose clean series it writes, `plugins.md`, 3),
their runs' lines in its log. Where each takes up, the start says: acquisition in its run's
line (the record as it found it, the span asked), each writer by its group's
committed offsets, read before it starts (`rpk` in the stack's Redpanda, no
group joined), and in its log whether it reads from them or, its store holding
no bar, from the topics' start. A component already running is not started
again. `make stack-down` (and, for the test stack, `make
stack-test-stop`) stops each component (SIGTERM, waiting for it to end), then
the stack, its volumes kept: the platform paused. A start continues it, each
component from where it stopped: acquisition from the record, the bar after the
series' last on `market.raw`, with its re-read; each writer from its group's
committed offsets. `STACK=test` runs it on the test stack, `CATALOG` naming a
catalog of the run's own whose plugin keeps its pages in a temporary folder
(`-pages`); acquisition outlasts a run that fails, the next at its time; a
component that ends on its own (a writer's fault, the stack gone) says why in
its log, and the next start starts it again.

| Service | Host endpoint | Used by |
|---|---|---|
| Redpanda broker | `localhost:19092` | every producer and consumer (each program's `-brokers`, by default) |
| Schema registry | `localhost:18081` | not used by the platform |
| Redpanda Console | `http://localhost:8080` | browsing topics and schemas |
| TimescaleDB | `localhost:5432`, `postgres`/`password` | a store on the core, in a database of its own |
| ClickHouse | `http://localhost:8123` (HTTP), `localhost:9000` (native), `default`/`password` | a store on the core, in a database of its own |

The stack makes none of the platform's schema: TimescaleDB starts with the database
`cryptodb` and no table of the platform's in it, ClickHouse with only the server's
own databases. The platform makes its own topics (next section), and a store on the
core its own database (its README under `consumers/`).

`make stack-up` pulls the images the machine does not have. Under Compose 5.7.0,
over a VPN, its pull of several images at once has hung for over 20 minutes, where
`docker pull` of each image by its tag, one at a time, worked; `make stack-up`
then starts from the local images. The images and their tags:
`nix run --inputs-from . services#stack -- config --images`.

## 4. Platform setup

```sh
make build                            # bin/: the core's programs (make help, build)
bin/provision                         # the core's topics, market.raw and market.corrections (core/README.md)
```

The core's programs and its additions take `-brokers` (the local stack's
`localhost:19092` by default) and read nothing of `config/` but the
catalog, which acquisition and `query` read (`plugins.md`, 3).

## 5. Tests

The tests that reach a stack need the test stack running (`make stack-test-up`); run by hand, `-count=1` runs every
test afresh, since Go's test cache keys no state of a stack unless `make check` sets it (`go.toolchain.md`, 4):

```sh
QI_WAREHOUSE_TEST_URL=postgres://postgres:password@localhost:15432/postgres QI_KAFKA_BROKERS=localhost:29092 \
QI_CLICKHOUSE_TEST_URL=http://default:password@localhost:18123 GOPROXY=off go test -count=1 ./core/... ./consumers/...
```

## 6. Verification checklist

```sh
make check                                      # gofmt, vet, tests; the agents' spec
docker exec qicore-redpanda rpk topic list
```

## 7. Continuous integration

There is none: the core, the library and the additions take market's Go form from the flake, so a job that checks
them enters the flake's shell (`docs/decisions.md`, Open, CI in the flake's shell).

## 8. Known gaps

- No Confluent component is part of the stack: broker, schema
  registry and console are Redpanda's; every program uses franz-go.
