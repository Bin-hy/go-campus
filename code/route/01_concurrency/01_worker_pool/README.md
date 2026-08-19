# Go 并发任务池（fan-out / fan-in）

## 难度：⭐⭐ 中等

## 考点
- goroutine + channel 并发模型
- fan-out（分发）与 fan-in（合并）
- channel 关闭语义（生产者 close、`range` 退出）
- context 取消与超时

## 题目描述

实现 `RunPool`：把 `jobs` 分发给 `workers` 个 worker 并发处理（fan-out），汇总结果返回（fan-in）。

要求：
1. `workers` 个 goroutine 同时消费任务，处理函数 `fn(ctx, job)` 返回 `(int, error)`
2. 所有任务处理完成后返回结果切片（顺序不做要求）
3. `ctx` 取消时尽快返回 `ctx.Err()`，不要阻塞
4. `fn` 返回 error 时，整个池返回该 error

## 函数签名

```go
type JobFunc func(ctx context.Context, job int) (int, error)

func RunPool(ctx context.Context, jobs []int, workers int, fn JobFunc) ([]int, error)
```

## 示例

```go
results, err := RunPool(ctx, []int{1, 2, 3}, 3, func(ctx context.Context, j int) (int, error) {
	return j * j, nil
})
// results 包含 {1, 4, 9}（顺序不限），err == nil
```

## 提示
1. 用两个 channel：`in`（任务输入）、`out`（结果输出）
2. **关闭语义**：`in` 由生产者 `close`；`out` 必须等所有 worker 结束（`sync.WaitGroup`）后再 `close`，否则发送端 panic
3. 取消：生产与消费两端都用 `select` 监听 `ctx.Done()`
4. 思考：如果某个 worker 出错，如何让整个池尽快停止？（错误 channel + 广播退出）
