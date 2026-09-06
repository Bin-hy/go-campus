# 01 String：缓存对象 + 计数器

## 难度：⭐ 入门

## 考点
- `SET k v EX n`：写值带过期时间
- `GET` / `TTL`：读值 / 查剩余过期时间
- `INCRBY`：原子计数器

## 题目描述

1. `CacheUser`：把用户信息序列化为 JSON 写入 `user:{id}`，过期 60 秒；再读回 JSON 和 TTL。
2. `IncrViews`：给 `article:{id}:views` 计数器原子加 n，返回最新值。

## 函数签名

```go
func CacheUser(ctx context.Context, c *redis.Client, id int64, name string) (jsonStr string, ttl time.Duration, err error)
func IncrViews(ctx context.Context, c *redis.Client, articleID int64, n int64) (int64, error)
```

## 提示

1. JSON 序列化用 `encoding/json` 的 `json.Marshal`
2. `c.Set(ctx, key, value, 60*time.Second)` 一条命令完成 SET + EXPIRE
3. `c.TTL(ctx, key)` 返回剩余时间（time.Duration）
4. 计数器不存在时 `INCRBY` 会从 0 开始，无需初始化

## 运行测试

```bash
cd code/backend && docker compose up -d redis
cd 02-redis/05_types/01_string && go test -v
```
