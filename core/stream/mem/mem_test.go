package mem_test

import (
	"testing"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/mem"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/streamtest"
)

func TestContract(t *testing.T) {
	streamtest.Run(t, mem.New(), "core-contract-mem")
}
