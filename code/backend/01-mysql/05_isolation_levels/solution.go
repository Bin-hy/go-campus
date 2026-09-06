package isolationlevels

import (
	"context"
	"database/sql"
)

// LevelReport 记录在某一隔离级别下能复现出的并发问题。
// 做完这道题你应该能回答：为什么不同业务要选不同隔离级别。
type LevelReport struct {
	// DirtyRead 是否读到了其他事务"未提交"的数据（脏读，最危险的乱子）
	DirtyRead bool
	// NonRepeatableRead 同事务内两次读同一行，结果是否可能不同
	// （他人 UPDATE 并提交后，第二次读到了新值）
	NonRepeatableRead bool
	// WriterBlocked 普通 SELECT 是否会阻塞并发的写者。
	// 只有 SERIALIZABLE 为 true：它的读带共享锁，读写互相等待。
	WriterBlocked bool
}

// 隔离级别标识（map 的 key）
const (
	LevelKeyRU           = "RU"           // READ UNCOMMITTED
	LevelKeyRC           = "RC"           // READ COMMITTED
	LevelKeyRR           = "RR"           // REPEATABLE READ
	LevelKeySerializable = "SERIALIZABLE" // SERIALIZABLE
)

// CompareIsolationLevels 用同一个"读写竞争剧本"分别在 RU / RC / RR / SERIALIZABLE
// 下复现一遍，返回每种级别观察到的异常集合（map key 用 LevelKey* 常量）。
//
// 剧本（每级别都重跑一遍，保证互不影响）：
//  1. 脏读剧本：事务 B 把 v 改成 200 但不提交，事务 A 去读 id=1
//  2. 不可重复读剧本：A 先读到 100，B 改成 200 并提交，A 再读一次
//  3. SERIALIZABLE 特判：A 普通 SELECT 后，B 的 UPDATE 是否被阻塞
//
// 期望结果（MySQL 8 实测）：
//
//	RU: {脏读: true,  不可重复读: true,  writerBlocked: false}
//	RC: {脏读: false, 不可重复读: true,  writerBlocked: false}
//	RR: {脏读: false, 不可重复读: false, writerBlocked: false}
//	SERIALIZABLE: {脏读: false, 不可重复读: false, writerBlocked: true}
func CompareIsolationLevels(ctx context.Context, db1, db2 *sql.DB) (map[string]LevelReport, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
