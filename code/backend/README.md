# Go 后端技术栈实验（code/backend）

> 配合 [后端技术栈强化](/后端技术栈强化/) 学习模块的配套练习，每个练习对应一篇学习文章。

## 使用方法

```bash
# 1. 起中间件（MySQL 8 / Redis 7 / Kafka KRaft / MinIO；07-k8s 为纯 Go 模拟，无需中间件）
docker compose up -d

# 2. 进入题目目录，阅读 README.md，补全 solution.go
cd 01-mysql/01_isolation && go test -v

# 3. 全部测试（依赖中间件已起）
go test ./...
```

做题流程：读 README → 补 `solution.go`（替换 `panic("not implemented")`）→ `go test -v` 验证 → 对照 `answer/answer.go`。

> 注意：`solution.go` 是题目骨架。07-k8s 与 09-object-storage 模块的骨架已随仓库提交并设置 `skip-worktree`（本地填写答案后的改动会被忽略，不会误提交）；其余模块由 `.gitignore` 忽略。参考答案统一在 `answer/answer.go`。

## 目录结构（26 个练习，全部对齐 phase1 结构）

| 模块 | 练习 |
|------|------|
| 01-mysql | 01_isolation、02_explain、03_mvcc、04_deadlock |
| 02-redis | 01_encoding、02_cache、03_lock、04_consistency |
| 03-kafka | 01_produce_consume、02_manual_commit、03_order、04_idempotent |
| 04-microservice | 01_circuit_breaker、02_ratelimit |
| 05-scenarios | 01_seckill、02_idempotent |
| 06-agent-backend | 01_agent_flow |
| 07-k8s | 01_reconcile、02_scheduler、03_rolling_update、04_service_lb、05_probe |
| 09-object-storage | 01_multipart_upload、02_presigned_url、03_bucket_policy、04_sts_assume_role |

每个练习目录结构统一：

```text
NN_topic/
├── README.md          题目 + 考点 + 难度 + 环境准备 + 提示
├── solution.go        待填空（panic("not implemented")）
├── solution_test.go   测试
└── answer/answer.go   参考答案
```
