# Health Checking 与 TLS / mTLS

## 难度：⭐⭐⭐ 困难

## 考点
- 标准 gRPC Health Checking 协议（`grpc.health.v1.Health/Check`）
- 返回 `NOT_SERVING` 与"直接断连"的区别：前者让 LB 主动摘流量
- TLS 服务端/客户端 credentials、`ServerName` 校验
- mTLS：`RequireAndVerifyClientCert`，双向校验
- 用 `crypto/x509` 现场生成 CA/证书，测试零外部依赖

## 题目描述

实现：

1. `GenerateCerts() (*CertBundle, error)`：自签 CA + 服务端证书（DNS `localhost`/`uservice.local`，IP `127.0.0.1`）+ 客户端证书；
2. `ServerCreds(certPEM, keyPEM, clientCAPEM)`：`clientCAPEM` 非空时开启 mTLS；
3. `ClientCreds(caPEM, clientCertPEM, clientKeyPEM, serverName)`：携带可选客户端证书；
4. `CheckHealth(ctx, conn, service)`：调用健康检查协议。

## 函数签名

```go
type CertBundle struct {
    CAPEM, ServerCertPEM, ServerKeyPEM, ClientCertPEM, ClientKeyPEM []byte
}

func GenerateCerts() (*CertBundle, error)
func ServerCreds(certPEM, keyPEM, clientCAPEM []byte) (credentials.TransportCredentials, error)
func ClientCreds(caPEM, clientCertPEM, clientKeyPEM []byte, serverName string) (credentials.TransportCredentials, error)
func CheckHealth(ctx context.Context, conn *grpc.ClientConn, service string) error
```

## 提示

1. CA 证书要 `IsCA: true` + `KeyUsageCertSign`；签发时用 `x509.CreateCertificate(rand, tmpl, caCert, &key.PublicKey, caKey)`。
2. 服务端证书必须包含客户端拨号时使用的名字（测试里是 `localhost`），否则 `x509: certificate is valid for ...`。
3. mTLS 服务端：`cfg.ClientAuth = tls.RequireAndVerifyClientCert` + `ClientCAs`。
4. `bufconn` 上也能跑 TLS 握手——它只是替换了底层 `net.Conn`。
5. 健康检查 `Check` 在 `NOT_SERVING` 时仍返回 OK 的 RPC（body 里是状态），而 `Watch` 会持续推送。

## 验收

- [ ] TLS 一元调用成功
- [ ] mTLS：不带客户端证书失败，带证书成功
- [ ] Health `Check` 能读到 `SERVING` → `NOT_SERVING`
- [ ] `go test ./01_grpc_basics/04_health_tls`

## 运行

```bash
cd code/microservice
go test -v ./01_grpc_basics/04_health_tls
```

## 面试追问

1. **readiness 用 health check 还是 TCP 探活？** gRPC 服务用 `grpc_health_probe` 或 K8s 原生 `grpc` 探针；TCP 只能证明端口开着，不能证明能处理请求。
2. **TLS 在哪里终止？** 入口常用 Ingress/网关终止；内部服务间建议 mTLS（零信任），或用服务网格自动注入。
3. **证书轮换怎么做？** `tls.Config.GetCertificate` 动态加载 + `GetClientCertificate`；K8s 用 cert-manager 挂 Secret，进程内监听文件变化。
