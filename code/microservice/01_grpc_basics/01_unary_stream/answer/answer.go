package unary_stream

import (
	"context"
	"fmt"
	"io"
	"sync"

	"google.golang.org/protobuf/proto"

	userv1 "gocampus/microservice/api/user/v1"
)

// 参考答案 / 可运行实现。
//
// 学习方式：先读 README，把本文件的方法体清空（改成 panic("not implemented")），
// 自己实现后再跑测试，最后与 answer/answer.go 对照。

// UserServer 内存版用户服务，实现四种 RPC。
type UserServer struct {
	userv1.UnimplementedUserServiceServer

	mu     sync.RWMutex
	users  map[int64]*userv1.User
	nextID int64
}

func NewUserServer() *UserServer {
	s := &UserServer{users: make(map[int64]*userv1.User)}
	for i := int64(1); i <= 7; i++ {
		s.users[i] = &userv1.User{Id: i, Name: fmt.Sprintf("user-%02d", i), Email: fmt.Sprintf("u%d@campus.dev", i), Version: 1}
		s.nextID = i
	}
	return s
}

// GetUser 一元调用：查一个用户。
func (s *UserServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[req.GetId()]
	if !ok {
		return nil, fmt.Errorf("user %d not found", req.GetId())
	}
	cp := proto.Clone(u).(*userv1.User)
	return cp, nil
}

// ListUsers 服务端流：按 page_size 分批推送。
func (s *UserServer) ListUsers(req *userv1.ListUsersRequest, stream userv1.UserService_ListUsersServer) error {
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = 3
	}
	s.mu.RLock()
	list := make([]*userv1.User, 0, s.nextID)
	for i := int64(1); i <= s.nextID; i++ {
		list = append(list, s.users[i])
	}
	s.mu.RUnlock()

	for i, u := range list {
		if i >= pageSize {
			break
		}
		if err := stream.Send(u); err != nil {
			return err
		}
	}
	return nil
}

// UploadUsers 客户端流：收完流后一次性返回汇总。
func (s *UserServer) UploadUsers(stream userv1.UserService_UploadUsersServer) error {
	var accepted, rejected int32
	for {
		u, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&userv1.UploadSummary{Accepted: accepted, Rejected: rejected})
		}
		if err != nil {
			return err
		}
		if u.GetName() == "" {
			rejected++
			continue
		}
		s.mu.Lock()
		s.nextID++
		u.Id = s.nextID
		u.Version = 1
		s.users[u.Id] = u
		s.mu.Unlock()
		accepted++
	}
}

// Chat 双向流：收到一条回一条。
func (s *UserServer) Chat(stream userv1.UserService_ChatServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := stream.Send(&userv1.ChatMessage{From: "server", Text: "echo: " + msg.GetText()}); err != nil {
			return err
		}
	}
}
