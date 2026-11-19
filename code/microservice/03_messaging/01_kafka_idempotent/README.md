# Kafka 可靠消费：手动提交 + 幂等表 + 重试 + 死信

## 难度：⭐⭐⭐⭐ 困难

## 考点
- at-least-once：先处理业务、再提交位点；崩溃发生在两者之间会重投
- 幂等表三态（processing / done / abort），避免"处理一半"被误判完成
- 重复投递的根因：提交失败、rebalance、producer 重试
- 毒消息：重试上限后进死信并**提交位点**，否则分区卡住
- 死信发送失败时不能提交（否则消息彻底丢失）
- 真实对应：kafka-go `FetchMessage` + `CommitMessages`，MySQL 唯一索引去重

## 题目描述

用接口抽象 Kafka，实现 `IdempotentConsumer.Handle(ctx, msg)`：

1. `TryBegin(msg.ID())` 占位；已处理过 → 直接 `Commit`；
2. 调 handler，失败重试 `MaxAttempts` 次；
3. 成功 → `Commit` 幂等表 → `Commit` Kafka 位点；
4. 超过上限 → 投死信 → 提交位点；
5. ctx 取消时 `Abort` 占位并返回。

## 函数签名

```go
type Message struct { Topic string; Partition int; Offset int64; Key, Value []byte }
func (m Message) ID() string // "topic/partition/offset"

type Consumer interface {
    Poll(ctx context.Context) (*Message, error)
    Commit(ctx context.Context, m *Message) error
}
type Handler func(ctx context.Context, m *Message) error
type DeadLetter interface { Publish(ctx context.Context, m *Message, reason error) error }
type DedupStore interface {
    TryBegin(key string) (fresh bool, err error)
    Commit(key string) error
    Abort(key string) error
}

func NewIdempotentConsumer(c Consumer, h Handler, d DedupStore, dlq DeadLetter, cfg Config) *IdempotentConsumer
func (c *IdempotentConsumer) Handle(ctx context.Context, msg *Message) error
func (c *IdempotentConsumer) Run(ctx context.Context) error
```

## 提示

1. `TryBegin` 必须是原子的（Redis SETNX / MySQL `INSERT ... ON DUPLICATE KEY`）。
2. **不要**先提交位点再处理：崩溃会丢消息。
3. `Run` 遇到 `io.EOF` 或 ctx 取消应正常返回 nil。
4. `Handle` 里任何"返回 error 且不提交"的路径，都等于让 Kafka 重投——想清楚是不是真的想重投。

## 验收

- [ ] 正常消息处理 1 次并提交
- [ ] 同一条消息投递 3 次，业务只执行 1 次，位点提交 3 次
- [ ] 业务成功但提交失败 → 重投时业务不重复执行，只补提交
- [ ] 持续失败 3 次 → 死信 1 条 + 提交位点
- [ ] 死信失败 → 不提交位点
- [ ] `go test -race ./03_messaging/01_kafka_idempotent`

## 运行

```bash
cd code/microservice
go test -race -v ./03_messaging/01_kafka_idempotent
```

## 真实 Kafka 写法要点（kafka-go）

```go
r := kafka.NewReader(kafka.ReaderConfig{
    Brokers: []string{"127.0.0.1:9092"},
    GroupID: "order-consumer",
    Topic:   "order.created",
    CommitInterval: 0, // 关闭自动提交 → 手动提交
})
for {
    m, err := r.FetchMessage(ctx)
    if err != nil { break }
    if err := handle(ctx, m); err != nil { /* 重试/死信 */ continue }
    _ = r.CommitMessages(ctx, m) // 处理成功后才提交
}
```

## 面试追问

1. **Kafka 能不能做到 exactly-once？** 生产端幂等 + 事务可以做到"流内 EOS"；跨外部副作用（扣库存、发通知）只能靠幂等表实现业务恰好一次。
2. **为什么位点提交放在处理之后？** 反过来会丢消息；放之后最多重复，重复由幂等表兜住。
3. **rebalance 会带来什么？** 分区换消费者，未提交的消息会重投；处理慢会导致反复 rebalance，要控制 `max.poll.interval`。
