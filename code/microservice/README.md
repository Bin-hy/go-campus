# Go 微服务实战（code/microservice）

> 配套文档：[Go 微服务实战教程](/microservice-code教程/)
> 目标：把 gRPC、服务治理、消息可靠性、可观测性与 K8s 部署从"看过"练到"能写、能跑、能排障"。

本模块是一个独立 Go module，包含两部分：

1. **练习**（`01_grpc_basics` ~ `05_deploy`）：每题 4 件套，`go test` 即可离线验证；
2. **Lab**（`lab/uservice`、`lab/gateway`、`lab/k8s`）：可真实启动多实例、负载均衡、优雅停机与 Prometheus 观测。

## 快速开始

```bash
cd code/microservice

# 1. 全部练习测试（无需 Docker / 网络）
go test ./...

# 2. 带数据竞争检测（治理类练习建议必跑）
go test -race ./02_governance/... ./03_messaging/... ./04_observability/...

# 3. 单题
go test -v ./01_grpc_basics/02_interceptor_auth
```

> 做题方式：先读题目目录里的 `README.md`，把 `solution.go` 的方法体清空后自己实现，
> 跑通测试，再对照 `answer/answer.go`。所有 `answer/` 目录不参与编译测试。
>
> 提交约定：仓库 `.gitignore` 忽略了 `**/solution.go`。按本仓库惯例，新增练习首次提交需
> `git add -f 01_grpc_basics/01_unary_stream/solution.go ...` 把骨架纳入版本库，后续改动再忽略。

## 练习地图

| 阶段 | 目录 | 题目 | 对应验收项 |
| ---- | ---- | ---- | ---------- |
| 一 · gRPC | `01_grpc_basics/01_unary_stream` | 四种 RPC：一元 / 服务端流 / 客户端流 / 双向流 | — |
| 一 · gRPC | `01_grpc_basics/02_interceptor_auth` | Unary/Stream 鉴权拦截器、metadata 透传 subject | — |
| 一 · gRPC | `01_grpc_basics/03_deadline_cancel` | Deadline、取消传播、错误码分类、服务端超时兜底 | 下游超时能及时终止 |
| 一 · gRPC | `01_grpc_basics/04_health_tls` | Health Checking、TLS/mTLS、自签证书 | 探针阻止流量进入未就绪实例 |
| 一 · gRPC | `01_grpc_basics/05_proto_compat` | 字段编号/线类型兼容、未知字段保留 | — |
| 二 · 治理 | `02_governance/01_registry_lb` | 注册中心 TTL/心跳、round-robin、P2C | 多实例负载均衡 |
| 二 · 治理 | `02_governance/02_retry_idempotent` | 指数退避 + jitter、幂等键 singleflight | 重试不重复扣款 |
| 二 · 治理 | `02_governance/03_resilience_graceful` | 令牌桶、熔断三态、优雅停机 drain | 滚动发布与优雅停机 |
| 三 · 消息 | `03_messaging/01_kafka_idempotent` | 手动提交、幂等表、重试、死信 | 重启不丢不重 |
| 三 · 消息 | `03_messaging/02_outbox_dlq` | 事务性 Outbox、relay、至少一次 | 事件最终发出 |
| 三 · 消息 | `03_messaging/03_saga` | 编排式 Saga、逆序补偿 | — |
| 四 · 观测 | `04_observability/01_otel_trace` | gRPC 拦截器 + W3C traceparent 跨服务传播 | 一次请求全链路可追 |
| 四 · 观测 | `04_observability/02_metrics` | Prometheus 计数器/直方图/在途 Gauge | 错误率与延迟可观测 |
| 四 · 部署 | `05_deploy/01_probe_hpa` | startup/readiness/liveness、HPA 公式 | 按负载扩容、滚动发布 |

## Lab：三实例 + 网关

```bash
# 终端 A/B/C：三个实例（实例名会拼进响应，方便看命中谁）
go run ./lab/uservice -addr :9001 -instance user-1 -metrics-addr :9101
go run ./lab/uservice -addr :9002 -instance user-2 -metrics-addr :9102
go run ./lab/uservice -addr :9003 -instance user-3 -metrics-addr :9103

# 终端 D：网关（HTTP :8080）
go run ./lab/gateway -targets 127.0.0.1:9001,127.0.0.1:9002,127.0.0.1:9003

# 轮询验证：name 后缀会在 user-1/2/3 之间切换
for i in $(seq 1 12); do curl -s "http://127.0.0.1:8080/user?id=1"; echo; done

# 幂等扣款：同一个 key 只有第一次 deduplicated=false
curl -s "http://127.0.0.1:8080/charge?key=k1&amount=100"; echo
curl -s "http://127.0.0.1:8080/charge?key=k1&amount=100"; echo

# 故障注入：服务端返回 Unavailable，观察 gRPC 自动重试与最终状态
curl -s "http://127.0.0.1:8080/fail"; echo

# 指标
curl -s http://127.0.0.1:9101/metrics | grep grpc_server
```

停掉一个实例（Ctrl+C）后再压测网关：请求应全部落到剩余实例；实例会先在 metadata 里把
health 置为 NOT_SERVING，`GracefulStop` 等待在途请求结束，10 秒兜底强制退出。

## K8s 清单

`lab/k8s/` 下有可直接 `kubectl apply -f` 的 Deployment / Service / HPA / PDB：

```bash
kubectl apply -f lab/k8s/
kubectl get pods,svc,hpa -w
```

镜像构建与推送见 `lab/k8s/README.md`。

## 目录约定

```text
code/microservice/
├── api/user/v1/           # proto 契约 + protoc 生成的 Go 代码
├── 01_grpc_basics/        # 每题：README + solution.go + solution_test.go + answer/
├── 02_governance/
├── 03_messaging/
├── 04_observability/
├── 05_deploy/
├── lab/
│   ├── uservice/          # 完整可运行服务：四种 RPC + 拦截器 + health + metrics + 优雅停机
│   ├── gateway/           # 客户端 LB + 截止时间 + 重试 + 熔断 + HTTP 入口
│   └── k8s/               # Deployment / Service / HPA / PDB
└── Makefile
```

## 重新生成 proto

```bash
make proto
# 等价于
protoc -I api --go_out=. --go_opt=module=gocampus/microservice \
       --go-grpc_out=. --go-grpc_opt=module=gocampus/microservice \
       api/user/v1/user.proto
```
