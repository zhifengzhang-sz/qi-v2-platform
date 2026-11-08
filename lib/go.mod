// lib, the platform's library (README.md): packages any app can use, the platform's or a user's. It takes only
// market's Go form and the standard library: market is required at v0.0.0 and replaced by the store path of the
// flake's package market-go in go.work (make go.work), never by a version of Go's module proxy.
module github.com/zhifengzhang-sz/qi-v2-platform/lib

go 1.26.0

require github.com/zhifengzhang-sz/qi-v2-contracts/market/go v0.0.0
