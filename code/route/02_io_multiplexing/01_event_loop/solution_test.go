package event_loop

import (
	"sync"
	"testing"
)

func TestLoop_BasicDispatch(t *testing.T) {
	l := NewLoop()
	var mu sync.Mutex
	called := map[int]int{}
	for _, fd := range []int{1, 2, 3} {
		fd := fd
		l.Add(fd, func(f int) {
			mu.Lock()
			called[f]++
			mu.Unlock()
		}, false)
	}
	l.Notify(1)
	l.Notify(2)
	l.Notify(3)
	l.Process()

	mu.Lock()
	defer mu.Unlock()
	for _, fd := range []int{1, 2, 3} {
		if called[fd] != 1 {
			t.Fatalf("fd %d called %d times, want 1", fd, called[fd])
		}
	}
}

func TestLoop_LT_ConsumeStopsRetrigger(t *testing.T) {
	l := NewLoop()
	calls := 0
	l.Add(7, func(fd int) {
		calls++
		l.Consume(fd) // LT：消费后不再触发
	}, false)
	l.Notify(7)
	l.Process()
	l.Process() // 已 Consume，不应再触发
	if calls != 1 {
		t.Fatalf("LT calls = %d, want 1 (consumed after first)", calls)
	}
}

func TestLoop_LT_PersistWithoutConsume(t *testing.T) {
	l := NewLoop()
	calls := 0
	l.Add(7, func(int) { calls++ }, false) // 不 Consume → 保持就绪
	l.Notify(7)
	l.Process()
	l.Process()
	if calls != 2 {
		t.Fatalf("LT calls = %d, want 2 (persist until consume)", calls)
	}
}

func TestLoop_ET_TriggerOnce(t *testing.T) {
	l := NewLoop()
	calls := 0
	l.Add(8, func(int) { calls++ }, true) // ET：只触发一次
	l.Notify(8)
	l.Process()
	l.Process()
	if calls != 1 {
		t.Fatalf("ET calls = %d, want 1", calls)
	}
}

func TestLoop_RemoveStopsDispatch(t *testing.T) {
	l := NewLoop()
	calls := 0
	l.Add(9, func(int) { calls++ }, false)
	l.Remove(9)
	l.Notify(9)
	l.Process()
	if calls != 0 {
		t.Fatalf("removed fd still dispatched: calls = %d", calls)
	}
}

func TestLoop_NotifyUnregisteredIgnored(t *testing.T) {
	l := NewLoop()
	l.Notify(99) // 未注册，不应 panic
	l.Process()
}

func TestLoop_EmptyProcess(t *testing.T) {
	l := NewLoop()
	l.Process() // 不应 panic
}
