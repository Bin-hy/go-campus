# 事务性 Outbox 与死信队列：事件最终一定发出去

## 难度：⭐⭐⭐⭐ 困难

## 考点
- 双写问题：写库成功 + 发 MQ 失败 → 事件丢失
- Outbox：业务数据与事件在**同一个本地事务**里写入；relay 异步搬运
- at-least-once：发布成功但标记失败会重复发；消费端必须幂等
- 重试上限 + 死信：毒事件不能永远卡住 relay
- 真实对应：`outbox` 表 + 定时 relay / CDC（Debezium）

## 题目描述

实现 `MemoryStore` 与 `Relay`：

1. `CreateOrderWithEvent`：原子地写订单 + outbox（空事件 ID 模拟事务失败，必须都不落库）；
2. `Relay.RunOnce`：拉未发送事件 → 发布 → `MarkSent`；
3. 发布失败累计次数；达到 `maxAttempts` → 死信 + `MarkDead`；
4. 单条失败不阻塞同批其他事件；`ctx` 取消立即停止。

## 函数签名

```go
type Event struct { ID, Topic string; Payload []byte; CreatedAt time.Time; Attempts int; Dead bool }
type Publisher interface { Publish(ctx context.Context, e Event) error }
type DeadLetter interface { Publish(ctx context.Context, e Event, reason error) error }
type Store interface {
    CreateOrderWithEvent(ctx context.Context, orderID string, e Event) error
    FetchUnsent(ctx context.Context, limit int) ([]Event, error)
    MarkSent(ctx context.Context, id string) error
    MarkDead(ctx context.Context, id string, attempts int) error
    RecordAttempt(ctx context.Context, id string, attempts int) error
    OrderExists(orderID string) bool
}

func NewRelay(store Store, pub Publisher, dlq DeadLetter, batch, maxAttempts int) *Relay
func (r *Relay) RunOnce(ctx context.Context) (int, error)
```

## 提示

1. 生产里 `CreateOrderWithEvent` 的两次 INSERT 必须在同一个数据库事务里（本题目用锁模拟）。
2. `MarkSent` 放在 `Publish` 之后；顺序反过来会丢事件。
3. `MarkSent` 失败 = 崩溃窗口，下次会重复发——这是 **at-least-once 的代价**，不是 bug。
4. 死信失败时**不要** MarkDead，否则事件无声消失。

## 验收

- [ ] 事务失败时订单和事件都不存在
- [ ] 3 条事件全部发布，发完 `FetchUnsent` 为空
- [ ] 某条持续失败 2 次后进死信，其他事件不受影响
- [ ] 模拟 MarkSent 失败 → 下轮重复发布（观察至少一次）
- [ ] `go test -race ./03_messaging/02_outbox_dlq`

## 运行

```bash
cd code/microservice
go test -race -v ./03_messaging/02_outbox_dlq
```

## 面试追问

1. **Outbox 和"先发 MQ 再写库"有什么区别？** 后者如果 MQ 成功、写库失败，消费者会看到"不存在的订单"；Outbox 保证事件只在业务成功后存在。
2. **relay 怎么保证顺序？** 按聚合 ID 分区 + 单线程搬运；跨聚合可并行。
3. **Outbox 表越来越大怎么办？** 定期归档已发送行；或用 CDC 读取 binlog，避免轮询。
4. **能不能用分布式事务替代？** 2PC 成本高、可用性差；Outbox + 幂等是互联网主流最终一致方案。
