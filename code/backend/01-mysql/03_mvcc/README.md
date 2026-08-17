# MVCC：快照读 vs 当前读

## 难度：⭐⭐⭐ 困难

## 考点
- RR 隔离级别下的快照读（普通 SELECT，走 MVCC）
- 当前读（SELECT ... FOR UPDATE，读最新并加锁）
- 两者的差异

## 环境准备

```bash
cd code/backend && docker compose up -d mysql
```

## 题目描述

实现 `SnapshotVsCurrentRead`：在 RR 隔离级别下，事务 A 先快照读一次，事务 B 把值改成 99 并提交，事务 A 再**快照读**一次、**当前读**（FOR UPDATE）一次。返回快照读到的值和当前读到的值。

## 函数签名

```go
func SnapshotVsCurrentRead(ctx context.Context, db1, db2 *sql.DB) (snapshot, current int, err error)
```

## 提示

1. 建表 `t_mvcc(id INT PRIMARY KEY, v INT)`，插入 `(1, 10)`
2. db1 开 RR 事务，快照读 v（得到 10）
3. db2 更新 v=99 并提交
4. db1 再快照读 v —— RR 下仍是 10（旧值）
5. db1 当前读 `SELECT v ... FOR UPDATE` —— 读到 99（最新值）
6. 返回快照读值和当前读值

## 运行测试

```bash
cd code/backend/01-mysql/03_mvcc && go test -v
```
