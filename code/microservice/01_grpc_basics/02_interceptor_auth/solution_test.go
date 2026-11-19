package interceptor_auth

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	userv1 "gocampus/microservice/api/user/v1"
)

type stubServer struct {
	userv1.UnimplementedUserServiceServer
	lastSubject string
}

func (s *stubServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.User, error) {
	s.lastSubject = SubjectFromContext(ctx)
	return &userv1.User{Id: req.GetId(), Name: "alice"}, nil
}

func (s *stubServer) ListUsers(req *userv1.ListUsersRequest, stream userv1.UserService_ListUsersServer) error {
	s.lastSubject = SubjectFromContext(stream.Context())
	return stream.Send(&userv1.User{Id: 1, Name: "stream"})
}

func validToken(ctx context.Context, tok string) (string, error) {
	if tok == "good-token" {
		return "user:42", nil
	}
	return "", errors.New("bad token")
}

// withToken 手工往 metadata 里塞 Bearer token（模拟不使用客户端拦截器的情况）。
func withToken(ctx context.Context, tok string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok)
}

func setup(t *testing.T) (userv1.UserServiceClient, *stubServer) {
	t.Helper()
	stub := &stubServer{}
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(
		grpc.UnaryInterceptor(UnaryAuthInterceptor(validToken)),
		grpc.StreamInterceptor(StreamAuthInterceptor(validToken)),
	)
	userv1.RegisterUserServiceServer(gs, stub)
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
	return userv1.NewUserServiceClient(conn), stub
}

func TestUnaryRejectsMissingToken(t *testing.T) {
	client, _ := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: 1})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("缺 token 应为 Unauthenticated，实际 %v", status.Code(err))
	}
}

func TestUnaryRejectsBadToken(t *testing.T) {
	client, _ := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn := client.(interface{})
	_ = conn
	// 手工带一个错误 token。
	ctx = withToken(ctx, "bad-token")
	_, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: 1})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("错误 token 应为 PermissionDenied，实际 %v", status.Code(err))
	}
}

func TestUnaryPassesSubject(t *testing.T) {
	stub := &stubServer{}
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(grpc.UnaryInterceptor(UnaryAuthInterceptor(validToken)))
	userv1.RegisterUserServiceServer(gs, stub)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(ClientAuthInterceptor("good-token")), // 客户端自动附加
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := userv1.NewUserServiceClient(conn).GetUser(ctx, &userv1.GetUserRequest{Id: 1}); err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if stub.lastSubject != "user:42" {
		t.Fatalf("handler 应拿到 subject=user:42，实际 %q", stub.lastSubject)
	}
	t.Log("鉴权拦截器透传 subject OK")
}

func TestStreamAuth(t *testing.T) {
	stub := &stubServer{}
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer(grpc.StreamInterceptor(StreamAuthInterceptor(validToken)))
	userv1.RegisterUserServiceServer(gs, stub)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(ClientStreamAuthInterceptor("good-token")),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stream, err := userv1.NewUserServiceClient(conn).ListUsers(ctx, &userv1.ListUsersRequest{PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("recv: %v", err)
	}
	if stub.lastSubject != "user:42" {
		t.Fatalf("流式 handler 应拿到 subject，实际 %q", stub.lastSubject)
	}

	// 无 token 的流必须被拒。
	conn2, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn2.Close() })
	s2, err := userv1.NewUserServiceClient(conn2).ListUsers(ctx, &userv1.ListUsersRequest{PageSize: 1})
	if err == nil {
		_, err = s2.Recv()
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("无 token 的流应为 Unauthenticated，实际 %v", status.Code(err))
	}
}
