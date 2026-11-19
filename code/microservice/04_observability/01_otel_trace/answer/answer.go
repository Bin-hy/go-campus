package otel_trace

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// 参考答案 / 可运行实现。
//
// 跨服务追踪的关键：把 W3C traceparent 放进 gRPC metadata，
// 下游 Extract 后作为自己 span 的 parent，形成一条完整链路。

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
func UnaryClientTrace(tp trace.TracerProvider) grpc.UnaryClientInterceptor {
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	tracer := tp.Tracer(tracerName)
	prop := propagation.TraceContext{}

	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, span := tracer.Start(ctx, method,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(attribute.String("rpc.system", "grpc")),
		)
		defer span.End()

		// 注入：md 是引用类型，Set 后需要放回 ctx。
		md, _ := metadata.FromOutgoingContext(ctx)
		if md == nil {
			md = metadata.New(nil)
		}
		prop.Inject(ctx, metadataCarrier{md: md})
		ctx = metadata.NewOutgoingContext(ctx, md)

		err := invoker(ctx, method, req, reply, cc, opts...)
		recordStatus(span, err)
		return err
	}
}

// UnaryServerTrace 服务端拦截器：先 Extract 上游 traceparent，再创建 server span。
func UnaryServerTrace(tp trace.TracerProvider) grpc.UnaryServerInterceptor {
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	tracer := tp.Tracer(tracerName)
	prop := propagation.TraceContext{}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			ctx = prop.Extract(ctx, metadataCarrier{md: md})
		}
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attribute.String("rpc.system", "grpc")),
		)
		defer span.End()

		resp, err := handler(ctx, req)
		recordStatus(span, err)
		return resp, err
	}
}

func recordStatus(span trace.Span, err error) {
	if err == nil {
		span.SetStatus(codes.Ok, "")
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// NewTestProvider 供测试/本地实验使用：把 span 全量收集到内存。
func NewTestProvider(exporter sdktrace.SpanExporter) *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
	)
}

// TraceID 从 ctx 取当前 trace id（日志里带上它就能和链路对上）。
func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}
