# 顺序消费：单分区内有序

## 难度：⭐⭐ 中等

## 考点
- Kafka 单分区有序性
- 单分区生产消费的顺序保持

## 环境准备

```bash
cd code/backend && docker compose up -d kafka
```

## 题目描述

实现 `ProduceConsumeOrdered`：把 `seq` 里的消息依次生产到**单分区** topic，再消费回来，返回消费到的顺序。单分区内消费顺序应等于生产顺序。

## 函数签名

```go
func ProduceConsumeOrdered(ctx context.Context, broker, topic string, seq []string) ([]string, error)
```

## 提示

1. Writer 不指定 Balancer（默认单分区），或显式用 `&kafka.Hash{}`
2. 依次生产 `seq` 里的消息
3. Reader 用唯一 GroupID，依次 `ReadMessage` 收集 `string(m.Value)`
4. 返回消费顺序

## 运行测试

```bash
cd code/backend/03-kafka/03_order && go test -v
```
