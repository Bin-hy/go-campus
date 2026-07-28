# 生产者-消费者模型

## 难度：⭐⭐ 中等

## 考点
- 多生产者多消费者模式
- channel 作为任务队列
- 优雅关闭：生产结束后通知消费者退出
- sync.WaitGroup 协调多 goroutine 生命周期

## 题目描述

实现一个多生产者-多消费者模型：

1. `Produce` — 启动 numProducers 个生产者，每个生产者生成 itemsPerProducer 个整数任务
2. `Consume` — 启动 numConsumers 个消费者，每个消费者从 channel 读取任务并处理
3. `Run` — 运行整个流水线，返回所有消费者处理过的任务结果之和

生产者生产的值：第 p 个生产者（从0开始）的第 i 个任务值为 `p*itemsPerProducer + i`

## 函数签名

```go
func Run(numProducers, numConsumers, itemsPerProducer int) int
```

## 示例

```go
// 2个生产者，各产3个任务：[0,1,2] 和 [3,4,5]
// 全部任务和 = 0+1+2+3+4+5 = 15
result := Run(2, 3, 3)
// result == 15
```

## 提示
1. 用一个 buffered channel 作为任务队列
2. 所有生产者完成后关闭 channel
3. 消费者用 `for task := range ch` 读取，channel 关闭后自动退出
4. 汇总结果需要并发安全（atomic 或结果 channel）
