# 手动提交：at-least-once 重复消费

## 难度：⭐⭐⭐ 困难

## 考点
- offset 手动提交（CommitInterval=0）
- at-least-once 语义下的重复消费
- 先处理后提交的顺序

## 环境准备

```bash
cd code/backend && docker compose up -d kafka
```

## 题目描述

实现 `ManualCommitAtLeastOnce`：生产 `first`、`second` 两条消息到 topic，用 group 消费：

1. 读两条（first、second）
2. 只提交 first（`CommitMessages` 提交到 first）
3. 关闭 reader，用同 group 重建 reader 再读

返回新 reader 读到的消息值（应为 `second`，因为 second 未提交，会重复读到）。

## 函数签名

```go
func ManualCommitAtLeastOnce(ctx context.Context, broker, topic, group string) (again string, err error)
```

## 提示

1. Reader 配置 `CommitInterval: 0`（禁用自动提交）
2. 读 first、second 两条
3. `r1.CommitMessages(ctx, m1)` 只提交 first
4. `r1.Close()` 后新建同 group 的 reader，再读一次，应重复读到 second

## 运行测试

```bash
cd code/backend/03-kafka/02_manual_commit && go test -v
```
