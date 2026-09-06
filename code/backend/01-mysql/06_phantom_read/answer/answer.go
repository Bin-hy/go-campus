package phantom

import (
	"context"
	"database/sql"
	"fmt"
)

const tblPhantom = "t_phantomread"

// PhantomReport 与题目目录 solution.go 中的定义保持一致（独立包可编译用）。
type PhantomReport struct {
	RRBefore int
	RRAfter  int
	RCBefore int
	RCAfter  int
}

// resetPhantomTable 重建测试表：两行数据落在查询范围内（20、30），
// 一行在范围外但会"插进范围中间"的间隙（待 B 插入的 v=25）。
func resetPhantomTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tblPhantom); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE "+tblPhantom+
		" (id INT PRIMARY KEY, v INT, KEY idx_v(v))"); err != nil {
		return err
	}
	for _, v := range []int{10, 20, 30, 40} {
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf("INSERT INTO %s (id, v) VALUES (%d, %d)", tblPhantom, v/10, v)); err != nil {
			return err
		}
	}
	return nil
}

// runScenario 在给定隔离级别下跑一遍幻读剧本，返回两次 COUNT。
func runScenario(ctx context.Context, db1, db2 *sql.DB, level sql.IsolationLevel) (before, after int, err error) {
	if err := resetPhantomTable(ctx, db1); err != nil {
		return 0, 0, err
	}
	txA, err := db1.BeginTx(ctx, &sql.TxOptions{Isolation: level})
	if err != nil {
		return 0, 0, err
	}
	defer txA.Rollback()

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE v BETWEEN 15 AND 35", tblPhantom)
	if err := txA.QueryRowContext(ctx, countSQL).Scan(&before); err != nil {
		return 0, 0, err
	}
	// B：往查询范围内插入 v=25 并提交（普通 Exec = 自动提交）
	if _, err := db2.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO %s (id, v) VALUES (9, 25)", tblPhantom)); err != nil {
		return 0, 0, err
	}
	if err := txA.QueryRowContext(ctx, countSQL).Scan(&after); err != nil {
		return 0, 0, err
	}
	return before, after, nil
}

// CompareSnapshotPhantom 参考答案：
// RR 复用第一次快照读的 Read View → B 提交的新行不可见 → After == Before；
// RC 每条语句新建 Read View → 第二次读能看到 B 的新行 → After == Before + 1。
func CompareSnapshotPhantom(ctx context.Context, db1, db2 *sql.DB) (PhantomReport, error) {
	var r PhantomReport
	var err error

	if r.RRBefore, r.RRAfter, err = runScenario(ctx, db1, db2, sql.LevelRepeatableRead); err != nil {
		return r, err
	}
	if r.RCBefore, r.RCAfter, err = runScenario(ctx, db1, db2, sql.LevelReadCommitted); err != nil {
		return r, err
	}
	return r, nil
}
