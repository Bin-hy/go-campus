# Go 微服务实战教程 Plan

## 目录设计

```text
docs/microservice-code教程/
├── index.md                         # 首页、路线图、章节导航
├── 01-环境与项目骨架.md
├── 02-gRPC四种RPC与流式.md
├── 03-拦截器与Metadata鉴权.md
├── 04-Deadline取消与错误码.md
├── 05-健康检查连接与TLS.md
├── 06-Proto兼容性与版本演进.md
├── 07-注册发现与客户端负载均衡.md
├── 08-超时重试指数退避与幂等.md
├── 09-熔断限流与优雅停机.md
├── 10-Kafka可靠投递与幂等消费.md
├── 11-重试死信与Outbox.md
├── 12-Saga与最终一致性.md
├── 13-OpenTelemetry全链路追踪.md
├── 14-Prometheus指标与Grafana.md
├── 15-K8s部署探针滚动发布与HPA.md
├── 16-实战验收8项清单.md
└── 17-面试题集.md
```

## 代码设计

```text
code/microservice/                  # module gocampus/microservice
├── api/user/v1/                    # proto + 生成代码
├── 01_grpc_basics/                 # 5 题
├── 02_governance/                  # 3 题
├── 03_messaging/                   # 3 题
├── 04_observability/               # 2 题
├── 05_deploy/                      # 1 题
└── lab/
    ├── uservice/                   # 完整服务 + Dockerfile
    ├── gateway/                    # 客户端 LB + 重试 + 熔断 + 优雅停机
    └── k8s/                        # Deployment/Service/HPA/PDB
```

## 阶段映射

| 用户计划 | 章节 | 练习 |
| -------- | ---- | ---- |
| 第一阶段 gRPC（1 周） | 01-06 | 01_grpc_basics（5） |
| 第二阶段 治理（2 周） | 07-09 | 02_governance（3） |
| 第三阶段 数据与消息（2 周） | 10-12 | 03_messaging（3） |
| 第四阶段 可观测与部署（2 周） | 13-15 | 04_observability（2）+ 05_deploy（1） |
| 实战验收 | 16 | lab 三实例 + 网关 + K8s |
| 面试收口 | 17 | — |

## 技术选型

| 组件 | 版本 | 理由 |
| ---- | ---- | ---- |
| google.golang.org/grpc | v1.77.0 | 当前缓存可用、API 稳定 |
| protobuf | v1.36.10 | 与 protoc-gen-go 匹配 |
| otel | v1.47.0 | trace/tracetest 完整 |
| prometheus/client_golang | v1.23.2 | 官方客户端 |
| segmentio/kafka-go | v0.4.51 | 仅文档示例，不进入 go.mod（练习用接口抽象，避免未使用依赖） |

## 风险与取舍

- **无 Docker 也能学**：治理/消息练习用接口抽象，避免 CI 依赖中间件。
- **Lab 不替代生产框架**：教学级注册中心/熔断器，讲清语义，文档给真实组件对照。
- **proto 生成物提交**：CI 不依赖 protoc；`make proto` 保证可复现。
- **HPA/PDB 需要集群**：清单提供，验证作为可选实操，公式用纯 Go 练习兜底。
