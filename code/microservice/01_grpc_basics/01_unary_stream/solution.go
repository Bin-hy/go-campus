package unary_stream

import (
	"context"
	"sync"

	userv1 "gocampus/microservice/api/user/v1"
)

// UserServer 内存版用户服务，实现四种 RPC。
// 提示：用 map[int64]*userv1.User + sync.RWMutex 保存数据；
// 返回前用 proto.Clone 复制，避免把内部状态暴露给调用方。
type UserServer struct {
	userv1.UnimplementedUserServiceServer

	mu     sync.RWMutex
	users  map[int64]*userv1.User
	nextID int64
}

// NewUserServer 初始化 1..7 号用户（user-01 ~ user-07）。
func NewUserServer() *UserServer {
	// TODO: 初始化 users 与 nextID
	panic("not implemented")
}

// GetUser 一元调用：按 id 查用户，不存在返回错误。
func (s *UserServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.User, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// ListUsers 服务端流：按 page_size 推送前 N 个用户；page_size<=0 用默认值。
func (s *UserServer) ListUsers(req *userv1.ListUsersRequest, stream userv1.UserService_ListUsersServer) error {
	// TODO: 实现你的代码（注意 Send 返回错误时要退出）
	panic("not implemented")
}

// UploadUsers 客户端流：收完全部用户后返回 {accepted, rejected}；
// 空名字计为 rejected，io.EOF 表示客户端发送完毕。
func (s *UserServer) UploadUsers(stream userv1.UserService_UploadUsersServer) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Chat 双向流：收到一条回一条 "echo: "+text；io.EOF 表示对方关闭发送方向。
func (s *UserServer) Chat(stream userv1.UserService_ChatServer) error {
	// TODO: 实现你的代码
	panic("not implemented")
}
