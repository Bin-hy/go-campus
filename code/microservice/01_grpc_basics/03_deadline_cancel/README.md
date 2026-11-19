# Deadline、取消传播与错误码

## 难度：⭐⭐⭐ 困难

## 考点
- `context.WithTimeout` 派生 deadline，`defer cancel()` 防泄漏
- deadline 会通过 HTTP/2 传到服务端；服务端必须监听 `ctx.Done()`
- 取消传播：客户端 cancel → 服务端 `ctx.Done()` 立刻触发
- `status.FromContextError` / `status.Code` 的错误码映射
- 服务端兜底超时（default + max），防止调用方传入超长 deadline
- 哪些错误码可重试、哪些不可

## 题目描述

1. `UnaryTimeoutInterceptor(defaultTimeout, maxTimeout)`：无 deadline 时补默认值；deadline 超过上限时裁剪。
2. `CallWithBudget(ctx, timeout) (ctx, cancel)`：派生带超时的子 ctx。
3. `SlowWork(ctx, d) error`：模拟慢下游，必须在 `ctx.Done()` 时立即返回 `DeadlineExceeded`/`Canceled`。
4. `Classify(err) Class`：把状态码映射成 `Fatal / Retryable / RetryAfterFix`。
5. `RemainingBudget(ctx)`：重试前判断剩余预算。

## 函数签名

```go
type Class int
const ( Fatal Class = iota; Retryable; RetryAfterFix )

func Classify(err error) Class
func UnaryTimeoutInterceptor(defaultTimeout, maxTimeout time.Duration) grpc.UnaryServerInterceptor
func CallWithBudget(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc)
func SlowWork(ctx context.Context, d time.Duration) error
func RemainingBudget(ctx context.Context) (time.Duration, bool)
```

## 提示

1. `select { case <-time.After(d): case <-ctx.Done(): }` 里 `ctx.Err()` 是 `context.DeadlineExceeded` 或 `context.Canceled`。
2. 转 gRPC 错误：`status.FromContextError(ctx.Err()).Err()`。
3. **不要**在 handler 里吞掉取消去做"补偿工作"——那会让调用方等更久。
4. `codes.Canceled` 表示调用方主动放弃，**不能重试**；`Unavailable/ResourceExhausted` 可以。

## 验收

- [ ] 客户端 50ms 超时、服务端睡 300ms：客户端收到 `DeadlineExceeded`，且服务端在 200ms 内被唤醒
- [ ] 主动 cancel → `Canceled`，且不进入重试
- [ ] 服务端兜底 100ms：调用方不传 deadline 也会超时
- [ ] `Classify` 对 6 种状态码分类正确
- [ ] `go test -race ./01_grpc_basics/03_deadline_cancel`

## 运行

```bash
cd code/microservice
go test -v ./01_grpc_basics/03_deadline_cancel
```

## 面试追问

1. **deadline 和超时有什么区别？** 超时是"我最多等多久"（相对）；deadline 是"最晚什么时候必须返回"（绝对）。跨服务传递的是 deadline，子调用剩余预算 = 父 deadline - 已耗时。
2. **为什么服务端要设 maxTimeout？** 调用方可能传 1 小时，慢请求会占满 goroutine/连接，故障时放大雪崩。
3. **重试时 deadline 怎么处理？** 所有重试共享同一个父 deadline；每次重试前用 `RemainingBudget` 判断，剩余不足就直接失败。
