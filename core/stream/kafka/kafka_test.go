package kafka_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/kafka"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/streamtest"
)

// The Kafka contract test needs a broker. It reads QI_KAFKA_BROKERS
// (comma-separated) and defaults to the test stack's Redpanda on
// localhost:29092 (qi-v2-services' test stack, make stack-test-up), never
// the stack's, which holds the platform's data; when no broker answers
// within 5 s it fails if the
// variable is set and is skipped if not. It reads QI_STACK_STATE, which
// make check sets, so Go's test cache keeps its result by the broker's
// state as well (go.toolchain.md, 4).
func TestContract(t *testing.T) {
	_ = os.Getenv("QI_STACK_STATE")
	brokers := os.Getenv("QI_KAFKA_BROKERS")
	set := brokers != ""
	if !set {
		brokers = "localhost:29092"
	}
	cfg := kafka.Config{Brokers: strings.Split(brokers, ","), ClientID: "core-test"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := kafka.WaitReady(ctx, cfg, 5*time.Second); err != nil {
		if set {
			t.Fatalf("no broker at QI_KAFKA_BROKERS=%s: %v", brokers, err)
		}
		t.Skipf("no broker at %s: %v", brokers, err)
	}
	streamtest.Run(t, kafka.New(cfg), "core-contract-kafka")
}
