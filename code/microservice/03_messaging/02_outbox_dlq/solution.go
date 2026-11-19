package outbox_dlq

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Event 是待投递的领域事件。
type Event struct {
	ID        string
	Topic     string
	Payload   []byte
	CreatedAt time.Time
	Attempts  int
	Dead      bool
}

// Publisher 抽象 Kafka producer。
type Publisher interface {
	Publish(ctx context.Context, e Event) error
}

// DeadLetter 抽象死信队列。
type DeadLetter interface {
	Publish(ctx context.Context, e Event, reason error) error
}

// Store 抽象"业务表 + outbox 表"。
type Store interface {
	// CreateOrderWithEvent 在一个本地事务里写订单和事件。
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

// CreateOrderWithEvent 原子地写订单 + outbox（事件 ID 为空模拟事务失败）。
func (s *MemoryStore) CreateOrderWithEvent(ctx context.Context, orderID string, e Event) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// FetchUnsent 返回未发送、未进死信的事件，按创建时间排序。
func (s *MemoryStore) FetchUnsent(ctx context.Context, limit int) ([]Event, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// MarkSent 标记已发送（提示：用一个字段表示 sent，避免再被 FetchUnsent 取到）。
func (s *MemoryStore) MarkSent(ctx context.Context, id string) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// MarkDead 标记为死信。
func (s *MemoryStore) MarkDead(ctx context.Context, id string, attempts int) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// RecordAttempt 记录一次失败尝试。
func (s *MemoryStore) RecordAttempt(ctx context.Context, id string, attempts int) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

func (s *MemoryStore) OrderExists(orderID string) bool {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Relay 把 outbox 里的事件搬运到 Kafka。
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
// 提示：先 Publish 再 MarkSent；失败累计次数，达上限转死信。
func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Run 常驻循环。
func (r *Relay) Run(ctx context.Context, interval time.Duration) {
	// TODO: 实现你的代码
	panic("not implemented")
}
