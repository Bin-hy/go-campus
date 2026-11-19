package otel_trace

import (
	"context"
	"net"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	otelcodes "go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	userv1 "gocampus/microservice/api/user/v1"
)

type okServer struct {
	userv1.UnimplementedUserServiceServer
}

func (okServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.User, error) {
	if req.GetId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "id required")
	}
	if TraceID(ctx) == "" {
		return nil, status.Error(codes.Internal, "missing trace context")
	}
	return &userv1.User{Id: req.GetId(), Name: "traced"}, nil
}

func newProvider() (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sr),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	return tp, sr
}

func startStack(t *testing.T, tp *sdktrace.TracerProvider) userv1.UserServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(grpc.UnaryInterceptor(UnaryServerTrace(tp)))
	userv1.RegisterUserServiceServer(gs, okServer{})
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(UnaryClientTrace(tp)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn)
}

func TestTracePropagationAcrossGRPC(t *testing.T) {
	tp, sr := newProvider()
	client := startStack(t, tp)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: 7}); err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatal(err)
	}

	spans := sr.Ended()
	if len(spans) != 2 {
		t.Fatalf("应产生 client + server 两个 span，实际 %d", len(spans))
	}
	var clientSpan, serverSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		switch s.SpanKind() {
		case oteltrace.SpanKindClient:
			clientSpan = s
		case oteltrace.SpanKindServer:
			serverSpan = s
		}
	}
	if clientSpan == nil || serverSpan == nil {
		t.Fatalf("缺少 client/server span")
	}
	if clientSpan.SpanContext().TraceID() != serverSpan.SpanContext().TraceID() {
		t.Fatal("client 与 server 必须在同一条 trace 上")
	}
	if serverSpan.Parent().SpanID() != clientSpan.SpanContext().SpanID() {
		t.Fatal("server span 的 parent 应是 client span")
	}
	if serverSpan.Status().Code != otelcodes.Ok {
		t.Fatalf("server span 状态应为 OK，实际 %v", serverSpan.Status())
	}
	t.Logf("trace=%s client=%s server=%s", clientSpan.SpanContext().TraceID(), clientSpan.SpanContext().SpanID(), serverSpan.SpanContext().SpanID())
}

func TestErrorMarksSpan(t *testing.T) {
	tp, sr := newProvider()
	client := startStack(t, tp)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: 0}); err == nil {
		t.Fatal("应返回错误")
	}
	_ = tp.ForceFlush(context.Background())

	var serverErr bool
	for _, s := range sr.Ended() {
		if s.SpanKind() == oteltrace.SpanKindServer && s.Status().Code == otelcodes.Error {
			serverErr = true
		}
	}
	if !serverErr {
		t.Fatal("失败请求的 server span 应标记 Error")
	}
}
