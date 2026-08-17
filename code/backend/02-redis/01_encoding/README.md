# Redis 底层编码切换：intset → listpack

## 难度：⭐⭐ 中等

## 考点
- OBJECT ENCODING 查看底层结构
- 小集合的紧凑编码与切换条件

## 环境准备

```bash
cd code/backend && docker compose up -d redis
```

## 题目描述

实现 `EncodingSwitch`：往一个 Set 里先加纯整数，再加一个字符串，返回两次的 `OBJECT ENCODING` 编码值。

## 函数签名

```go
func EncodingSwitch(ctx context.Context, c *redis.Client) (before, after string, err error)
```

## 提示

1. 清空当前库 `FlushDB`
2. `SAdd(key, 1, 2, 3, 4, 5)` 后查编码 —— 纯整数小集合是 `intset`
3. `SAdd(key, "hello")` 后再查编码 —— 混入非整数后是 `listpack`（Redis 7 小集合）
4. 用 `c.Do(ctx, "OBJECT", "ENCODING", key)` 查编码

## 运行测试

```bash
cd code/backend/02-redis/01_encoding && go test -v
```
