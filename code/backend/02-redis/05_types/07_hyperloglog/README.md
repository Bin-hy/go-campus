# 07 HyperLogLog：页面 UV 统计

## 难度：⭐ 入门

## 考点
- `PFADD` / `PFCOUNT`：基数估算
- `PFMERGE`：多个 HLL 合并（自动去重）
- 理解「12KB 固定内存 + 0.81% 误差」的取舍

## 题目描述

1. `AddUV`：记录页面 `uv:{page}` 的访问用户（可一次多个）。
2. `CountUV`：返回页面 UV 估算值。
3. `MergeUV`：把多个页面的 UV 合并到 `uv:{dest}`，返回合并后的 UV 估算值（跨页去重）。

## 函数签名

```go
func AddUV(ctx context.Context, c *redis.Client, page string, users ...string) error
func CountUV(ctx context.Context, c *redis.Client, page string) (int64, error)
func MergeUV(ctx context.Context, c *redis.Client, dest string, pages ...string) (int64, error)
```

## 提示

1. `c.PFAdd(ctx, key, users...)`，同一用户加多次也只算一次
2. `c.PFMerge(ctx, dest, sources...)` 合并后源 key 不变
3. 对比实验：同样的数据用 Set 存，`SCARD` 是精确值；HLL 是估算值

## 运行测试

```bash
cd code/backend && docker compose up -d redis
cd 02-redis/05_types/07_hyperloglog && go test -v
```
