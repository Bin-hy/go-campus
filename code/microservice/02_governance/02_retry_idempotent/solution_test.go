package retry_idempotent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryTransient(t *testing.T) {
	var calls atomic.Int32
	policy := Policy{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}

	attempts, err := Do(context.Background(), policy, func(error) bool { return true }, func(context.Context) error {
		if calls.Add(1) < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("应最终成功: %v", err)
	}
	if attempts != 3 || calls.Load() != 3 {
		t.Fatalf("应重试到第 3 次成功，attempts=%d calls=%d", attempts, calls.Load())
	}
	t.Logf("transient retry OK: attempts=%d", attempts)
}

func TestRetryStopsOnFatal(t *testing.T) {
	var calls atomic.Int32
	policy := Policy{MaxAttempts: 5, BaseDelay: time.Millisecond}
	_, err := Do(context.Background(), policy, func(err error) bool {
		return !errors.Is(err, ErrChargeDeclined)
	}, func(context.Context) error {
		calls.Add(1)
		return ErrChargeDeclined
	})
	if !errors.Is(err, ErrChargeDeclined) {
		t.Fatalf("应返回业务错误: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("不可重试错误只能调用 1 次，实际 %d", calls.Load())
	}
}

func TestRetryHonorsCancel(t *testing.T) {
	policy := Policy{MaxAttempts: 100, BaseDelay: 10 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(25*time.Millisecond, cancel)

	start := time.Now()
	_, err := Do(ctx, policy, func(error) bool { return true }, func(context.Context) error {
		return errors.New("always fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("应返回 context.Canceled，实际 %v", err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("取消应立即返回，实际耗时 %v", time.Since(start))
	}
}

func TestIdempotencySerial(t *testing.T) {
	store := NewIdempotencyStore(time.Minute)
	var executed atomic.Int32
	fn := func() (any, error) {
		executed.Add(1)
		return "charge_1", nil
	}

	v1, dedup1, err := store.Do("k1", fn)
	if err != nil || dedup1 || v1 != "charge_1" {
		t.Fatalf("首次: v=%v dedup=%v err=%v", v1, dedup1, err)
	}
	v2, dedup2, err := store.Do("k1", fn)
	if err != nil || !dedup2 || v2 != "charge_1" {
		t.Fatalf("重复: v=%v dedup=%v err=%v", v2, dedup2, err)
	}
	if executed.Load() != 1 {
		t.Fatalf("同一个 key 只应执行一次，实际 %d", executed.Load())
	}
	t.Log("幂等键去重 OK")
}

func TestIdempotencyConcurrent(t *testing.T) {
	store := NewIdempotencyStore(time.Minute)
	var executed atomic.Int32
	fn := func() (any, error) {
		executed.Add(1)
		time.Sleep(20 * time.Millisecond)
		return "charge_1", nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := store.Do("same-key", fn); err != nil {
				t.Errorf("Do: %v", err)
			}
		}()
	}
	wg.Wait()
	if executed.Load() != 1 {
		t.Fatalf("并发同 key 只应执行一次，实际 %d", executed.Load())
	}
	t.Log("并发幂等合并 OK")
}

func TestIdempotencyExpiresAndFailureNotCached(t *testing.T) {
	now := time.Unix(0, 0)
	store := NewIdempotencyStore(time.Second)
	store.SetClock(func() time.Time { return now })

	var calls atomic.Int32
	if _, _, err := store.Do("k", func() (any, error) {
		calls.Add(1)
		return nil, errors.New("boom")
	}); err == nil {
		t.Fatal("应返回错误")
	}
	// 失败不缓存：同一个 key 可以重新执行。
	if _, dedup, _ := store.Do("k", func() (any, error) {
		calls.Add(1)
		return "ok", nil
	}); dedup {
		t.Fatal("失败后不应命中缓存")
	}
	if calls.Load() != 2 {
		t.Fatalf("应执行两次，实际 %d", calls.Load())
	}

	now = now.Add(2 * time.Second)
	if _, dedup, _ := store.Do("k", func() (any, error) {
		calls.Add(1)
		return "again", nil
	}); dedup {
		t.Fatal("过期后不应命中缓存")
	}
	if calls.Load() != 3 {
		t.Fatalf("过期后应再执行，实际 %d", calls.Load())
	}
}

func TestBackoffGrowsAndCaps(t *testing.T) {
	p := Policy{BaseDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond, Jitter: func() float64 { return 0 }}
	if got := p.Backoff(0); got != 10*time.Millisecond {
		t.Fatalf("第 0 次退避应为 10ms，实际 %v", got)
	}
	if got := p.Backoff(2); got != 40*time.Millisecond {
		t.Fatalf("第 2 次退避应为 40ms，实际 %v", got)
	}
	if got := p.Backoff(10); got != 50*time.Millisecond {
		t.Fatalf("应封顶 50ms，实际 %v", got)
	}
}
