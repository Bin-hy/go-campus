package interceptor_auth

import (
	"context"

	"google.golang.org/grpc"
)

// 目标：不把鉴权代码散落在每个 handler 里，而是用拦截器统一处理。

type subjectKey struct{}

// Validator 校验 token 并返回调用主体（用户/租户）。
type Validator func(ctx context.Context, token string) (subject string, err error)

// SubjectFromContext 从 ctx 取调用主体。
func SubjectFromContext(ctx context.Context) string {
	s, _ := ctx.Value(subjectKey{}).(string)
	return s
}

// UnaryAuthInterceptor 一元鉴权：
// 缺 token → Unauthenticated；校验失败 → PermissionDenied；
// 成功则把 subject 写入 ctx 后调用 handler。
//
// 提示：metadata.FromIncomingContext(ctx) 取 "authorization"，
// 格式为 "Bearer <token>"；context.WithValue 写入 subjectKey。
func UnaryAuthInterceptor(v Validator) grpc.UnaryServerInterceptor {
	// TODO: 实现你的代码
	panic("not implemented")
}

// StreamAuthInterceptor 流式鉴权：同样校验并把 subject 放进 stream.Context()。
//
// 提示：需要包一层 grpc.ServerStream 并覆盖 Context() 方法。
func StreamAuthInterceptor(v Validator) grpc.StreamServerInterceptor {
	// TODO: 实现你的代码
	panic("not implemented")
}

// ClientAuthInterceptor 客户端一元拦截器：自动附加 authorization 头。
func ClientAuthInterceptor(token string) grpc.UnaryClientInterceptor {
	// TODO: 实现你的代码
	panic("not implemented")
}

// ClientStreamAuthInterceptor 客户端流拦截器：同样自动附加。
func ClientStreamAuthInterceptor(token string) grpc.StreamClientInterceptor {
	// TODO: 实现你的代码
	panic("not implemented")
}
