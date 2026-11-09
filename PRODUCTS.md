# qi-v2-platform: what it owns, gives and takes

## Responsibility

The platform owns the streaming architecture and the operations that run on the stream: the core, its topics and
their schemas, and the additions on it, a publisher or a consumer each, among them acquisition, which reads a source
through its plugin, and the cleanings it runs. It owns the market data: its acquisition, its record on the core's
topics, kept forever, its cleaning, and the K-line data sets made from it, each a version fixed once released. It does
not own the infrastructure, which `qi-v2-services` publishes, the market
data contract, which is `qi-v2-contracts`', the chart library, which is `qi-v2-visualization`'s, the common tools,
which are `qi-v2-nix`'s, an engine that reads its topics and what it makes, or any consumer's code.

## Products

| Product | What | Form | Version | Release | Doc |
|---|---|---|---|---|---|
| the platform contract | the core's topics and their schemas, the programs and their flags, the plugin protocols; a user reads the topics, not the stores | `file` | `none` | `main` | `README.md` |
| the platform's library | Go packages any app can use, on market's Go form and the standard library alone: Massive's reader | `library` | `none` | `main` | `lib/README.md` |
| `qi-v2-platform-klines-massive-spy-1m` | a K-line data set: SPY's clean minutes of the validation 0.1.0, the sessions 2024-11-06 to 2026-11-06, a file a month of market 1.1.0's K-lines, an image on GHCR, one layer a file, pinned by `datasets/qi-v2-platform-klines-massive-spy-1m.lock` | `image` | `1.0.0` | `qi-v2-platform-klines-massive-spy-1m-1.0.0` | `datasets/README.md` |

## Takes

| Product | From | Pinned by |
|---|---|---|
| `qi-v2` | `qi-v2-nix` | `flake.lock` |
| `nixpkgs` | `qi-v2`, which it follows | `flake.lock` |
| `kit` | `qi-v2-nix` | `flake.lock` |
| `services` | `qi-v2-services` | `flake.lock` |
| `contracts` | `qi-v2-contracts`, its package `market-go` (market 1.1.0's Go form) and market's vectors | `flake.lock` |
| `visualization` | `qi-v2-visualization`, its packages `qi-chart` (`@qi/chart` 3.0.0) and `lightweight-charts` (Lightweight Charts 5.2.1), from which the charting consumer's page is built | `flake.lock` |
