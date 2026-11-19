package unary_stream

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	userv1 "gocampus/microservice/api/user/v1"
)

// 用 bufconn 在内存里起一个真实 gRPC server，避免依赖端口与网络。
func startTestServer(t *testing.T) userv1.UserServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	userv1.RegisterUserServiceServer(gs, NewUserServer())
	go func() {
		if err := gs.Serve(lis); err != nil {
			t.Logf("server stopped: %v", err)
		}
	}()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn)
}

func TestUnary(t *testing.T) {
	c := startTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	u, err := c.GetUser(ctx, &userv1.GetUserRequest{Id: 3})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.GetName() != "user-03" || u.GetEmail() == "" {
		t.Fatalf("unexpected user: %+v", u)
	}
	t.Logf("unary OK: %+v", u)
}

func TestServerStream(t *testing.T) {
	c := startTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := c.ListUsers(ctx, &userv1.ListUsersRequest{PageSize: 3})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	var got int
	for {
		u, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		got++
		if u.GetId() == 0 {
			t.Fatal("empty user")
		}
	}
	if got != 3 {
		t.Fatalf("服务端流应返回 3 条，实际 %d", got)
	}
	t.Log("server stream OK")
}

func TestClientStream(t *testing.T) {
	c := startTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := c.UploadUsers(ctx)
	if err != nil {
		t.Fatalf("UploadUsers: %v", err)
	}
	// 2 条合法 + 1 条空名字（应被拒绝）
	if err := stream.Send(&userv1.User{Name: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&userv1.User{Name: ""}); err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&userv1.User{Name: "b"}); err != nil {
		t.Fatal(err)
	}
	sum, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv: %v", err)
	}
	if sum.GetAccepted() != 2 || sum.GetRejected() != 1 {
		t.Fatalf("汇总错误: accepted=%d rejected=%d", sum.GetAccepted(), sum.GetRejected())
	}
	t.Logf("client stream OK: %+v", sum)
}

func TestBidiStream(t *testing.T) {
	c := startTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := c.Chat(ctx)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	for _, text := range []string{"hello", "world"} {
		if err := stream.Send(&userv1.ChatMessage{From: "client", Text: text}); err != nil {
			t.Fatal(err)
		}
		reply, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		if reply.GetText() != "echo: "+text {
			t.Fatalf("got %q", reply.GetText())
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatal(err)
	}
	t.Log("bidi stream OK")
}
