package kafka_idempotent

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeConsumer struct {
	mu       sync.Mutex
	msgs     []*Message
	idx      int
	commits  []*Message
	commitFn func(*Message) error
}

func (f *fakeConsumer) Poll(ctx context.Context) (*Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.idx >= len(f.msgs) {
		return nil, io.EOF
	}
	m := f.msgs[f.idx]
	f.idx++
	return m, nil
}

func (f *fakeConsumer) Commit(ctx context.Context, m *Message) error {
	if f.commitFn != nil {
		if err := f.commitFn(m); err != nil {
			return err
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits = append(f.commits, m)
	return nil
}

func (f *fakeConsumer) committed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.commits)
}

type fakeDLQ struct {
	mu   sync.Mutex
	msgs []*Message
	err  error
}

func (d *fakeDLQ) Publish(ctx context.Context, m *Message, reason error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return d.err
	}
	d.msgs = append(d.msgs, m)
	return nil
}

func msg(off int64, val string) *Message {
	return &Message{Topic: "orders", Partition: 0, Offset: off, Value: []byte(val)}
}

func TestHandleSuccessCommits(t *testing.T) {
	fc := &fakeConsumer{}
	var calls atomic.Int32
	c := NewIdempotentConsumer(fc, func(context.Context, *Message) error {
		calls.Add(1)
		return nil
	}, NewMemoryDedup(), &fakeDLQ{}, Config{MaxAttempts: 3})

	if err := c.Handle(context.Background(), msg(1, "a")); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || fc.committed() != 1 {
		t.Fatalf("calls=%d commits=%d", calls.Load(), fc.committed())
	}
}

func TestDuplicateDeliveryProcessedOnce(t *testing.T) {
	fc := &fakeConsumer{}
	var calls atomic.Int32
	c := NewIdempotentConsumer(fc, func(context.Context, *Message) error {
		calls.Add(1)
		return nil
	}, NewMemoryDedup(), &fakeDLQ{}, Config{MaxAttempts: 3})

	m := msg(1, "a")
	_ = c.Handle(context.Background(), m)
	_ = c.Handle(context.Background(), m) // 重复投递
	_ = c.Handle(context.Background(), m)

	if calls.Load() != 1 {
		t.Fatalf("业务只应执行一次，实际 %d", calls.Load())
	}
	if fc.committed() != 3 {
		t.Fatalf("每次投递都应提交位点，实际 %d", fc.committed())
	}
	t.Log("重复消费被幂等表挡住 OK")
}

func TestCommitFailureRedeliversButNoDoubleEffect(t *testing.T) {
	var failCommit atomic.Bool
	failCommit.Store(true)
	fc := &fakeConsumer{commitFn: func(*Message) error {
		if failCommit.Load() {
			return errors.New("broker unavailable")
		}
		return nil
	}}
	var calls atomic.Int32
	c := NewIdempotentConsumer(fc, func(context.Context, *Message) error {
		calls.Add(1)
		return nil
	}, NewMemoryDedup(), &fakeDLQ{}, Config{MaxAttempts: 3})

	m := msg(7, "a")
	// 第一次：业务成功，但提交失败 → 返回错误，实际生产会重投。
	if err := c.Handle(context.Background(), m); err == nil {
		t.Fatal("提交失败应返回错误")
	}
	failCommit.Store(false)
	// 重投：幂等表已标记完成，不再执行业务，只补提交。
	if err := c.Handle(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("业务应只执行一次，实际 %d", calls.Load())
	}
	if fc.committed() != 1 {
		t.Fatalf("应成功提交 1 次，实际 %d", fc.committed())
	}
	t.Log("at-least-once + 幂等 = 业务恰好一次 OK")
}

func TestPoisonMessageGoesToDLQ(t *testing.T) {
	fc := &fakeConsumer{}
	dlq := &fakeDLQ{}
	var calls atomic.Int32
	c := NewIdempotentConsumer(fc, func(context.Context, *Message) error {
		calls.Add(1)
		return errors.New("bad payload")
	}, NewMemoryDedup(), dlq, Config{MaxAttempts: 3})

	if err := c.Handle(context.Background(), msg(9, "bad")); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("应重试 3 次，实际 %d", calls.Load())
	}
	if len(dlq.msgs) != 1 {
		t.Fatalf("应投递 1 条死信，实际 %d", len(dlq.msgs))
	}
	if fc.committed() != 1 {
		t.Fatalf("毒消息处理后必须提交位点，否则分区卡死；实际 %d", fc.committed())
	}
}

func TestDLQFailureDoesNotCommit(t *testing.T) {
	fc := &fakeConsumer{}
	dlq := &fakeDLQ{err: errors.New("dlq down")}
	c := NewIdempotentConsumer(fc, func(context.Context, *Message) error {
		return errors.New("bad")
	}, NewMemoryDedup(), dlq, Config{MaxAttempts: 1})

	if err := c.Handle(context.Background(), msg(1, "bad")); err == nil {
		t.Fatal("死信发送失败应返回错误")
	}
	if fc.committed() != 0 {
		t.Fatal("死信未成功时不能提交位点")
	}
}

func TestRunStopsOnContext(t *testing.T) {
	fc := &fakeConsumer{msgs: []*Message{msg(1, "a"), msg(2, "b")}}
	var calls atomic.Int32
	c := NewIdempotentConsumer(fc, func(context.Context, *Message) error {
		calls.Add(1)
		return nil
	}, NewMemoryDedup(), &fakeDLQ{}, Config{MaxAttempts: 1})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Run 在 EOF 时返回；两条应都被处理。
	if calls.Load() != 2 {
		t.Fatalf("应处理 2 条，实际 %d", calls.Load())
	}
}
