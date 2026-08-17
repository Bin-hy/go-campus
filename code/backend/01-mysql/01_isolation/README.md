# 隔离级别验证：可重复读

## 难度：⭐⭐ 中等

## 考点
- RR（可重复读）隔离级别
- 事务内两次读的一致性
- 并发事务提交的可见性

## 环境准备

先起 MySQL（见 `code/backend/docker-compose.yml`）：

```bash
cd code/backend && docker compose up -d mysql
```

## 题目描述

MySQL 默认隔离级别是 **RR（可重复读）**。实现 `VerifyRepeatableRead`：

> 事务 A 读两次，事务 B 在中间修改并提交，返回事务 A 两次读到的值，验证 RR 下两次读结果一致。

## 函数签名

```go
func VerifyRepeatableRead(ctx context.Context, db1, db2 *sql.DB) (first, second int, err error)
```

## 提示

1. 建表 `t_isolation(id INT PRIMARY KEY, v INT)`，插入 `(1, 100)`
2. `db1` 开启 RR 事务，读一次 `v`
3. `db2` 更新 `v = 200`（`db2` 的 UPDATE 默认自动提交）
4. `db1` 再读一次 `v`
5. 返回两次读到的值 —— RR 下两次应该相同（可重复读）

## 运行测试

```bash
cd code/backend/01-mysql/01_isolation && go test -v
```

## 参考

对照 `answer/answer.go` 检查你的实现。
