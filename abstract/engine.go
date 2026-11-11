package abstract

import "context"

// A Worker is the market model engine's worker: it reads the clean topics it is given and writes the engine's own
// topics, which are the topics it is given to append to. What their messages are is left open: each topic's schema
// fixes it.
type Worker interface {
	Work(ctx context.Context, clean ReadSet, own AppendSet) error
}

// Engine is the market model engine in a working space, which may have none: the clean topics it reads, the topics
// it writes, and its worker.
type Engine struct {
	// Clean is the topics the worker reads: each one of acquisition's clean topics, or the working space is refused.
	Clean []AnyTopic
	// Topics is the engine's own topics, each with its logs: no two of one name, and none of the name of one of
	// acquisition's.
	Topics []TopicLogs
	Worker Worker
}
