package outbox_dlq

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// 参考答案 / 可运行实现。
//
// 问题：订单写库成功，但发 Kafka 失败，事件就永远丢了。
// Outbox：把"要发的事件"和"业务数据"放在同一个本地事务里写进同一张表，
// 再由 relay 异步搬运到 Kafka。业务库是唯一事实来源，事件最终一定发出去。

// Event 是待投递的领域事件。
type Event struct {
	ID        string
	Topic     string
	Payload   []byte
	CreatedAt time.Time
	Attempts  int
	Dead      bool
}

// Publisher 抽象 Kafka producer（生产中要求 acks=all + 幂等 producer）。
type Publisher interface {
	Publish(ctx context.Context, e Event) error
}

// DeadLetter 抽象死信队列。
type DeadLetter interface {
	Publish(ctx context.Context, e Event, reason error) error
}

// Store 抽象"业务表 + outbox 表"。
type Store interface {
	// CreateOrderWithEvent 在一个本地事务里写订单和事件：
	// 要么都成功，要么都不写（这是 Outbox 成立的前提）。
	CreateOrderWithEvent(ctx context.Context, orderID string, e Event) error
	FetchUnsent(ctx context.Context, limit int) ([]Event, error)
	MarkSent(ctx context.Context, id string) error
	MarkDead(ctx context.Context, id string, attempts int) error
	RecordAttempt(ctx context.Context, id string, attempts int) error
	OrderExists(orderID string) bool
}

// MemoryStore 内存实现，用一把锁模拟事务的原子性。
type MemoryStore struct {
	mu     sync.Mutex
	orders map[string]bool
	rows   map[string]*Event
	order  []string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{orders: make(map[string]bool), rows: make(map[string]*Event)}
}

// ErrTxAborted 模拟事务提交前失败。
var ErrTxAborted = errors.New("transaction aborted")

func (s *MemoryStore) CreateOrderWithEvent(ctx context.Context, orderID string, e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.ID == "" {
		return ErrTxAborted
	}
	// 先写内存副本，任何一步失败都不落库（模拟回滚）。
	s.orders[orderID] = true
	e.CreatedAt = time.Now()
	s.rows[e.ID] = &e
	s.order = append(s.order, e.ID)
	return nil
}

func (s *MemoryStore) FetchUnsent(ctx context.Context, limit int) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Event
	for _, id := range s.order {
		row := s.rows[id]
		if row == nil || row.Dead || row.Attempts < 0 {
			continue
		}
		if !s.isSent(id) {
			out = append(out, *row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// sent 用单独集合表示，避免把"已发送"混进事件本身。
func (s *MemoryStore) isSent(id string) bool {
	row, ok := s.rows[id]
	return !ok || row.Attempts == -1
}

func (s *MemoryStore) MarkSent(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row, ok := s.rows[id]; ok {
		row.Attempts = -1
	}
	return nil
}

func (s *MemoryStore) MarkDead(ctx context.Context, id string, attempts int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row, ok := s.rows[id]; ok {
		row.Dead = true
		row.Attempts = attempts
	}
	return nil
}

func (s *MemoryStore) RecordAttempt(ctx context.Context, id string, attempts int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row, ok := s.rows[id]; ok {
		row.Attempts = attempts
	}
	return nil
}

func (s *MemoryStore) OrderExists(orderID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.orders[orderID]
}

// Relay 把 outbox 里的事件搬运到 Kafka。
// 崩溃恢复后继续搬（原子性靠业务库，重复靠消费端幂等）。
type Relay struct {
	store       Store
	pub         Publisher
	dlq         DeadLetter
	batch       int
	maxAttempts int
}

func NewRelay(store Store, pub Publisher, dlq DeadLetter, batch, maxAttempts int) *Relay {
	if batch <= 0 {
		batch = 100
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	return &Relay{store: store, pub: pub, dlq: dlq, batch: batch, maxAttempts: maxAttempts}
}

// RunOnce 拉一批未发送事件并投递，返回成功条数。
// 单条失败不会卡住整批：记录尝试次数，超过上限转死信。
func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	events, err := r.store.FetchUnsent(ctx, r.batch)
	if err != nil {
		return 0, err
	}
	sent := 0
	var firstErr error
	for _, e := range events {
		if err := ctx.Err(); err != nil {
			return sent, err
		}
		if err := r.pub.Publish(ctx, e); err != nil {
			attempts := e.Attempts + 1
			if attempts >= r.maxAttempts {
				if r.dlq != nil {
					if derr := r.dlq.Publish(ctx, e, err); derr != nil {
						_ = r.store.RecordAttempt(ctx, e.ID, attempts)
						if firstErr == nil {
							firstErr = derr
						}
						continue
					}
				}
				_ = r.store.MarkDead(ctx, e.ID, attempts)
				continue
			}
			_ = r.store.RecordAttempt(ctx, e.ID, attempts)
			if firstErr == nil {
				firstErr = fmt.Errorf("publish %s: %w", e.ID, err)
			}
			continue
		}
		// 注意：Publish 成功但 MarkSent 失败 = 至少一次，消费端必须幂等。
		if err := r.store.MarkSent(ctx, e.ID); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		sent++
	}
	return sent, firstErr
}

// Run 常驻循环（生产中由定时任务或单独进程驱动）。
func (r *Relay) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = r.RunOnce(ctx)
		}
	}
}
