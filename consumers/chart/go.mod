// chart, the charting consumer, on the core (../../core/README.md): the core's stream and clean market data schema, and
// the standard library. The core is a module of the workspace (go.work, make go.work), which Go takes for it, and is not
// required here: a requirement of a workspace module at a version sends Go to the module proxy for it. market's Go form
// is required at v0.0.0 and replaced in go.work by the flake's package market-go.
module github.com/zhifengzhang-sz/qi-v2-platform/consumers/chart

go 1.26.0

require github.com/zhifengzhang-sz/qi-v2-contracts/market/go v0.0.0

require (
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.30 // indirect
	github.com/twmb/franz-go v1.22.0 // indirect
	github.com/twmb/franz-go/pkg/kadm v1.19.0 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.14.0 // indirect
)
