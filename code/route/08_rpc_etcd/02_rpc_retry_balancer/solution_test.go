package rpc_retry_balancer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type fakeRegistry struct {
	mu    sync.Mutex
	addrs []string
}

func newFakeRegistry(addrs ...string) *fakeRegistry {
	return &fakeRegistry{addrs: append([]string{}, addrs...)}
}

func (r *fakeRegistry) Discover(context.Context) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.addrs...), nil
}

type hitCounter struct {
	mu sync.Mutex
	m  map[string]int
}

func (h *hitCounter) inc(addr string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.m[addr]++
}

func (h *hitCounter) get(addr string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.m[addr]
}

func (h *hitCounter) total() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, v := range h.m {
		n += v
	}
	return n
}

func newCounter() *hitCounter { return &hitCounter{m: map[string]int{}} }

// ---------------------------------------------------------------------------
// 测试
// ---------------------------------------------------------------------------

// 3 个健康实例，6 次调用按轮询均匀分布（a,b,c 各 2 次）
func TestCall_RoundRobinDistribution(t *testing.T) {
	registry := newFakeRegistry("a", "b", "c")
	counter := newCounter()
	transport := func(_ context.Context, addr, _ string, _ any) (any, error) {
		counter.inc(addr)
		return "ok:" + addr, nil
	}
	caller := NewCaller(registry, transport, Options{
		Timeout:       time.Second,
		MaxRetry:      0,
		BackoffBase:   time.Millisecond,
		FailThreshold: 5,
		Cooldown:      time.Second,
	})

	for i := 0; i < 6; i++ {
		resp, err := caller.Call(context.Background(), "Clip.Generate", i)
		if err != nil {
			t.Fatalf("Call #%d err = %v", i, err)
		}
		_ = resp
	}
	for _, addr := range []string{"a", "b", "c"} {
		if n := counter.get(addr); n != 2 {
			t.Fatalf("addr %s 被调用 %d 次, want 2（轮询应均匀分布）", addr, n)
		}
	}
}

// 首次调用 flaky 失败 → 自动重试到其他实例成功；共 2 次调用、落在两个不同实例
func TestCall_RetryFallback(t *testing.T) {
	registry := newFakeRegistry("a", "b", "c")
	counter := newCounter()
	first := true
	transport := func(_ context.Context, addr, _ string, _ any) (any, error) {
		counter.inc(addr)
		if first {
			first = false
			return nil, errors.New("flaky: " + addr)
		}
		return "ok", nil
	}
	caller := NewCaller(registry, transport, Options{
		Timeout:       time.Second,
		MaxRetry:      2,
		BackoffBase:   time.Millisecond,
		FailThreshold: 100, // 本测试不触发熔断
		Cooldown:      time.Second,
	})

	resp, err := caller.Call(context.Background(), "Clip.Generate", "req")
	if err != nil {
		t.Fatalf("Call err = %v, want nil（应自动重试成功）", err)
	}
	if resp != "ok" {
		t.Fatalf("resp = %v, want ok", resp)
	}
	if n := counter.total(); n != 2 {
		t.Fatalf("总调用次数 = %d, want 2（1 次失败 + 1 次重试成功）", n)
	}
	// 两次调用必须落在不同实例（轮询游标保证）
	hitAddrs := []string{}
	for _, a := range []string{"a", "b", "c"} {
		if counter.get(a) > 0 {
			hitAddrs = append(hitAddrs, a)
		}
	}
	if len(hitAddrs) != 2 {
		t.Fatalf("命中实例 = %v, want 恰好 2 个不同实例", hitAddrs)
	}
}

// 实例连续失败达阈值 → 熔断 Open：后续调用快速失败（ErrNoInstance），不再打它
func TestCircuitBreaker_OpenAndFastFail(t *testing.T) {
	registry := newFakeRegistry("a")
	counter := newCounter()
	transport := func(_ context.Context, addr, _ string, _ any) (any, error) {
		counter.inc(addr)
		return nil, errors.New("down: " + addr)
	}
	caller := NewCaller(registry, transport, Options{
		Timeout:       time.Second,
		MaxRetry:      1,
		BackoffBase:   time.Millisecond,
		FailThreshold: 2,
		Cooldown:      10 * time.Second, // 长冷却：本测试期间不会恢复
	})

	// 第一次调用：2 次 attempt 都失败 → 连续失败达 2 → 熔断
	if _, err := caller.Call(context.Background(), "M", "req"); err == nil {
		t.Fatal("调用应失败")
	}
	if n := counter.get("a"); n != 2 {
		t.Fatalf("熔断前 hits = %d, want 2", n)
	}

	// 第二次调用：实例已熔断 → 无可用实例 → ErrNoInstance，且不再发起真实调用
	_, err := caller.Call(context.Background(), "M", "req")
	if err != ErrNoInstance {
		t.Fatalf("err = %v, want ErrNoInstance（熔断快速失败）", err)
	}
	if n := counter.get("a"); n != 2 {
		t.Fatalf("熔断后 hits = %d, want 2（不应再调用已熔断实例）", n)
	}
}

// 冷却期后 Half-Open 探测：成功则恢复 Closed 重新纳入轮询
func TestCircuitBreaker_RecoverAfterCooldown(t *testing.T) {
	registry := newFakeRegistry("a")
	counter := newCounter()
	down := true
	transport := func(_ context.Context, addr, _ string, _ any) (any, error) {
		counter.inc(addr)
		if down {
			return nil, errors.New("down")
		}
		return "ok", nil
	}
	caller := NewCaller(registry, transport, Options{
		Timeout:       time.Second,
		MaxRetry:      1,
		BackoffBase:   time.Millisecond,
		FailThreshold: 2,
		Cooldown:      150 * time.Millisecond,
	})

	if _, err := caller.Call(context.Background(), "M", "req"); err == nil {
		t.Fatal("down 期间调用应失败并熔断")
	}
	if n := counter.get("a"); n != 2 {
		t.Fatalf("熔断前 hits = %d, want 2", n)
	}

	// 服务恢复 + 冷却期结束 → Half-Open 探测成功 → 熔断关闭
	down = false
	time.Sleep(300 * time.Millisecond)
	if _, err := caller.Call(context.Background(), "M", "req"); err != nil {
		t.Fatalf("恢复后 Call err = %v, want nil（Half-Open 探测成功）", err)
	}
	if n := counter.get("a"); n != 3 {
		t.Fatalf("探测后 hits = %d, want 3", n)
	}

	// 熔断已关闭：正常调用
	if _, err := caller.Call(context.Background(), "M", "req"); err != nil {
		t.Fatalf("恢复后再次 Call err = %v, want nil", err)
	}
	if n := counter.get("a"); n != 4 {
		t.Fatalf("恢复后 hits = %d, want 4", n)
	}
}

// Half-Open 探测仍失败 → 保持 Open（重新计时冷却）
func TestCircuitBreaker_ProbeFailStaysOpen(t *testing.T) {
	registry := newFakeRegistry("a")
	transport := func(_ context.Context, addr, _ string, _ any) (any, error) {
		return nil, errors.New("still down: " + addr)
	}
	caller := NewCaller(registry, transport, Options{
		Timeout:       time.Second,
		MaxRetry:      0,
		BackoffBase:   time.Millisecond,
		FailThreshold: 1,
		Cooldown:      100 * time.Millisecond,
	})

	if _, err := caller.Call(context.Background(), "M", "req"); err == nil {
		t.Fatal("应失败并熔断")
	}
	time.Sleep(250 * time.Millisecond) // 过冷却期 → Half-Open 探测
	if _, err := caller.Call(context.Background(), "M", "req"); err == nil {
		t.Fatal("探测仍失败，不应成功")
	}
	// 探测失败后重新 Open（冷却重新计时）：立即再调 → 快速失败
	_, err := caller.Call(context.Background(), "M", "req")
	if err != ErrNoInstance {
		t.Fatalf("err = %v, want ErrNoInstance（探测失败后重新熔断）", err)
	}
}

// 单次调用超时：transport 阻塞超过 Timeout → 立即返回 DeadlineExceeded，不无限等
func TestCall_Timeout(t *testing.T) {
	registry := newFakeRegistry("a")
	transport := func(ctx context.Context, addr, _ string, _ any) (any, error) {
		select {
		case <-ctx.Done(): // 超时/取消 → 立即返回
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
			return "late", nil
		}
	}
	caller := NewCaller(registry, transport, Options{
		Timeout:       80 * time.Millisecond,
		MaxRetry:      0,
		BackoffBase:   time.Millisecond,
		FailThreshold: 5,
		Cooldown:      time.Second,
	})

	start := time.Now()
	_, err := caller.Call(context.Background(), "Slow.Method", "req")
	if err != context.DeadlineExceeded {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("超时返回太慢: %v", elapsed)
	}
}

// 退避等待期间上层 ctx 取消 → 立即返回，不继续重试
func TestCall_ContextCancelDuringBackoff(t *testing.T) {
	registry := newFakeRegistry("a")
	transport := func(_ context.Context, addr, _ string, _ any) (any, error) {
		return nil, errors.New("down: " + addr)
	}
	caller := NewCaller(registry, transport, Options{
		Timeout:       time.Second,
		MaxRetry:      3,
		BackoffBase:   50 * time.Millisecond,
		FailThreshold: 100,
		Cooldown:      time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := caller.Call(ctx, "M", "req")
	if err != context.DeadlineExceeded {
		t.Fatalf("err = %v, want context.DeadlineExceeded（退避中取消应立刻返回）", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("取消响应太慢: %v", elapsed)
	}
}
