package cache_breakdown

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeCache 内存版 Cache 实现（模拟 Redis）
type fakeCache struct {
	mu   sync.Mutex
	data map[string]string
	lock map[string]bool
}

func newFakeCache() *fakeCache {
	return &fakeCache{data: map[string]string{}, lock: map[string]bool{}}
}

func (f *fakeCache) Get(_ context.Context, key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	return v, ok, nil
}

func (f *fakeCache) Set(_ context.Context, key, val string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = val
	return nil
}

func (f *fakeCache) SetNX(_ context.Context, key, val string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lock[key] {
		return false, nil
	}
	f.lock[key] = true
	return true, nil
}

func (f *fakeCache) Del(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.lock, key)
	return nil
}

func TestGetWithMutex_SingleLoad(t *testing.T) {
	c := newFakeCache()
	var loads int32
	loader := func(ctx context.Context, key string) (string, error) {
		atomic.AddInt32(&loads, 1)
		time.Sleep(50 * time.Millisecond) // 模拟慢 DB
		return "db:" + key, nil
	}

	const n = 20
	var wg sync.WaitGroup
	vals := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			vals[i], errs[i] = GetWithMutex(context.Background(), c, loader, "hot:clip")
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&loads); got != 1 {
		t.Fatalf("loader called %d times, want 1", got)
	}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d error: %v", i, errs[i])
		}
		if vals[i] != "db:hot:clip" {
			t.Fatalf("goroutine %d val = %q, want db:hot:clip", i, vals[i])
		}
	}
}

func TestGetWithMutex_CacheHit_NoLoad(t *testing.T) {
	c := newFakeCache()
	_ = c.Set(context.Background(), "k", "cached", time.Minute)
	var loads int32
	v, err := GetWithMutex(context.Background(), c,
		func(ctx context.Context, key string) (string, error) {
			atomic.AddInt32(&loads, 1)
			return "db", nil
		}, "k")
	if err != nil {
		t.Fatal(err)
	}
	if v != "cached" {
		t.Fatalf("v = %q, want cached", v)
	}
	if loads != 0 {
		t.Fatalf("loader called %d times, want 0", loads)
	}
}

func TestGetWithMutex_LoaderError_ReleasesLock(t *testing.T) {
	c := newFakeCache()
	boom := errors.New("db down")
	_, err := GetWithMutex(context.Background(), c,
		func(ctx context.Context, key string) (string, error) { return "", boom }, "k")
	if err != boom {
		t.Fatalf("err = %v, want boom", err)
	}
	// 锁应已释放：第二次调用能重新抢锁并重建
	var loads int32
	v, err := GetWithMutex(context.Background(), c,
		func(ctx context.Context, key string) (string, error) {
			atomic.AddInt32(&loads, 1)
			return "ok", nil
		}, "k")
	if err != nil {
		t.Fatal(err)
	}
	if v != "ok" || loads != 1 {
		t.Fatalf("v=%q loads=%d, want ok/1", v, loads)
	}
}

func TestGetWithMutex_SpinnerTimeout(t *testing.T) {
	c := newFakeCache()
	// 锁被其他持有者占用且缓存一直不出现 → 自旋 5 次后超时
	c.mu.Lock()
	c.lock["lock:k"] = true
	c.mu.Unlock()

	_, err := GetWithMutex(context.Background(), c,
		func(ctx context.Context, key string) (string, error) { return "x", nil }, "k")
	if err != ErrRebuildTimeout {
		t.Fatalf("err = %v, want ErrRebuildTimeout", err)
	}
}
