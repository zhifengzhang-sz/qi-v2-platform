# qi-v2-platform (README.md): make targets, run inside `nix develop` (flake.nix, docs/process.md), which gives every
# tool they call: Go and its caches, git, gh, and the kit at dev/agents (make install); of the host, only what
# flake.nix's hostTools lets in (Nix, the Docker client). Outside it, make stops: no tool is taken from anywhere
# else.
ifneq ($(PLATFORM_SHELL),1)
$(error run make inside this project's shell: nix develop, then make …, or nix develop -c make … (docs/process.md))
endif

# The recipes run in the flake's bash, not the host's /bin/sh.
SHELL := $(shell command -v bash)
ifeq ($(SHELL),)
$(error no bash in this project's shell)
endif

# The scripts (dev/scripts/), run by the flake's Python.
S  := dev/scripts
PY := python3
AGENT_KIND ?= all

.PHONY: help install agents agents-check run-start run-check verify locks-check products-check check check-core check-tutorial go.work modules build start stack-up stack-down stack-test-up stack-test-stop stack-test-down

help: ## the targets
	@grep -E '^[a-z.-]+:.*##' $(MAKEFILE_LIST) | sed 's/:.*## /\t/' | expand -t 18

# ---------------------------------------------------------------- the project set up (docs/process.md, The machine)

install: ## the project set up on this machine: git's hooks at dev/hooks (the pre-commit runs make verify); the kit linked at dev/agents from flake.lock's pin, then make agents
	git config core.hooksPath dev/hooks
	nix build "$(KIT)" -o dev/agents
	$(MAKE) agents

# The kit: the flake kit of the public qi-v2-nix, at the commit flake.lock pins (nix build --inputs-from drops its
# ?dir=, so its reference is read from the lock); read when install runs, not by every other target.
KIT = $(shell $(PY) -c "import json; n = json.load(open('flake.lock'))['nodes']['kit']['locked']; print(f\"{n['type']}:{n['owner']}/{n['repo']}/{n['rev']}?dir={n['dir']}\")")

# The agents (agents/README.md): the spec in agents/, each kind's files (AGENTS.md, CLAUDE.md, .claude/agents/,
# .github/agents/) written from it by the kit, ignored by git.
agents: ## each kind's agent files from agents/ (AGENTS.md, CLAUDE.md, .claude/agents/, .github/agents/), by the kit at dev/agents; AGENT_KIND=claude-code|copilot|all (default all); after any change to agents/
	@test -x dev/agents/bin/kit || { echo "agents: no kit at dev/agents: make install"; exit 1; }
	dev/agents/bin/kit generate --kind $(AGENT_KIND) --root .

agents-check: ## agents/ decoded by the kit, nothing written
	@test -x dev/agents/bin/kit || { echo "agents-check: no kit at dev/agents: make install"; exit 1; }
	dev/agents/bin/kit check --root .

# A role's run checked on its output, where the agent's kind cannot restrict its tools: run-start before the run,
# run-check after it (the kit's README).
run-start: ## before a role's run: the work tree recorded; make run-start ROLE=reviewer
	@test -x dev/agents/bin/kit || { echo "run-start: no kit at dev/agents: make install"; exit 1; }
	@test -n "$(ROLE)" || { echo "run-start: ROLE=<role> is needed"; exit 1; }
	dev/agents/bin/kit run-start --role $(ROLE) --root .

run-check: ## after it: what changed against the role's writes (none: nothing; brief-files: only FILES=a,b; else reported); make run-check ROLE=writer FILES=docs/x.md
	@test -x dev/agents/bin/kit || { echo "run-check: no kit at dev/agents: make install"; exit 1; }
	@test -n "$(ROLE)" || { echo "run-check: ROLE=<role> is needed"; exit 1; }
	dev/agents/bin/kit run-check --role $(ROLE) $(if $(FILES),--files $(FILES)) --root .

verify: ## the checks a commit requires (dev/scripts/verify.py): the targets that check the staged paths, one after another; VERIFY=--all for every one; the pre-commit hook runs it
	$(PY) $(S)/verify.py $(VERIFY)

locks-check: ## whether each lock is outdated (docs/process.md, A session): each input of flake.lock against its source (the kit's kit locks), the project's one lock; a line a lock, current or outdated with what updates it; changes nothing
	@dev/agents/bin/kit locks --root .

products-check: ## whether PRODUCTS.md states what the project has (the kit's kit products)
	@test -x dev/agents/bin/kit || { echo "products-check: no kit at dev/agents: make install"; exit 1; }
	dev/agents/bin/kit products --root .

# ---------------------------------------------------------------- the Go workspace (docs/impl/engineering/go.toolchain.md, 2)

# The Go modules of this repository: the core, the platform's library (lib/, lib/README.md) and the core's additions.
# Every go.mod of them requires market's Go form at v0.0.0, which go.work replaces with the store path of the flake's
# package market-go (QI_MARKET_GO, the shell's): go.work is written here, by the shell on entry and before every
# target that runs Go on them, and never committed. Their targets run Go with GOPROXY=off, so that Go fetches nothing
# while it vets, tests or builds: market's Go form is the flake's, and every other module they take is in Go's module
# cache already, put there by make modules, which each of them runs first.
CORE := core lib publishers/acquisition consumers/printer consumers/timescaledb-writer consumers/clickhouse-writer consumers/query plugins/massive-reader plugins/validation
CORE_PKGS := $(addprefix ./,$(addsuffix /...,$(CORE)))
# The tutorial's programs (docs/tutorial/README.md), each a module of its own folder under its level's, found by its
# go.mod, so that removing its folder removes it; built by the pages' own commands, not by make build.
TUTORIAL := $(patsubst %/go.mod,%,$(sort $(wildcard docs/tutorial/*/*/go.mod)))
TUTORIAL_PKGS := $(addprefix ./,$(addsuffix /...,$(TUTORIAL)))
MARKET_GO := github.com/zhifengzhang-sz/qi-v2-contracts/market/go

go.work: ## the Go workspace at the root: the core, the library, the additions and the tutorial's programs, market-go at QI_MARKET_GO; written only when its text changes; never committed
	@test -n "$(QI_MARKET_GO)" && test -f "$(QI_MARKET_GO)/go.mod" || { echo "go.work: no market-go at QI_MARKET_GO ('$(QI_MARKET_GO)'): the shell gives it (flake.nix); enter it again after a change of flake.lock"; exit 1; }
	@new=$$(echo "// written by make go.work from flake.lock's market-go (QI_MARKET_GO); never committed"; \
	  echo "go 1.26.0"; echo; echo "use ("; for m in $(CORE) $(TUTORIAL); do printf '\t./%s\n' $$m; done; echo ")"; echo; \
	  echo "replace $(MARKET_GO) v0.0.0 => $(QI_MARKET_GO)"); \
	if [ "$$new" != "$$(cat go.work 2>/dev/null)" ]; then printf '%s\n' "$$new" > go.work; echo "go.work: written, market-go at $(QI_MARKET_GO)"; fi

modules: go.work ## the modules the workspace's packages and their tests need, but market's Go form (go.work's), into Go's module cache (the shell's GOMODCACHE), from the Go module proxy with no login, each checked against its go.sum; a module the cache holds is not fetched again; run first by every target that runs Go
	@go list -deps -test $(CORE_PKGS) $(TUTORIAL_PKGS) >/dev/null

# ---------------------------------------------------------------- the checks (docs/impl/engineering/go.toolchain.md)

# Each part once, and what it reads decides its work (go.toolchain.md, 4): gofmt and the kit's check run every time,
# each in under a tenth of a second, less than asking Nix for a check of the flake takes; go vet and the tests are
# keyed by Go's caches, the tests that reach the test stack by its state too (QI_STACK_STATE, dev/stack-state.mk).
check: check-core check-tutorial agents-check ## every check: the core, the library and the additions, the tutorial's programs, the agents' spec; FULL=1, every vet and every test afresh, none from Go's caches

# The stack's state, QI_STACK_STATE, for the tests that reach the test stack (dev/stack-state.mk).
STATE_GOALS := check check-core
include dev/stack-state.mk

check-core: modules ## the core, the library and the additions (core/README.md, lib/README.md): gofmt with no unformatted file, go vet, go test (its own vet off: go vet has run), GOPROXY=off; the tests that reach the test stack (the stream contract test, the stores') kept by its state (QI_STACK_STATE), each that needs a server skipping and saying so without it (QI_WAREHOUSE_TEST_URL, QI_KAFKA_BROKERS, QI_CLICKHOUSE_TEST_URL: go.toolchain.md, 4)
	f=$$(gofmt -l $(CORE)); test -z "$$f" || { echo "gofmt: not formatted:"; echo "$$f"; exit 1; }
ifeq ($(FULL),1)
	GOPROXY=off go vet -a $(CORE_PKGS)
	GOPROXY=off go test -vet=off -count=1 $(CORE_PKGS)
else
	GOPROXY=off go vet $(CORE_PKGS)
	GOPROXY=off go test -vet=off $(CORE_PKGS)
endif

check-tutorial: modules ## the tutorial's programs (docs/tutorial/README.md), as check-core the core: gofmt, go vet, go build (its output discarded), go test (its own vet off), GOPROXY=off
	f=$$(gofmt -l $(TUTORIAL)); test -z "$$f" || { echo "gofmt: not formatted:"; echo "$$f"; exit 1; }
ifeq ($(FULL),1)
	GOPROXY=off go vet -a $(TUTORIAL_PKGS)
	GOPROXY=off go build -a $(TUTORIAL_PKGS)
	GOPROXY=off go test -vet=off -count=1 $(TUTORIAL_PKGS)
else
	GOPROXY=off go vet $(TUTORIAL_PKGS)
	GOPROXY=off go build $(TUTORIAL_PKGS)
	GOPROXY=off go test -vet=off $(TUTORIAL_PKGS)
endif

build: modules ## the core's and its additions' programs built into bin/ (ignored by git): provision, and each addition of CORE, a program by its folder's name (the library none)
	GOPROXY=off go build -o bin/ ./core/cmd/provision $(addprefix ./,$(filter-out core lib,$(CORE)))

# ---------------------------------------------------------------- the services stack (docs/impl/engineering/dev.environment.md, 3)

# Two stacks (dev.environment.md, 3): the stack, which holds the platform's data, and the test stack, for tests, beside
# it and wiped whenever a test is done. Never --remove-orphans, and -v only in stack-test-down: Docker's project
# `services` on this machine also holds another program's container, and the stack's volumes hold the data.
stack-up: ## the services stack up and healthy on the machine's Docker engine: qi-v2-services' package `stack`, at flake.lock's pin; it holds the platform's data
	nix run --inputs-from . services#stack -- up -d --wait

stack-down: ## the services stack down, the platform's components on it stopped first (dev/scripts/run.py); its volumes, which hold the data, stay; make start continues
	$(PY) $(S)/run.py stop stack
	nix run --inputs-from . services#stack -- down

stack-test-up: ## the test stack up and healthy, beside the stack (the package's stack-test: qitest-*, each port the stack's plus 10000, localhost:29092, 15432, 18123); for tests
	nix run --inputs-from . services#stack-test -- up -d --wait

stack-test-stop: ## the test stack down, its components stopped first, its volumes kept (make start STACK=test continues; make stack-test-down wipes it)
	$(PY) $(S)/run.py stop test
	nix run --inputs-from . services#stack-test -- down

stack-test-down: ## the test stack wiped, its components stopped first: its containers, network and volumes (down -v, the one target that runs it); whenever a test is done
	$(PY) $(S)/run.py stop test
	nix run --inputs-from . services#stack-test -- down -v

# ---------------------------------------------------------------- the platform running (dev.environment.md, 3)

# One start: the stack up, the core's programs built, the core's topics provisioned, then acquisition and both stores'
# writers, each a process of its own that keeps running while the stack is up (dev/scripts/run.py); a stack's down
# stops them first, and a start continues each from where it stopped.
STACK ?= stack
start: modules ## the stack up (STACK=test: the test stack, CATALOG= a catalog of the run's own), the core's programs built, its topics provisioned, then acquisition (-follow) and both stores' writers in the background, each from where it stopped; logs and pids in ~/.local/state/qi/run/STACK/
	@case "$(STACK)" in stack|test) ;; *) echo "start: STACK=stack or STACK=test, not $(STACK)"; exit 1 ;; esac
	$(MAKE) $(if $(filter test,$(STACK)),stack-test-up,stack-up)
	GOPROXY=off go build -o bin/ ./core/cmd/provision $(addprefix ./,$(filter-out core lib,$(CORE)))
	CATALOG="$(CATALOG)" DATASET="$(DATASET)" $(PY) $(S)/run.py start $(STACK)
