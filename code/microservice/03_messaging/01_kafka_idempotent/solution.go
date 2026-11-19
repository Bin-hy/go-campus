package kafka_idempotent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Message 是消费到的一条消息。
type Message struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
}

func (m Message) ID() string {
	return fmt.Sprintf("%s/%d/%d", m.Topic, m.Partition, m.Offset)
}

// Consumer 抽象"拉取 + 提交位点"。真实实现是 kafka-go Reader。
type Consumer interface {
	Poll(ctx context.Context) (*Message, error)
	Commit(ctx context.Context, m *Message) error
}

// Handler 业务处理。返回 nil 才会提交位点。
type Handler func(ctx context.Context, m *Message) error

// DeadLetter 死信队列：毒消息进这里，人工排查，不阻塞分区。
type DeadLetter interface {
	Publish(ctx context.Context, m *Message, reason error) error
}

// DedupStore 幂等去重表。
// TryBegin/Commit/Abort 三态，避免"处理到一半崩溃"被误判为已完成。
type DedupStore interface {
	TryBegin(key string) (fresh bool, err error)
	Commit(key string) error
	Abort(key string) error
}

// Config 消费端参数。
type Config struct {
	MaxAttempts int           // 单条消息最多处理几次，超过进死信
	RetryDelay  time.Duration // 每次重试间隔
}

// IdempotentConsumer 至少一次消费 + 业务幂等 + 手动提交 + 死信。
type IdempotentConsumer struct {
	consumer Consumer
	handler  Handler
	dedup    DedupStore
	dlq      DeadLetter
	cfg      Config
}

func NewIdempotentConsumer(c Consumer, h Handler, d DedupStore, dlq DeadLetter, cfg Config) *IdempotentConsumer {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	return &IdempotentConsumer{consumer: c, handler: h, dedup: d, dlq: dlq, cfg: cfg}
}

// Run 消费循环：处理一条、提交一条；ctx 取消或 Poll 返回 io.EOF 时返回 nil。
func (c *IdempotentConsumer) Run(ctx context.Context) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Handle 处理一条消息（导出以便单测）：
//  1. 先做幂等占位：已处理过 → 直接提交位点；
//  2. 重试 handler；成功 → 标记完成 → 提交位点；
//  3. 超过重试 → 投递死信 → 提交位点（毒消息不能卡住分区）。
func (c *IdempotentConsumer) Handle(ctx context.Context, msg *Message) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// MemoryDedup 内存版去重表。
type MemoryDedup struct {
	mu      sync.Mutex
	state   map[string]int // 1=processing 2=done
	aborted int
}

func NewMemoryDedup() *MemoryDedup {
	return &MemoryDedup{state: make(map[string]int)}
}

func (d *MemoryDedup) TryBegin(key string) (bool, error) {
	// TODO: 实现你的代码（已存在返回 false）
	panic("not implemented")
}

func (d *MemoryDedup) Commit(key string) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

func (d *MemoryDedup) Abort(key string) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

func (d *MemoryDedup) Done(key string) bool {
	// TODO: 实现你的代码
	panic("not implemented")
}
