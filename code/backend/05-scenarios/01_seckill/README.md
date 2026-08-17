# 秒杀防超卖：Redis Lua 原子扣减

## 难度：⭐⭐⭐ 困难

## 考点
- Redis Lua 保证"判断 + 扣减"原子性
- 高并发下防超卖

## 环境准备

```bash
cd code/backend && docker compose up -d redis
```

## 题目描述

实现 `Seckill`：初始化库存 10，用 100 个 goroutine 并发抢，每个 goroutine 用 Lua 脚本原子扣减库存（库存 > 0 才 `DECR`）。返回成功抢到的次数（应为 10，不超卖）。

## 函数签名

```go
func Seckill(ctx context.Context, c *redis.Client) (success int, err error)
```

## 提示

1. `c.Set(ctx, "seckill:stock", 10, 0)` 初始化库存
2. Lua 脚本：`local stock = tonumber(redis.call("GET",KEYS[1])); if stock and stock > 0 then return redis.call("DECR",KEYS[1]) end; return -1`
3. 100 个 goroutine 并发 `c.Eval`，返回值 >= 0 表示抢成功，计数 +1
4. 用 sync.WaitGroup 等待全部完成

## 运行测试

```bash
cd code/backend/05-scenarios/01_seckill && go test -v
```
