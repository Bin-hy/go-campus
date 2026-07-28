package trace_propagation

import "context"

type contextKey string

const traceIDKey contextKey = "trace_id"
const userIDKey contextKey = "user_id"

// WithTraceID 将 traceID 注入 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// GetTraceID 从 context 提取 traceID，不存在返回空字符串
func GetTraceID(ctx context.Context) string {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// WithUserID 将 userID 注入 context
func WithUserID(ctx context.Context, userID string) context.Context {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// GetUserID 从 context 提取 userID，不存在返回空字符串
func GetUserID(ctx context.Context) string {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Middleware 模拟中间件链：注入 traceID 和 userID 后调用 handler
// handler 通过 context 获取 traceID 和 userID
func Middleware(traceID, userID string, handler func(ctx context.Context) string) string {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
