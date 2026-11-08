Good question. The landscape shifted in 2025. Here's where things stand:

**Kafka 4.0** (released March 2025) completed the full removal of ZooKeeper. The new metadata system, KRaft, is now the only way to run Kafka. That was Redpanda's original pitch — "we don't need ZooKeeper and Kafka does" — and it's no longer valid.

**But Redpanda still leads on what matters for us.** Redpanda broker restarts happen in under one second, compared to Kafka restarts measured in minutes. And the core advantage remains: a single binary with no external dependencies, built from scratch in C++ with a thread-per-core architecture that eliminates garbage collection pauses.

**For this programme specifically**, the throughput battle is irrelevant — our event rate is bounded by the K-line rate (at most 1 bar per minute for M1 data, ~10 events per bar across the pipeline). We're not doing GB/s. What matters is:

1. **Tail latency.** A provisional trading signal needs to reach its consumer fast. Redpanda claims significantly lower latencies, up to 10x faster P99.9 tail latencies. At our throughput, both platforms are fast, but Redpanda's deterministic latency (no GC pauses) is the right property for trading signals.

2. **Operational simplicity.** Single binary, built-in Schema Registry, HTTP proxy, and message broker, eliminating dependencies on JVM, ZooKeeper, or KRaft. We don't want to run a separate Confluent Schema Registry.

3. **Kafka API compatibility.** Existing Kafka producers, consumers, and tools can typically be used with Redpanda without code changes. Our Rust/TypeScript Kafka clients work unchanged. If we ever need to switch to Kafka, it's a broker address change.

4. **The honest counterargument.** Kafka 4.0 closed the operational complexity gap that Redpanda was originally built to exploit. And Kafka's ecosystem is deeper — Kafka Streams, Kafka Connect, Flink integration are more mature on real Kafka.

**Other alternatives considered:**

- **Apache Pulsar** — multi-tenancy and geo-replication are overkill for us. More complex to operate (separate BookKeeper layer).
- **NATS JetStream** — lightweight and fast, but smaller ecosystem. No native Schema Registry. Would work for our scale but we'd lose the Kafka ecosystem compatibility.
- **Confluent Cloud** — managed Kafka. Good if we want zero ops, but vendor lock-in and cost at the cloud tier.

**Decision: Redpanda.**

For our use case (low throughput, latency-sensitive signals, small team, need built-in Schema Registry, want Kafka API compatibility as an exit path), Redpanda is the right choice. The operational simplicity alone justifies it — one Docker container vs Kafka + KRaft configuration.
