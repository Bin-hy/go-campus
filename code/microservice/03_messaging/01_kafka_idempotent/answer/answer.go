package kafka_idempotent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// 参考答案 / 可运行实现。
//
// 这里用接口抽象 Kafka，让语义可以离线验证；文档 10 章给出 kafka-go 的真实写法。

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

// Consumer 抽象"拉取 + 提交位点"。真实实现是 kafka-go Reader：
// FetchMessage 拉取，CommitMessages 手动提交。
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

// DedupStore 幂等去重表（生产为 MySQL 唯一索引 / Redis SETNX）。
// 用 TryBegin/Commit/Abort 三态，避免"处理到一半崩溃"被误判为已完成。
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

// Run 消费循环：处理一条、提交一条。
// ctx 取消时返回 nil（优雅停机）；Poll 返回 io.EOF 时也返回 nil。
func (c *IdempotentConsumer) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		msg, err := c.consumer.Poll(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		if msg == nil {
			continue
		}
		c.Handle(ctx, msg)
	}
}

// Handle 处理一条消息（导出以便单测）：
//  1. 先做幂等占位：已处理过 → 直接提交位点；
//  2. 重试 handler；成功 → 标记完成 → 提交位点；
//  3. 超过重试 → 投递死信 → 提交位点（毒消息不能卡住分区）。
func (c *IdempotentConsumer) Handle(ctx context.Context, msg *Message) error {
	key := msg.ID()
	fresh, err := c.dedup.TryBegin(key)
	if err != nil {
		return err
	}
	if !fresh {
		// 重复投递（例如"处理成功但提交位点失败"），直接提交即可。
		return c.consumer.Commit(ctx, msg)
	}

	var lastErr error
	for attempt := 0; attempt < c.cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			_ = c.dedup.Abort(key) // 让下次重投还能重新处理
			return err
		}
		lastErr = c.handler(ctx, msg)
		if lastErr == nil {
			if err := c.dedup.Commit(key); err != nil {
				_ = c.dedup.Abort(key)
				return err // 不提交位点，让 Kafka 重投
			}
			return c.consumer.Commit(ctx, msg)
		}
		if c.cfg.RetryDelay > 0 {
			t := time.NewTimer(c.cfg.RetryDelay)
			select {
			case <-ctx.Done():
				t.Stop()
				_ = c.dedup.Abort(key)
				return ctx.Err()
			case <-t.C:
			}
		}
	}

	// 毒消息：进死信，然后提交位点，分区才能继续往前走。
	_ = c.dedup.Abort(key)
	if c.dlq != nil {
		if err := c.dlq.Publish(ctx, msg, lastErr); err != nil {
			return err // 死信都发不出去：不提交，等待重投
		}
	}
	return c.consumer.Commit(ctx, msg)
}

// MemoryDedup 内存版去重表。
type MemoryDedup struct {
	mu      sync.Mutex
	state   map[string]int // 1=processing 2=done
	err     error
	aborted int
}

func NewMemoryDedup() *MemoryDedup {
	return &MemoryDedup{state: make(map[string]int)}
}

func (d *MemoryDedup) TryBegin(key string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return false, d.err
	}
	if _, ok := d.state[key]; ok {
		return false, nil
	}
	d.state[key] = 1
	return true, nil
}

func (d *MemoryDedup) Commit(key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state[key] = 2
	return nil
}

func (d *MemoryDedup) Abort(key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.state, key)
	d.aborted++
	return nil
}

func (d *MemoryDedup) Done(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state[key] == 2
}
