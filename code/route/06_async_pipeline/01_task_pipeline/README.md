# AI 剪辑任务异步管道（MQ 语义模拟）

## 难度：⭐⭐⭐ 中等偏难

## 考点
- 生产者 / 消费者模型、任务队列
- 多 worker 并发消费（模拟 Kafka consumer group：一个任务只被一个 worker 消费）
- at-least-once：失败重试（模拟"处理成功才提交 offset"）
- 优雅关闭：Close 后拒绝新任务、等待已提交任务处理完

## 题目描述

用内存队列模拟文档 6.5 的「AI 剪辑任务」异步链路：提交（Producer）→ 队列（Kafka topic）→ 多 worker 消费（Consumer group）→ 处理结果事件（clip-result topic）。

实现 `Pipeline`：

1. `NewPipeline(ctx, workers, maxRetry, process)`：启动 `workers` 个 worker 并发消费
2. `Submit(ctx, task)`：入队；管道已 `Close` 返回 `ErrClosed`；`ctx` 取消返回 `ctx.Err()`
3. 每个任务最多尝试 `maxRetry+1` 次（首次 + maxRetry 次重试），全部失败则 `Result.Err` 非空（at-least-once 语义）
4. `Results()`：返回处理结果事件通道（模拟 clip-result topic）
5. `Close()`：停止接收新任务，**等待已提交任务全部处理完**，然后关闭 `Results()`（幂等）

## 函数签名

```go
type Task struct {
	ID      string
	Payload string
}

type Result struct {
	Task Task
	Err  error
}

var ErrClosed = errors.New("pipeline closed")

type Pipeline struct{ /* 自行设计 */ }

func NewPipeline(ctx context.Context, workers, maxRetry int, process func(ctx context.Context, t Task) error) *Pipeline
func (p *Pipeline) Submit(ctx context.Context, t Task) error
func (p *Pipeline) Results() <-chan Result
func (p *Pipeline) Close()
```

## 提示
1. 队列用 buffered channel；worker 用 `for t := range jobs`，`Close` 关闭 jobs 后 worker 排空队列退出
2. `Close` 里 `wg.Wait()` 之后再 `close(results)`（否则发送端 panic）
3. `Submit` 用 `select` 同时监听"已关闭"信号与 `ctx.Done()`
4. 思考：真实 Kafka 中"手动提交 offset"如何保证 at-least-once？（处理成功才提交，崩溃后从未提交处重放，代价是可能重复处理 → 消费端要幂等）
5. 注意：`Close` 与 `Submit` 并发调用需要外部加锁（延伸思考，本练习不要求）

## 验收
- [ ] 6 任务 3 worker：6 条结果事件，全部成功
- [ ] 失败 2 次后成功的任务：`process` 共被调用 3 次，结果无错误
- [ ] 永远失败的任务：结果带错误（重试耗尽）
- [ ] `Close` 后 `Submit` 返回 `ErrClosed`
