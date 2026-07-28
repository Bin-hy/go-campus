//go:build ignore

package answer

import "context"

type contextKey string

const traceIDKey contextKey = "trace_id"
const userIDKey contextKey = "user_id"

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

func Middleware(traceID, userID string, handler func(ctx context.Context) string) string {
	ctx := context.Background()
	ctx = WithTraceID(ctx, traceID)
	ctx = WithUserID(ctx, userID)
	return handler(ctx)
}
