# Go 微服务实战教程 Task

## 一、代码骨架

- [x] 创建 `code/microservice` module 与 `api/user/v1/user.proto`
- [x] protoc 生成 `user.pb.go`、`user_grpc.pb.go`
- [x] `Makefile`：proto / build / test / race / run
- [x] 依赖：grpc、protobuf、otel、prometheus、kafka-go

## 二、练习（14）

- [x] 01_grpc_basics/01_unary_stream
- [x] 01_grpc_basics/02_interceptor_auth
- [x] 01_grpc_basics/03_deadline_cancel
- [x] 01_grpc_basics/04_health_tls
- [x] 01_grpc_basics/05_proto_compat
- [x] 02_governance/01_registry_lb
- [x] 02_governance/02_retry_idempotent
- [x] 02_governance/03_resilience_graceful
- [x] 03_messaging/01_kafka_idempotent
- [x] 03_messaging/02_outbox_dlq
- [x] 03_messaging/03_saga
- [x] 04_observability/01_otel_trace
- [x] 04_observability/02_metrics
- [x] 05_deploy/01_probe_hpa
- [x] 每题 README + solution + test + answer
- [x] 骨架 `go build ./...` 通过；用 answer 覆盖后 `go test ./...` 全绿；关键目录 `-race` 全绿
- [x] solution.go 为待填空骨架，answer/answer.go 为完整参考
- [x] solution 首次入库后设为 skip-worktree

## 三、Lab

- [x] lab/uservice：四种 RPC、拦截器、health、metrics、trace、TLS、优雅停机
- [x] lab/gateway：round_robin、deadline、retryPolicy、熔断、幂等、优雅停机
- [x] lab/k8s：Deployment（3 副本、三探针、preStop）、Service/Headless、HPA、PDB、Gateway
- [x] Dockerfile（uservice/gateway）
- [x] 三实例 + 网关本地运行说明

## 四、文档（17 篇 + 首页）

- [x] index
- [x] 01 环境与项目骨架
- [x] 02 gRPC 四种 RPC 与流式
- [x] 03 拦截器与 Metadata 鉴权
- [x] 04 Deadline、取消与错误码
- [x] 05 健康检查、连接与 TLS
- [x] 06 Proto 兼容性与版本演进
- [x] 07 注册发现与客户端负载均衡
- [x] 08 超时重试、指数退避与幂等
- [x] 09 熔断、限流与优雅停机
- [x] 10 Kafka 可靠投递与幂等消费
- [x] 11 重试、死信与事务性 Outbox
- [x] 12 Saga 与最终一致性
- [x] 13 OpenTelemetry 全链路追踪
- [x] 14 Prometheus 指标与 Grafana
- [x] 15 K8s 部署、探针、滚动发布与 HPA
- [x] 16 实战验收：8 项清单
- [x] 17 面试题集

## 五、集成

- [x] VitePress 顶栏入口
- [x] VitePress 侧边栏（四阶段 + 验收 + 相关链接）
- [x] `docs/后端技术栈强化/index.md` S4 互链
- [x] `docs/index.md` 推荐顺序
- [x] `code/README.md` 目录树
