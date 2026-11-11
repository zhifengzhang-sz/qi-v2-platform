# The Go toolchain

What the platform's Go modules use, and the commands a contributor runs. Most of it is the `go` command itself; Go
comes from the flake (`nix develop`), golangci-lint is optional and not in it. The modules: those the root `Makefile`
names in `MODULES`, one workspace (2).

## 1. The tools

| Tool | For | Notes |
|---|---|---|
| the **`go` command**: `go build`, `go run`, `go mod` | building, running, the dependencies | dependencies are declared in each module's `go.mod` and pinned by its `go.sum`; modules live in the shell's `GOMODCACHE`, under `~/.cache/platform/` |
| **`gofmt`** (ships with Go) | format | no configuration exists or is wanted: every Go codebase is formatted the same way |
| **`go vet`** (ships with Go) | lint | narrow on purpose: bad `Printf` formats, copied locks, unreachable code, misuse of the standard library |
| **`go test`** and the standard `testing` package | tests | tests are `*_test.go` files beside the code they test; no test framework is added |
| the root **`Makefile`** | the targets | `make check` |

## 2. Installing

Go is the flake's (`nix develop`; its version: `dev.environment.md`, 2).

**The modules.** Each is a module of its own in a folder of its own, named once, in the root `Makefile`'s `MODULES`;
what each takes is in its `go.mod`. Today there are two: the platform's library (`lib/README.md`), which takes market
alone, and the definition's interfaces (`abstract/README.md`), which take the standard library alone. They are one
workspace, `go.work` at the root, which `make go.work` writes from the shell's `QI_MARKET_GO` (the
shell writes it on entry too) and git never holds: it uses the modules of `MODULES` and replaces market's Go form,
which a `go.mod` requires at `v0.0.0`, by the store path of the flake's package `market-go` (the input `contracts`,
`qi-v2-contracts` at `market-1.1.0`); `go.sum` has no line for it. A module does not require another module of the
workspace in its `go.mod`: the workspace gives it, and a requirement of a workspace module at a version sends Go to
the module proxy for it. Every target that runs Go on them runs `make modules` first, which puts in Go's module cache
each module their packages and their tests need but market's Go form, from the Go module proxy with no login, each
checked against its `go.sum`, and fetches nothing the cache holds; Go then runs with `GOPROXY=off`, so that it
fetches nothing while it vets or tests.

`make check-lib` and `make check-abstract` run this file's checks, each on its own module, from the root's `Makefile`,
with `GOPROXY=off`; `make check` runs both.

## 3. Daily commands

Run from the root, in the shell:

```sh
make check                    # gofmt (fails on any unformatted file), go vet, go test (what each reruns: 4)
make check FULL=1             # the same, every vet and every test afresh, none from Go's caches
gofmt -w lib                                     # format everything
GOPROXY=off go vet ./lib/...                     # lint one module
GOPROXY=off go test -count=1 ./lib/...           # one module's tests, afresh (a test's result is cached otherwise)
GOPROXY=off go test -count=1 -v ./lib/...        # verbose
GOPROXY=off go test -count=1 -run TestVectors ./lib/massive/   # one test by name (a regular expression)
GOPROXY=off go test -race ./lib/...              # the race detector: data races between goroutines
GOPROXY=off go test -cover ./lib/...             # coverage per package
```

## 4. What the tests read, and what `make check` reruns

| Variable | Tests | Value |
|---|---|---|
| `QI_MARKET_VECTORS` | the library's tests that hold its K-lines' base to market's vectors (`lib/README.md`) | the shell's, market's vectors in the input `contracts` |
| `QI_DEFINITION` | the test of `abstract/` that holds its README's table to the definition's declarations (`abstract/README.md`); `make check-abstract` stops where it is not set | the shell's, the definition's package, the input `definition` |

No test reaches a stack today. A test that does runs on the test stack (`make stack-test-up`, `dev.environment.md`,
3), never on the stack, whose data is kept; the test stack is wiped when the tests are done (`make stack-test-down`).

**What `make check` reruns.** Each step runs once (the root's `Makefile`), and Go's caches decide what it does: a
package is compiled and vetted again only when what goes into it changes (its sources, its dependencies' compiled
form, the toolchain, the build's flags and environment); a test's result is kept by its test binary, its flags, the
environment variables it reads and the files it opens (by modification time and size). `go vet`'s analyzers include
every one `go test` runs, so `go test` runs with its own vet off (`-vet=off`).

`FULL=1` runs every vet (`go vet -a`, which compiles the standard library again) and every test afresh.

`gofmt` and the kit's check of `agents/` (`make agents-check`) read only this repository's files and the flake's
inputs, and each runs every time: each takes under a tenth of a second.

## 5. Conventions

- **Libraries and programs.** The platform's library (`lib/`) and the definition's interfaces (`abstract/`) are
  libraries; a program is a `main` package with flags,
  its logic tested beside it.
- **Standard library first.** `log/slog` for logging, `flag` for options, `testing` for tests. A dependency is added
  only when the standard library has nothing.
- **Errors are values.** Wrap with context (`fmt.Errorf("bars: write %s: %w", pair, err)`); a program logs and exits
  non-zero on a fatal error.

## 6. Optional: golangci-lint

`go vet` is narrow. The usual complement is [golangci-lint](https://golangci-lint.run), which runs staticcheck,
errcheck, unused and others under one configuration. It is not part of `make check` yet; adopting it means adding a
`.golangci.yml` and fixing its first findings.
