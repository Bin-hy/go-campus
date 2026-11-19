package resilience_graceful

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestTokenBucketBurstAndRefill(t *testing.T) {
	now := time.Unix(0, 0)
	tb := NewTokenBucket(10, 3) // 每秒 10 个，桶容量 3
	tb.SetClock(func() time.Time { return now })

	for i := 0; i < 3; i++ {
		if !tb.Allow() {
			t.Fatalf("突发前 3 个应放行（第 %d 个）", i)
		}
	}
	if tb.Allow() {
		t.Fatal("桶空后应拒绝")
	}

	now = now.Add(150 * time.Millisecond) // 补 1.5 个令牌
	if !tb.Allow() {
		t.Fatal("补充后应放行")
	}
	if tb.Allow() {
		t.Fatal("只剩 0.5 个令牌，应拒绝")
	}
	t.Log("令牌桶突发 + 补充 OK")
}

func TestBreakerTransitions(t *testing.T) {
	now := time.Unix(0, 0)
	b := NewBreaker(BreakerConfig{MaxFailures: 3, Cooldown: time.Second})
	b.SetClock(func() time.Time { return now })

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("Closed 阶段应放行（第 %d 次）", i)
		}
		b.Failure()
	}
	if b.State() != BreakerOpen {
		t.Fatalf("连续 3 次失败应 Open，实际 %v", b.State())
	}
	if b.Allow() {
		t.Fatal("Open 冷却期内应拒绝")
	}

	now = now.Add(time.Second)
	if !b.Allow() {
		t.Fatal("冷却结束应进入 Half-Open 并放行探测")
	}
	if b.State() != BreakerHalfOpen {
		t.Fatalf("应为 Half-Open，实际 %v", b.State())
	}
	b.Success()
	if b.State() != BreakerClosed {
		t.Fatalf("探测成功应 Closed，实际 %v", b.State())
	}

	// 再失败到 Open，Half-Open 探测失败要重新 Open。
	for i := 0; i < 3; i++ {
		b.Allow()
		b.Failure()
	}
	now = now.Add(time.Second)
	if !b.Allow() {
		t.Fatal("应放行 Half-Open 探测")
	}
	b.Failure()
	if b.State() != BreakerOpen {
		t.Fatalf("探测失败应重新 Open，实际 %v", b.State())
	}
}

func TestDrainRejectsNewAndWaitsInflight(t *testing.T) {
	d := NewDrain()
	if err := d.Acquire(); err != nil {
		t.Fatal(err)
	}
	if err := d.Acquire(); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		done <- d.Shutdown(ctx)
	}()

	// 给 Shutdown 一点时间进入阻塞。
	time.Sleep(20 * time.Millisecond)
	if err := d.Acquire(); !errors.Is(err, ErrDraining) {
		t.Fatalf("draining 后新请求应被拒，实际 %v", err)
	}

	d.Release()
	select {
	case err := <-done:
		t.Fatalf("仍有 1 个在途请求，Shutdown 不应返回: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	d.Release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("在途清零后 Shutdown 应返回")
	}
	t.Log("优雅停机：拒绝新请求 + 等待在途清零 OK")
}

func TestDrainTimeout(t *testing.T) {
	d := NewDrain()
	_ = d.Acquire()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := d.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("超时应返回 DeadlineExceeded，实际 %v", err)
	}
}

func TestConcurrentAcquireRelease(t *testing.T) {
	d := NewDrain()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := d.Acquire(); err == nil {
				d.Release()
			}
		}()
	}
	wg.Wait()
	if d.Inflight() != 0 {
		t.Fatalf("在途应归零，实际 %d", d.Inflight())
	}
}
