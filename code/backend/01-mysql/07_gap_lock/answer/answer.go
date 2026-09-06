package gaplock

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GapLockReport 与题目目录 solution.go 中的定义保持一致（独立包可编译用）。
type GapLockReport struct {
	RRInsertBlocked bool
	RCInsertBlocked bool
	RCPhantomRows   int
}

const tblGap = "t_gaplock"

// resetGapTable 重建表：v = 10 与 v = 30 两行，二者之间是空间隙 (10, 30)。
func resetGapTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tblGap); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE "+tblGap+
		" (id INT PRIMARY KEY, v INT, KEY idx_v(v))"); err != nil {
		return err
	}
	for _, v := range []int{10, 30} {
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf("INSERT INTO %s (id, v) VALUES (%d, %d)", tblGap, v/10, v)); err != nil {
			return err
		}
	}
	return nil
}

// runScenario 在指定级别下跑"范围当前读 + 并发插入 v=20"剧本，返回：
//   - insertBlocked：B 的 INSERT 是否被 A 的锁挡住（RR 应为 true，RC 应为 false）
//   - phantomRows：若插入未被阻塞，A 在当前事务里再次当前读命中的行数（RC 下应为 3）
func runScenario(ctx context.Context, db1, db2 *sql.DB, level sql.IsolationLevel) (insertBlocked bool, phantomRows int, err error) {
	if err := resetGapTable(ctx, db1); err != nil {
		return false, 0, err
	}
	txA, err := db1.BeginTx(ctx, &sql.TxOptions{Isolation: level})
	if err != nil {
		return false, 0, err
	}
	defer txA.Rollback()

	// A：当前读，命中 v=10、v=30（RR 还会用 next-key lock 覆盖中间间隙）
	var locked int
	if err := txA.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE v BETWEEN 10 AND 30 FOR UPDATE", tblGap)).Scan(&locked); err != nil {
		return false, 0, err
	}

	// B 的插入放 goroutine：被锁挡住就一直等，直到 A 释放
	wctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := db2.ExecContext(wctx,
			fmt.Sprintf("INSERT INTO %s (id, v) VALUES (9, 20)", tblGap))
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return false, 0, err
		}
		// 插入没被挡住。RC 下这是正常的（只有记录锁，没有间隙锁）。
		// A 的事务还开着：再次当前读 → 能看到 B 提交的新行吗？
		if err := txA.QueryRowContext(ctx,
			fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE v BETWEEN 10 AND 30 FOR UPDATE", tblGap)).Scan(&phantomRows); err != nil {
			return false, 0, err
		}
		return false, phantomRows, nil
	case <-time.After(500 * time.Millisecond):
		// 500ms 还没完成 → 被 A 的锁阻塞（RR 下由 next-key lock 造成）
		if err := txA.Commit(); err != nil { // 释放锁，让 B 的插入收尾
			return false, 0, err
		}
		select {
		case err := <-done:
			if err != nil {
				return false, 0, err
			}
			return true, 0, nil
		case <-time.After(4 * time.Second):
			return false, 0, fmt.Errorf("释放锁后 B 的插入仍未完成（疑似卡死）")
		}
	}
}

// CompareGapLock 参考答案。
func CompareGapLock(ctx context.Context, db1, db2 *sql.DB) (GapLockReport, error) {
	var r GapLockReport
	var err error

	if r.RRInsertBlocked, _, err = runScenario(ctx, db1, db2, sql.LevelRepeatableRead); err != nil {
		return r, err
	}
	if r.RCInsertBlocked, r.RCPhantomRows, err = runScenario(ctx, db1, db2, sql.LevelReadCommitted); err != nil {
		return r, err
	}
	return r, nil
}
