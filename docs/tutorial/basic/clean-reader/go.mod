// clean-reader, the tutorial's first reader (../README.md), on franz-go, its admin client and market's Go form alone:
// no package of the platform's. market's Go form is required at v0.0.0 and replaced in go.work by the flake's package
// market-go; franz-go and its admin client at the core's versions.
module github.com/zhifengzhang-sz/qi-v2-platform/docs/tutorial/basic/clean-reader

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
