package abstract

import (
	"context"
	"fmt"
	"sync"
)

// group runs the parts of a run, each in a goroutine of its own, under one context. The first part that fails ends
// the context, which stops the others; wait returns once every part has returned, so that nothing of the run is
// left running after it, with that first error, or nil where every part returned nil.
type group struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
	err    error
}

func newGroup(ctx context.Context) *group {
	ctx, cancel := context.WithCancel(ctx)
	return &group{ctx: ctx, cancel: cancel}
}

// run starts a part; name says which it is in the error it may give.
func (g *group) run(name string, part func(ctx context.Context) error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := part(g.ctx); err != nil {
			g.once.Do(func() {
				g.err = fmt.Errorf("%s: %w", name, err)
				g.cancel()
			})
		}
	}()
}

func (g *group) wait() error {
	g.wg.Wait()
	g.cancel()
	return g.err
}

// series is a log inside acquisition, the raw series or the clean series: one writer, and a number of readers fixed
// when it is made, each given every message in order. The writer never waits for a reader: a part that reads
// nothing, or slowly, holds no other part back. For each reader the series holds the messages the reader has not
// taken yet, and no others; a reader that has returned (leave) takes no more, and nothing is held for it.
type series[T any] struct {
	mu      sync.Mutex
	pending [][]T           // for each reader, the messages it has not taken, in order
	gone    []bool          // for each reader, whether it has returned
	grown   []chan struct{} // for each reader that waits, closed when a message is there for it
}

func newSeries[T any](readers int) *series[T] {
	return &series[T]{pending: make([][]T, readers), gone: make([]bool, readers), grown: make([]chan struct{}, readers)}
}

func (s *series[T]) Append(ctx context.Context, m T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.pending {
		if s.gone[i] {
			continue
		}
		s.pending[i] = append(s.pending[i], m)
		if s.grown[i] != nil {
			close(s.grown[i])
			s.grown[i] = nil
		}
	}
	return nil
}

// cursor is reader i's reading end.
func (s *series[T]) cursor(i int) Cursor[T] { return seriesCursor[T]{of: s, reader: i} }

// leave says reader i has returned: what is held for it is dropped, and no more is.
func (s *series[T]) leave(i int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gone[i], s.pending[i] = true, nil
}

type seriesCursor[T any] struct {
	of     *series[T]
	reader int
}

func (c seriesCursor[T]) Next(ctx context.Context) (T, error) {
	s, i := c.of, c.reader
	for {
		s.mu.Lock()
		if len(s.pending[i]) > 0 {
			m := s.pending[i][0]
			var none T
			s.pending[i][0] = none
			s.pending[i] = s.pending[i][1:]
			s.mu.Unlock()
			return m, nil
		}
		if s.grown[i] == nil {
			s.grown[i] = make(chan struct{})
		}
		grown := s.grown[i]
		s.mu.Unlock()
		select {
		case <-grown:
		case <-ctx.Done():
			var none T
			return none, ctx.Err()
		}
	}
}
