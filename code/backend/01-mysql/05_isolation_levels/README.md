# 四种隔离级别横向对比：什么级别能挡住什么乱子

## 难度：⭐⭐⭐ 中等偏上

## 考点
- 脏读 / 不可重复读在不同隔离级别下的表现（用**同一个剧本**跑四种级别做对比）
- MVCC Read View 生成时机：RC 每条语句新建，RR 整个事务复用同一份
- SERIALIZABLE 的本质差异：普通 SELECT 也变成共享锁读（读写互斥）
- **各种隔离级别的真实适用场景**（做完一定要看文末"实战选型"）

## 环境准备

先起 MySQL：

```bash
cd code/backend && docker compose up -d mysql
```

## 为什么练这道题

书本上只会给你一张表："RU 有脏读，RC 有不可重复读，RR 没有……"。背下来很容易，但面试官一句
"你线上到底用 RC 还是 RR？为什么？"就露馅了。这道题让你**亲手复现**三类并发乱子，并亲眼看到
隔离级别从松到严，分别"多挡了哪一道"——这是你做选型判断的直觉来源。

| 隔离级别 | 脏读 | 不可重复读 | 普通 SELECT 是否阻塞写者 |
|---------|------|-----------|------------------------|
| RU | ✅ 能读到 | ✅ | 否（MVCC 无锁读） |
| RC | ❌ | ✅ | 否（MVCC 无锁读） |
| RR（MySQL 默认） | ❌ | ❌ | 否（MVCC 无锁读） |
| SERIALIZABLE | ❌ | ❌ | ✅（读变共享锁） |

## 题目描述

实现 `CompareIsolationLevels`：用**两个连接**（`db1` 扮演事务 A，`db2` 扮演事务 B）在
RU / RC / RR / SERIALIZABLE 下分别重放以下剧本，返回每种级别观察到的异常集合：

1. **脏读剧本**：B 把 `v` 改成 200 但**不提交**，A 去读——A 读到了吗？
2. **不可重复读剧本**：A 先读到 100，B 改成 200 并**提交**，A 再读一次——两次一样吗？
3. **SERIALIZABLE 特判**：A 普通 `SELECT` 之后，B 的 `UPDATE` 会不会被阻塞？

## 函数签名

```go
type LevelReport struct {
	DirtyRead         bool // 读到其他事务未提交的数据
	NonRepeatableRead bool // 同事务两次读同一行结果不同
	WriterBlocked     bool // 普通 SELECT 会阻塞并发写者（仅 SERIALIZABLE）
}

func CompareIsolationLevels(ctx context.Context, db1, db2 *sql.DB) (map[string]LevelReport, error)
```

返回的 map 以 `LevelKeyRU` / `LevelKeyRC` / `LevelKeyRR` / `LevelKeySerializable` 为 key。

## 提示

1. 每个剧本开始前先 `DROP TABLE` + `CREATE TABLE t_isolevels (id INT PRIMARY KEY, v INT)` +
   `INSERT (1, 100)`，保证上一轮结果不污染下一轮。
2. 开事务用 `db.BeginTx(ctx, &sql.TxOptions{Isolation: <级别>})`，driver 会在该连接上先执行
   `SET TRANSACTION ISOLATION LEVEL ...` 再 `START TRANSACTION`。
3. 脏读剧本：B 要开**独立事务**改数据且**不提交**，A 去读；读完 B 再 Rollback。
4. 不可重复读剧本：A 的事务要保持打开，B 用**自动提交**（直接 `db2.Exec`）改完即提交。
5. SERIALIZABLE 剧本：A `SELECT` 后 B 的 `UPDATE` 会一直等锁，所以 B 要放 goroutine 里执行，
   主流程等 400ms 没返回就判定"被阻塞"，然后 A `Commit` 释放锁让 B 收尾。
6. 对照 `answer/answer.go` 检查。注意理解：为什么 RU 能脏读、RC 不能？为什么 RR 下两次读
   一致？核心都在 Read View：**RC 每次读新建视图，RR 复用第一次读建立的视图**。

## 运行测试

```bash
cd code/backend/01-mysql/05_isolation_levels && go test -v
```

## 做完后自查（面试常问）

- 为什么线上几乎没人用 RU？答：脏读读到的是**可能回滚**的废数据，业务无法信任。
- RC 和 RR 差别在哪一行？答：RC 允许"已提交数据在事务中途变化"（不可重复读），RR 不允许。
- 想提高写并发时为什么倾向 RC？答：RC 不产生间隙锁（下两道题会验证），锁冲突少；代价是
  读不稳定，需要业务容忍或用锁/重查兜底。
- 什么业务必须 RR？答：报表/对账/批量统计这类"一个事务里多次查询要基于同一时刻快照"的场景。

## 实战选型：每种隔离级别该用在哪儿

- **RU**：几乎不用。脏读意味着读到别人未提交、随时可能回滚的数据，业务无法依赖；只适合
  "看个大概、不在乎准确性"的内部调试。
- **RC**：互联网高并发核心链路常用（订单、库存、支付流水等短事务）。它**没有间隙锁**，
  写写冲突少、并发高；MySQL 8.0 默认 `binlog_format=ROW`，RC 也足够安全。缺点：同一事务内
  多次读结果可能漂移——需要"读到就锁定/重查"或业务上接受最终一致。
- **RR**：MySQL 默认。适合**长事务 + 多次读需要一致快照**的场景：对账、报表汇总、批量导出，
  一次事务内所有查询基于同一时间点，前后数字能对上。代价：间隙锁多，插入并发容易互相等待。
- **SERIALIZABLE**：读也加锁，读写完全互斥，吞吐最低。只有当"不允许任何并发不一致、且写
  冲突少/并发低"（如某些资金强一致场景）才考虑；绝大多数业务用不上它。

## 参考

对照 `answer/answer.go` 检查你的实现。
