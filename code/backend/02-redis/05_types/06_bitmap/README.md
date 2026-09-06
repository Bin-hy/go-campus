# 06 Bitmap：用户签到 + 日活统计

## 难度：⭐⭐ 中等

## 考点
- `SETBIT` / `GETBIT`：按位读写
- `BITCOUNT`：统计 1 的个数
- `BITOP AND`：位运算求交集
- 理解 Bitmap 是 String 的位操作，offset 即「用户 ID / 日期序号」

## 题目描述

1. `Sign`：用户 uid 在 `sign:{uid}:{month}` 的第 day 天签到（offset = day-1）。
2. `SignCount`：统计该用户当月签到天数。
3. `MarkActive` / `ActiveCount`：记录并统计某天 `active:{date}` 的活跃用户（offset = uid）。
4. `BothActiveCount`：统计两天都活跃的用户数（BITOP AND + BITCOUNT）。

## 函数签名

```go
func Sign(ctx context.Context, c *redis.Client, uid int64, month string, day int) error
func SignCount(ctx context.Context, c *redis.Client, uid int64, month string) (int64, error)
func MarkActive(ctx context.Context, c *redis.Client, date string, uid int64) error
func ActiveCount(ctx context.Context, c *redis.Client, date string) (int64, error)
func BothActiveCount(ctx context.Context, c *redis.Client, date1, date2 string) (int64, error)
```

## 提示

1. `c.SetBit(ctx, key, offset, 1)`，offset 是 int64
2. BITOP 用 `c.Do(ctx, "BITOP", "AND", destKey, key1, key2)` 或 `c.BitOpAnd`（go-redis 有封装）
3. BITOP 的结果要落到一个临时 key，别忘了它的存在
4. 1 亿用户的日活 bitmap 只占约 12MB，想想为什么

## 运行测试

```bash
cd code/backend && docker compose up -d redis
cd 02-redis/05_types/06_bitmap && go test -v
```
