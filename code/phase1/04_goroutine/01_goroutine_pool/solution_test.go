package goroutine_pool

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_Basic(t *testing.T) {
	pool := NewPool(4)

	var count int64
	for i := 0; i < 20; i++ {
		pool.Submit(func() {
			atomic.AddInt64(&count, 1)
		})
	}

	pool.Wait()

	if count != 20 {
		t.Errorf("期望执行20个任务，实际执行 %d 个", count)
	}
}

func TestPool_ConcurrencyLimit(t *testing.T) {
	maxWorkers := 3
	pool := NewPool(maxWorkers)

	var current int64
	var maxObserved int64
	var mu sync.Mutex

	for i := 0; i < 50; i++ {
		pool.Submit(func() {
			c := atomic.AddInt64(&current, 1)

			mu.Lock()
			if c > maxObserved {
				maxObserved = c
			}
			mu.Unlock()

			time.Sleep(20 * time.Millisecond)
			atomic.AddInt64(&current, -1)
		})
	}

	pool.Wait()

	if maxObserved > int64(maxWorkers) {
		t.Errorf("并发数超过限制：观察到最大 %d，限制 %d", maxObserved, maxWorkers)
	}
	if maxObserved == 0 {
		t.Error("没有观察到并发执行")
	}
}

func TestPool_OrderIndependent(t *testing.T) {
	pool := NewPool(2)

	results := make([]int, 10)
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		idx := i
		pool.Submit(func() {
			mu.Lock()
			results[idx] = idx * idx
			mu.Unlock()
		})
	}

	pool.Wait()

	for i := 0; i < 10; i++ {
		if results[i] != i*i {
			t.Errorf("results[%d] = %d，期望 %d", i, results[i], i*i)
		}
	}
}

func TestPool_SingleWorker(t *testing.T) {
	pool := NewPool(1)

	var sequence []int
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		val := i
		pool.Submit(func() {
			mu.Lock()
			sequence = append(sequence, val)
			mu.Unlock()
		})
	}

	pool.Wait()

	if len(sequence) != 5 {
		t.Errorf("期望5个任务完成，得到 %d", len(sequence))
	}
}

func TestPool_WaitWithNoTasks(t *testing.T) {
	pool := NewPool(4)
	// 不提交任何任务直接 Wait，不应死锁
	done := make(chan struct{})
	go func() {
		pool.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait() 无任务时不应阻塞超过2秒")
	}
}

func TestPool_PanicRecovery(t *testing.T) {
	pool := NewPool(2)

	var count int64

	// 提交一些会 panic 的任务
	pool.Submit(func() {
		panic("intentional panic")
	})

	// 后续任务应该仍然能执行
	for i := 0; i < 5; i++ {
		pool.Submit(func() {
			atomic.AddInt64(&count, 1)
		})
	}

	pool.Wait()

	// 至少部分任务应该完成（panic 的 worker 可能需要恢复）
	// 这个测试是可选的，如果你的实现包含 panic recovery
	// 如果没有 recovery，至少不应该导致程序崩溃
	t.Logf("panic 后完成了 %d 个任务", count)
}

func TestPool_LargeNumberOfTasks(t *testing.T) {
	pool := NewPool(10)

	var count int64
	numTasks := 10000

	for i := 0; i < numTasks; i++ {
		pool.Submit(func() {
			atomic.AddInt64(&count, 1)
		})
	}

	pool.Wait()

	if count != int64(numTasks) {
		t.Errorf("期望完成 %d 个任务，实际 %d", numTasks, count)
	}
}
