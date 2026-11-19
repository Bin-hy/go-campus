package otel_trace

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const tracerName = "gocampus/microservice"

// metadataCarrier 让 otel 的 TextMapPropagator 能读写 gRPC metadata。
type metadataCarrier struct {
	md metadata.MD
}

func (c metadataCarrier) Get(key string) string {
	if vs := c.md.Get(key); len(vs) > 0 {
		return vs[0]
	}
	return ""
}

func (c metadataCarrier) Set(key, value string) { c.md.Set(key, value) }

func (c metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c.md))
	for k := range c.md {
		keys = append(keys, k)
	}
	return keys
}

// UnaryClientTrace 客户端拦截器：
//  1. 以当前 ctx 为 parent 创建 client span；
//  2. 把 traceparent 注入 outgoing metadata；
//  3. 调用结束后记录状态。
//
// 提示：propagation.TraceContext{}.Inject(ctx, metadataCarrier{md})；
// metadata 修改后要 metadata.NewOutgoingContext 放回 ctx。
func UnaryClientTrace(tp trace.TracerProvider) grpc.UnaryClientInterceptor {
	// TODO: 实现你的代码
	panic("not implemented")
}

// UnaryServerTrace 服务端拦截器：先 Extract 上游 traceparent，再创建 server span。
func UnaryServerTrace(tp trace.TracerProvider) grpc.UnaryServerInterceptor {
	// TODO: 实现你的代码
	panic("not implemented")
}

// TraceID 从 ctx 取当前 trace id（日志里带上它就能和链路对上）。
func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}
