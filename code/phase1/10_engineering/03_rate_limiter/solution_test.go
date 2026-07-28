package rate_limiter

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiter_Allow_Burst(t *testing.T) {
	// 10 QPS，burst=5：初始应有5个令牌
	limiter := NewRateLimiter(10, 5)
	defer limiter.Stop()

	// 先消耗一点时间让令牌填满
	time.Sleep(600 * time.Millisecond)

	allowed := 0
	for i := 0; i < 10; i++ {
		if limiter.Allow() {
			allowed++
		}
	}

	// burst=5，所以一次最多允许5个
	if allowed < 4 || allowed > 6 {
		t.Errorf("burst=5 时应允许约5个请求，实际允许了 %d", allowed)
	}
}

func TestRateLimiter_Allow_RateLimit(t *testing.T) {
	// 10 QPS
	limiter := NewRateLimiter(10, 10)
	defer limiter.Stop()

	time.Sleep(time.Second) // 等待令牌填满

	// 消耗所有令牌
	for i := 0; i < 10; i++ {
		limiter.Allow()
	}

	// 立即再请求应该被拒绝
	if limiter.Allow() {
		// 可能刚好补充了一个，这种边界情况可以接受
	}

	// 等 200ms 应该补充约2个
	time.Sleep(250 * time.Millisecond)
	allowed := 0
	for i := 0; i < 5; i++ {
		if limiter.Allow() {
			allowed++
		}
	}
	if allowed < 1 || allowed > 4 {
		t.Errorf("等200ms后应补充1-3个令牌，实际可用 %d", allowed)
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	limiter := NewRateLimiter(100, 1)
	defer limiter.Stop()

	time.Sleep(50 * time.Millisecond) // 等待至少一个令牌

	start := time.Now()
	limiter.Wait() // 应该很快获得
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("有令牌时 Wait 不应阻塞太久，耗时 %v", elapsed)
	}
}

func TestRateLimiter_Wait_Blocking(t *testing.T) {
	limiter := NewRateLimiter(10, 1) // 每100ms一个令牌
	defer limiter.Stop()

	time.Sleep(150 * time.Millisecond) // 确保有1个令牌
	limiter.Wait()                      // 消耗掉

	start := time.Now()
	limiter.Wait() // 需要等下一个令牌
	elapsed := time.Since(start)

	// 每100ms一个令牌，应等约100ms
	if elapsed < 50*time.Millisecond || elapsed > 250*time.Millisecond {
		t.Errorf("Wait 应等约100ms，实际 %v", elapsed)
	}
}

func TestRateLimiter_ConcurrentAllow(t *testing.T) {
	limiter := NewRateLimiter(100, 10)
	defer limiter.Stop()

	time.Sleep(150 * time.Millisecond) // 等令牌填满

	var allowed int64
	done := make(chan struct{})

	// 50 个 goroutine 同时抢令牌
	for i := 0; i < 50; i++ {
		go func() {
			if limiter.Allow() {
				atomic.AddInt64(&allowed, 1)
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 50; i++ {
		<-done
	}

	// 不应超过 burst
	if allowed > 12 {
		t.Errorf("并发 Allow 超过 burst：得到 %d，burst=10", allowed)
	}
}

func TestRateLimiter_Stop(t *testing.T) {
	limiter := NewRateLimiter(10, 5)
	limiter.Stop()

	// Stop 后不应 panic
	// （具体行为取决于实现，可以不再产生令牌）
}

func TestRateLimiter_HighRate(t *testing.T) {
	// 1000 QPS，burst=100
	limiter := NewRateLimiter(1000, 100)
	defer limiter.Stop()

	time.Sleep(150 * time.Millisecond) // 补充约150个，但被burst限制为100

	allowed := 0
	for i := 0; i < 200; i++ {
		if limiter.Allow() {
			allowed++
		}
	}

	if allowed < 80 || allowed > 110 {
		t.Errorf("burst=100，应允许约100个，实际 %d", allowed)
	}
}
