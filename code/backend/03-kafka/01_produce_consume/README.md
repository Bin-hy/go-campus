# Kafka 生产消费：发 3 条收 3 条

## 难度：⭐⭐ 中等

## 考点
- kafka-go 的 Writer/Reader 基本用法
- acks 配置与消息收发

## 环境准备

```bash
cd code/backend && docker compose up -d kafka
```

## 题目描述

实现 `ProduceAndConsume`：把 `msgs` 里的消息依次生产到指定 topic，再消费回来，返回消费到的消息集合（value → true）。

## 函数签名

```go
func ProduceAndConsume(ctx context.Context, broker, topic string, msgs []string) (map[string]bool, error)
```

## 提示

1. 用 `kafka.Writer{Addr: kafka.TCP(broker), Topic: topic, RequiredAcks: kafka.RequireAll}` 生产
2. 每条消息用 `WriteMessages(ctx, kafka.Message{Value: []byte(m)})`
3. 用 `kafka.NewReader(kafka.ReaderConfig{Brokers: []string{broker}, Topic: topic, GroupID: "g"})` 消费
4. 循环 `len(msgs)` 次 `ReadMessage`，把 `string(m.Value)` 放入 map

## 运行测试

```bash
cd code/backend/03-kafka/01_produce_consume && go test -v
```
