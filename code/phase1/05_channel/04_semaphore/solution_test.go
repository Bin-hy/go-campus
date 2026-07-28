package semaphore

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSemaphore_Basic(t *testing.T) {
	sem := NewSemaphore(3)

	if sem.Available() != 3 {
		t.Errorf("初始可用数应为3，得到 %d", sem.Available())
	}

	sem.Acquire()
	if sem.Available() != 2 {
		t.Errorf("Acquire 后应为2，得到 %d", sem.Available())
	}

	sem.Acquire()
	sem.Acquire()
	if sem.Available() != 0 {
		t.Errorf("3次 Acquire 后应为0，得到 %d", sem.Available())
	}

	sem.Release()
	if sem.Available() != 1 {
		t.Errorf("Release 后应为1，得到 %d", sem.Available())
	}
}

func TestSemaphore_Blocking(t *testing.T) {
	sem := NewSemaphore(1)
	sem.Acquire() // 占满

	acquired := make(chan struct{})
	go func() {
		sem.Acquire() // 应阻塞
		close(acquired)
	}()

	// 确认阻塞
	select {
	case <-acquired:
		t.Fatal("不应立即获取到")
	case <-time.After(100 * time.Millisecond):
		// 预期阻塞
	}

	sem.Release() // 释放后应该获取到

	select {
	case <-acquired:
		// OK
	case <-time.After(time.Second):
		t.Fatal("Release 后应获取到许可")
	}
}

func TestSemaphore_TryAcquire_Success(t *testing.T) {
	sem := NewSemaphore(2)

	if !sem.TryAcquire(100 * time.Millisecond) {
		t.Error("有可用许可时 TryAcquire 应成功")
	}
}

func TestSemaphore_TryAcquire_Timeout(t *testing.T) {
	sem := NewSemaphore(1)
	sem.Acquire() // 占满

	start := time.Now()
	result := sem.TryAcquire(100 * time.Millisecond)
	elapsed := time.Since(start)

	if result {
		t.Error("无可用许可时 TryAcquire 应返回 false")
	}
	if elapsed < 80*time.Millisecond {
		t.Errorf("应等待至少 80ms，实际 %v", elapsed)
	}
}

func TestSemaphore_ConcurrencyLimit(t *testing.T) {
	maxConcurrency := 3
	sem := NewSemaphore(maxConcurrency)

	var currentCount int64
	var maxObserved int64
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem.Acquire()
			defer sem.Release()

			current := atomic.AddInt64(&currentCount, 1)
			// 记录观察到的最大并发数
			for {
				old := atomic.LoadInt64(&maxObserved)
				if current <= old || atomic.CompareAndSwapInt64(&maxObserved, old, current) {
					break
				}
			}

			time.Sleep(50 * time.Millisecond) // 模拟工作
			atomic.AddInt64(&currentCount, -1)
		}()
	}

	wg.Wait()

	if maxObserved > int64(maxConcurrency) {
		t.Errorf("最大并发数超过限制：观察到 %d，限制 %d", maxObserved, maxConcurrency)
	}
}

func TestSemaphore_TryAcquire_Released(t *testing.T) {
	sem := NewSemaphore(1)
	sem.Acquire()

	// 50ms 后释放
	go func() {
		time.Sleep(50 * time.Millisecond)
		sem.Release()
	}()

	// 200ms 超时应该能获取到（50ms 后释放）
	if !sem.TryAcquire(200 * time.Millisecond) {
		t.Error("等待 Release 后应获取成功")
	}
}
