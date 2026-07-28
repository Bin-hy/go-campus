package cancelable_task

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestLongTask_Complete(t *testing.T) {
	ctx := context.Background()
	steps, err := LongTask(ctx, 5, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("不应有错误: %v", err)
	}
	if steps != 5 {
		t.Errorf("应完成5步，实际 %d", steps)
	}
}

func TestLongTask_Cancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()

	steps, err := LongTask(ctx, 10, 10*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("期望 DeadlineExceeded，得到 %v", err)
	}
	if steps < 2 || steps > 5 {
		t.Errorf("35ms 超时应完成2-4步，实际 %d", steps)
	}
}

func TestTaskWithCleanup_Normal(t *testing.T) {
	var cleaned int64
	err := TaskWithCleanup(
		context.Background(),
		func(ctx context.Context) error { return nil },
		func() { atomic.AddInt64(&cleaned, 1) },
	)
	if err != nil {
		t.Errorf("不应有错误: %v", err)
	}
	if atomic.LoadInt64(&cleaned) != 1 {
		t.Error("cleanup 应被调用恰好一次")
	}
}

func TestTaskWithCleanup_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var cleaned int64

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	err := TaskWithCleanup(
		ctx,
		func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		func() { atomic.AddInt64(&cleaned, 1) },
	)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("期望 Canceled，得到 %v", err)
	}
	if atomic.LoadInt64(&cleaned) != 1 {
		t.Error("取消后 cleanup 也应被调用恰好一次")
	}
}

func TestHeartbeat_Sends(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	hb := Heartbeat(ctx, 20*time.Millisecond)

	count := 0
	for range hb {
		count++
	}

	if count < 3 || count > 6 {
		t.Errorf("100ms 内 20ms 间隔应收到约4-5次心跳，实际 %d", count)
	}
}

func TestHeartbeat_StopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	hb := Heartbeat(ctx, 10*time.Millisecond)

	<-hb // 收到一次
	cancel()

	// channel 应该很快关闭
	time.Sleep(50 * time.Millisecond)
	_, ok := <-hb
	if ok {
		// 可能还有缓冲中的，再读一次
		for range hb {
		}
	}
}
