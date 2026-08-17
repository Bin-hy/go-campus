# 缓存穿透：空值缓存

## 难度：⭐⭐ 中等

## 考点
- 缓存穿透的根因与方案
- 空值缓存（短 TTL）

## 环境准备

```bash
cd code/backend && docker compose up -d redis
```

## 题目描述

模拟"查询不存在的数据"：实现 `CachePenetration`，第一次查缓存 miss、打 DB 且发现不存在，缓存一个空值；第二次再查应命中空值缓存、不再打 DB。返回实际打 DB 的次数（应为 1）。

## 函数签名

```go
func CachePenetration(ctx context.Context, c *redis.Client) (dbHits int, err error)
```

## 提示

1. 假设 key 不存在于 DB，用一个 `dbHit` 计数器模拟打 DB
2. 第一次：`Get` miss → `dbHit++` → 缓存空值 `Set(key, "", 10s)`
3. 第二次：`Get` 命中空值 → 不再打 DB
4. 返回 `dbHit`

## 运行测试

```bash
cd code/backend/02-redis/02_cache && go test -v
```
