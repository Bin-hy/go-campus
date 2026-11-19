# OpenTelemetry 全链路追踪：traceparent 跨 gRPC 传播

## 难度：⭐⭐⭐ 困难

## 考点
- Trace / Span / SpanContext（trace_id + span_id + flags）
- W3C `traceparent`：`00-<trace-id>-<parent-id>-<flags>`
- 客户端注入、服务端提取，父子关系如何建立
- 拦截器统一埋点，业务代码零侵入
- 错误标记：`RecordError` + `SetStatus(Error)`
- 真实对应 grpc-go 的 `otelgrpc` 官方拦截器

## 题目描述

实现：

1. `UnaryClientTrace(tp)`：创建 client span → 把 traceparent 注入 outgoing metadata → 调用；
2. `UnaryServerTrace(tp)`：从 incoming metadata 提取 → 以它为 parent 创建 server span；
3. `TraceID(ctx)`：从 span context 取 trace id（打日志用）；
4. `metadataCarrier`：适配 otel `TextMapPropagator`。

## 函数签名

```go
func UnaryClientTrace(tp trace.TracerProvider) grpc.UnaryClientInterceptor
func UnaryServerTrace(tp trace.TracerProvider) grpc.UnaryServerInterceptor
func TraceID(ctx context.Context) string
```

## 提示

1. 用 `propagation.TraceContext{}` 的 `Inject/Extract`，不要自己拼 traceparent。
2. metadata 是 `map[string][]string`；`Set` 后要 `metadata.NewOutgoingContext(ctx, md)` 放回 ctx。
3. 服务端必须先 `Extract` 再 `tracer.Start`，否则 parent 丢失，会变成两条独立 trace。
4. 采样决策随 traceparent 的 flags 传播；测试用 `AlwaysSample`。

## 验收

- [ ] 一次调用产生 2 个 span（client + server）
- [ ] 两者 trace_id 相同
- [ ] server span 的 parent span_id == client span_id
- [ ] handler 内 `TraceID(ctx)` 非空
- [ ] 失败请求的 server span 状态为 Error
- [ ] `go test ./04_observability/01_otel_trace`

## 运行

```bash
cd code/microservice
go test -v ./04_observability/01_otel_trace
```

## 接入真实后端

```go
import (
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

exp, _ := otlptracegrpc.New(ctx) // 默认读 OTEL_EXPORTER_OTLP_ENDPOINT
tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
otel.SetTracerProvider(tp)
otel.SetTextMapPropagator(propagation.TraceContext{})
```

接入 Jaeger/Tempo 后，用 trace_id 搜索就能看到完整跨服务调用树。

## 面试追问

1. **trace_id 和 span_id 的作用？** trace_id 标识整条链路；span_id 标识一个操作；parent_span_id 建立调用树。
2. **采样在哪做？** 通常入口（网关）做一次决策并随 traceparent 传播，避免各服务重复采样导致链路断裂；也可用 tail-based sampling 按错误/延迟采样。
3. **日志怎么和链路关联？** 每条日志带上 trace_id/span_id；Loki/Jaeger 支持按 trace_id 跳转。
4. **异步消息怎么串？** 生产消息时把 traceparent 写进 message headers，消费端提取后继续同一 trace（或用 span link 表示因果）。
