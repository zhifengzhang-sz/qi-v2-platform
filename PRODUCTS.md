# qi-v2-platform: what it owns, gives and takes

## Responsibility

The platform owns the working space the platform's definition defines (`qi-v2-platform-definition`), in Go: the
definition's interfaces in code, the topics' schemas, the binding to Kafka, acquisition, warehousing, visualization and
their wiring; and the platform's library, what reads a source, as packages any app can use. It is built on the
definition from the ground up, one product at a time; the library stands, and each other product has its row here once
it is built. It does not own the definition, which is `qi-v2-platform-definition`'s, the infrastructure, which
`qi-v2-services` publishes, the market data contract, which is `qi-v2-contracts`', the common tools, which are
`qi-v2-nix`'s, the market model engine and what it makes, or any consumer's code.

## Products

| Product | What | Form | Version | Release | Doc |
|---|---|---|---|---|---|
| the platform's library | Go packages any app can use, on market's Go form and the standard library alone: Massive's reader | `library` | `none` | `main` | `lib/README.md` |
| `abstract-level` | the definition's interfaces in Go, on the standard library alone: the open parts of the working space's components as interfaces; a topic's shape, the three kinds of data and the wiring of a working space built in | `library` | `none` | `main` | `abstract/README.md` |

## Takes

| Product | From | Pinned by |
|---|---|---|
| `qi-v2` | `qi-v2-nix` | `flake.lock` |
| `nixpkgs` | `qi-v2`, which it follows | `flake.lock` |
| `kit` | `qi-v2-nix` | `flake.lock` |
| `services` | `qi-v2-services`, its package `stack` (0.3.0): the stack and the test stack | `flake.lock` |
| `contracts` | `qi-v2-contracts`, its package `market-go` (market 1.1.0's Go form) and market's vectors | `flake.lock` |
| `definition` | `qi-v2-platform-definition`, its package `qi-v2-platform-definition` (0.1.0): the platform's formal definition, its Lean as source | `flake.lock` |
