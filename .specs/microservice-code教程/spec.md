# Go 微服务实战教程 Spec

## 背景

用户是 Go 开发者，目标字节跳动 AI 应用开发实习。已有 [S4 微服务理论](/后端技术栈强化/04-microservice/架构与gRPC)、[S8 分布式理论](/后端技术栈强化/08-distributed/分布式系统能力全景) 与 [K8s Code 教程](/k8s-code教程/)，但缺少一套"能写、能跑、能验"的 Go 微服务实战主线。

用户给出的学习计划分四阶段：gRPC 完整 → 服务治理 → 数据与消息 → 可观测与部署，并要求完成 8 项实战验收。

## 目标

- 新建独立栏目 `docs/microservice-code教程/`：17 篇（首页 + 15 章 + 验收 + 面试题集）。
- 新建独立模块 `code/microservice/`：14 个练习 + 三实例 Lab + 网关 + K8s 清单。
- 与 S4/S8/K8s 教程互链，不重复讲原理，只讲"怎么写、怎么跑、怎么验"。
- 所有练习离线可测；Lab 可本机真实启动三实例并观察负载均衡、幂等、优雅停机。

## 功能需求

- F1 环境与骨架：module 结构、proto 生成、分层、bufconn 测试。
- F2 gRPC：四种 RPC、拦截器与 metadata、deadline/取消/错误码、health/TLS/mTLS、proto 兼容。
- F3 治理：注册发现 TTL/心跳、round-robin/P2C、重试退避+jitter+幂等、熔断限流、优雅停机。
- F4 消息：Kafka 至少一次 + 幂等消费 + 死信、Outbox relay、Saga 补偿。
- F5 观测：OTel traceparent 跨 gRPC 传播、Prometheus RED 指标、PromQL/Grafana/告警。
- F6 部署：Deployment/Service/Headless、三种探针、preStop、滚动发布、HPA/PDB。
- F7 验收：8 项清单逐条复现步骤 + 预期 + 失败模式 + 排障剧本。
- F8 面试：20+ 高频题，三层追问。
- F9 代码规范：每题 `README.md + solution.go + solution_test.go + answer/answer.go`；Lab 可 `go run`。
- F10 集成：VitePress 导航/侧边栏、S4 索引、首页推荐顺序、`code/README.md`。

## 非功能需求

- N1 可运行：`go build ./...`、`go vet ./...` 全绿；`solution.go` 骨架可编译；参考答案覆盖后可 `go test ./...` 全绿，治理/消息/观测练习 `-race` 通过。
- N2 风格一致：中文、比喻、mermaid、每章「练习 / 面试追问 / 串起来」。
- N3 渐进：四阶段对应四组目录，练习编号与章节编号对齐。
- N4 零外部依赖测试：Kafka/etcd 用接口抽象，真 Kafka 作为可选手工实验。

## 不做的事

- 不引入服务网格（Istio）与云厂商托管服务。
- 不实现完整网关/注册中心产品，只做教学级实现与真实组件对照。
- 不覆盖多集群、跨云容灾细节（S8/架构师栏目已有）。
- 不跑真实 Kafka 集成测试作为 CI 必过项。

## 验收标准

- AC1 `docs/microservice-code教程/` 存在 ≥ 17 篇，每章含 mermaid、练习、面试追问、串起来。
- AC2 `code/microservice/` 14 个练习骨架可编译；用 `answer/answer.go` 覆盖后 `go test ./...` 全绿、关键目录 `-race` 全绿；提交时 solution 骨架入库并设为 skip-worktree。
- AC3 三实例 Lab + 网关可本机运行，8 项验收清单可复现。
- AC4 VitePress 注册导航/侧边栏，与 S4/S8/K8s 互链。
- AC5 `npm run docs:build` 构建通过。
