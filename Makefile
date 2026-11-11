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

.PHONY: help install agents agents-check run-start run-check verify locks-check products-check check check-lib check-abstract check-abstract-package go.work modules stack-up stack-down stack-test-up stack-test-stop stack-test-down

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

# The Go modules of this repository: the platform's library (lib/, lib/README.md) and the definition's interfaces
# (abstract/, abstract/README.md), which take the standard library alone. The library's go.mod requires market's Go form
# at v0.0.0, which go.work replaces with the store path of the flake's package market-go (QI_MARKET_GO, the shell's);
# chanlun's Go form, for a module that requires it at v0.0.0 (none does yet), is replaced likewise by the store path of
# the package chanlun-go (QI_CHANLUN_GO): go.work is written here, by the shell on entry and before every target that
# runs Go, and never committed. The targets run Go with GOPROXY=off, so that Go fetches nothing while it vets or tests:
# the contracts' Go forms are the flake's, and every other module a module takes is in Go's module cache already, put
# there by make modules, which each of them runs first.
MODULES := lib abstract
MODULES_PKGS := $(addprefix ./,$(addsuffix /...,$(MODULES)))
MARKET_GO := github.com/zhifengzhang-sz/qi-v2-contracts/market/go
CHANLUN_GO := github.com/zhifengzhang-sz/qi-v2-contracts/chanlun/go

go.work: ## the Go workspace at the root: the modules of MODULES, market-go at QI_MARKET_GO, chanlun-go at QI_CHANLUN_GO; written only when its text changes; never committed
	@test -n "$(QI_MARKET_GO)" && test -f "$(QI_MARKET_GO)/go.mod" || { echo "go.work: no market-go at QI_MARKET_GO ('$(QI_MARKET_GO)'): the shell gives it (flake.nix); enter it again after a change of flake.nix or flake.lock"; exit 1; }
	@test -n "$(QI_CHANLUN_GO)" && test -f "$(QI_CHANLUN_GO)/go.mod" || { echo "go.work: no chanlun-go at QI_CHANLUN_GO ('$(QI_CHANLUN_GO)'): the shell gives it (flake.nix); enter it again after a change of flake.nix or flake.lock"; exit 1; }
	@new=$$(echo "// written by make go.work from flake.lock's market-go (QI_MARKET_GO) and chanlun-go (QI_CHANLUN_GO); never committed"; \
	  echo "go 1.26.0"; echo; echo "use ("; for m in $(MODULES); do printf '\t./%s\n' $$m; done; echo ")"; echo; \
	  echo "replace $(MARKET_GO) v0.0.0 => $(QI_MARKET_GO)"; \
	  echo "replace $(CHANLUN_GO) v0.0.0 => $(QI_CHANLUN_GO)"); \
	if [ "$$new" != "$$(cat go.work 2>/dev/null)" ]; then printf '%s\n' "$$new" > go.work; echo "go.work: written, market-go at $(QI_MARKET_GO), chanlun-go at $(QI_CHANLUN_GO)"; fi

modules: go.work ## the modules the workspace's packages and their tests need, but the contracts' Go forms (go.work's), into Go's module cache (the shell's GOMODCACHE), from the Go module proxy with no login, each checked against its go.sum; a module the cache holds is not fetched again; run first by every target that runs Go
	@go list -deps -test $(MODULES_PKGS) >/dev/null

# ---------------------------------------------------------------- the checks (docs/impl/engineering/go.toolchain.md)

# Each part once, and what it reads decides its work (go.toolchain.md, 4): gofmt and the kit's check run every time;
# go vet and the tests are keyed by Go's caches. FULL=1: every vet and every test afresh.
ifneq ($(filter-out 1,$(FULL)),)
$(error FULL is 1 or unset, not $(FULL))
endif

# A module's checks, $(1) its folder: gofmt with no unformatted file, go vet, go test (its own vet off: go vet has
# run), with GOPROXY=off.
ifeq ($(FULL),1)
VET_FLAGS := -a
TEST_FLAGS := -count=1
endif
define go-check
	f=$$(gofmt -l $(1)); test -z "$$f" || { echo "gofmt: not formatted:"; echo "$$f"; exit 1; }
	GOPROXY=off go vet $(VET_FLAGS) ./$(1)/...
	GOPROXY=off go test -vet=off $(TEST_FLAGS) ./$(1)/...
endef

check: check-lib check-abstract agents-check ## every check: the library, the definition's interfaces, the agents' spec; FULL=1, every vet and every test afresh, none from Go's caches

check-lib: modules ## the library (lib/README.md): gofmt with no unformatted file, go vet, go test (its own vet off: go vet has run), GOPROXY=off
	$(call go-check,lib)

# Its test of the README's table reads the definition's Lean at QI_DEFINITION, the shell's: a shell entered before
# flake.lock held the definition has none, and the target stops there.
check-abstract: modules ## the definition's interfaces (abstract/README.md): gofmt with no unformatted file, go vet, go test, GOPROXY=off; its tests run a working space in memory and hold its README's table to the definition at QI_DEFINITION
	@test -n "$(QI_DEFINITION)" && test -f "$(QI_DEFINITION)/Wiring.lean" || { echo "check-abstract: no definition at QI_DEFINITION ('$(QI_DEFINITION)'): the shell gives it (flake.nix); enter it again after a change of flake.lock"; exit 1; }
	$(call go-check,abstract)

# The flake package abstract-level (abstract/README.md, Taking it), checked as a taker takes it: built offline from
# this flake (nix build --offline: nothing fetched; a file git does not track is not in it); what it holds, exactly
# go.mod, README.md, LICENSE and abstract/'s Go files but its tests, each the same as the work tree's; then a module
# in a temporary folder, the test of $(ABSTRACT_TAKER) with a go.mod written here that requires the module and
# replaces its path with the package's store path; go vet and go test there, GOPROXY=off and GOWORK=off, so that the
# package alone gives the module; the folder removed after.
ABSTRACT_MODULE := github.com/zhifengzhang-sz/qi-v2-platform/abstract
ABSTRACT_TAKER := dev/taker/abstract-level

check-abstract-package: ## the flake package abstract-level built offline, and taken by a module in a temporary folder whose go.mod replaces the module's path with the package's store path: gofmt, go vet and go test of dev/taker/abstract-level/'s test, which wires a working space and runs it, GOPROXY=off
	f=$$(gofmt -l $(ABSTRACT_TAKER)); test -z "$$f" || { echo "gofmt: not formatted:"; echo "$$f"; exit 1; }
	@set -eu; \
	p=$$(nix build --offline --no-link --print-out-paths .#abstract-level); \
	echo "check-abstract-package: abstract-level built offline, $$p"; \
	want=$$( { printf '%s\n' go.mod README.md LICENSE; ( cd abstract && ls *.go | grep -v '_test\.go$$' ); } | sort ); \
	got=$$( ls -A "$$p" | sort ); \
	test "$$got" = "$$want" || { echo "check-abstract-package: the package holds:"; echo "$$got"; echo "and should hold:"; echo "$$want"; exit 1; }; \
	for f in $$want; do \
	  s=abstract/$$f; test "$$f" != LICENSE || s=LICENSE; \
	  cmp -s "$$p/$$f" "$$s" || { echo "check-abstract-package: $$f in the package is not $$s"; exit 1; }; \
	done; \
	echo "check-abstract-package: the package holds $$(echo "$$want" | wc -l) files, go.mod, README.md, LICENSE and the module's Go files but its tests, each the work tree's"; \
	t=$$(mktemp -d); trap 'rm -rf "$$t"' EXIT; \
	cp $(ABSTRACT_TAKER)/*.go "$$t"/; \
	printf 'module taker\n\ngo 1.26.0\n\nrequire %s v0.0.0\n\nreplace %s v0.0.0 => %s\n' $(ABSTRACT_MODULE) $(ABSTRACT_MODULE) "$$p" > "$$t/go.mod"; \
	cd "$$t"; \
	GOWORK=off GOPROXY=off go vet $(VET_FLAGS) ./...; \
	GOWORK=off GOPROXY=off go test -vet=off $(TEST_FLAGS) ./...

# ---------------------------------------------------------------- the services stack (docs/impl/engineering/dev.environment.md, 3)

# Two stacks (dev.environment.md, 3): the stack, whose data is kept, and the test stack, for tests, beside it and
# wiped whenever a test is done. Never --remove-orphans, and -v only in stack-test-down: Docker's project `services`
# on this machine also holds another program's container, and the stack's volumes hold the data.
stack-up: ## the services stack up and healthy on the machine's Docker engine: qi-v2-services' package `stack`, at flake.lock's pin; its data is kept
	nix run --inputs-from . services#stack -- up -d --wait

stack-down: ## the services stack down; its volumes, which hold the data, stay
	nix run --inputs-from . services#stack -- down

stack-test-up: ## the test stack up and healthy, beside the stack (the package's stack-test: qitest-*, each port the stack's plus 10000, localhost:29092, 15432, 18123); for tests
	nix run --inputs-from . services#stack-test -- up -d --wait

stack-test-stop: ## the test stack down, its volumes kept (make stack-test-down wipes it)
	nix run --inputs-from . services#stack-test -- down

stack-test-down: ## the test stack wiped: its containers, network and volumes (down -v, the one target that runs it); whenever a test is done
	nix run --inputs-from . services#stack-test -- down -v
