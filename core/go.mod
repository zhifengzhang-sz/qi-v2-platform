// The core (README.md): the stream, the provisioning of the raw market data topic and the raw market data schema.
// market's Go form is required at v0.0.0 and replaced by the store path of the flake's package market-go in go.work
// (make go.work), never by a version of Go's module proxy or a path beside this repository.
module github.com/zhifengzhang-sz/qi-v2-platform/core

go 1.26.0

require (
	github.com/twmb/franz-go v1.22.0
	github.com/twmb/franz-go/pkg/kadm v1.19.0
	github.com/zhifengzhang-sz/qi-v2-contracts/market/go v0.0.0
)

require (
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.30 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.14.0 // indirect
)
