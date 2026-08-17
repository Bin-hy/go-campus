package mvcc

import (
	"context"
	"database/sql"
)

// SnapshotVsCurrentRead 参考答案。
func SnapshotVsCurrentRead(ctx context.Context, db1, db2 *sql.DB) (int, int, error) {
	if _, err := db1.ExecContext(ctx, `DROP TABLE IF EXISTS t_mvcc`); err != nil {
		return 0, 0, err
	}
	if _, err := db1.ExecContext(ctx, `CREATE TABLE t_mvcc (id INT PRIMARY KEY, v INT)`); err != nil {
		return 0, 0, err
	}
	if _, err := db1.ExecContext(ctx, `INSERT INTO t_mvcc (id, v) VALUES (1, 10)`); err != nil {
		return 0, 0, err
	}

	// 事务 A：RR 隔离级别
	tx, err := db1.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// A 先快照读
	var first int
	if err := tx.QueryRowContext(ctx, `SELECT v FROM t_mvcc WHERE id = 1`).Scan(&first); err != nil {
		return 0, 0, err
	}

	// B 更新并提交
	if _, err := db2.ExecContext(ctx, `UPDATE t_mvcc SET v = 99 WHERE id = 1`); err != nil {
		return 0, 0, err
	}

	// A 快照读：RR 下仍是旧值
	var snapshot int
	if err := tx.QueryRowContext(ctx, `SELECT v FROM t_mvcc WHERE id = 1`).Scan(&snapshot); err != nil {
		return 0, 0, err
	}

	// A 当前读：读到最新值
	var current int
	if err := tx.QueryRowContext(ctx, `SELECT v FROM t_mvcc WHERE id = 1 FOR UPDATE`).Scan(&current); err != nil {
		return 0, 0, err
	}

	return snapshot, current, nil
}
