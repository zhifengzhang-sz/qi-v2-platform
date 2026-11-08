package raw

import (
	"context"
	"fmt"
	"slices"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// Spec is a core topic as provisioning makes it, market.raw and market.corrections alike: Partitions partitions, every
// record kept, deleted never compacted, each record's timestamp its publisher's.
func Spec(topic string, replicas int16) stream.TopicSpec {
	return stream.TopicSpec{Name: topic, Partitions: Partitions, Replication: replicas, RetentionMS: -1,
		CleanupPolicy: "delete", TimestampType: TimestampType}
}

// configs is a core topic's configuration as the schemas have it, each set on
// the topic itself, none left to the cluster's defaults: every record kept,
// by time and by size; deleted, never compacted; each record's timestamp
// its publisher's.
var configs = map[string]string{"retention.ms": Unlimited, "retention.bytes": Unlimited, "cleanup.policy": "delete",
	"message.timestamp.type": TimestampType}

// Provisioned is what Provision did to a topic: made, or, on the topic
// already there, each configuration whose value it changed, to the value it
// had.
type Provisioned struct {
	Topic   string
	Created bool
	Set     map[string]string
}

// Provision provisions each of the core's topics (Topics), in order, and gives what it did to each.
func Provision(ctx context.Context, adm stream.Admin, replicas int16) ([]Provisioned, error) {
	var out []Provisioned
	for _, t := range Topics {
		p, err := ProvisionTopic(ctx, adm, Spec(t, replicas))
		if err != nil {
			return out, err
		}
		out = append(out, p)
	}
	return out, nil
}

// ProvisionTopic makes a topic of the core's schemas by its spec when it is
// not there (its partitions the spec's), and sets the schemas'
// configuration on the topic, made or already there, every record kept,
// deleted never compacted, each record's timestamp its publisher's. It
// refuses a topic of another partition count, or one compacted or stamped
// by the broker, which a setting cannot mend: a message's partition is its
// schema's, a compacted topic may have lost records, and a record stamped
// by the broker has lost its bar's date.
func ProvisionTopic(ctx context.Context, adm stream.Admin, spec stream.TopicSpec) (Provisioned, error) {
	topic := spec.Name
	p := Provisioned{Topic: topic}
	names, err := adm.List(ctx)
	if err != nil {
		return p, fmt.Errorf("raw: provision: %w", err)
	}
	p.Created = !slices.Contains(names, topic)
	if err := adm.Ensure(ctx, spec); err != nil {
		return p, fmt.Errorf("raw: provision: %w", err)
	}
	n, err := adm.Partitions(ctx, topic)
	if err != nil {
		return p, fmt.Errorf("raw: provision: %w", err)
	}
	if n[topic] != spec.Partitions {
		return p, fmt.Errorf("raw: provision: %s has %d partitions, the schema %d", topic, n[topic], spec.Partitions)
	}
	c, err := adm.Configs(ctx, topic)
	if err != nil {
		return p, fmt.Errorf("raw: provision: %w", err)
	}
	if c["cleanup.policy"] != "delete" || c["message.timestamp.type"] != TimestampType {
		return p, fmt.Errorf("raw: provision: %s has cleanup.policy %q and message.timestamp.type %q, the schema delete and %s",
			topic, c["cleanup.policy"], c["message.timestamp.type"], TimestampType)
	}
	for k, v := range configs {
		if c[k] != v && !p.Created {
			if p.Set == nil {
				p.Set = map[string]string{}
			}
			p.Set[k] = c[k]
		}
	}
	if err := adm.Alter(ctx, topic, configs); err != nil {
		return p, fmt.Errorf("raw: provision: %w", err)
	}
	if c, err = adm.Configs(ctx, topic); err != nil {
		return p, fmt.Errorf("raw: provision: %w", err)
	}
	for k, v := range configs {
		if c[k] != v {
			return p, fmt.Errorf("raw: provision: %s has %s %q after it was set, the schema %s", topic, k, c[k], v)
		}
	}
	return p, nil
}
