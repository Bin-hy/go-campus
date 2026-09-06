# 当前读与间隙锁：RR 靠锁堵幻读，RC 用并发换正确性

## 难度：⭐⭐⭐⭐ 困难

## 考点
- 快照读 vs 当前读：普通 SELECT 走 MVCC 不加锁；`SELECT ... FOR UPDATE` / `UPDATE` / `DELETE`
  读最新版本并加锁
- RR 下的 **next-key lock（临键锁）** = 记录锁 + 间隙锁：堵住"往间隙里插入新行"
- RC 下间隙锁被关闭，只锁已存在的记录 → 并发插入不会被挡，但**当前读也会幻读**
- 为什么高并发业务选 RC：没有间隙锁 → 插入/写写冲突大幅减少（代价见最后）

## 环境准备

```bash
cd code/backend && docker compose up -d mysql
```

## 为什么练这道题

上一题证明：**快照读**层面 RR 靠 MVCC 免疫幻读。但面试官紧接着会问：
"那 `SELECT ... FOR UPDATE` 呢？RR 的当前读靠什么防幻读？"
答案：**next-key lock**。本题让你亲手验证两件事：

1. RR 下 A `FOR UPDATE` 锁住范围后，B 往"空位"插入会被**阻塞**（这就是间隙锁在起作用）；
2. RC 下同样的操作，B 插入**秒过**，而 A 在当前事务里再次当前读，竟然**多看到一行**——
   当前读层面的幻读真实存在。RC 用"放弃防幻读"换来了更少的锁冲突。

## 题目描述

实现 `CompareGapLock`：表里只有 v = 10 与 v = 30 两行（中间是空"间隙"）。
事务 A 执行 `SELECT COUNT(*) ... WHERE v BETWEEN 10 AND 30 FOR UPDATE`，随后
事务 B 往间隙插入 v = 20。分别在 **RR** 与 **RC** 下重放：

- RR 期望：B 的插入**被阻塞**（间隙被锁），直到 A 提交才成功
- RC 期望：B 的插入**不被阻塞**；之后 A 在同一事务里再次 `FOR UPDATE`，
  能读到 10 / 20 / 30 **三行**（RCPhantomRows = 3，幻影行出现了）

## 函数签名

```go
type GapLockReport struct {
	RRInsertBlocked bool // RR：B 插入是否被阻塞（期望 true）
	RCInsertBlocked bool // RC：B 插入是否被阻塞（期望 false）
	RCPhantomRows   int  // RC：A 再次当前读命中的行数（期望 3）
}

func CompareGapLock(ctx context.Context, db1, db2 *sql.DB) (GapLockReport, error)
```

## 提示

1. 建表必须有二级索引，锁才会落在索引上：`CREATE TABLE t_gaplock (id INT PRIMARY KEY, v INT, KEY idx_v(v))`。
2. A 用 `BeginTx` 指定隔离级别并保持事务打开；B 的插入放 **goroutine**（可能被锁阻塞）。
3. 判断"是否被阻塞"：插入发起后主流程等 500ms——没返回说明在等锁；然后 A `Commit` 释放锁，
   等 goroutine 收尾并检查插入无报错。
4. RC 场景：B 插入应立刻成功；趁 A 事务还开着，再跑一次相同的 `FOR UPDATE` 统计行数。
5. 别把 `t_gaplock` 的间隙数据想复杂：10 和 30 之间的空隙就是 (10, 30)，插 20 正好落进去。

## 运行测试

```bash
cd code/backend/01-mysql/07_gap_lock && go test -v
```

## 做完后自查（面试必问）

- RR 为什么能防当前读幻读？答：范围扫描不仅锁命中的记录，还锁住记录之间的间隙
  （next-key lock），别人插不进"被锁范围"内的新行，两次当前读的行集就不会变。
- 唯一索引等值命中的情况下还锁间隙吗？答：唯一索引等值命中已存在记录时只加记录锁、
  不锁间隙（MySQL 的优化）；而**普通索引 / 范围 / 等值未命中**都要锁间隙。想验证？改表为
  `id` 唯一等值 `WHERE id = 1 FOR UPDATE`，再插一个新 id 试试会不会被挡。
- RC 为什么能容忍当前读幻读？答：RC 认为"每条语句看到最新已提交"就够了，防幻读属于
  应用层该操心的事；配合 MySQL 8 的 `binlog_format=ROW`，也不会出现 5.7 时代
  "RC + STATEMENT 行格式导致主从数据不一致"的坑。
- 生产选型一句话：**写多、要求响应快的短事务用 RC；一次事务内要多次读一致快照的用 RR。**

## 进阶思考（做了加分）

MySQL 5.7 及以前，若 `binlog_format=STATEMENT`，RC 下主从复制可能出问题，很多团队被迫用 RR；
8.0 默认 ROW 格式后 RC 才成为高并发默认选项。结合本实验想想：**为什么间隙锁的存在
会让 RR 在高并发插入场景下更容易死锁、更难扩并发？**（提示：锁的范围从"几行"变成了"一段区间"。）

## 参考

对照 `answer/answer.go` 检查你的实现。
