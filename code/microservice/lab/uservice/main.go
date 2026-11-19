// uservice 是教程配套的可运行用户服务：
//   - 一元 / 服务端流 / 客户端流 / 双向流 四种 RPC
//   - 鉴权、日志、恢复、指标、链路追踪拦截器
//   - gRPC Health Checking 与反射
//   - 可选的 TLS / mTLS
//   - /metrics（Prometheus）与优雅停机
//
// 启动三个实例（三个终端）：
//
//	go run ./lab/uservice -addr :9001 -instance user-1
//	go run ./lab/uservice -addr :9002 -instance user-2
//	go run ./lab/uservice -addr :9003 -instance user-3
//
// 再用网关把请求轮询到三个实例：
//
//	go run ./lab/gateway -targets 127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	userv1 "gocampus/microservice/api/user/v1"
)

type server struct {
	userv1.UnimplementedUserServiceServer

	instance string
	store    *store
	metrics  *metrics

	healthy atomic.Bool // readiness：false 时 K8s 不转发流量
}

type store struct {
	mu     sync.RWMutex
	users  map[int64]*userv1.User
	nextID int64

	chargesMu sync.Mutex
	charges   map[string]*userv1.ChargeReply // 幂等键 → 首次扣款结果
}

func newStore() *store {
	s := &store{
		users:   make(map[int64]*userv1.User),
		charges: make(map[string]*userv1.ChargeReply),
	}
	for i := int64(1); i <= 20; i++ {
		s.users[i] = &userv1.User{
			Id:      i,
			Name:    fmt.Sprintf("user-%02d", i),
			Email:   fmt.Sprintf("user%02d@campus.dev", i),
			Version: 1,
		}
		s.nextID = i
	}
	return s
}

// ---------------- 四种 RPC ----------------

func (s *server) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.User, error) {
	// 从 metadata 读取调用方，演示元数据透传。
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if caller := md.Get("x-caller"); len(caller) > 0 {
			log.Printf("[%s] GetUser id=%d caller=%s", s.instance, req.GetId(), caller[0])
		}
	}
	// 演示故障注入：metadata 里带 x-fail=true 就返回内部错误。
	if md, ok := metadata.FromIncomingContext(ctx); ok && len(md.Get("x-fail")) > 0 {
		return nil, status.Error(codes.Unavailable, "injected failure")
	}

	// 模拟下游耗时；同时尊重 ctx 的 deadline / 取消。
	select {
	case <-time.After(20 * time.Millisecond):
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}

	s.store.mu.RLock()
	defer s.store.mu.RUnlock()
	u, ok := s.store.users[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "user %d not found", req.GetId())
	}
	cp := proto.Clone(u).(*userv1.User)
	cp.Name = fmt.Sprintf("%s@%s", cp.Name, s.instance) // 便于观察负载均衡命中了哪个实例
	return cp, nil
}

func (s *server) ListUsers(req *userv1.ListUsersRequest, stream userv1.UserService_ListUsersServer) error {
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = 5
	}
	s.store.mu.RLock()
	users := make([]*userv1.User, 0, len(s.store.users))
	for i := int64(1); i <= s.store.nextID; i++ {
		users = append(users, s.store.users[i])
	}
	s.store.mu.RUnlock()

	for i, u := range users {
		if i >= pageSize {
			break
		}
		if err := stream.Send(u); err != nil {
			return err // 客户端断开/取消时 Send 返回错误
		}
	}
	return nil
}

func (s *server) UploadUsers(stream userv1.UserService_UploadUsersServer) error {
	var accepted, rejected int32
	for {
		u, err := stream.Recv()
		if errors.Is(err, context.Canceled) {
			return status.Error(codes.Canceled, "client canceled")
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return stream.SendAndClose(&userv1.UploadSummary{Accepted: accepted, Rejected: rejected})
			}
			return err
		}
		if strings.TrimSpace(u.GetName()) == "" {
			rejected++
			continue
		}
		s.store.mu.Lock()
		s.store.nextID++
		u.Id = s.store.nextID
		u.Version = 1
		s.store.users[u.Id] = u
		s.store.mu.Unlock()
		accepted++
	}
}

func (s *server) Chat(stream userv1.UserService_ChatServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return nil
		}
		if err := stream.Send(&userv1.ChatMessage{
			From: s.instance,
			Text: fmt.Sprintf("echo(%s): %s", msg.GetFrom(), msg.GetText()),
		}); err != nil {
			return err
		}
	}
}

func (s *server) Charge(ctx context.Context, req *userv1.ChargeRequest) (*userv1.ChargeReply, error) {
	key := req.GetIdempotencyKey()
	if key == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}

	// 幂等：同一个 key 只扣一次，重复请求返回首次结果。
	s.store.chargesMu.Lock()
	defer s.store.chargesMu.Unlock()
	if hit, ok := s.store.charges[key]; ok {
		cp := proto.Clone(hit).(*userv1.ChargeReply)
		cp.Deduplicated = true
		return cp, nil
	}
	reply := &userv1.ChargeReply{
		ChargeId:    fmt.Sprintf("ch_%s_%d", s.instance, time.Now().UnixNano()),
		AmountCents: req.GetAmountCents(),
	}
	s.store.charges[key] = reply
	return proto.Clone(reply).(*userv1.ChargeReply), nil
}

// ---------------- 拦截器 ----------------

// unaryAuth：校验 authorization: Bearer <token>。
// 健康检查必须免鉴权：K8s 探针和 gRPC 客户端的 health checking 不会带业务 token，
// 如果把它一起拦掉，实例会被误判为不健康、永远摘不回来。
func unaryAuth(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if token == "" || strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(ctx, req)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		auth := md.Get("authorization")
		if len(auth) == 0 || auth[0] != "Bearer "+token {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid token")
		}
		return handler(ctx, req)
	}
}

func streamAuth(token string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if token == "" || strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(srv, ss)
		}
		md, _ := metadata.FromIncomingContext(ss.Context())
		auth := md.Get("authorization")
		if len(auth) == 0 || auth[0] != "Bearer "+token {
			return status.Error(codes.Unauthenticated, "missing or invalid token")
		}
		return handler(srv, ss)
	}
}

// unaryRecovery：把 handler panic 转成 Internal，避免整个进程崩溃。
func unaryRecovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered in %s: %v", info.FullMethod, r)
				err = status.Errorf(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// unaryTrace：为每个请求创建 server span，并提取上游 traceparent。
func unaryTrace(tp trace.TracerProvider) grpc.UnaryServerInterceptor {
	tracer := tp.Tracer("gocampus/uservice")
	prop := propagation.TraceContext{}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = prop.Extract(ctx, mdCarrier(md))
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attribute.String("rpc.system", "grpc")),
		)
		defer span.End()
		resp, err := handler(ctx, req)
		if err != nil {
			span.RecordError(err)
		}
		return resp, err
	}
}

type mdCarrier metadata.MD

func (m mdCarrier) Get(key string) string {
	if vs := metadata.MD(m).Get(key); len(vs) > 0 {
		return vs[0]
	}
	return ""
}
func (m mdCarrier) Set(key, val string) { metadata.MD(m).Set(key, val) }
func (m mdCarrier) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------------- 指标 ----------------

type metrics struct {
	instance string
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inflight *prometheus.GaugeVec
}

func newMetrics(reg prometheus.Registerer, instance string) *metrics {
	m := &metrics{
		instance: instance,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_server_requests_total",
			Help: "Total gRPC requests by method and code.",
		}, []string{"method", "code", "instance"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_server_handling_seconds",
			Help:    "gRPC handler latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "instance"}),
		inflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "grpc_server_inflight_requests",
			Help: "In-flight gRPC requests.",
		}, []string{"method", "instance"}),
	}
	reg.MustRegister(m.requests, m.duration, m.inflight)
	return m
}

func (m *metrics) unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		m.inflight.WithLabelValues(info.FullMethod, m.instance).Inc()
		defer m.inflight.WithLabelValues(info.FullMethod, m.instance).Dec()

		resp, err := handler(ctx, req)
		code := status.Code(err).String()
		m.requests.WithLabelValues(info.FullMethod, code, m.instance).Inc()
		m.duration.WithLabelValues(info.FullMethod, m.instance).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

// ---------------- main ----------------

func main() {
	var (
		addr        = flag.String("addr", ":9001", "gRPC listen address")
		instance    = flag.String("instance", "user-1", "instance name shown in responses")
		metricsAddr = flag.String("metrics-addr", ":9101", "Prometheus /metrics listen address")
		token       = flag.String("token", "campus-secret", "bearer token; empty disables auth")
		certFile    = flag.String("cert", "", "TLS cert file (enables TLS)")
		keyFile     = flag.String("key", "", "TLS key file")
		otlp        = flag.Bool("otlp", false, "print spans to stdout (set OTEL_EXPORTER_OTLP_ENDPOINT for OTLP)")
	)
	flag.Parse()

	srv := &server{instance: *instance, store: newStore()}
	srv.healthy.Store(true)

	// --- 初始化 tracing ---
	tp := trace.NewNoopTracerProvider()
	if *otlp {
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			log.Fatalf("init trace exporter: %v", err)
		}
		provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
		defer provider.Shutdown(context.Background())
		otel.SetTracerProvider(provider)
		otel.SetTextMapPropagator(propagation.TraceContext{})
		tp = provider
	}

	// --- 初始化 metrics ---
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())
	met := newMetrics(reg, *instance)

	// --- gRPC server 选项 ---
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			unaryRecovery(),
			met.unary(),
			unaryTrace(tp),
			unaryAuth(*token),
		),
		grpc.ChainStreamInterceptor(
			streamAuth(*token),
		),
	}
	if *certFile != "" {
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("load TLS: %v", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	gs := grpc.NewServer(opts...)
	userv1.RegisterUserServiceServer(gs, srv)

	// Health Checking：K8s 的 grpc 探针直接调它。
	hs := health.NewServer()
	healthpb.RegisterHealthServer(gs, hs)
	// 空字符串是"整体服务"状态，K8s 原生 grpc 探针默认查它。
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	hs.SetServingStatus("user.v1.UserService", healthpb.HealthCheckResponse_SERVING)

	// 反射：grpcurl 无需 proto 文件即可调试。
	reflection.Register(gs)

	// --- 优雅停机 ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 关闭顺序：先标记未就绪 → 等 K8s 摘流量 → 再 GracefulStop。
	go func() {
		<-ctx.Done()
		log.Printf("[%s] signal received, draining...", *instance)
		srv.healthy.Store(false)
		hs.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		hs.SetServingStatus("user.v1.UserService", healthpb.HealthCheckResponse_NOT_SERVING)

		drained := make(chan struct{})
		go func() {
			gs.GracefulStop() // 停止接收新连接，等在途请求处理完
			close(drained)
		}()
		select {
		case <-drained:
			log.Printf("[%s] graceful stop done", *instance)
		case <-time.After(10 * time.Second):
			log.Printf("[%s] drain timeout, forcing stop", *instance)
			gs.Stop()
		}
	}()

	// --- /metrics 与健康端点 ---
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			if !srv.healthy.Load() {
				http.Error(w, "draining", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			if !srv.healthy.Load() {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
		})
		log.Printf("[%s] metrics on %s", *instance, *metricsAddr)
		if err := http.ListenAndServe(*metricsAddr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("metrics server: %v", err)
		}
	}()

	// --- 启动 gRPC ---
	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}
	log.Printf("[%s] gRPC listening on %s (tls=%v auth=%v)", *instance, *addr, *certFile != "", *token != "")
	if err := gs.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
