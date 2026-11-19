# Lab：uservice 完整服务

> 教程章节：02 四种 RPC、03 拦截器、04 Deadline、05 Health/TLS、09 优雅停机、13 Trace、14 Metrics、15 部署

## 它演示什么

| 能力 | 代码位置 | 验证方式 |
| ---- | -------- | -------- |
| 一元 / 服务端流 / 客户端流 / 双向流 | `GetUser` `ListUsers` `UploadUsers` `Chat` | grpcurl 或网关 |
| 鉴权拦截器 | `unaryAuth` `streamAuth` | 不带 token → Unauthenticated |
| 恢复拦截器 | `unaryRecovery` | handler panic 不影响进程 |
| OTel 链路 | `unaryTrace` + `-otlp` | 控制台输出 span |
| Prometheus 指标 | `metrics.unary` + `/metrics` | `curl :9101/metrics` |
| Health Checking | `grpc_health_v1` | `grpcurl ... grpc.health.v1.Health/Check` |
| 幂等写接口 | `Charge` | 同 key 第二次 `deduplicated=true`（内存表；多实例需共享存储，见第 08 章） |
| 优雅停机 | `signal.NotifyContext` + `GracefulStop` | Ctrl+C 观察日志 |
| TLS / mTLS | `-cert -key` | 证书可用 `04_health_tls` 的 `GenerateCerts` 生成 |

## 本地运行

```bash
# 终端 A/B/C
go run ./lab/uservice -addr :9001 -instance user-1 -metrics-addr :9101
go run ./lab/uservice -addr :9002 -instance user-2 -metrics-addr :9102
go run ./lab/uservice -addr :9003 -instance user-3 -metrics-addr :9103

# 用 grpcurl 直接调（服务端已开启反射，不需要 proto 文件）
grpcurl -plaintext -H 'authorization: Bearer campus-secret' \
  -d '{"id": 1}' 127.0.0.1:9001 user.v1.UserService/GetUser

# 健康检查
grpcurl -plaintext 127.0.0.1:9001 grpc.health.v1.Health/Check

# 指标
curl -s http://127.0.0.1:9101/metrics | grep -E 'grpc_server_(requests|handling|inflight)'

# 优雅停机：Ctrl+C 后应看到 draining → graceful stop done
```

## 观察负载均衡

启动三个实例 + 网关（见 `lab/gateway`），连续请求 `/user`，响应里的 `name` 会带 `@user-N` 后缀。

## 故障注入

- `metadata: x-fail=true` → 返回 `Unavailable`（用于观察重试/熔断）；
- 停掉一个实例 → 网关的 round_robin 自动把流量分到剩余实例；
- 把 `-token` 改成其他值、网关仍用旧 token → 全部 `Unauthenticated`。
