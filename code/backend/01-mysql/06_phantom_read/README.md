# 幻读（快照读视角）：RR 稳如泰山，RC 结果漂移

## 难度：⭐⭐⭐ 中等偏上

## 考点
- 幻读的本质：同事务两次**范围查询**，第二次多出（或少了）**别人插入并提交**的行
- 快照读（普通 SELECT）层面：RR 靠 MVCC 同一 Read View 免疫幻读，RC 每次读重建视图所以会幻读
- 为下一道题（当前读 + 间隙锁）做铺垫：本题只谈**读**，不谈锁

## 环境准备

```bash
cd code/backend && docker compose up -d mysql
```

## 为什么练这道题

"不可重复读"是**同一行**的值变了；"幻读"是**行数**变了（多出/少了行）。很多同学能把两句定义背熟，
却说不清：MySQL 默认 RR 到底挡不挡幻读？答案要分"快照读"和"当前读"两种情况——
这道题让你**亲眼看到快照读这一半**：RR 两次 COUNT 一样，RC 第二次 COUNT 多出一行。

先想一个问题再动手：RC 和 RR 只差一个 Read View 的**生成时机**，为什么就能造成
"多出一行"这种差别？（提示：RC 每次语句都新建视图 → 视图里能看到刚提交的 INSERT；
RR 复用第一次读的视图 → 视图里"没有"那行。）

## 题目描述

实现 `CompareSnapshotPhantom`：表里先放 v = 10 / 20 / 30 / 40 四行。事务 A 在给定隔离级别下
查询 `COUNT(*) WHERE v BETWEEN 15 AND 35`（命中 20、30 两行）；随后事务 B 插入 v = 25
并提交（25 落在范围内）；A 再查一次同样的 COUNT。

分别在 **RR** 和 **RC** 下重放剧本，返回两次 COUNT 的前后值：

- RR 期望：`2 → 2`（复用同一快照，B 的行不可见）
- RC 期望：`2 → 3`（幻读，第二次读能看到 B 已提交的行）

## 函数签名

```go
type PhantomReport struct {
	RRBefore int
	RRAfter  int
	RCBefore int
	RCAfter  int
}

func CompareSnapshotPhantom(ctx context.Context, db1, db2 *sql.DB) (PhantomReport, error)
```

## 提示

1. 建表要带二级索引：`CREATE TABLE t_phantomread (id INT PRIMARY KEY, v INT, KEY idx_v(v))`。
2. 每个隔离级别重放前先 `DROP TABLE` 重建并灌入 10/20/30/40，避免上一轮结果残留。
3. A 用 `db1.BeginTx(ctx, &sql.TxOptions{Isolation: <级别>})` 开事务并保持打开；
   B 的插入直接用 `db2.Exec`（自动提交，插完即对 RC 的后续语句可见）。
4. 插入值选 **v = 25**：它落在查询范围 (15, 35) 内，能制造"多一行"，同时不撞已有数据。
5. 两道 COUNT 用同一句 SQL，只是分别在 RR / RC 场景各跑一遍。

## 运行测试

```bash
cd code/backend/01-mysql/06_phantom_read && go test -v
```

## 做完后自查（面试常问）

- RR 下快照读能挡住幻读，靠的是什么？答：MVCC + 事务级 Read View，B 的行"晚于视图诞生"，
  版本链上不可见，读都不需要加锁。
- 那 RR 是不是就彻底没有幻读了？答：不是。`SELECT ... FOR UPDATE`、`UPDATE`、`DELETE`
  这类**当前读**不加 MVCC 保护，RR 是靠 **next-key lock（临键锁）** 堵住幻读的——
  这正是下一道题 `07_gap_lock` 要验证的。
- RC 的幻读在实际业务里怎么体现？答：同一事务里分页/统计查询会前后对不上；要做"当前读"
  也拦不住并发插入。业务要么接受，要么升级 RR，要么自己加锁/加版本号兜底。

## 实战选型（呼应 05 的结论）

- 需要"一次事务内多次范围统计必须一致"（对账、报表、批量分页导出）→ 用 RR，快照读免费提供一致性。
- 高并发短事务 OLTP → 用 RC，虽然快照读会漂移，但通常每个请求就一两条 SQL，漂移影响小。

## 参考

对照 `answer/answer.go` 检查你的实现。
