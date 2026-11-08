// massive-reader, Massive's reader, a source plugin (../../docs/impl/engineering/plugins.md, 3), a thin program over
// the platform's library (../../lib/README.md), its package lib/massive; it imports nothing else of the platform. The
// library is a module of the workspace (go.work, make go.work), which Go takes for it, and is not required here: a
// requirement of a workspace module at a version sends Go to the module proxy for it. market's Go form is required at
// v0.0.0 and replaced in go.work by the flake's package market-go.
module github.com/zhifengzhang-sz/qi-v2-platform/plugins/massive-reader

go 1.26.0

require github.com/zhifengzhang-sz/qi-v2-contracts/market/go v0.0.0
