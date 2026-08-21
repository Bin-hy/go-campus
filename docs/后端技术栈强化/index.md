# 后端技术栈强化

> 面向字节「后端 agent 决策」岗位的后端能力补齐。按 **S1 → S9** 依赖顺序学（S7 为工程化底座加分项，S8 为分布式理论兜底，S9 为海量素材落地），共 16 周。

## 学习路径

```text
S1 MySQL（数据层）→ S2 Redis（缓存层）→ S3 Kafka（异步层）
        → S4 微服务（服务层）→ S5 高并发场景题（综合）→ S6 Agent Backend（串联）
        → S7 K8s（工程化底座）→ S8 分布式理论（共识与一致性兜底）
        → S9 对象存储（海量素材落地）
```

- 每个核心模块都要达到五级能力：**L1 原理 → L2 机制 → L3 故障 → L4 设计 → L5 追问**（能连续回答三层追问不卡壳）。
- 每篇文档按「原理 → 机制 → 故障 → 设计」组织，末尾带面试追问。

---

## S1 MySQL 深入（2.5 周）

| 文章 | 内容 |
|------|------|
| [存储引擎与 B+ 树](./01-mysql/存储引擎与B+树) | InnoDB 体系、B+ 树、Buffer Pool、redo/WAL、三大日志 |
| [事务与 MVCC](./01-mysql/事务与MVCC) | ACID、隔离级别、read view、快照读 vs 当前读、锁、死锁 |
| [索引与 SQL 优化](./01-mysql/索引与SQL优化) | 回表/覆盖/最左前缀/索引下推、explain、慢查询 |
| [主从复制与高可用](./01-mysql/主从复制与高可用) | binlog 复制、半同步、主从延迟、MHA/MGR |
| [面试题集](./01-mysql/面试题集) | 6 题追问 3 层 |

## S2 Redis 深入（2 周）

| 文章 | 内容 |
|------|------|
| [数据结构底层](./02-redis/数据结构底层) | SDS、dict 渐进 rehash、skiplist、底层切换 |
| [持久化与高可用](./02-redis/持久化与高可用) | RDB/AOF、主从、哨兵、Cluster |
| [缓存问题与一致性](./02-redis/缓存问题与一致性) | 穿透/击穿/雪崩、Cache Aside、延迟双删、binlog |
| [分布式锁与场景](./02-redis/分布式锁与场景) | SET NX EX、watchdog、红锁争议、限流/布隆/ID |
| [面试题集](./02-redis/面试题集) | 6 题追问 3 层 |

## S3 Kafka 深入（2 周）

| 文章 | 内容 |
|------|------|
| [架构与存储](./03-kafka/架构与存储) | partition/ISR、顺序写、零拷贝、acks |
| [生产消费语义](./03-kafka/生产消费语义) | 幂等、事务、消费组、rebalance、offset |
| [可靠性与积压](./03-kafka/可靠性与积压) | 不丢/不重/顺序、积压处理 |
| [面试题集](./03-kafka/面试题集) | 6 题追问 3 层 |

## S4 Go 微服务（2 周）

| 文章 | 内容 |
|------|------|
| [架构与 gRPC](./04-microservice/架构与gRPC) | 拆分原则、Protobuf、HTTP/2、四种流 |
| [治理与稳定性](./04-microservice/治理与稳定性) | 注册发现、负载均衡、可观测性、优雅上下线、熔断限流降级 |
| [面试题集](./04-microservice/面试题集) | 6 题追问 3 层 |

## S5 高并发场景题（2.5 周）

| 文章 | 场景 |
|------|------|
| [场景题（上）](./05-high-concurrency/场景题-上) | 秒杀、抢红包、分布式 ID、缓存一致性 |
| [场景题（中）](./05-high-concurrency/场景题-中) | 分布式锁、限流、接口幂等、消息可靠性 |
| [场景题（下）](./05-high-concurrency/场景题-下) | Feed 流、点赞计数、排行榜、短链、海量数据 |

## S6 Agent Backend（1 周）

| 文章 | 内容 |
|------|------|
| [系统设计与串联](./06-agent-backend/系统设计与串联) | 四组件串联成 Agent 决策系统 |

## S7 K8s 容器编排（1.5 周）

| 文章 | 内容 |
|------|------|
| [架构与核心对象](./07-k8s/架构与核心对象) | 控制面/数据面、Pod、声明式 API、核心对象 |
| [调度与控制器](./07-k8s/调度与控制器) | 控制循环、滚动更新、HPA、requests/limits |
| [网络与存储](./07-k8s/网络与存储) | Service/Ingress/DNS、CNI、PV/PVC/StorageClass |
| [高可用与故障排查](./07-k8s/高可用与故障排查) | 探针、OOM、get/describe/logs 排障、etcd 高可用 |
| [面试题集](./07-k8s/面试题集) | 6 题追问 3 层 |

---

## S8 分布式理论（1 周）

> 分布式基础理论兜底，与 S4 微服务、S7 K8s（etcd）互相印证。配套：[第二阶段 · 分布式系统面试详解](/第二阶段-知识详解/分布式系统面试详解)（考点凝练背诵版）。

| 文章 | 内容 |
|------|------|
| [CAP 与 BASE 理论](./08-distributed/CAP与BASE理论) | CAP 证明与误区、PACELC、CP/AP 选型、最终一致落地模式 |
| [Raft 算法详解](./08-distributed/Raft算法详解) | 选举/日志复制/提交规则/安全性/成员变更/快照/线性一致读、etcd 工程实现 |
| [面试题集](./08-distributed/面试题集) | 8 题追问 3 层 |

## S9 对象存储（1.5 周）

> 面向剪映/CapCut AI 剪辑的海量素材落地：对象存储原理、S3 API 实战、安全与生产实践。配套代码走真实 MinIO SDK（`minio-go/v7`），本地学、线上换 endpoint 即用于 TOS/OSS/COS。

| 文章 | 内容 |
|------|------|
| [为什么需要对象存储](./09-object-storage/为什么需要对象存储) | 块/文件/对象三模型、扁平命名空间、对象不可变、S3 兼容生态 |
| [架构与核心机制](./09-object-storage/架构与核心机制) | 三层架构、一致性哈希、多副本 vs 纠删码、multipart、一致性模型 |
| [S3 API 与 Go 实战](./09-object-storage/S3-API与Go实战) | minio-go CRUD、手写分片三步协议、预签名 URL 直传直下、CDN 回源 |
| [安全与生产实践](./09-object-storage/安全与生产实践) | 三层权限与最小化、预签名边界、加密、防盗刷、版本/对象锁、AIGC 校验 |
| [面试题集](./09-object-storage/面试题集) | 6 题追问 3 层 |

## 配套实验

实验代码在 `code/backend/`（MySQL/Redis/Kafka 实验需 `docker compose up -d` 起中间件，`go test ./...` 验证；09-object-storage 需 `docker compose up -d minio` 起 MinIO 容器；07-k8s 为纯 Go 模拟，直接 `go test` 即可，可选装 kind 起单机集群 + `kubectl` 实操）：

| 模块 | 实验 |
|------|------|
| 01-mysql | 隔离级别、explain、MVCC、死锁 |
| 02-redis | 编码切换、缓存三大问题、分布式锁、一致性 |
| 03-kafka | 生产消费、手动提交、顺序、幂等 |
| 04-microservice | 熔断状态机、令牌桶、滑动窗口 |
| 05-scenarios | 秒杀防超卖、接口幂等 |
| 06-agent-backend | Redis→Kafka→MySQL 完整闭环 |
| 07-k8s | 纯 Go 模拟：HPA 控制循环/调度/滚动更新/Service 转发/探针（code/backend/07-k8s）；可选 kind 单机集群实操 |
| 09-object-storage | 分片上传三步协议、预签名直传直下、最小权限桶策略（真实 MinIO SDK，需 MinIO 容器） |

## 待定（backlog）

分布式进阶（分库分表 / 分布式事务 / 分布式锁 / 一致性哈希）、etcd 手写实践 —— 后续单独规划。（分布式理论部分已由 S8 覆盖）
