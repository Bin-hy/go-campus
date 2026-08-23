package rpc_retry_balancer

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNoInstance 当前没有可用（未熔断）实例
var ErrNoInstance = errors.New("no available instance")

// Registry 服务注册中心（生产实现为 etcd：注册/续租/Watch，这里只留"发现"）
type Registry interface {
	Discover(ctx context.Context) ([]string, error)
}

// Transport 向指定地址发起一次 RPC 调用（生产实现为 gRPC 一元调用）
type Transport func(ctx context.Context, addr, method string, req any) (any, error)

// Options RPC 客户端治理参数
type Options struct {
	Timeout       time.Duration // 单次调用超时
	MaxRetry      int           // 最多重试次数
	BackoffBase   time.Duration // 指数退避基础值
	FailThreshold int           // 连续失败多少次触发熔断
	Cooldown      time.Duration // 熔断冷却期，之后进入 Half-Open 探测
}

// 熔断器三态
const (
	stateClosed   = 0
	stateOpen     = 1
	stateHalfOpen = 2
)

// breaker 单个实例的熔断器
type breaker struct {
	mu       sync.Mutex
	state    int // stateClosed / stateOpen / stateHalfOpen
	failures int // 连续失败次数
	openAt   time.Time
}

// Caller RPC 客户端：负载均衡 + 超时 + 重试退避 + 熔断
type Caller struct {
	registry  Registry
	transport Transport
	opts      Options
	rr        atomic.Uint64 // round-robin 游标（并发安全）
	breakers  sync.Map      // addr -> *breaker
}

func NewCaller(registry Registry, transport Transport, opts Options) *Caller {
	return &Caller{registry: registry, transport: transport, opts: opts}
}

// Call 一次带治理的调用：发现 → 过滤熔断实例 → 轮询选择 → 带超时调用 → 失败重试
func (c *Caller) Call(ctx context.Context, method string, req any) (any, error) {
	var lastErr error
	for attempt := 0; attempt <= c.opts.MaxRetry; attempt++ {
		// 每次重试都重新发现 + 过滤：熔断/下线的实例自动被剔除
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

		// round-robin 选实例
		addr := available[int(c.rr.Add(1)-1)%len(available)]

		callCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout) // 单次调用超时
		resp, err := c.transport(callCtx, addr, method, req)
		cancel()

		if err == nil {
			c.recordSuccess(addr)
			return resp, nil
		}
		lastErr = err
		c.recordFailure(addr) // 连续失败计数，达阈值熔断

		// 指数退避 + jitter 后重试；同时响应上层 ctx 取消
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff(c.opts.BackoffBase, attempt)):
		}
	}
	return nil, lastErr
}

// allow 该实例是否可被选中：Open 且过冷却期 → Half-Open 放行探测；Open 未到期 → 拒绝
func (c *Caller) allow(addr string) bool {
	b := c.breakerFor(addr)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == stateOpen {
		if time.Since(b.openAt) > c.opts.Cooldown {
			b.state = stateHalfOpen // 冷却结束：放一个探测请求
			return true
		}
		return false
	}
	return true // closed / halfOpen
}

// recordSuccess 成功：清零失败计数；Half-Open 探测成功 → 恢复 Closed
func (c *Caller) recordSuccess(addr string) {
	b := c.breakerFor(addr)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	if b.state == stateHalfOpen {
		b.state = stateClosed
	}
}

// recordFailure 失败：Half-Open 探测失败 → 回 Open；Closed 下连续失败达阈值 → Open
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

// backoff 指数退避 + 随机抖动：base*2^attempt + rand[0, base)
// jitter 是关键：把所有客户端的重试"打散"，避免同一时刻一起重试（重试风暴）
func backoff(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	exp := time.Duration(1<<uint(attempt)) * base
	jitter := time.Duration(rand.Int63n(int64(base) + 1))
	return exp + jitter
}
