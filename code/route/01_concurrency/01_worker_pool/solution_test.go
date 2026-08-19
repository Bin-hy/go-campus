package worker_pool

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"
)

func TestRunPool_Basic(t *testing.T) {
	results, err := RunPool(context.Background(), []int{1, 2, 3, 4, 5, 6}, 3,
		func(ctx context.Context, j int) (int, error) { return j * j, nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Ints(results)
	want := []int{1, 4, 9, 16, 25, 36}
	for i := range want {
		if results[i] != want[i] {
			t.Fatalf("results = %v, want %v", results, want)
		}
	}
}

func TestRunPool_Empty(t *testing.T) {
	results, err := RunPool(context.Background(), nil, 3,
		func(ctx context.Context, j int) (int, error) { return j, nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %v", results)
	}
}

func TestRunPool_OneWorker(t *testing.T) {
	results, err := RunPool(context.Background(), []int{1, 2, 3}, 1,
		func(ctx context.Context, j int) (int, error) { return j * 10, nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Ints(results)
	if results[0] != 10 || results[1] != 20 || results[2] != 30 {
		t.Fatalf("unexpected results: %v", results)
	}
}

func TestRunPool_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := RunPool(ctx, []int{1, 2, 3, 4, 5, 6, 7, 8}, 3,
		func(ctx context.Context, j int) (int, error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return j, nil
			}
		})
	if err == nil {
		t.Fatal("expected context deadline error, got nil")
	}
}

func TestRunPool_FnError(t *testing.T) {
	boom := errors.New("boom")
	_, err := RunPool(context.Background(), []int{1, 2, 3}, 2,
		func(ctx context.Context, j int) (int, error) {
			if j == 2 {
				return 0, boom
			}
			return j, nil
		})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
