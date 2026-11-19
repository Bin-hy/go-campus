package interceptor_auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// 参考答案 / 可运行实现。
//
// 目标：不把鉴权代码散落在每个 handler 里，而是用拦截器统一处理。

type subjectKey struct{}

// Validator 校验 token 并返回调用主体（用户/租户）。
// 生产实现：查 JWT 签名、调鉴权中心、查 Redis 会话等。
type Validator func(ctx context.Context, token string) (subject string, err error)

// SubjectFromContext 从 ctx 取调用主体。
func SubjectFromContext(ctx context.Context) string {
	s, _ := ctx.Value(subjectKey{}).(string)
	return s
}

// tokenFromMD 从 metadata 取出 "authorization: Bearer xxx"。
func tokenFromMD(md metadata.MD) (string, bool) {
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return "", false
	}
	v := strings.TrimSpace(vals[0])
	const prefix = "Bearer "
	if !strings.HasPrefix(v, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(strings.TrimPrefix(v, prefix))
	return tok, tok != ""
}

// UnaryAuthInterceptor 一元鉴权：缺 token → Unauthenticated；校验失败 → PermissionDenied。
func UnaryAuthInterceptor(v Validator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		tok, ok := tokenFromMD(md)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}
		subject, err := v(ctx, tok)
		if err != nil {
			return nil, status.Error(codes.PermissionDenied, "invalid token")
		}
		return handler(context.WithValue(ctx, subjectKey{}, subject), req)
	}
}

// StreamAuthInterceptor 流式鉴权：注意要把带 subject 的 ctx 包回 ServerStream。
func StreamAuthInterceptor(v Validator) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}
		tok, ok := tokenFromMD(md)
		if !ok {
			return status.Error(codes.Unauthenticated, "missing bearer token")
		}
		subject, err := v(ss.Context(), tok)
		if err != nil {
			return status.Error(codes.PermissionDenied, "invalid token")
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: context.WithValue(ss.Context(), subjectKey{}, subject)})
	}
}

// wrappedStream 覆盖 Context()，让 handler 里的 ctx 能拿到 subject。
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

// ClientAuthInterceptor 客户端一元拦截器：自动附加 authorization 头。
func ClientAuthInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if token != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// ClientStreamAuthInterceptor 客户端流拦截器：同样自动附加。
func ClientStreamAuthInterceptor(token string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if token != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}
