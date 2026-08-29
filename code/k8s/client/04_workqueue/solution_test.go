package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testQueue 可控的测试队列：记录 Add 次数，提供 Done 计数信号。
type testQueue struct {
	mu       sync.Mutex
	items    []string
	adds     int
	gets     int
	dones    int
	shutdown bool
	notify   chan struct{} // 每次 Done 后触发一次
}

func newTestQueue(items ...string) *testQueue {
	q := &testQueue{notify: make(chan struct{}, 100)}
	q.items = append(q.items, items...)
	q.adds = len(items)
	return q
}

func (q *testQueue) Add(item string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.adds++
	q.items = append(q.items, item)
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

func (q *testQueue) Get() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.shutdown || len(q.items) == 0 {
		return "", true
	}
	item := q.items[0]
	q.items = q.items[1:]
	q.gets++
	return item, false
}

func (q *testQueue) Done(item string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.dones++
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

func (q *testQueue) ShutDown() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.shutdown = true
}

func (q *testQueue) stats() (adds, gets, dones int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.adds, q.gets, q.dones
}

func TestWorkersProcessAllItems(t *testing.T) {
	q := newTestQueue("a", "b", "c", "d", "e")
	var processed atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	StartWorkers(ctx, q, 3, func(item string) error {
		processed.Add(1)
		time.Sleep(10 * time.Millisecond)
		return nil
	}, &wg)

	// 等待 5 项全部 Done（Done 计数 = 初始项数）
	waitDone(t, q, 5, "5 个初始项全部处理")
	cancel()
	wg.Wait()

	if got := processed.Load(); got != 5 {
		t.Fatalf("应处理 5 项，实际 %d", got)
	}
	if adds, _, dones := q.stats(); dones != 5 {
		t.Fatalf("Done 应为 5（adds=%d dones=%d）", adds, dones)
	}
	t.Log("并发 worker 全部处理完成验证通过")
}

func TestWorkersRetryOnError(t *testing.T) {
	q := newTestQueue("bad")
	var attempts atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	StartWorkers(ctx, q, 1, func(item string) error {
		if attempts.Add(1) < 3 {
			return &errRetry{} // 前两次失败 → 重新入队
		}
		return nil
	}, &wg)

	// 失败会重新 Add：最终 adds = 1(初始) + 2(重试) = 3，dones = 3
	waitDone(t, q, 3, "失败项重试到成功")
	cancel()
	wg.Wait()

	if got := attempts.Load(); got != 3 {
		t.Fatalf("应重试到成功（3 次尝试），实际 %d", got)
	}
	t.Log("失败重试验证通过")
}

type errRetry struct{}

func (e *errRetry) Error() string { return "retry" }

func waitDone(t *testing.T, q *testQueue, wantDones int, msg string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, dones := q.stats(); dones >= wantDones {
			return
		}
		select {
		case <-q.notify:
		case <-time.After(20 * time.Millisecond):
		}
	}
	_, _, dones := q.stats()
	t.Fatalf("%s 超时：dones=%d want=%d", msg, dones, wantDones)
}
