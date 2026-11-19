package deadline_cancel

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 参考答案 / 可运行实现。

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
// 这是重试策略的第一道闸：只重试幂等且可恢复的错误。
func Classify(err error) Class {
	switch status.Code(err) {
	case codes.OK:
		return Fatal
	case codes.Unavailable, codes.ResourceExhausted:
		return Retryable
	case codes.DeadlineExceeded, codes.Aborted:
		// 已经等超时了，再重试可能更糟；只有剩余预算充足才由上层决定。
		return Retryable
	case codes.Canceled:
		return Fatal // 调用方主动取消，重试违背意图
	case codes.Unknown, codes.Internal:
		return Retryable
	default:
		return Fatal
	}
}

// UnaryTimeoutInterceptor 给没有 deadline 的请求兜底，并裁剪过长的 deadline。
// 生产上服务端必须设上限，防止调用方传一个 1 小时的 deadline 拖住资源。
func UnaryTimeoutInterceptor(defaultTimeout, maxTimeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := ctx.Deadline(); !ok && defaultTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
			defer cancel()
		}
		if dl, ok := ctx.Deadline(); ok && maxTimeout > 0 {
			if time.Until(dl) > maxTimeout {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, maxTimeout)
				defer cancel()
			}
		}
		return handler(ctx, req)
	}
}

// CallWithBudget 派生一个带超时的子 ctx，并保证 cancel 被调用（防 context 泄漏）。
// 生产代码里永远不要在循环中直接 context.WithTimeout 而不 defer cancel。
func CallWithBudget(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// SlowWork 模拟下游慢调用：必须监听 ctx.Done()，否则超时也停不下来。
// 返回值的 err 用 status.FromContextError 转成标准 gRPC 错误码。
func SlowWork(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return status.FromContextError(ctx.Err()).Err()
	}
}

// RemainingBudget 返回剩余时间；没有 deadline 时返回 ok=false。
// 重试前先看预算：剩余时间不够一次调用就别重试了。
func RemainingBudget(ctx context.Context) (time.Duration, bool) {
	dl, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}
	return time.Until(dl), true
}
