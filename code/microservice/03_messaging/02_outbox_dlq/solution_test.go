package outbox_dlq

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakePublisher struct {
	mu      sync.Mutex
	sent    []string
	counts  map[string]int
	failIDs map[string]bool
}

func newFakePublisher() *fakePublisher {
	return &fakePublisher{counts: make(map[string]int), failIDs: make(map[string]bool)}
}

func (p *fakePublisher) Publish(ctx context.Context, e Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failIDs[e.ID] {
		return errors.New("kafka unavailable")
	}
	p.sent = append(p.sent, e.ID)
	p.counts[e.ID]++
	return nil
}

func (p *fakePublisher) total(id string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.counts[id]
}

type fakeDLQ struct {
	mu   sync.Mutex
	msgs []string
}

func (d *fakeDLQ) Publish(ctx context.Context, e Event, reason error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.msgs = append(d.msgs, e.ID)
	return nil
}

func ev(id string) Event { return Event{ID: id, Topic: "order.created", Payload: []byte(id)} }

func TestOutboxIsAtomicWithBusinessWrite(t *testing.T) {
	s := NewMemoryStore()
	// 事件 ID 为空 → 模拟事务提交前失败：订单和事件都不应落库。
	if err := s.CreateOrderWithEvent(context.Background(), "order-1", Event{}); !errors.Is(err, ErrTxAborted) {
		t.Fatalf("应返回 ErrTxAborted，实际 %v", err)
	}
	if s.OrderExists("order-1") {
		t.Fatal("事务失败时订单不应存在")
	}
	if rows, _ := s.FetchUnsent(context.Background(), 10); len(rows) != 0 {
		t.Fatal("事务失败时事件不应存在")
	}
}

func TestRelayPublishesAll(t *testing.T) {
	s := NewMemoryStore()
	for i := 1; i <= 3; i++ {
		if err := s.CreateOrderWithEvent(context.Background(), "order-"+string(rune('0'+i)), ev("e"+string(rune('0'+i)))); err != nil {
			t.Fatal(err)
		}
	}
	p := newFakePublisher()
	r := NewRelay(s, p, &fakeDLQ{}, 10, 3)
	sent, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sent != 3 {
		t.Fatalf("应发送 3 条，实际 %d", sent)
	}
	if rows, _ := s.FetchUnsent(context.Background(), 10); len(rows) != 0 {
		t.Fatalf("发送后不应有未发送事件，实际 %d", len(rows))
	}
}

func TestRelayRetriesThenDeadLetters(t *testing.T) {
	s := NewMemoryStore()
	_ = s.CreateOrderWithEvent(context.Background(), "o1", ev("e1"))
	_ = s.CreateOrderWithEvent(context.Background(), "o2", ev("e2"))
	p := newFakePublisher()
	p.failIDs["e2"] = true
	dlq := &fakeDLQ{}
	r := NewRelay(s, p, dlq, 10, 2)

	// 第 1 轮：e1 成功；e2 失败 1 次。
	if _, err := r.RunOnce(context.Background()); err == nil {
		t.Fatal("e2 失败应返回错误")
	}
	// 第 2 轮：e2 第 2 次失败 → 超过上限 → 死信（不再返回错误，避免卡住队列）。
	if _, err := r.RunOnce(context.Background()); err != nil {
		t.Fatalf("死信后应返回 nil，实际 %v", err)
	}
	if len(dlq.msgs) != 1 || dlq.msgs[0] != "e2" {
		t.Fatalf("e2 应进死信，实际 %v", dlq.msgs)
	}
	// 死信后不再阻塞后续事件。
	if rows, _ := s.FetchUnsent(context.Background(), 10); len(rows) != 0 {
		t.Fatalf("死信后不应再有未发送事件，实际 %d", len(rows))
	}
	if p.total("e1") != 1 {
		t.Fatalf("e1 不应重复发送，实际 %d", p.total("e1"))
	}
}

// flakyStore 模拟"发布成功但标记已发送失败"的崩溃窗口。
type flakyStore struct {
	*MemoryStore
	failMark bool
}

func (f *flakyStore) MarkSent(ctx context.Context, id string) error {
	if f.failMark {
		return errors.New("db commit failed")
	}
	return f.MemoryStore.MarkSent(ctx, id)
}

func TestAtLeastOnceOnCrashWindow(t *testing.T) {
	base := NewMemoryStore()
	s := &flakyStore{MemoryStore: base, failMark: true}
	_ = s.CreateOrderWithEvent(context.Background(), "o1", ev("e1"))

	p := newFakePublisher()
	r := NewRelay(s, p, &fakeDLQ{}, 10, 3)

	if _, err := r.RunOnce(context.Background()); err == nil {
		t.Fatal("MarkSent 失败应返回错误")
	}
	if p.total("e1") != 1 {
		t.Fatal("第一次应已发布")
	}
	// 恢复后重跑：因为位点没标记，会再发一次（至少一次）。
	s.failMark = false
	if _, err := r.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p.total("e1") != 2 {
		t.Fatalf("应观察到达 least-once 的重复，实际 %d 次", p.total("e1"))
	}
	t.Log("崩溃窗口导致重复发送，消费端必须幂等（对应上一题）")
}

func TestRelayHonorsContext(t *testing.T) {
	s := NewMemoryStore()
	_ = s.CreateOrderWithEvent(context.Background(), "o1", ev("e1"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := newFakePublisher()
	r := NewRelay(s, p, &fakeDLQ{}, 10, 3)
	if _, err := r.RunOnce(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("应返回 context.Canceled，实际 %v", err)
	}
}

func TestRelayRunStops(t *testing.T) {
	s := NewMemoryStore()
	p := newFakePublisher()
	r := NewRelay(s, p, &fakeDLQ{}, 10, 3)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	r.Run(ctx, 5*time.Millisecond)
}
