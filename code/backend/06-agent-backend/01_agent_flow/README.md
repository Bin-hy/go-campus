# Agent Backend 闭环：Redis → Kafka → 决策 → MySQL

## 难度：⭐⭐⭐ 困难

## 考点
- 四组件协作闭环
- Redis 会话记忆、Kafka 任务分发、MySQL 落库

## 环境准备

```bash
cd code/backend && docker compose up -d mysql redis kafka
```

## 题目描述

实现 `AgentFlow`，模拟后端 Agent 决策的完整闭环：

1. 写 Redis 会话上下文 `session:{taskID} = "帮我搜索 Go 后端资料"`
2. 投递任务到 Kafka（key = taskID）
3. 消费任务，读 Redis 上下文做决策（含"搜索"则选 search-tool）
4. 生成结果 `[search-tool] 结果：找到了 Go 后端资料`
5. 写回 Redis `result:{taskID}` + MySQL 落库（表 task_results）
6. 返回结果

## 函数签名

```go
func AgentFlow(ctx context.Context, db *sql.DB, rdb *redis.Client, broker, topic, taskID string) (result string, err error)
```

## 提示

- 建表：`CREATE TABLE IF NOT EXISTS task_results (task_id VARCHAR(64) PRIMARY KEY, result VARCHAR(255))`
- Kafka：Writer 投递，Reader（GroupID 唯一）消费
- 决策用 `strings.Contains(session, "搜索")`

## 运行测试

```bash
cd code/backend/06-agent-backend/01_agent_flow && go test -v
```
