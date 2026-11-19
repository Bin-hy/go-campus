package health_tls

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	userv1 "gocampus/microservice/api/user/v1"
)

type okServer struct {
	userv1.UnimplementedUserServiceServer
}

func (okServer) GetUser(context.Context, *userv1.GetUserRequest) (*userv1.User, error) {
	return &userv1.User{Id: 1, Name: "ok"}, nil
}

func startTLSServer(t *testing.T, opts ...grpc.ServerOption) (*bufconn.Listener, *health.Server) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(opts...)
	userv1.RegisterUserServiceServer(gs, okServer{})
	hs := health.NewServer()
	healthpb.RegisterHealthServer(gs, hs)
	hs.SetServingStatus("user.v1.UserService", healthpb.HealthCheckResponse_SERVING)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	return lis, hs
}

func TestTLSRoundTrip(t *testing.T) {
	bundle, err := GenerateCerts()
	if err != nil {
		t.Fatal(err)
	}
	serverCreds, err := ServerCreds(bundle.ServerCertPEM, bundle.ServerKeyPEM, nil)
	if err != nil {
		t.Fatal(err)
	}
	lis, _ := startTLSServer(t, grpc.Creds(serverCreds))

	clientCreds, err := ClientCreds(bundle.CAPEM, nil, nil, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(clientCreds),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	u, err := userv1.NewUserServiceClient(conn).GetUser(ctx, &userv1.GetUserRequest{Id: 1})
	if err != nil {
		t.Fatalf("TLS 调用失败: %v", err)
	}
	if u.GetName() != "ok" {
		t.Fatalf("got %+v", u)
	}
	t.Log("TLS round trip OK")
}

func TestMTLSRejectsClientWithoutCert(t *testing.T) {
	bundle, err := GenerateCerts()
	if err != nil {
		t.Fatal(err)
	}
	// 服务端要求客户端证书；客户端只验证服务端，不带自己的证书。
	serverCreds, err := ServerCreds(bundle.ServerCertPEM, bundle.ServerKeyPEM, bundle.CAPEM)
	if err != nil {
		t.Fatal(err)
	}
	lis, _ := startTLSServer(t, grpc.Creds(serverCreds))

	clientCreds, err := ClientCreds(bundle.CAPEM, nil, nil, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(clientCreds),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := userv1.NewUserServiceClient(conn).GetUser(ctx, &userv1.GetUserRequest{Id: 1}); err == nil {
		t.Fatal("mTLS 下不带客户端证书应失败")
	} else {
		t.Logf("预期失败: %v", err)
	}
}

func TestMTLSAcceptsClientCert(t *testing.T) {
	bundle, err := GenerateCerts()
	if err != nil {
		t.Fatal(err)
	}
	serverCreds, err := ServerCreds(bundle.ServerCertPEM, bundle.ServerKeyPEM, bundle.CAPEM)
	if err != nil {
		t.Fatal(err)
	}
	lis, _ := startTLSServer(t, grpc.Creds(serverCreds))

	clientCreds, err := ClientCreds(bundle.CAPEM, bundle.ClientCertPEM, bundle.ClientKeyPEM, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(clientCreds),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := userv1.NewUserServiceClient(conn).GetUser(ctx, &userv1.GetUserRequest{Id: 1}); err != nil {
		t.Fatalf("带客户端证书应成功: %v", err)
	}
	t.Log("mTLS OK")
}

func TestHealthCheck(t *testing.T) {
	lis, hs := startTLSServer(t)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := CheckHealth(ctx, conn, "user.v1.UserService"); err != nil {
		t.Fatalf("健康检查应为 SERVING: %v", err)
	}

	// 模拟进入停机流程：先置 NOT_SERVING，负载均衡器会把实例摘掉。
	hs.SetServingStatus("user.v1.UserService", healthpb.HealthCheckResponse_NOT_SERVING)
	time.Sleep(20 * time.Millisecond)
	reply, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: "user.v1.UserService"})
	if err != nil {
		t.Fatalf("NOT_SERVING 时 Check 仍应返回状态: %v", err)
	}
	if reply.GetStatus() != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("got %v", reply.GetStatus())
	}
	t.Log("health check NOT_SERVING OK")
}
