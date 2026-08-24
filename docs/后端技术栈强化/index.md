# 后端技术栈强化

> 面向字节「后端 agent 决策」岗位的后端能力补齐。按 **S1 → S10** 依赖顺序学（S7 为工程化底座加分项，S8 为分布式理论兜底，S9 为海量素材落地，S10 为 AI 应用的向量检索底座），共 18 周。

## 学习路径

```text
S1 MySQL（数据层）→ S2 Redis（缓存层）→ S3 Kafka（异步层）
        → S4 微服务（服务层）→ S5 高并发场景题（综合）→ S6 Agent Backend（串联）
        → S7 K8s（工程化底座）→ S8 分布式理论（共识与一致性兜底）
        → S9 对象存储（海量素材落地）→ S10 向量数据库 Milvus（AI 检索底座）
```

- 每个核心模块都要达到五级能力：**L1 原理 → L2 机制 → L3 故障 → L4 设计 → L5 追问**（能连续回答三层追问不卡壳）。
- 每篇文档按「原理 → 机制 → 故障 → 设计」组织，末尾带面试追问。

---

## S1 MySQL 深入（3 周）

> **结构**：先过一遍 MySQL 基础（库表/SQL/表设计/事务索引入门，4 章），再进深入篇。深入篇重点三件套：**B+ 树索引 → 事务隔离级别与 MVCC → SQL 慢查询优化**。

### 基础篇（先过一遍，2~3 天）

| 文章 | 内容 |
|------|------|
| [MySQL 入门与库表操作](./01-mysql/基础-MySQL入门与库表操作) | MySQL 逻辑架构、库/表 DDL、字符集 utf8mb4、ALTER 锁表坑、TRUNCATE vs DELETE |
| [SQL 语法详解](./01-mysql/基础-SQL语法详解) | SELECT 执行顺序、WHERE/GROUP BY/HAVING、五类 JOIN、子查询、窗口函数、常用函数 |
| [表设计与约束范式](./01-mysql/基础-表设计与约束范式) | 数据类型选型（DECIMAL/主键 BIGINT）、约束、主键设计、三范式与反范式、设计陷阱 |
| [事务与索引入门](./01-mysql/基础-事务与索引入门) | ACID 四性、事务语法、脏读/不可重复读/幻读、四种隔离级别入门、索引是什么 |

### 深入篇（重点：B+ 树 / 隔离级别 / 慢查询优化）

| 文章 | 内容 |
|------|------|
| [存储引擎与 B+ 树](./01-mysql/存储引擎与B+树) | InnoDB 体系、B+ 树（3 层存 2200 万行怎么算）、Buffer Pool、redo/WAL、三大日志、两阶段提交 |
| [事务与 MVCC](./01-mysql/事务与MVCC) | ACID、**四种隔离级别**、read view、快照读 vs 当前读、next-key lock、死锁 |
| [索引与 SQL 优化](./01-mysql/索引与SQL优化) | 聚簇/二级索引、回表/覆盖/最左前缀、索引失效场景、EXPLAIN 入门 |
| [慢查询优化实战](./01-mysql/慢查询优化实战) | 慢查询日志、EXPLAIN 全字段、OPTIMIZER_TRACE；**六大类优化手段**（重构SQL减量/隐式转换与函数/排序分组陷阱/冷热分离·分区表·分库分表/锁与事务姿势/SQL Hints）、profiling 实测验证、完整案例 |
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
| [单体拆分与边界设计](./04-microservice/单体拆分与边界设计) | 什么时候拆、边界（DDD 限界上下文/康威/database-per-service）、迁移技巧（绞杀者/防腐层/数据双写/避免 2PC/灰度） |
| [架构与 gRPC](./04-microservice/架构与gRPC) | 拆分原则、Protobuf、HTTP/2、四种流 |
| [治理与稳定性](./04-microservice/治理与稳定性) | 注册发现、负载均衡、可观测性、优雅上下线、熔断限流降级 |
| [面试题集](./04-microservice/面试题集) | 8 题追问 3 层 |

## S5 高并发场景题（2.5 周）

> 先读第一篇（一致性与故障边界）建立"故障/一致性"底层逻辑，再逐个场景实战。

| 文章 | 场景 |
|------|------|
| [一致性与故障边界](./05-high-concurrency/一致性与故障边界) | 一致性强弱谱系、故障模型、分区下 AP/CP 完整行为、MySQL/Redis/etcd/Kafka/注册中心/锁/分布式事务"节点挂了"逐层分析、最终一致 vs 强一致实现图谱、5 条深挖追问链 |
| [场景题（上）](./05-high-concurrency/场景题-上) | 秒杀、抢红包、分布式 ID、缓存一致性（每个场景带故障与一致性边界） |
| [场景题（中）](./05-high-concurrency/场景题-中) | 分布式锁（主从丢锁/脑裂/fencing token）、限流、接口幂等、消息可靠性（四段防线） |
| [场景题（下）](./05-high-concurrency/场景题-下) | Feed 流、点赞计数、排行榜、短链、海量数据（分片/扩容/对账） |

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
| [etcd 详解与工程实践](./08-distributed/etcd详解与工程实践) | MVCC/Watch/Lease/事务 CAS、分布式锁、服务发现与注册中心、watch 风暴、K8s 场景 |
| [面试题集](./08-distributed/面试题集) | 10 题追问 3 层 |

## S9 对象存储（1.5 周）

> 面向剪映/CapCut AI 剪辑的海量素材落地：对象存储原理、S3 API 实战、安全与生产实践、STS 临时凭证。配套代码走真实 MinIO SDK（`minio-go/v7`），本地学、线上换 endpoint 即用于 TOS/OSS/COS。

| 文章 | 内容 |
|------|------|
| [为什么需要对象存储](./09-object-storage/为什么需要对象存储) | 块/文件/对象三模型、扁平命名空间、对象不可变、S3 兼容生态 |
| [架构与核心机制](./09-object-storage/架构与核心机制) | 三层架构、一致性哈希、多副本 vs 纠删码、multipart、一致性模型 |
| [S3 API 与 Go 实战](./09-object-storage/S3-API与Go实战) | minio-go CRUD、手写分片三步协议、预签名 URL 直传直下、CDN 回源 |
| [安全与生产实践](./09-object-storage/安全与生产实践) | 三层权限与最小化、预签名边界、加密、防盗刷、版本/对象锁、AIGC 校验 |
| [STS 临时凭证](./09-object-storage/STS临时凭证) | 不下发长期 AK、AssumeRole 换临时三元组、会话策略取交集、客户端直传 scoped 凭证链路 |
| [面试题集](./09-object-storage/面试题集) | 6 题追问 3 层 |

## S10 向量数据库 Milvus（2 周）

> 面向 AI 应用（RAG / 素材语义检索）的向量检索底座，与第三阶段 RAG 章节、S9 对象存储互相印证。从**向量检索原理**（ANN：IVF/HNSW/PQ）讲到 Milvus 架构、索引检索、一致性、部署与 Go 实战，最后收在面试题集。配套代码走真实 `milvus-go-sdk/v2`（需 `docker compose up -d` 起 Milvus standalone）。

| 文章 | 内容 |
|------|------|
| [向量检索原理](./10-milvus/01-向量检索原理) | Embedding、L2/内积/余弦、精确 vs ANN、IVF/HNSW/PQ/DISKANN、Recall@k/QPS 评估 |
| [Milvus 架构与核心概念](./10-milvus/02-Milvus架构与核心概念) | 定位与选型（vs FAISS/ES/pgvector）、存算分离架构（proxy/coordinator/datanode/querynode/indexnode + etcd/MinIO/Pulsar）、Collection/Segment/Shard |
| [Collection 设计与数据写入](./10-milvus/03-Collection设计与数据写入) | Schema/主键/向量字段/动态字段、分区、写入 WAL 全链路、Segment 生命周期 |
| [向量索引与检索](./10-milvus/04-向量索引与检索) | 索引全景（FLAT/IVF_FLAT/IVF_SQ8/IVF_PQ/HNSW/DISKANN/GPU）、Search/Query、标量过滤下推、HybridSearch 混合检索 |
| [一致性·事务与数据管理](./10-milvus/05-一致性事务与数据管理) | 四种一致性级别与 TSO 水印、DML 事务、Load/Release、Flush/Compact、TTL、milvus-backup |
| [部署·监控与生产实践](./10-milvus/06-部署监控与生产实践) | standalone vs cluster、Docker Compose/K8s Operator、资源扩容、Prometheus/Grafana、故障排查 |
| [Milvus Go 实战](./10-milvus/07-Milvus-Go实战) | milvus-go-sdk 连接/建 Collection/插入/索引/检索/过滤完整代码、文档语义检索小项目 |
| [面试题集](./10-milvus/08-面试题集) | 8 题追问 3 层：ANN 原理、HNSW/IVF-PQ 参数、选型、过滤下推、一致性、评估指标、RAG 落地 |

## 配套实验

实验代码在 `code/backend/`（MySQL/Redis/Kafka 实验需 `docker compose up -d` 起中间件，`go test ./...` 验证；09-object-storage 需 `docker compose up -d minio` 起 MinIO 容器；10-milvus 需 `docker compose -f code/backend/docker-compose.milvus.yml up -d` 起 Milvus standalone；07-k8s 为纯 Go 模拟，直接 `go test` 即可，可选装 kind 起单机集群 + `kubectl` 实操）：

| 模块 | 实验 |
|------|------|
| 01-mysql | 隔离级别、explain、MVCC、死锁 |
| 02-redis | 编码切换、缓存三大问题、分布式锁、一致性 |
| 03-kafka | 生产消费、手动提交、顺序、幂等 |
| 04-microservice | 熔断状态机、令牌桶、滑动窗口 |
| 05-scenarios | 秒杀防超卖、接口幂等 |
| 06-agent-backend | Redis→Kafka→MySQL 完整闭环 |
| 07-k8s | 纯 Go 模拟：HPA 控制循环/调度/滚动更新/Service 转发/探针（code/backend/07-k8s）；可选 kind 单机集群实操 |
| 09-object-storage | 分片上传三步协议、预签名直传直下、最小权限桶策略、STS 最小权限临时凭证（真实 MinIO SDK，需 MinIO 容器） |
| 10-milvus | 连接/建 Collection/插入/检索/过滤全链路（真实 milvus-go-sdk，`code/backend/10-milvus`：`go run .` 跑七步演示、`go test -v .` 跑集成测试；需 `docker compose -f code/backend/docker-compose.milvus.yml up -d` 起 Milvus standalone） |

## 待定（backlog）

分布式进阶（分库分表 / 分布式事务 / 一致性哈希）—— 后续单独规划。（分布式理论已由 S8 覆盖：CAP/BASE、Raft、etcd 详解与工程实践；etcd/分布式锁/RPC 治理的 Go 手写实践见 [路线专题 Day 8-9 代码集](/习题集和答案/route/)。）
