# Redis 分布式锁：SET NX EX + Lua 安全解锁

## 难度：⭐⭐⭐ 困难

## 考点
- SET NX EX 原子加锁（防止崩溃死锁）
- Lua 保证"判断 + 删除"原子（防止误删别人的锁）

## 环境准备

```bash
cd code/backend && docker compose up -d redis
```

## 题目描述

实现 `AcquireLock` 和 `ReleaseLock`：

- `AcquireLock`：用 `SET key value NX EX` 原子加锁，返回是否成功。
- `ReleaseLock`：用 Lua 脚本，只有当锁的 value 与自己匹配时才删除，返回是否解锁成功。

## 函数签名

```go
func AcquireLock(ctx context.Context, c *redis.Client, key, val string) (bool, error)
func ReleaseLock(ctx context.Context, c *redis.Client, key, val string) (bool, error)
```

## 提示

1. `AcquireLock` 用 `c.SetNX(ctx, key, val, 30*time.Second)`（SetNX 内部即 SET NX EX）
2. `ReleaseLock` 用 `c.Eval` 执行 Lua：`if redis.call("GET",KEYS[1])==ARGV[1] then return redis.call("DEL",KEYS[1]) else return 0 end`

## 运行测试

```bash
cd code/backend/02-redis/03_lock && go test -v
```
