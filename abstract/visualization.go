package abstract

import "context"

// A Chart is visualization: a pure consumer. It reads the clean topics it is given and the market model engine's
// topics, all of them, of which there are none without the engine; it writes no topic. What it gives is the
// updates of the chart in the HTML page, messages View, which the definition leaves open: a log that leaves the
// system, the chart on the page at any moment being what the updates so far make.
type Chart[View any] interface {
	Draw(ctx context.Context, clean ReadSet, model ReadSet, chart Appender[View]) error
}
