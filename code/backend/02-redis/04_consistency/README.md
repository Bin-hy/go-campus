# 缓存一致性：Cache Aside（先更新 DB 再删缓存）

## 难度：⭐⭐ 中等

## 考点
- Cache Aside 的读写顺序
- 为什么"先更新 DB 再删缓存"

## 环境准备

```bash
cd code/backend && docker compose up -d redis
```

## 题目描述

用一个 map 模拟数据库（初始 `item:1 = "old-value"`）。实现 `CacheAside`：

1. 读：缓存 miss → 读 DB → 回填缓存
2. 写：**先更新 DB，再删缓存**（Cache Aside 顺序）
3. 写后再读：因缓存已删，应回源 DB 拿到新值

返回"写后再读"得到的值。

## 函数签名

```go
func CacheAside(ctx context.Context, c *redis.Client) (afterWrite string, err error)
```

## 提示

1. 初始 DB `item:1 = "old-value"`
2. 首次读：缓存 miss，回填 `old-value`
3. 写：DB 更新为 `new-value`，然后 `Del` 缓存
4. 再读：缓存已删，回源 DB 拿到 `new-value`
5. 返回 `new-value`

## 运行测试

```bash
cd code/backend/02-redis/04_consistency && go test -v
```
