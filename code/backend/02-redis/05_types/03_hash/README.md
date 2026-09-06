# 03 Hash：用户资料 + 购物车

## 难度：⭐ 入门

## 考点
- `HSET` / `HGETALL`：对象字段的读写
- `HINCRBY`：字段级原子自增（String 缓存做不到的部分更新）

## 题目描述

1. `SaveProfile`：把 name、city、age 写入 hash `user:{id}`。
2. `IncrAge`：给 `user:{id}` 的 age 字段加 1，返回新年龄和全部字段。
3. `AddToCart`：给购物车 `cart:{uid}` 中商品 sku 的数量加 delta（可为负），返回该商品当前数量。

## 函数签名

```go
func SaveProfile(ctx context.Context, c *redis.Client, id int64, name, city string, age int) error
func IncrAge(ctx context.Context, c *redis.Client, id int64) (int64, map[string]string, error)
func AddToCart(ctx context.Context, c *redis.Client, uid int64, sku string, delta int64) (int64, error)
```

## 提示

1. `c.HSet(ctx, key, "name", name, "city", city, "age", age)` 可一次写多个字段
2. `c.HIncrBy(ctx, key, "age", 1)` 返回增加后的值
3. `c.HGetAll(ctx, key)` 返回 `map[string]string`

## 运行测试

```bash
cd code/backend && docker compose up -d redis
cd 02-redis/05_types/03_hash && go test -v
```
