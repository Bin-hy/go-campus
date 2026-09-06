# 02 List：任务队列 + 最新动态

## 难度：⭐⭐ 中等

## 考点
- `LPUSH` + `BRPOP`：经典阻塞式消息队列
- `LTRIM`：固定长度的最新列表
- `LRANGE`：范围读取

## 题目描述

1. `SubmitTask`：把任务名 `LPUSH` 进 `task:queue`。
2. `FetchTask`：用 `BRPOP` 阻塞地从队列取一个任务，超时返回 `redis.Nil` 错误。
3. `PushFeed`：把 postID 加入 `feed:{uid}` 列表头部，并用 `LTRIM` 只保留最新 5 条，返回当前列表（最新在前）。

## 函数签名

```go
func SubmitTask(ctx context.Context, c *redis.Client, task string) error
func FetchTask(ctx context.Context, c *redis.Client, timeout time.Duration) (string, error)
func PushFeed(ctx context.Context, c *redis.Client, uid int64, postID string) ([]string, error)
```

## 提示

1. `c.BRPop(ctx, timeout, "task:queue")` 返回 `[]string{key, value}`，超时返回 `redis.Nil`
2. `LPUSH` + `RPOP/BRPOP` 才是「先进先出」的队列（左侧进、右侧出）
3. `LTRIM key 0 4` 表示只保留下标 0~4，即最新 5 条
4. LPUSH / LTRIM / LRANGE 可以用 Pipeline 合并发送（非必须）

## 运行测试

```bash
cd code/backend && docker compose up -d redis
cd 02-redis/05_types/02_list && go test -v
```
