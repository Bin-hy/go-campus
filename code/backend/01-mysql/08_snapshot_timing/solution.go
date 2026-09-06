package snapshot

import (
	"context"
	"database/sql"
)

// ProbeSnapshotTiming 验证 RR 的"一致快照"到底在哪一刻建立。
//
// 剧本（两条初始行都是 v=100）：
//  1. 事务 A（RR）第一条语句是"当前读"：SELECT v FROM t_snaptime WHERE id=1 FOR UPDATE
//     —— 只加锁读最新值，并【不会】建立快照读的 Read View
//  2. 事务 B：UPDATE id=2 SET v=200 并提交（id=2 没有被 A 锁住，能立刻提交）
//  3. 事务 A 这时才做第一条"快照读"：SELECT v FROM t_snaptime WHERE id=2
//     —— Read View 此刻才建立，B 早已提交 → 读到 200
//
// 返回第 3 步读到的值（期望 200）。
// 如果以为"RR 的快照在 BEGIN 那一刻就建立"，会误猜成 100——这正是本题要纠正的误区。
func ProbeSnapshotTiming(ctx context.Context, db1, db2 *sql.DB) (value int, err error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
