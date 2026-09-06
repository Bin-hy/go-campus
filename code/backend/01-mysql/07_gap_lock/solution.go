package gaplock

import (
	"context"
	"database/sql"
)

// GapLockReport 记录同一个"范围当前读 + 并发插入"剧本在 RR 与 RC 下的差异：
// 事务 A 对 v BETWEEN 10 AND 30 执行 SELECT ... FOR UPDATE（当前读，会加锁），
// 事务 B 往两行之间的"间隙"插入 v=20。
type GapLockReport struct {
	// RR：next-key lock（记录锁 + 间隙锁）堵住插入 → B 的 INSERT 被阻塞，直到 A 结束
	RRInsertBlocked bool
	// RC：间隙锁被关闭，只锁已存在的记录 → B 的 INSERT 不被阻塞
	RCInsertBlocked bool
	// RC：B 插入提交后，A 在当前事务里再跑一次 FOR UPDATE，会看到 3 行（幻读）！
	RCPhantomRows int
}

// CompareGapLock 验证"当前读"这一半的幻读防线（RR 用间隙锁，RC 放弃间隙锁换并发）：
// 数据：两行 v = 10 / v = 30（之间是空"间隙"）。
//
// 期望结果（MySQL 8 实测）：
//
//	RR: B 插入 v=20 被阻塞（间隙 (10,30) 被锁），A 提交后 B 才插入成功
//	RC: B 插入 v=20 秒过（只锁记录不锁间隙）；随后 A 在当前事务里再次当前读
//	    会读到 10、20、30 三行 → RCPhantomRows == 3（当前读层面出现幻影行）
func CompareGapLock(ctx context.Context, db1, db2 *sql.DB) (GapLockReport, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
