# 幂等消费：同 key 重复只处理一次

## 难度：⭐⭐ 中等

## 考点
- 消费端幂等（去重表）
- at-least-once 下用幂等消化重复

## 环境准备

```bash
cd code/backend && docker compose up -d kafka
```

## 题目描述

实现 `ConsumeIdempotent`：生产两条**同 key** 的消息（模拟重复），消费两条，用"去重表"（map 记录已处理的 key）保证同 key 只处理一次。返回实际处理次数（应为 1）。

## 函数签名

```go
func ConsumeIdempotent(ctx context.Context, broker, topic string) (handled int, err error)
```

## 提示

1. 生产两条 `kafka.Message{Key: []byte("order-1"), Value: []byte("pay")}`
2. 消费两条，用 `map[string]bool` 记录已处理的 key
3. 遇到未见过的 key 才 `handled++`
4. 返回 handled

## 运行测试

```bash
cd code/backend/03-kafka/04_idempotent && go test -v
```
