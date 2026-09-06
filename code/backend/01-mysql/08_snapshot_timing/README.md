# RR 快照的"出生时刻"：不是 BEGIN，而是第一条快照读

## 难度：⭐⭐⭐⭐ 困难

## 考点
- RR 的 Read View（一致快照）**不是在 BEGIN 时建立**，而是在**第一条快照读（普通 SELECT）**时建立
- 当前读（FOR UPDATE / UPDATE / DELETE）不加 MVCC 快照保护，不会建立 Read View
- 由此产生的工程坑：RR 事务"先写后读"时，读到的可能比事务开始时要"新"
- `START TRANSACTION WITH CONSISTENT SNAPSHOT` 的用途：让快照在事务一开始就固化

## 环境准备

```bash
cd code/backend && docker compose up -d mysql
```

## 为什么练这道题

很多人背"RR = 整个事务用同一个快照"，然后想当然地以为"快照 = BEGIN 那一刻的数据库"。
**这是错的，而且是面试高频陷阱。** MySQL 的实现是：Read View 在事务执行到
**第一条快照读**时才生成（懒加载）。如果事务的第一条语句是 `UPDATE` / `SELECT ... FOR UPDATE`
这类当前读，那么"快照点"会被推迟——之后的第一条普通 SELECT 能看见
**别人在那之后提交的数据**。本题就是让你亲手戳破这个误区。

## 题目描述

实现 `ProbeSnapshotTiming`，复现如下剧本（两条初始行 v 都等于 100）：

1. 事务 A（RR）第一条语句做**当前读**：`SELECT v FROM t_snaptime WHERE id = 1 FOR UPDATE`
2. 事务 B：把 `id = 2` 改成 200 并提交（id=2 没被 A 锁住，能立即提交）
3. 事务 A 这时才执行**第一条快照读**：`SELECT v FROM t_snaptime WHERE id = 2`

返回第 3 步读到的值。猜 100 还是 200？跑一下就知道——正确答案是 **200**：
A 的快照在步骤 3 才建立，B 的提交早已完成，所以它"看见"了 200。

## 函数签名

```go
func ProbeSnapshotTiming(ctx context.Context, db1, db2 *sql.DB) (value int, err error)
```

## 提示

1. 建表 `t_snaptime(id INT PRIMARY KEY, v INT)`，插入 (1,100)、(2,100)。
2. A 用 `db1.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})` 开事务。
3. 步骤 1 的 `FOR UPDATE` 只锁 id=1，别锁 id=2，否则 B 的 UPDATE 会被挡住、剧本就断了。
4. 步骤 2 用 `db2.Exec`（自动提交），一步到位。
5. 对比 01_isolation：那边 A 的**第一条语句就是普通 SELECT**，所以快照在 B 提交前就建立，
   最终读到旧值 100；这里第一条是当前读，快照延后，结果变成 200——差别全在语句顺序。

## 运行测试

```bash
cd code/backend/01-mysql/08_snapshot_timing && go test -v
```

## 做完后自查（面试必问）

- RR 快照到底什么时候建立？答：第一条**快照读**时；`BEGIN` / `START TRANSACTION` 本身不建快照。
- 那想要"事务一开始就固定快照"怎么办？答：`START TRANSACTION WITH CONSISTENT SNAPSHOT`，
  在事务最开头显式建立 Read View（MySQL 8 语义更严，读会等事务真正开始）。
- 这个特性在生产里会惹什么祸？答：长事务里若先做 UPDATE 再大量 SELECT，SELECT 的数据口径
  可能混入"UPDATE 之后别人提交的内容"，统计结果和自己直觉不符；对账类事务要么把查询放前面，
  要么用 `WITH CONSISTENT SNAPSHOT` 固化口径。
- 顺带理清三个概念：**快照读**（普通 SELECT，MVCC 不加锁）、**当前读**（FOR UPDATE / UPDATE /
  DELETE，读最新并加锁）、**Read View 建立时机**（第一条快照读）——这三者能讲清，
  RR 的幻读、可重复读、以及本实验的"快照延后"就全部串起来了。

## 参考

对照 `answer/answer.go` 检查你的实现。
