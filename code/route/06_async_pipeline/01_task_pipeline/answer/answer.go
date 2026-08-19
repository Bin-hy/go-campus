//go:build ignore

package answer

import (
	"context"
	"errors"
	"sync"
)

type Task struct {
	ID      string
	Payload string
}

type Result struct {
	Task Task
	Err  error
}

var ErrClosed = errors.New("pipeline closed")

// Pipeline 参考答案
type Pipeline struct {
	ctx      context.Context
	jobs     chan Task
	results  chan Result
	closed   chan struct{}
	closeOne sync.Once
	wg       sync.WaitGroup
	process  func(ctx context.Context, t Task) error
	maxRetry int
}

func NewPipeline(ctx context.Context, workers, maxRetry int, process func(ctx context.Context, t Task) error) *Pipeline {
	if workers <= 0 {
		workers = 1
	}
	p := &Pipeline{
		ctx:      ctx,
		jobs:     make(chan Task, 64),
		results:  make(chan Result, 64),
		closed:   make(chan struct{}),
		process:  process,
		maxRetry: maxRetry,
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for t := range p.jobs {
				p.handle(t)
			}
		}()
	}
	return p
}

func (p *Pipeline) Submit(ctx context.Context, t Task) error {
	select {
	case <-p.closed:
		return ErrClosed
	default:
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	select {
	case p.jobs <- t:
		return nil
	case <-p.closed:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pipeline) Results() <-chan Result {
	return p.results
}

func (p *Pipeline) Close() {
	p.closeOne.Do(func() {
		close(p.closed)
		close(p.jobs)
		p.wg.Wait()
		close(p.results)
	})
}

func (p *Pipeline) handle(t Task) {
	var err error
	for attempt := 0; attempt <= p.maxRetry; attempt++ {
		if err = p.process(p.ctx, t); err == nil {
			break
		}
	}
	p.results <- Result{Task: t, Err: err}
}
