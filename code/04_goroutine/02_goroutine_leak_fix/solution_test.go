package goroutine_leak_fix

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestFixedSearch_ReturnsFirst(t *testing.T) {
	slow := func(q string) string {
		time.Sleep(500 * time.Millisecond)
		return "slow: " + q
	}
	fast := func(q string) string {
		time.Sleep(10 * time.Millisecond)
		return "fast: " + q
	}

	result := FixedSearch("hello", slow, fast)
	if result != "fast: hello" {
		t.Errorf("应返回最快的结果，得到 %q", result)
	}
}

func TestFixedSearch_NoLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	for i := 0; i < 10; i++ {
		slow := func(q string) string {
			time.Sleep(time.Second)
			return "slow"
		}
		fast := func(q string) string {
			time.Sleep(10 * time.Millisecond)
			return "fast"
		}
		FixedSearch("test", slow, fast)
	}

	// 等待 goroutine 退出
	time.Sleep(200 * time.Millisecond)
	after := runtime.NumGoroutine()

	leaked := after - before
	if leaked > 5 {
		t.Errorf("可能存在 goroutine 泄漏：运行前 %d，运行后 %d（泄漏 %d）",
			before, after, leaked)
	}
}

func TestFixedGenerator_Basic(t *testing.T) {
	done := make(chan struct{})
	ch := FixedGenerator(done)

	for i := 0; i < 5; i++ {
		v := <-ch
		if v != i {
			t.Errorf("期望 %d，得到 %d", i, v)
		}
	}
	close(done)
}

func TestFixedGenerator_NoLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	for i := 0; i < 20; i++ {
		done := make(chan struct{})
		ch := FixedGenerator(done)
		<-ch // 只读一个
		close(done)
	}

	time.Sleep(200 * time.Millisecond)
	after := runtime.NumGoroutine()

	leaked := after - before
	if leaked > 3 {
		t.Errorf("Generator 泄漏：前 %d，后 %d（泄漏 %d）", before, after, leaked)
	}
}

func TestFixedWorker_Executes(t *testing.T) {
	done := make(chan struct{})
	var count int64

	go FixedWorker(done, 20*time.Millisecond, func() {
		atomic.AddInt64(&count, 1)
	})

	time.Sleep(100 * time.Millisecond)
	close(done)

	c := atomic.LoadInt64(&count)
	if c < 3 || c > 7 {
		t.Errorf("100ms 内以 20ms 间隔应执行约5次，实际 %d 次", c)
	}
}

func TestFixedWorker_StopsOnDone(t *testing.T) {
	done := make(chan struct{})
	var count int64

	go FixedWorker(done, 10*time.Millisecond, func() {
		atomic.AddInt64(&count, 1)
	})

	time.Sleep(50 * time.Millisecond)
	close(done)
	countAtStop := atomic.LoadInt64(&count)

	time.Sleep(100 * time.Millisecond)
	countAfter := atomic.LoadInt64(&count)

	if countAfter > countAtStop+1 {
		t.Errorf("关闭后不应继续执行：关闭时 %d，之后 %d", countAtStop, countAfter)
	}
}

func TestFixedWorker_NoLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	for i := 0; i < 10; i++ {
		done := make(chan struct{})
		go FixedWorker(done, 10*time.Millisecond, func() {})
		time.Sleep(30 * time.Millisecond)
		close(done)
	}

	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	if after-before > 3 {
		t.Errorf("Worker 泄漏：前 %d，后 %d", before, after)
	}
}
