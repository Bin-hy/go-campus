//go:build ignore

package answer

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var ErrNoInstance = errors.New("no available instance")

type Registry interface {
	Discover(ctx context.Context) ([]string, error)
}

type Transport func(ctx context.Context, addr, method string, req any) (any, error)

type Options struct {
	Timeout       time.Duration
	MaxRetry      int
	BackoffBase   time.Duration
	FailThreshold int
	Cooldown      time.Duration
}

const (
	stateClosed   = 0
	stateOpen     = 1
	stateHalfOpen = 2
)

type breaker struct {
	mu       sync.Mutex
	state    int
	failures int
	openAt   time.Time
}

// Caller 参考答案：轮询负载均衡 + 超时 + 指数退避重试 + 熔断三态
type Caller struct {
	registry  Registry
	transport Transport
	opts      Options
	rr        atomic.Uint64
	breakers  sync.Map
}

func NewCaller(registry Registry, transport Transport, opts Options) *Caller {
	return &Caller{registry: registry, transport: transport, opts: opts}
}

func (c *Caller) Call(ctx context.Context, method string, req any) (any, error) {
	var lastErr error
	for attempt := 0; attempt <= c.opts.MaxRetry; attempt++ {
		addrs, err := c.registry.Discover(ctx)
		if err != nil {
			return nil, err
		}
		available := make([]string, 0, len(addrs))
		for _, a := range addrs {
			if c.allow(a) {
				available = append(available, a)
			}
		}
		if len(available) == 0 {
			return nil, ErrNoInstance
		}

		addr := available[int(c.rr.Add(1)-1)%len(available)]

		callCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
		resp, err := c.transport(callCtx, addr, method, req)
		cancel()

		if err == nil {
			c.recordSuccess(addr)
			return resp, nil
		}
		lastErr = err
		c.recordFailure(addr)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff(c.opts.BackoffBase, attempt)):
		}
	}
	return nil, lastErr
}

func (c *Caller) allow(addr string) bool {
	b := c.breakerFor(addr)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == stateOpen {
		if time.Since(b.openAt) > c.opts.Cooldown {
			b.state = stateHalfOpen
			return true
		}
		return false
	}
	return true
}

func (c *Caller) recordSuccess(addr string) {
	b := c.breakerFor(addr)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	if b.state == stateHalfOpen {
		b.state = stateClosed
	}
}

func (c *Caller) recordFailure(addr string) {
	b := c.breakerFor(addr)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == stateHalfOpen {
		b.state = stateOpen
		b.openAt = time.Now()
		return
	}
	b.failures++
	if b.failures >= c.opts.FailThreshold {
		b.state = stateOpen
		b.openAt = time.Now()
	}
}

func (c *Caller) breakerFor(addr string) *breaker {
	b, _ := c.breakers.LoadOrStore(addr, &breaker{state: stateClosed})
	return b.(*breaker)
}

func backoff(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	exp := time.Duration(1<<uint(attempt)) * base
	jitter := time.Duration(rand.Int63n(int64(base) + 1))
	return exp + jitter
}
