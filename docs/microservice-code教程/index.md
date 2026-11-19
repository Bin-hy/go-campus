# Go 微服务实战教程

> 目标：把 gRPC、服务治理、消息可靠性、可观测性与 K8s 部署，从"背过八股"练到 **"能写、能跑、能排障"**。
> 与 [S4 微服务理论](/后端技术栈强化/04-microservice/架构与gRPC)、[S8 分布式理论](/后端技术栈强化/08-distributed/分布式系统能力全景) 互补：理论讲"为什么"，本教程讲"**怎么写、怎么跑、怎么验**"。

## 这条教程解决什么问题

很多人学微服务的路径是：看得懂架构图，说得清 CAP，但真让他写一个"有鉴权、有超时、能重试、能熔断、能追踪、能优雅发布"的服务，就卡住了。

这套教程把微服务拆成 20 个可验证的小能力，每个能力都对应一段能跑的 Go 代码：

- **写得出**：gRPC 四种流、拦截器、metadata、TLS、健康检查；
- **扛得住**：超时、重试、退避、幂等、熔断、限流、优雅停机；
- **不丢不重**：Kafka 手动提交、幂等消费、死信、Outbox、Saga；
- **看得见**：OpenTelemetry 链路、Prometheus 指标、Grafana 看板；
- **发得稳**：K8s 探针、滚动发布、HPA、PDB。

## 你需要什么

- Go 1.26+（本机实测 1.27；依赖链要求 1.26）；
- 可选：Docker（Kafka 实验）、minikube / 任一 K8s 集群（第 15 章）；
- 不装任何中间件也能完成 **14 个练习**：治理与消息类练习全部用接口抽象，离线可测。

```bash
cd code/microservice
go test ./...          # 14 个练习全部离线通过
go test -race ./02_governance/... ./03_messaging/... ./04_observability/...
```

::: tip 关于依赖下载
本机 `proxy.golang.org` 可能超时，拉依赖时可用：

```bash
export GOPROXY=https://goproxy.cn,direct
```
:::

## 学习路线

```mermaid
flowchart LR
    A[01 环境与骨架] --> B[02 四种 RPC]
    B --> C[03 拦截器与鉴权]
    C --> D[04 Deadline 与错误码]
    D --> E[05 Health / TLS]
    E --> F[06 Proto 兼容]
    F --> G[07 注册发现与 LB]
    G --> H[08 重试与幂等]
    H --> I[09 熔断限流与停机]
    I --> J[10 Kafka 幂等消费]
    J --> K[11 Outbox 与死信]
    K --> L[12 Saga]
    L --> M[13 OTel 链路]
    M --> N[14 Prometheus]
    N --> O[15 K8s 部署]
    O --> P[16 八项验收]
    P --> Q[17 面试题集]
    style A fill:#e6fcf5,stroke:#087f5b
    style G fill:#fff9db,stroke:#f59f00
    style J fill:#fff9db,stroke:#f59f00
    style O fill:#fff9db,stroke:#f59f00
    style P fill:#ffe3e3,stroke:#c92a2a
```

> 阶段对应关系：01-06 = gRPC 学完整；07-09 = 服务治理；10-12 = 数据与消息；13-15 = 可观测与部署；16 是验收，17 是面试收口。

## 章节导航

| 阶段 | 章 | 主题 | 代码 |
| ---- | -- | ---- | ---- |
| gRPC | [01](./01-环境与项目骨架) | 模块骨架、proto 工作流、分层 | `code/microservice/api` |
| gRPC | [02](./02-gRPC四种RPC与流式) | Unary / 服务端流 / 客户端流 / 双向流 | `01_grpc_basics/01_unary_stream` |
| gRPC | [03](./03-拦截器与Metadata鉴权) | 拦截器链、metadata、Bearer 鉴权 | `01_grpc_basics/02_interceptor_auth` |
| gRPC | [04](./04-Deadline取消与错误码) | deadline、取消传播、错误码分类 | `01_grpc_basics/03_deadline_cancel` |
| gRPC | [05](./05-健康检查连接与TLS) | Health、连接状态、TLS/mTLS | `01_grpc_basics/04_health_tls` |
| gRPC | [06](./06-Proto兼容性与版本演进) | 字段编号、reserved、未知字段 | `01_grpc_basics/05_proto_compat` |
| 治理 | [07](./07-注册发现与客户端负载均衡) | TTL/心跳、round-robin、P2C | `02_governance/01_registry_lb` |
| 治理 | [08](./08-超时重试指数退避与幂等) | 退避+jitter、幂等键、重试预算 | `02_governance/02_retry_idempotent` |
| 治理 | [09](./09-熔断限流与优雅停机) | 熔断三态、令牌桶、drain | `02_governance/03_resilience_graceful` |
| 消息 | [10](./10-Kafka可靠投递与幂等消费) | 手动提交、幂等表、死信 | `03_messaging/01_kafka_idempotent` |
| 消息 | [11](./11-重试死信与Outbox) | 本地消息表、relay、至少一次 | `03_messaging/02_outbox_dlq` |
| 消息 | [12](./12-Saga与最终一致性) | 编排、逆序补偿、幂等补偿 | `03_messaging/03_saga` |
| 观测 | [13](./13-OpenTelemetry全链路追踪) | traceparent、注入/提取 | `04_observability/01_otel_trace` |
| 观测 | [14](./14-Prometheus指标与Grafana) | RED、直方图、PromQL | `04_observability/02_metrics` |
| 部署 | [15](./15-K8s部署探针滚动发布与HPA) | 探针、滚动、HPA、PDB | `05_deploy/01_probe_hpa` `lab/k8s` |
| 验收 | [16](./16-实战验收8项清单) | 8 项能力逐条演练 | `lab/` 三实例 + 网关 |
| 面试 | [17](./17-面试题集) | 20 道高频追问 | — |

## 怎么用

1. **先读图与概念**，再进对应目录看 `README.md`。
2. 把 `solution.go` 的方法体清空，自己写一遍；跑 `go test -v`；最后对照 `answer/answer.go`。
3. 每章末尾的「练习」给出具体命令，「面试追问」用于自测——能连续答三层才算过。
4. 第 16 章是总验收：按清单逐条跑 lab，能复现、能解释、能排障。

## 与仓库其他栏目的关系

- 理论底座：[S4 Go 微服务](/后端技术栈强化/04-microservice/架构与gRPC)（拆分、gRPC 原理、治理概念）；
- 分布式深入：[S8 分布式理论](/后端技术栈强化/08-distributed/分布式系统能力全景)（CAP、Raft、etcd、gRPC LB、限流熔断）；
- 部署与编排：[K8s Code 教程](/k8s-code教程/)（client-go、Controller、HPA、探针）；
- 架构视角：[架构师修炼](/架构师修炼/)（一致性、幂等、容量规划、故障演练）。

本教程只做一件事：**把上面这些知识拧成一套能跑的微服务工程**。
