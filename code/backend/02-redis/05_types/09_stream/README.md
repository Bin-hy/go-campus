# 09 Stream：订单事件流 + 消费组

## 难度：⭐⭐⭐ 进阶

## 考点
- `XADD`：追加消息（自动生成 ID）
- `XGROUP CREATE`：创建消费组
- `XREADGROUP ... >`：组内消费新消息
- `XACK`：确认消费（Pending 机制，消息不丢的关键）

## 题目描述

围绕 stream `order:events`：

1. `PublishOrderEvent`：追加一条订单事件（字段 `orderID`、`type`），返回消息 ID。
2. `EnsureGroup`：为 stream 创建消费组（从头 `0` 开始消费）；组已存在时忽略 BUSYGROUP 错误。
3. `ConsumeOne`：以组内消费者身份读一条新消息并 XACK，返回 (消息ID, 字段map)。

## 函数签名

```go
func PublishOrderEvent(ctx context.Context, c *redis.Client, orderID int64, eventType string) (string, error)
func EnsureGroup(ctx context.Context, c *redis.Client, group string) error
func ConsumeOne(ctx context.Context, c *redis.Client, group, consumer string) (string, map[string]string, error)
```

## 提示

1. `c.XAdd(ctx, &redis.XAddArgs{Stream: key, Values: map[string]any{...}})`
2. 判断 BUSYGROUP：`strings.Contains(err.Error(), "BUSYGROUP")`
3. `c.XReadGroup(ctx, &redis.XReadGroupArgs{Group: g, Consumer: c1, Streams: []string{key, ">"}, Count: 1, Block: 2*time.Second})`，`>` 表示「只读新消息」
4. 返回值是 `[]redis.XStream`，消息在 `Streams[0].Messages[0]`
5. 读完必须 `c.XAck(ctx, key, group, id)`，否则消息留在 Pending 列表
6. 思考：对比 02_list 的队列，Stream 的消费组 + ACK 解决了什么问题？

## 运行测试

```bash
cd code/backend && docker compose up -d redis
cd 02-redis/05_types/09_stream && go test -v
```
