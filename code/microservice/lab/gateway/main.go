// gateway 是教程配套的客户端网关，演示：
//   - 静态多实例 + 客户端 round_robin 负载均衡
//   - 每次调用设置 deadline（超时可终止）
//   - 服务配置级重试 + 指数退避（仅对幂等/可重试错误）
//   - 简易熔断（连续失败后快速失败）
//   - 透传 authorization / traceparent / x-caller
//
// 用法（先启动三个 uservice 实例）：
//
//	go run ./lab/gateway -targets 127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003
//
// 观察：
//
//	for i in $(seq 1 12); do curl -s "http://127.0.0.1:8080/user?id=1"; echo; done
//	# name 后缀会在 user-1 / user-2 / user-3 间轮询
//	curl -s "http://127.0.0.1:8080/charge?key=k1&amount=100"; echo  # 重复调用只扣一次
//	curl -s "http://127.0.0.1:8080/fail"; echo                      # 注入失败，观察重试与最终 503
package main

import (
	"context"
	"encoding/json"
	"flag"
	"hash/fnv"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/status"

	userv1 "gocampus/microservice/api/user/v1"
	_ "google.golang.org/grpc/health" // 注册客户端健康检查
)

var (
	targets     = flag.String("targets", "127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003", "comma separated gRPC targets")
	httpAddr    = flag.String("http-addr", "127.0.0.1:8080", "HTTP listen address")
	token       = flag.String("token", "campus-secret", "bearer token")
	callTimeout = flag.Duration("timeout", 800*time.Millisecond, "per-call deadline")
)

// breaker 简易熔断：连续失败 N 次后冷却期内直接失败。
type breaker struct {
	mu        sync.Mutex
	failures  int
	threshold int
	cooldown  time.Duration
	openUntil time.Time
}

func (b *breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return time.Now().After(b.openUntil)
}

func (b *breaker) record(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err == nil {
		b.failures = 0
		return
	}
	b.failures++
	if b.failures >= b.threshold {
		b.openUntil = time.Now().Add(b.cooldown)
		b.failures = 0
	}
}

var (
	requests atomic.Int64
	failures atomic.Int64
)

func main() {
	flag.Parse()

	addrs := strings.Split(*targets, ",")
	// 服务配置：round_robin + 重试策略（仅 UNAVAILABLE 可重试）。
	serviceConfig := `{
	  "loadBalancingConfig": [{"round_robin": {}}],
	  "healthCheckConfig": {"serviceName": "user.v1.UserService"},
	  "methodConfig": [{
	    "name": [{}],
	    "timeout": "0.8s",
	    "retryPolicy": {
	      "maxAttempts": 3,
	      "initialBackoff": "0.05s",
	      "maxBackoff": "0.5s",
	      "backoffMultiplier": 2,
	      "retryableStatusCodes": ["UNAVAILABLE"]
	    }
	  }]
	}`
	// 两种寻址方式：
	//   - 逗号分隔的静态列表 → manual resolver + round_robin（本地三实例）
	//   - 单个名字 → dns:/// + round_robin（K8s Headless Service，DNS 返回全部 Pod IP）
	target := "static:///uservice"
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(serviceConfig),
	}
	if len(addrs) == 1 {
		target = "dns:///" + strings.TrimSpace(addrs[0])
	} else {
		r := manual.NewBuilderWithScheme("static")
		r.InitialState(resolver.State{Addresses: buildAddrs(addrs)})
		opts = append(opts, grpc.WithResolvers(r))
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := userv1.NewUserServiceClient(conn)

	// 教学折中：lab 的幂等表在每个实例的内存里。
	// 多实例下按幂等键做"粘性路由"，保证同一个 key 命中同一个实例，才能观察到去重。
	// 生产必须把幂等表放到共享存储（Redis/MySQL），见第 08、16 章。
	chargeRoute, err := newStickyRoute(addrs, serviceConfig, client)
	if err != nil {
		log.Fatalf("charge route: %v", err)
	}
	defer chargeRoute.Close()

	br := &breaker{threshold: 5, cooldown: 3 * time.Second}

	http.HandleFunc("/user", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(req.URL.Query().Get("id"), 10, 64)
		ctx, cancel := context.WithTimeout(req.Context(), *callTimeout)
		defer cancel()

		// 注入鉴权与调用方元数据。
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+*token, "x-caller", "gateway")

		u, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: id})
		writeResult(w, u, err)
	})

	http.HandleFunc("/fail", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), *callTimeout)
		defer cancel()
		ctx = metadata.AppendToOutgoingContext(ctx,
			"authorization", "Bearer "+*token,
			"x-caller", "gateway",
			"x-fail", "true", // 服务端看到它就返回 Unavailable
		)

		if !br.allow() {
			failures.Add(1)
			http.Error(w, "circuit open: fail fast", http.StatusServiceUnavailable)
			return
		}
		u, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: 1})
		br.record(err)
		writeResult(w, u, err)
	})

	http.HandleFunc("/charge", func(w http.ResponseWriter, req *http.Request) {
		key := req.URL.Query().Get("key")
		amount, _ := strconv.ParseInt(req.URL.Query().Get("amount"), 10, 64)
		ctx, cancel := context.WithTimeout(req.Context(), *callTimeout)
		defer cancel()
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+*token)

		reply, err := chargeRoute.Pick(key).Charge(ctx, &userv1.ChargeRequest{
			IdempotencyKey: key,
			UserId:         1,
			AmountCents:    amount,
		})
		writeResult(w, reply, err)
	})

	http.HandleFunc("/stats", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, map[string]any{
			"requests": requests.Load(),
			"failures": failures.Load(),
		})
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	signals, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{Addr: *httpAddr, Handler: nil}
	go func() {
		log.Printf("gateway on %s -> %v", *httpAddr, addrs)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	<-signals.Done()
	log.Printf("gateway draining...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx) // 停止接新请求，等待在途 HTTP 请求完成
	log.Printf("gateway stopped")
}

func buildAddrs(addrs []string) []resolver.Address {
	out := make([]resolver.Address, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, resolver.Address{Addr: strings.TrimSpace(a)})
	}
	return out
}

// stickyRoute 按幂等键哈希到固定实例（教学用）。
// 只解决"多实例下演示内存幂等表"的问题；实例故障时 key 会漂移，仍可能重复扣款。
// 生产正确做法：幂等表放 Redis/MySQL，所有实例共享。
type stickyRoute struct {
	clients  []userv1.UserServiceClient
	conns    []*grpc.ClientConn
	fallback userv1.UserServiceClient
}

func newStickyRoute(addrs []string, serviceConfig string, fallback userv1.UserServiceClient) (*stickyRoute, error) {
	sr := &stickyRoute{fallback: fallback}
	if len(addrs) < 2 {
		return sr, nil // DNS 模式：交给 round_robin，幂等需共享存储
	}
	for _, a := range addrs {
		c, err := grpc.NewClient(strings.TrimSpace(a),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultServiceConfig(serviceConfig),
		)
		if err != nil {
			sr.Close()
			return nil, err
		}
		sr.conns = append(sr.conns, c)
		sr.clients = append(sr.clients, userv1.NewUserServiceClient(c))
	}
	return sr, nil
}

func (s *stickyRoute) Pick(key string) userv1.UserServiceClient {
	if len(s.clients) == 0 {
		return s.fallback
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return s.clients[h.Sum32()%uint32(len(s.clients))]
}

func (s *stickyRoute) Close() {
	for _, c := range s.conns {
		_ = c.Close()
	}
}

func writeResult(w http.ResponseWriter, v any, err error) {
	requests.Add(1)
	if err != nil {
		failures.Add(1)
		st := status.Convert(err)
		code := http.StatusBadGateway
		switch st.Code() {
		case codes.NotFound:
			code = http.StatusNotFound
		case codes.DeadlineExceeded:
			code = http.StatusGatewayTimeout
		case codes.Unavailable:
			code = http.StatusServiceUnavailable
		case codes.Unauthenticated:
			code = http.StatusUnauthorized
		case codes.InvalidArgument:
			code = http.StatusBadRequest
		}
		// 先写状态码，再写 body（WriteHeader 之后 body 才使用该状态码）。
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": st.Message(), "code": st.Code().String()})
		return
	}
	writeJSON(w, v)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("encode: %v", err)
	}
}
