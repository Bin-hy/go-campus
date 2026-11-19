package metrics

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	userv1 "gocampus/microservice/api/user/v1"
)

type srv struct {
	userv1.UnimplementedUserServiceServer
}

func (srv) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.User, error) {
	if req.GetId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "id required")
	}
	time.Sleep(5 * time.Millisecond)
	return &userv1.User{Id: req.GetId(), Name: "metric"}, nil
}

func setup(t *testing.T) (userv1.UserServiceClient, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(grpc.UnaryInterceptor(m.UnaryServerInterceptor()))
	userv1.RegisterUserServiceServer(gs, srv{})
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
	return userv1.NewUserServiceClient(conn), reg
}

func TestMetricsCountAndCode(t *testing.T) {
	client, reg := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, _ = client.GetUser(ctx, &userv1.GetUserRequest{Id: 1})
	_, _ = client.GetUser(ctx, &userv1.GetUserRequest{Id: 0}) // InvalidArgument

	ok := counterValue(t, reg, "/user.v1.UserService/GetUser", "OK")
	bad := counterValue(t, reg, "/user.v1.UserService/GetUser", "InvalidArgument")
	if ok != 1 || bad != 1 {
		t.Fatalf("OK=%v InvalidArgument=%v", ok, bad)
	}
	t.Logf("requests_total{code=OK}=%v {code=InvalidArgument}=%v", ok, bad)
}

func TestMetricsHistogram(t *testing.T) {
	client, reg := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for i := 0; i < 3; i++ {
		_, _ = client.GetUser(ctx, &userv1.GetUserRequest{Id: 1})
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var count uint64
	for _, mf := range mfs {
		if mf.GetName() != "grpc_server_handling_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			count += m.GetHistogram().GetSampleCount()
		}
	}
	if count != 3 {
		t.Fatalf("histogram 采样数应为 3，实际 %d", count)
	}
	t.Logf("handling_seconds sample_count=%d", count)
}

func TestMetricsInflightReturnsToZero(t *testing.T) {
	client, reg := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _ = client.GetUser(ctx, &userv1.GetUserRequest{Id: 2})

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "grpc_server_inflight_requests" {
			continue
		}
		for _, m := range mf.GetMetric() {
			if got := m.GetGauge().GetValue(); got != 0 {
				t.Fatalf("请求结束后 inflight 应归零，实际 %v", got)
			}
		}
	}
}

func counterValue(t *testing.T, reg *prometheus.Registry, method, code string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "grpc_server_requests_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			if labelValue(m, "method") == method && labelValue(m, "code") == code {
				return m.GetCounter().GetValue()
			}
		}
	}
	t.Fatalf("未找到 counter method=%s code=%s", method, code)
	return 0
}

func labelValue(m *dto.Metric, name string) string {
	for _, l := range m.GetLabel() {
		if l.GetName() == name {
			return l.GetValue()
		}
	}
	return ""
}
