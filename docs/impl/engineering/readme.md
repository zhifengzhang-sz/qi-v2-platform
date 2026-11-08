## Engineering Documentation

These are the authoritative engineering specifications.

### Development

- [Development Environment](dev.environment.md) — repository layout, tool versions, the qi-v2-services stack and its test stack, `make start`, platform setup, tests
- [The Core](../../../core/README.md) — the core, a Go module of its own: the stream, the provisioning of `market.raw` and `market.corrections`, the raw market data schema 0.3.0, the market corrections schema 0.1.0, the clean market data schema 0.1.0 and the topics' names; the additions on it (acquisition, the printer, `query`, the stores, Massive's reader, the validation)
- [Plugins](plugins.md) — extending the platform without changing it: source plugins and cleanings run by acquisition, the catalog, readers of the core's topics; the rules; what is proposed (a source plugin's live stream). Tutorial, on the core: [`../../tutorial/README.md`](../../tutorial/README.md)
- [Go Toolchain](go.toolchain.md) — the Go counterparts of bun, Biome and Vitest (`go`, `gofmt`, `go vet`, `go test`, the Makefile), the modules and their workspace, daily commands, tests that need the services stack, conventions

### Kafka

- [Carrier](kafka/carrier.md) — Redpanda vs Kafka evaluation and decision
