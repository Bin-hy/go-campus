package resilience_graceful

import (
	"context"
	"errors"
	"sync"
	"time"
)

// 参考答案 / 可运行实现。

var (
	ErrRateLimited = errors.New("rate limited")
	ErrOpen        = errors.New("circuit open")
	ErrDraining    = errors.New("server draining")
)

// TokenBucket 令牌桶限流：支持突发 burst，之后按 rate 匀速补充。
type TokenBucket struct {
	mu     sync.Mutex
	rate   float64 // 每秒补充多少令牌
	burst  float64 // 桶容量
	tokens float64
	last   time.Time
	now    func() time.Time
}

func NewTokenBucket(ratePerSec float64, burst int) *TokenBucket {
	return &TokenBucket{
		rate:   ratePerSec,
		burst:  float64(burst),
		tokens: float64(burst),
		now:    time.Now,
		last:   time.Now(),
	}
}

func (b *TokenBucket) SetClock(fn func() time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.now = fn
	b.last = fn()
}

func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens = min(b.burst, b.tokens+elapsed*b.rate)
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (b *TokenBucket) Tokens() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tokens
}

// BreakerState 三态。
type BreakerState int

const (
	BreakerClosed BreakerState = iota
	BreakerOpen
	BreakerHalfOpen
)

type BreakerConfig struct {
	MaxFailures int           // 连续失败多少次打开
	Cooldown    time.Duration // Open 持续多久后进入 Half-Open
}

type Breaker struct {
	mu       sync.Mutex
	cfg      BreakerConfig
	state    BreakerState
	failures int
	openedAt time.Time
	now      func() time.Time
}

func NewBreaker(cfg BreakerConfig) *Breaker {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	return &Breaker{cfg: cfg, state: BreakerClosed, now: time.Now}
}

func (b *Breaker) SetClock(fn func() time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.now = fn
}

// Allow 是否放行这次调用。
// Open 且冷却结束 → 进入 Half-Open，只放一个探测请求。
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == BreakerOpen && b.now().Sub(b.openedAt) >= b.cfg.Cooldown {
		b.state = BreakerHalfOpen
	}
	return b.state != BreakerOpen
}

func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = BreakerClosed
}

func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == BreakerHalfOpen {
		b.state = BreakerOpen
		b.openedAt = b.now()
		return
	}
	b.failures++
	if b.failures >= b.cfg.MaxFailures {
		b.state = BreakerOpen
		b.openedAt = b.now()
		b.failures = 0
	}
}

func (b *Breaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Drain 优雅停机门闸：
// 收到 SIGTERM 后先 StopAccepting，等在途请求清零，再真正退出。
type Drain struct {
	mu       sync.Mutex
	draining bool
	inflight int
	zero     chan struct{} // 每次归零时关闭并重建
}

func NewDrain() *Drain {
	return &Drain{zero: make(chan struct{})}
}

func (d *Drain) Acquire() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.draining {
		return ErrDraining
	}
	d.inflight++
	return nil
}

func (d *Drain) Release() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.inflight--
	if d.inflight == 0 {
		close(d.zero)
		d.zero = make(chan struct{})
	}
}

// StopAccepting 停止接新请求；返回的 channel 在在途请求清零时关闭。
func (d *Drain) StopAccepting() <-chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.draining = true
	return d.zero
}

func (d *Drain) Inflight() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.inflight
}

// Shutdown 等待在途请求完成，超时返回 ctx 错误（调用方据此强制退出）。
func (d *Drain) Shutdown(ctx context.Context) error {
	zero := d.StopAccepting()
	select {
	case <-zero:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
