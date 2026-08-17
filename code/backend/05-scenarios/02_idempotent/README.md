# 接口幂等：唯一键 SETNX 拦截重复

## 难度：⭐⭐ 中等

## 考点
- 幂等与唯一键
- Redis SETNX 的"仅首次成功"语义

## 环境准备

```bash
cd code/backend && docker compose up -d redis
```

## 题目描述

实现 `Idempotent`：模拟"同一订单重复提交"。

1. 用 `SETNX order:12345` 模拟首次下单（应成功）
2. 再次 `SETNX order:12345` 模拟重复下单（应失败，被拦截）

返回重复请求是否被拦截（应为 true）。

## 函数签名

```go
func Idempotent(ctx context.Context, c *redis.Client) (blocked bool, err error)
```

## 提示

- `c.SetNX(ctx, "order:12345", "1", 0)` 首次返回 true，第二次返回 false
- 返回"第二次是否失败"（即 !dup）

## 运行测试

```bash
cd code/backend/05-scenarios/02_idempotent && go test -v
```
