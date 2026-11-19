package retry_idempotent

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"
)

// 参考答案 / 可运行实现。

// Policy 控制重试节奏。
type Policy struct {
	MaxAttempts int            // 总尝试次数（含首次），<=0 视为 1
	BaseDelay   time.Duration  // 指数退避基数
	MaxDelay    time.Duration  // 单次退避上限
	Jitter      func() float64 // 返回 [0,1) 的随机数；nil 用 math/rand
}

func (p Policy) attempts() int {
	if p.MaxAttempts <= 0 {
		return 1
	}
	return p.MaxAttempts
}

// Backoff 第 attempt 次失败后的等待：base*2^attempt，封顶 MaxDelay，再加抖动。
// 抖动是关键：把客户端重试打散，避免同一时刻一起重试（重试风暴）。
func (p Policy) Backoff(attempt int) time.Duration {
	if p.BaseDelay <= 0 {
		return 0
	}
	d := p.BaseDelay
	for i := 0; i < attempt; i++ {
		d *= 2
		if p.MaxDelay > 0 && d >= p.MaxDelay {
			d = p.MaxDelay
			break
		}
	}
	j := p.Jitter
	if j == nil {
		j = rand.Float64
	}
	return d + time.Duration(float64(d)*0.2*j()) // ±20% 抖动
}

// Do 执行带重试的调用。
//   - 只有 isRetryable(err) 为 true 才重试；
//   - 每次退避都监听 ctx.Done()，上层取消立刻返回；
//   - 返回实际尝试次数，便于观测与打点。
func Do(ctx context.Context, p Policy, isRetryable func(error) bool, fn func(context.Context) error) (int, error) {
	var err error
	for attempt := 0; attempt < p.attempts(); attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return attempt, ctxErr
		}
		err = fn(ctx)
		if err == nil {
			return attempt + 1, nil
		}
		if isRetryable != nil && !isRetryable(err) {
			return attempt + 1, err
		}
		if attempt == p.attempts()-1 {
			break
		}
		delay := p.Backoff(attempt)
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return attempt + 1, ctx.Err()
		case <-timer.C:
		}
	}
	return p.attempts(), err
}

// IdempotencyStore 幂等键存储：
// 同一个 key 在 TTL 内只真正执行一次，其余请求复用第一次的结果。
// 并发同 key 用 singleflight 合并，避免"同时都不存在、同时都执行"。
type IdempotencyStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]*entry
}

type entry struct {
	done    chan struct{}
	val     any
	err     error
	expires time.Time
	ready   bool
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]*entry),
	}
}

func (s *IdempotencyStore) SetClock(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = fn
}

// Do 返回 (结果, 是否命中幂等缓存, error)。
func (s *IdempotencyStore) Do(key string, fn func() (any, error)) (any, bool, error) {
	s.mu.Lock()
	now := s.now()
	if e, ok := s.entries[key]; ok {
		if e.ready && now.Before(e.expires) {
			s.mu.Unlock()
			return e.val, true, e.err
		}
		if !e.ready {
			// 已有请求在执行：等它完成，复用结果。
			s.mu.Unlock()
			<-e.done
			s.mu.Lock()
			val, err := e.val, e.err
			s.mu.Unlock()
			return val, true, err
		}
		// 过期：清理后重新执行。
		delete(s.entries, key)
	}
	e := &entry{done: make(chan struct{})}
	s.entries[key] = e
	s.mu.Unlock()

	val, err := fn()

	s.mu.Lock()
	e.val, e.err = val, err
	if err == nil {
		e.ready = true
		e.expires = s.now().Add(s.ttl)
	} else {
		// 失败不缓存：允许调用方用同一个 key 重试。
		delete(s.entries, key)
	}
	s.mu.Unlock()
	close(e.done)
	return val, false, err
}

// ErrChargeDeclined 用于演示"不可重试"的业务错误。
var ErrChargeDeclined = errors.New("charge declined")
