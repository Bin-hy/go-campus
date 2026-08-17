# EXPLAIN 索引优化：无索引全表扫 vs 加索引走 ref

## 难度：⭐⭐ 中等

## 考点
- EXPLAIN 的 type 字段（ALL vs ref）
- 索引对执行计划的影响
- 数据选择性对优化器的影响

## 环境准备

```bash
cd code/backend && docker compose up -d mysql
```

## 题目描述

实现 `CompareExplain`：建一张表，插入 500 行 `name` 各不相同的记录，返回「加索引前」和「加索引后」查询 `WHERE name='user_250'` 的 EXPLAIN `type` 值。

## 函数签名

```go
func CompareExplain(ctx context.Context, db *sql.DB) (before, after string, err error)
```

## 提示

1. 建表 `t_explain(id INT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(50), age INT)`
2. 循环插入 `name = "user_%d"`（保证选择性高，否则优化器可能仍选全表扫）
3. 用 `db.Query("EXPLAIN " + query)` 读取结果，取 `type` 列
4. 加索引 `CREATE INDEX idx_name ON t_explain(name)` 后再查一次

## 运行测试

```bash
cd code/backend/01-mysql/02_explain && go test -v
```
