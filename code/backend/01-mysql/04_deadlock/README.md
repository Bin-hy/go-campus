# 死锁复现：交叉加锁

## 难度：⭐⭐⭐ 困难

## 考点
- 行锁与死锁的形成条件（互相等待成环）
- InnoDB 死锁检测与回滚一方
- 减少死锁的实践

## 环境准备

```bash
cd code/backend && docker compose up -d mysql
```

## 题目描述

实现 `TriggerDeadlock`：两个事务交叉加锁两行数据（A 先锁 id=1 再锁 id=2，B 先锁 id=2 再锁 id=1），触发 InnoDB 死锁检测。返回是否检测到死锁（报 `Deadlock found`）。

## 函数签名

```go
func TriggerDeadlock(ctx context.Context, db1, db2 *sql.DB) (deadlocked bool, err error)
```

## 提示

1. 建表 `t_deadlock(id INT PRIMARY KEY, v INT)`，插入 `(1,10)`、`(2,20)`
2. A 开事务，`UPDATE ... WHERE id=1`（锁 id=1）
3. B 开事务，`UPDATE ... WHERE id=2`（锁 id=2）
4. A 再 `UPDATE ... WHERE id=2`（阻塞等 B）—— 可放 goroutine
5. B 再 `UPDATE ... WHERE id=1`（阻塞等 A）→ 成环 → 死锁
6. 检查 A 或 B 的报错是否包含 "Deadlock"，是则返回 true

## 运行测试

```bash
cd code/backend/01-mysql/04_deadlock && go test -v
```
