# Interceptor 与 Metadata：统一鉴权，不污染业务代码

## 难度：⭐⭐ 中等

## 考点
- Unary / Stream 两类拦截器，以及 `grpc.ChainUnaryInterceptor` 的顺序
- `metadata`：`authorization: Bearer <token>` 的读取与注入
- 把 subject 放进 `context`，handler 直接取用
- 流式拦截器必须包一层 `ServerStream` 才能替换 `Context()`
- 错误码：缺 token → `Unauthenticated`；token 无效 → `PermissionDenied`

## 题目描述

实现四个拦截器：

1. `UnaryAuthInterceptor(validator)`：从 incoming metadata 取 Bearer token，校验通过后把 subject 写入 ctx；
2. `StreamAuthInterceptor(validator)`：同样校验，且 **handler 里 `stream.Context()` 能拿到 subject**；
3. `ClientAuthInterceptor(token)`：客户端一元拦截器，自动附加 authorization；
4. `ClientStreamAuthInterceptor(token)`：客户端流拦截器，同样附加。

并实现 `SubjectFromContext(ctx) string`。

## 函数签名

```go
type Validator func(ctx context.Context, token string) (subject string, err error)

func UnaryAuthInterceptor(v Validator) grpc.UnaryServerInterceptor
func StreamAuthInterceptor(v Validator) grpc.StreamServerInterceptor
func ClientAuthInterceptor(token string) grpc.UnaryClientInterceptor
func ClientStreamAuthInterceptor(token string) grpc.StreamClientInterceptor
func SubjectFromContext(ctx context.Context) string
```

## 提示

1. metadata 的 key 小写；`md.Get("authorization")` 返回 slice。
2. 不要用字符串拼接错误：统一 `status.Error(codes.X, msg)`。
3. 流式拦截器里，`handler` 拿到的是原始 `ServerStream`；要透传 ctx 必须：

```go
type wrappedStream struct {
    grpc.ServerStream
    ctx context.Context
}
func (w *wrappedStream) Context() context.Context { return w.ctx }
```

4. 客户端拦截器里 `metadata.AppendToOutgoingContext(ctx, ...)` 即可，不用手动 `Set`。

## 验收

- [ ] 不带 token → `Unauthenticated`
- [ ] 错误 token → `PermissionDenied`
- [ ] 正确 token → handler 中 `SubjectFromContext` 得到 `user:42`
- [ ] 流式接口同样生效（无 token 的流在第一次 `Recv` 时失败）
- [ ] `go test -race ./01_grpc_basics/02_interceptor_auth`

## 运行

```bash
cd code/microservice
go test -v ./01_grpc_basics/02_interceptor_auth
```

## 面试追问

1. **拦截器顺序怎么决定？** 按注册顺序"洋葱模型"执行：`ChainUnaryInterceptor(recovery, metrics, trace, auth)`，恢复要在最外层才能兜住 panic。
2. **鉴权为什么放拦截器而不是网关？** 网关做粗粒度（是否登录），服务做细粒度（能否操作这个资源）；两边都做，零信任。
3. **`metadata` 和 `context.Value` 的区别？** metadata 跨进程（走 HTTP/2 header），context.Value 只在进程内传递。
