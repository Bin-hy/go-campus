package task_pipeline

import (
	"context"
	"errors"
	"sync"
	"testing"
)

var errFlaky = errors.New("flaky")

func TestPipeline_Basic(t *testing.T) {
	ctx := context.Background()
	p := NewPipeline(ctx, 3, 1, func(ctx context.Context, t Task) error { return nil })
	for i := 0; i < 6; i++ {
		if err := p.Submit(ctx, Task{ID: string(rune('a' + i))}); err != nil {
			t.Fatal(err)
		}
	}
	p.Close()

	got := map[string]bool{}
	for r := range p.Results() {
		if r.Err != nil {
			t.Fatalf("task %s err = %v", r.Task.ID, r.Err)
		}
		got[r.Task.ID] = true
	}
	if len(got) != 6 {
		t.Fatalf("results = %d, want 6", len(got))
	}
}

func TestPipeline_RetryThenSuccess(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	attempts := map[string]int{}
	p := NewPipeline(ctx, 1, 2, func(ctx context.Context, t Task) error {
		mu.Lock()
		attempts[t.ID]++
		n := attempts[t.ID]
		mu.Unlock()
		if n < 3 { // 前两次失败，第三次成功
			return errFlaky
		}
		return nil
	})
	_ = p.Submit(ctx, Task{ID: "t1"})
	p.Close()

	got := 0
	for r := range p.Results() {
		if r.Err != nil {
			t.Fatalf("err = %v", r.Err)
		}
		got++
	}
	if got != 1 {
		t.Fatalf("results = %d, want 1", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if attempts["t1"] != 3 {
		t.Fatalf("attempts = %d, want 3 (首次 + maxRetry=2 次重试)", attempts["t1"])
	}
}

func TestPipeline_ExhaustRetries(t *testing.T) {
	ctx := context.Background()
	p := NewPipeline(ctx, 2, 1, func(ctx context.Context, t Task) error { return errFlaky })
	_ = p.Submit(ctx, Task{ID: "x"})
	_ = p.Submit(ctx, Task{ID: "y"})
	p.Close()

	n := 0
	for r := range p.Results() {
		if r.Err == nil {
			t.Fatalf("expected error for %s", r.Task.ID)
		}
		n++
	}
	if n != 2 {
		t.Fatalf("results = %d, want 2", n)
	}
}

func TestPipeline_SubmitAfterClose(t *testing.T) {
	ctx := context.Background()
	p := NewPipeline(ctx, 1, 0, func(ctx context.Context, t Task) error { return nil })
	p.Close()
	if err := p.Submit(ctx, Task{ID: "z"}); err != ErrClosed {
		t.Fatalf("err = %v, want ErrClosed", err)
	}
}

func TestPipeline_ContextCancelSubmit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := NewPipeline(ctx, 1, 0, func(ctx context.Context, t Task) error { return nil })
	_ = p.Submit(ctx, Task{ID: "a"})
	cancel()
	if err := p.Submit(ctx, Task{ID: "b"}); err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	p.Close()

	// 已提交的 a 仍会被处理
	found := false
	for r := range p.Results() {
		if r.Task.ID == "a" {
			found = true
		}
	}
	if !found {
		t.Fatal("task a not processed")
	}
}
