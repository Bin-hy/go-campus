# gRPC 四种 RPC：Unary / Server / Client / Bidi Stream

## 难度：⭐⭐ 中等

## 考点
- 四种 RPC 的适用场景与 Go 侧 API
- 服务端流 `Send`、客户端流 `Recv + SendAndClose`、双向流的分帧
- `io.EOF` 在流里的语义（客户端正常结束 / 服务端正常结束）
- `bufconn`：不占端口、在内存里起真实 gRPC server 的测试方式

## 题目描述

实现 `UserServer`（内嵌 `userv1.UnimplementedUserServiceServer`），完成四个方法：

| 方法 | 类型 | 行为 |
| ---- | ---- | ---- |
| `GetUser` | 一元 | 按 id 查用户；不存在返回 error |
| `ListUsers` | 服务端流 | 按 `page_size` 推送前 N 个用户 |
| `UploadUsers` | 客户端流 | 收完全部用户后返回 `{accepted, rejected}`；空名字计为 rejected |
| `Chat` | 双向流 | 收到一条回一条 `"echo: "+text`，客户端的 `CloseSend` 触发 `io.EOF` |

初始数据 1..7，`user-01` ~ `user-07`。

## 函数签名

```go
type UserServer struct {
    userv1.UnimplementedUserServiceServer
    // 自行设计存储
}

func NewUserServer() *UserServer

func (s *UserServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.User, error)
func (s *UserServer) ListUsers(req *userv1.ListUsersRequest, stream userv1.UserService_ListUsersServer) error
func (s *UserServer) UploadUsers(stream userv1.UserService_UploadUsersServer) error
func (s *UserServer) Chat(stream userv1.UserService_ChatServer) error
```

## 提示

1. 存储用 `map[int64]*userv1.User` + `sync.RWMutex`；返回前**克隆**一份，避免调用方修改内部数据（`proto.Clone`）。
2. 客户端流里 `Recv()` 返回 `io.EOF` 表示客户端发完了，此时调用 `SendAndClose`。
3. 双向流里任意一侧 `io.EOF` 都表示对方关闭了发送方向；服务端收到后直接 `return nil`。
4. 测试用 `bufconn.Listen` + `grpc.WithContextDialer`，服务端地址写 `passthrough:///bufnet` 即可。

## 验收

- [ ] `GetUser(3)` 返回 `user-03`
- [ ] `ListUsers(page_size=3)` 恰好收到 3 条
- [ ] 客户端流发 3 条（含 1 条空名字）返回 `accepted=2 rejected=1`
- [ ] 双向流发两条收到两条 echo
- [ ] `go test -race ./01_grpc_basics/01_unary_stream` 通过

## 运行

```bash
cd code/microservice
go test -v ./01_grpc_basics/01_unary_stream
```

## 面试追问

1. **四种流分别适合什么场景？** 一元=普通查询；服务端流=大结果集分页/订阅推送；客户端流=批量上传/聚合；双向流=实时对话/协同。
2. **流的背压怎么处理？** HTTP/2 流控窗口天然限速；goroutine 里不断 `Send` 会被 `Send` 阻塞，不要额外再开无界队列。
3. **服务端流为什么不能用 `defer stream.Send`？** Send 是即时动作，必须显式在循环里判断错误。
