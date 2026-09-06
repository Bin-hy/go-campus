# 05 ZSet：热搜排行榜 + 延迟任务队列

## 难度：⭐⭐ 中等

## 考点
- `ZADD` / `ZINCRBY`：带分数的成员与原子加分
- `ZREVRANGE ... WITHSCORES`：Top N 排行榜
- `ZRANGEBYSCORE` + `ZREM`：按分数范围取并删除（延迟队列核心）

## 题目描述

1. `AddHot`：给热搜榜 `hot:search` 中的关键词加 delta 热度。
2. `TopN`：返回热搜榜前 n 名（分数从高到低，带分数）。
3. `AddDelayTask`：把任务加入延迟队列 `delay:queue`，score 为执行时间戳（毫秒）。
4. `PopDueTasks`：取出所有 score <= now 的到期任务并从队列删除，返回任务列表。

## 函数签名

```go
func AddHot(ctx context.Context, c *redis.Client, keyword string, delta float64) error
func TopN(ctx context.Context, c *redis.Client, n int64) ([]redis.Z, error)
func AddDelayTask(ctx context.Context, c *redis.Client, task string, runAt time.Time) error
func PopDueTasks(ctx context.Context, c *redis.Client, now time.Time) ([]string, error)
```

## 提示

1. `c.ZRevRangeWithScores(ctx, key, 0, n-1)` 一次拿到成员和分数
2. 时间戳用 `runAt.UnixMilli()` 转 float64 作为 score
3. 取到期任务：`c.ZRangeByScore(ctx, key, &redis.ZRangeBy{Min: "-inf", Max: 毫秒值})`
4. 取完记得 `ZRem`，否则任务会被重复消费（思考：高并发下如何原子化？可用 Lua 或 ZRANGEBYSCORE+ZREM 事务）

## 运行测试

```bash
cd code/backend && docker compose up -d redis
cd 02-redis/05_types/05_zset && go test -v
```
