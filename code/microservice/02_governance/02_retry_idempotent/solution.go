package retry_idempotent

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Policy 控制重试节奏。
type Policy struct {
	MaxAttempts int            // 总尝试次数（含首次），<=0 视为 1
	BaseDelay   time.Duration  // 指数退避基数
	MaxDelay    time.Duration  // 单次退避上限
	Jitter      func() float64 // 返回 [0,1) 的随机数；nil 用 math/rand
}

// Backoff 第 attempt 次失败后的等待：base*2^attempt，封顶 MaxDelay，再加抖动。
// 提示：抖动取 ±20%，Jitter 为 nil 时用 rand.Float64。
func (p Policy) Backoff(attempt int) time.Duration {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Do 执行带重试的调用。
//   - 只有 isRetryable(err) 为 true 才重试；
//   - 每次退避都监听 ctx.Done()，上层取消立刻返回；
//   - 返回实际尝试次数。
func Do(ctx context.Context, p Policy, isRetryable func(error) bool, fn func(context.Context) error) (int, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// IdempotencyStore 幂等键存储：
// 同一个 key 在 TTL 内只真正执行一次，其余请求复用第一次的结果。
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
// 提示：并发同 key 要合并（后来的请求等待第一个完成）；
// 成功才缓存，失败要删除 key 允许重试。
func (s *IdempotencyStore) Do(key string, fn func() (any, error)) (any, bool, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// ErrChargeDeclined 用于演示"不可重试"的业务错误。
var ErrChargeDeclined = errors.New("charge declined")
