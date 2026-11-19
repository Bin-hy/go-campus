package deadline_cancel

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// Class 表示一次失败该如何处理。
type Class int

const (
	// Fatal：重试也没用（参数错、未鉴权、不存在）。
	Fatal Class = iota
	// Retryable：可换实例/退避后重试（网络抖动、实例下线）。
	Retryable
	// RetryAfterFix：先降级/等待恢复（熔断打开、限流）。
	RetryAfterFix
)

// Classify 把 gRPC 状态码映射成"要不要重试"。
// 提示：Unavailable/ResourceExhausted/Internal/Unknown 可重试；
// Canceled/InvalidArgument/NotFound/Unauthenticated 不可重试；
// DeadlineExceeded 交给上层判断剩余预算。用 status.Code(err)。
func Classify(err error) Class {
	// TODO: 实现你的代码
	panic("not implemented")
}

// UnaryTimeoutInterceptor 给没有 deadline 的请求兜底，并裁剪过长的 deadline。
func UnaryTimeoutInterceptor(defaultTimeout, maxTimeout time.Duration) grpc.UnaryServerInterceptor {
	// TODO: 实现你的代码
	panic("not implemented")
}

// CallWithBudget 派生一个带超时的子 ctx，并保证 cancel 被调用。
func CallWithBudget(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// SlowWork 模拟下游慢调用：必须监听 ctx.Done()，
// 超时/取消时用 status.FromContextError 返回标准错误码。
func SlowWork(ctx context.Context, d time.Duration) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// RemainingBudget 返回剩余时间；没有 deadline 时返回 ok=false。
func RemainingBudget(ctx context.Context) (time.Duration, bool) {
	// TODO: 实现你的代码
	panic("not implemented")
}
