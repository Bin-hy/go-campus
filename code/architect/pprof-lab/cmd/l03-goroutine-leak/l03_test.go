package main

import (
	"runtime"
	"testing"
	"time"
)

// waitFor 轮询等待条件成立，超时就返回 false（避免测试用固定 sleep 变得又慢又脆）。
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

// TestTaskManagerConverges 是 L03 的核心测试：fix 实现必须能真正收敛 goroutine。
// 运行：go test -v -run Converges ./cmd/l03-goroutine-leak
func TestTaskManagerConverges(t *testing.T) {
	base := runtime.NumGoroutine()

	mgr := newTaskManager()
	t.Cleanup(mgr.Close)

	const tasks = 20
	mgr.Start(tasks)

	want := int64(tasks * workerPerTask)
	if !waitFor(time.Second, func() bool { return mgr.Active() == want }) {
		t.Fatalf("后台 goroutine 没起来: active=%d want=%d", mgr.Active(), want)
	}

	peak := runtime.NumGoroutine()
	if peak < base+int(want) {
		t.Fatalf("fix 模式应该真的起了 %d 个 goroutine: base=%d peak=%d", want, base, peak)
	}

	before, after := mgr.Stop()
	t.Logf("goroutines: base=%d peak=%d Stop前=%d Stop后=%d", base, peak, before, after)

	if got := mgr.Active(); got != 0 {
		t.Fatalf("Stop 之后不应还有存活 worker: active=%d", got)
	}
	if !waitFor(time.Second, func() bool { return runtime.NumGoroutine() <= base+3 }) {
		t.Fatalf("goroutine 未收敛: base=%d now=%d", base, runtime.NumGoroutine())
	}
}

// TestLeakTaskNeverStops 验证 bug 实现只会涨不会降。
// 注意：这个测试会（有意）在测试进程里留下泄漏的 goroutine，这正是 L03 想演示的现象。
func TestLeakTaskNeverStops(t *testing.T) {
	before := runtime.NumGoroutine()

	leakTask()
	leakTask()

	want := before + 2*leakPerStart
	if !waitFor(time.Second, func() bool { return runtime.NumGoroutine() >= want }) {
		t.Fatalf("leakTask 没有泄漏足够的 goroutine: before=%d now=%d want>=%d",
			before, runtime.NumGoroutine(), want)
	}

	// 再等一段时间，确认它不会自己降下来（这就是"泄漏"的定义）
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()
	t.Logf("goroutines: before=%d after=%d（泄漏 %d 个，不会回落）", before, after, 2*leakPerStart)

	if after < want {
		t.Fatalf("泄漏的 goroutine 不应该消失: before=%d after=%d", before, after)
	}
}
