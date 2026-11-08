# The Go toolchain

What the platform's Go modules use, set beside bun, Biome and
Vitest, which the retired TypeScript apps used, and the commands a
contributor runs. Most
of it is the `go` command itself; Go comes from the flake
(`nix develop`), golangci-lint is optional and not in it. The modules:
the core, the platform's library and the core's additions (the root
`Makefile`'s `CORE`), and the tutorial's programs (`TUTORIAL`), one
workspace (2).

## 1. The mapping

| TypeScript | Go | Notes |
|---|---|---|
| **bun**: runtime, package manager, build | the **`go` command**: `go build`, `go run`, `go mod` | dependencies are declared in each module's `go.mod` and pinned by its `go.sum`; there is no `node_modules` (modules live in the shell's `GOMODCACHE`, under `~/.cache/platform/`) |
| **Biome**: format | **`gofmt`** (ships with Go) | no configuration exists or is wanted: every Go codebase is formatted the same way |
| **Biome**: lint | **`go vet`** (ships with Go) | narrow on purpose: bad `Printf` formats, copied locks, unreachable code, misuse of the standard library |
| **Vitest** | **`go test`** and the standard `testing` package | tests are `*_test.go` files beside the code they test; no test framework is added |
| `package.json` scripts | the root **`Makefile`** | `make check`, `make build` |

## 2. Installing

Go is the flake's (`nix develop`; its version: `dev.environment.md`, 2).

**The modules.** The core (`core/README.md`), the platform's library (`lib/README.md`) and each addition on the
core, a module of its own in a folder of its own, named once, in the root `Makefile`'s `CORE`; what each takes is in
its `go.mod`: market, the core but for a source plugin and the library, the library but for the core and the additions
that do not use it, and the modules it needs besides (franz-go, YAML, pgx), each at the version every module of the
workspace that takes it takes; the library takes market alone. The tutorial's programs (`../../tutorial/README.md`)
are modules too, each of its own folder under its level's, found by its `go.mod` (the `Makefile`'s `TUTORIAL`), so
that removing its folder removes it; each takes what an addition takes, the reader of the basic level franz-go and
market alone. They are one workspace, `go.work` at the root, which `make go.work` writes from the shell's
`QI_MARKET_GO` (the shell writes it on entry too) and git never holds: it uses the modules of `CORE` and `TUTORIAL`
and replaces market's Go form, which every `go.mod` of them requires at `v0.0.0`, by the store path of the flake's
package `market-go` (the input `contracts`, `qi-v2-contracts` at `market-1.1.0`); `go.sum` has no line for it. An
addition does not require the core or the library in its `go.mod`: the workspace gives them, and a requirement of a
workspace module at a version sends Go to the module proxy for it. Every target that runs Go on them runs `make
modules` first, which puts in Go's module cache each module their packages and their tests need but market's Go form,
from the Go module proxy with no login, each checked against its `go.sum`, and fetches nothing the cache holds; Go
then runs with `GOPROXY=off`, so that it fetches nothing while it vets, tests or builds.

The programs build with `make build`, the core's and its additions' into `bin/` (ignored by git; the library is no
program); the tutorial's, by its pages' `go build`, into `bin/`. `make check-core` runs this file's checks on the
modules of `CORE`, from the root's `Makefile`, with `GOPROXY=off`; `make check-tutorial` the same on those of
`TUTORIAL`, with a `go build` of each, its output discarded.

## 3. Daily commands

Run from the root, in the shell:

```sh
make check                    # gofmt (fails on any unformatted file), go vet, go test (what each reruns: 4)
make check FULL=1             # the same, every vet and every test afresh, none from Go's caches
gofmt -w core lib publishers consumers plugins   # format everything
GOPROXY=off go vet ./core/...                    # lint one module
GOPROXY=off go test -count=1 ./core/...          # one module's tests, afresh (a test's result is cached otherwise)
GOPROXY=off go test -count=1 -v ./publishers/acquisition/...   # verbose
GOPROXY=off go test -count=1 -run TestDerived ./core/clean/    # one test by name (a regular expression)
GOPROXY=off go test -race ./core/...             # the race detector: data races between goroutines
GOPROXY=off go test -cover ./core/...            # coverage per package
```

## 4. Tests that need the services stack

Some tests need real infrastructure. The tests that reach TimescaleDB or
ClickHouse skip, and say why, when their variable is unset; the stream
contract test, with `QI_KAFKA_BROKERS` unset, tries the test stack's
`localhost:29092` and skips, saying so, when no broker answers within 5 s;
with it set, it fails then.

The tests run on the test stack (`make stack-test-up`, `dev.environment.md`, 3), never on the stack, which holds the
platform's data; it is wiped when they are done (`make stack-test-down`).

| Variable | Tests | Value on the test stack |
|---|---|---|
| `QI_WAREHOUSE_TEST_URL` | the tests that reach TimescaleDB, the TimescaleDB store's: each creates and drops its own scratch database | `postgres://postgres:password@localhost:15432/postgres` |
| `QI_KAFKA_BROKERS` | the stream contract test against Redpanda | `localhost:29092`, also its value when unset |
| `QI_CLICKHOUSE_TEST_URL` | the tests that reach ClickHouse, the ClickHouse store's: each creates and drops its own scratch database | `http://default:password@localhost:18123` |
| `QI_MARKET_VECTORS` | the library's tests that hold its K-lines' base to market's vectors (`lib/README.md`) | the shell's, market's vectors in the input `contracts` |

```sh
QI_WAREHOUSE_TEST_URL=postgres://postgres:password@localhost:15432/postgres \
QI_KAFKA_BROKERS=localhost:29092 \
QI_CLICKHOUSE_TEST_URL=http://default:password@localhost:18123 \
GOPROXY=off go test -count=1 ./core/... ./consumers/...
```

**What `make check` reruns.** Each step runs once (the root's `Makefile`),
and Go's caches decide what it does: a package is compiled and vetted again
only when what goes into it changes (its sources, its dependencies' compiled
form, the toolchain, the build's flags and environment); a test's result
is kept by its test binary, its flags, the environment variables it
reads and the files it opens (by modification time and size: the
fixtures, `/etc/hosts`, `/etc/resolv.conf`).
`go vet`'s analyzers include every one `go test` runs, so `go test` runs
with its own vet off (`-vet=off`).

**The stack's state.** Go keeps a test's result by the URL it reads, not
by the server behind it, so `make check` sets `QI_STACK_STATE`, the state
of the test stack's TimescaleDB, Redpanda and ClickHouse by `docker
inspect` of `qitest-timescaledb`, `qitest-redpanda` and
`qitest-clickhouse`: each container's name, ID,
image's ID, status, health and start time, or docker's answer when there
is no such container. The test stack wiped and made again is a new state. A new image, a recreated container, a restart, a
change of health or the stack down is a new state, and the tests that
reach the stack run again; with the same state, Go's cache keeps their
results. The rule: every way to the stack reads `QI_STACK_STATE`, and a
test leaves nothing on a server that a later run could see. The ways:
a store's test on the core, through its own scratch helper, in a database
of its own on TimescaleDB or ClickHouse that it drops; and the stream
contract test (`core/stream/kafka`), on a topic of its own that it
deletes, failing if it cannot (Redpanda then keeps none of its consumer
group). The state is computed by `dev/stack-state.mk`, which the root's
`Makefile` includes. A test that reaches the stack another way, or leaves
something there, is changed to go through these; none does. A run
killed midway leaves its database or topic behind, under a name no later
run takes; a database left so on TimescaleDB keeps TimescaleDB's scheduler
running its policies, which hold background workers the next runs need,
so it is dropped by hand.

`make check` stops when `QI_WAREHOUSE_TEST_URL`, `QI_KAFKA_BROKERS` or
`QI_CLICKHOUSE_TEST_URL` names the stack's server (`…@localhost:5432/…`,
`localhost:19092`, `…@localhost:8123`), `FULL=1` too: the stack holds the
platform's data, and no test runs on it. It stops too when there is no
docker to read the state, and when one names a server not the test
stack's, whose state is in no key; `FULL=1` runs then, every test afresh. Not in the
key: a change on a running server without a restart (a configuration
reloaded, an object made in `template1`, Redpanda's cluster settings),
another server answering on the test stack's ports while its containers are
gone, and, `QI_KAFKA_BROKERS` unset, a broker that stops answering before
docker marks it unhealthy, the contract test's skip then kept; after any,
`FULL=1`. A `go test` run by hand reads no state, so
it takes `-count=1`, as above.

`FULL=1` runs every vet (`go vet -a`, which compiles the standard library
again) and every test afresh.

`gofmt` and the kit's check of `agents/` (`make agents-check`) read only
this repository's files and the flake's inputs, and each runs every time:
each takes under a tenth of a second, and a check of the flake would take
more, Nix's evaluation of this flake alone taking about a second.

## 5. Conventions

- **Libraries and programs.** The core's packages (`stream`, `raw`,
  `clean`) and the platform's library (`lib/`) are libraries;
  `core/cmd/provision` and each addition are programs, each a `main`
  package with flags, its logic tested beside it.
- **Standard library first.** `log/slog` for logging, `flag` for
  options, `testing` for tests. A dependency is added only when the
  standard library has nothing (franz-go for Kafka, pgx for Postgres,
  yaml).
- **Errors are values.** Wrap with context (`fmt.Errorf("bars: write %s: %w", pair, err)`);
  a program logs and exits non-zero on a fatal error.

## 6. Optional: golangci-lint

`go vet` is narrower than Biome's linter. The usual complement is
[golangci-lint](https://golangci-lint.run), which runs staticcheck,
errcheck, unused and others under one configuration. It is not part of
`make check` yet; adopting it means adding a `.golangci.yml` and
fixing its first findings.
