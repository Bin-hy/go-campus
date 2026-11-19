package deadline_cancel

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	userv1 "gocampus/microservice/api/user/v1"
)

// slowServer 的 GetUser 会睡多久由 name 里的时长控制（简化为固定 300ms）。
type slowServer struct {
	userv1.UnimplementedUserServiceServer
	started chan struct{}
	ended   chan struct{}
	elapsed time.Duration
}

func (s *slowServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.User, error) {
	close(s.started)
	start := time.Now()
	err := SlowWork(ctx, 300*time.Millisecond)
	s.elapsed = time.Since(start)
	close(s.ended)
	if err != nil {
		return nil, err
	}
	return &userv1.User{Id: req.GetId(), Name: "slow"}, nil
}

func start(t *testing.T, srv *slowServer, opts ...grpc.ServerOption) userv1.UserServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(opts...)
	userv1.RegisterUserServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn)
}

func TestDeadlineTerminatesCall(t *testing.T) {
	srv := &slowServer{started: make(chan struct{}), ended: make(chan struct{})}
	client := start(t, srv)

	ctx, cancel := CallWithBudget(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: 1})
	elapsed := time.Since(start)

	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("应为 DeadlineExceeded，实际 %v", status.Code(err))
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("超时应在 50ms 左右返回，实际 %v", elapsed)
	}

	<-srv.ended // 服务端也必须被唤醒，而不是继续睡 300ms
	if srv.elapsed > 200*time.Millisecond {
		t.Fatalf("服务端没有及时响应取消，耗时 %v", srv.elapsed)
	}
	t.Logf("deadline 生效：client=%v server=%v", elapsed, srv.elapsed)
}

func TestCancelPropagates(t *testing.T) {
	srv := &slowServer{started: make(chan struct{}), ended: make(chan struct{})}
	client := start(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-srv.started
		cancel() // 调用方主动取消
	}()

	_, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: 1})
	if status.Code(err) != codes.Canceled {
		t.Fatalf("应为 Canceled，实际 %v", status.Code(err))
	}
	select {
	case <-srv.ended:
	case <-time.After(time.Second):
		t.Fatal("服务端未收到取消")
	}
	if Classify(err) != Fatal {
		t.Fatal("Canceled 不应被重试")
	}
}

func TestTimeoutInterceptorClamps(t *testing.T) {
	srv := &slowServer{started: make(chan struct{}), ended: make(chan struct{})}
	client := start(t, srv, grpc.UnaryInterceptor(UnaryTimeoutInterceptor(100*time.Millisecond, 100*time.Millisecond)))

	// 调用方不给 deadline，也不设置客户端超时：服务端兜底 100ms。
	start := time.Now()
	_, err := client.GetUser(context.Background(), &userv1.GetUserRequest{Id: 1})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("服务端兜底超时应为 DeadlineExceeded，实际 %v", status.Code(err))
	}
	if time.Since(start) > 250*time.Millisecond {
		t.Fatalf("兜底超时未生效，耗时 %v", time.Since(start))
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		err  error
		want Class
	}{
		{status.Error(codes.Unavailable, ""), Retryable},
		{status.Error(codes.ResourceExhausted, ""), Retryable},
		{status.Error(codes.Internal, ""), Retryable},
		{status.Error(codes.NotFound, ""), Fatal},
		{status.Error(codes.InvalidArgument, ""), Fatal},
		{status.Error(codes.Canceled, ""), Fatal},
	}
	for _, c := range cases {
		if got := Classify(c.err); got != c.want {
			t.Errorf("%v: got %v want %v", status.Code(c.err), got, c.want)
		}
	}
}

func TestRemainingBudget(t *testing.T) {
	if _, ok := RemainingBudget(context.Background()); ok {
		t.Fatal("无 deadline 应返回 ok=false")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	d, ok := RemainingBudget(ctx)
	if !ok || d <= 0 || d > 200*time.Millisecond {
		t.Fatalf("剩余预算异常: %v ok=%v", d, ok)
	}
}
