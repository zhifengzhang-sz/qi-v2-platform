# The tutorial: a component of your own on the core

A component is added to the platform without changing it, through the core's topics and their schemas
(`../../core/README.md`): it reads `market.raw`, `market.corrections` or a clean topic, and a cleaning of its own is
run by acquisition by the cleaning's protocol (`../impl/engineering/plugins.md`, 3). The tutorial teaches it in three
levels, in this order, each a folder of its page and its programs:

| Level | What it teaches | Its programs |
|---|---|---|
| [`basic/`](basic/README.md) | reading the platform: the test stack filled from Massive by `make start`; the core's topics read with `query`, what each is, the zero K-line, a correction, the clean topic and its index; then a reader of your own reading the clean topic from its start to its end, in no group, on the contracts alone | `clean-reader` |
| [`medium/`](medium/README.md) | a consumer on the core: the clean topic followed in the group of its name, each K-line kept before its offset is committed, a stop and a start again continuing from the committed offset, a copy dropped | `clean-follower` |
| [`advanced/`](advanced/README.md) | a cleaning of your own: tried by hand, lines in and answers out; its clean topic made by acquisition's first reset; corrections of your own, up to its tolerance and past it, and the reset; the reset held off while a reader of the topic runs, made once it is stopped, the reader reading the topic made again from its start | `despike`, `correct-high` |

Not in it: a component publishing a topic of its own, an engine of your own, a source plugin (its protocol:
`plugins.md`, 3).

**On the test stack.** Every example runs on the test stack (`make stack-test-up`, every port the stack's plus
10000: Redpanda at `localhost:29092`, TimescaleDB at `15432`, ClickHouse at `18123`;
`../impl/engineering/dev.environment.md`, 3), never on the stack, which holds the platform's data: `market.raw` keeps
every message forever, so nothing a trial writes there could be taken out again. Each program here that reads or
writes a topic takes the test stack's broker by default, and the one that writes, `correct-high`, refuses any other.
When you are done, `make stack-test-down` wipes the test stack.

**In Go, or any language.** The programs are Go, because the platform's shell holds Go and franz-go's modules and no
Kafka client for Python. Any language with a Kafka client reads the same topics by `core/README.md`'s schemas: each
value is one JSON object in canonical form, its K-line market 1.1.0's base K-line and the source's own fields.

**What you need.** The machine (`../process.md`, The machine), the repository, its shell (`nix develop`), and a key
of your own for Massive in `~/.config/qi/massive_key`, one line; Massive's free plan is enough. Every command is given
whole, as you type it from the repository's root, in the shell, one terminal throughout. Each output is as it came on
the test stack on 2026-11-08, its last session 2026-11-06; yours differs in its times, in its temporary folder, and in
the days after 2026-11-06, a run giving every minute up to the last session that is 20 h old.

**The programs.** Each is a Go module of its own folder (`basic/clean-reader`, `medium/clean-follower`,
`advanced/despike`, `advanced/correct-high`), in the workspace (`go.work`, found by its `go.mod`), built into `bin/` by
the page's `go build`, and built and vetted by `make check` (`check-tutorial`); removing its folder removes it.
