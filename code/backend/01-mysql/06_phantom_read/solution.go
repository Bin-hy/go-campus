package phantom

import (
	"context"
	"database/sql"
)

// PhantomReport 记录同一"范围查询两次"剧本在 RR 与 RC 下的结果差异：
// 事务 A 先 COUNT 一次范围，事务 B 往范围内插入一行并提交，A 再 COUNT 一次。
type PhantomReport struct {
	// RR：两次 COUNT 相同（快照读复用同一 Read View，看不到 B 新插入的行）
	RRBefore int
	RRAfter  int
	// RC：第二次 COUNT 每次读都新建 Read View，能看到 B 已提交的新行 → 结果漂移
	RCBefore int
	RCAfter  int
}

// CompareSnapshotPhantom 验证"幻读"在快照读（普通 SELECT）层面的差异：
// 数据：v = 10 / 20 / 30 / 40，查询范围 v BETWEEN 15 AND 35（应命中 20、30 两行）。
// 剧本：A 开事务先 COUNT（Before）→ B 插入 v=25 并提交 → A 再 COUNT（After）。
//
// 期望结果（MySQL 8 实测）：
//
//	RR: Before == After == 2（MVCC 快照免疫幻读）
//	RC: Before == 2，After == 3（幻读：多出来一行）
func CompareSnapshotPhantom(ctx context.Context, db1, db2 *sql.DB) (PhantomReport, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
