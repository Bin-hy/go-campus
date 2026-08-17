package isolation

import (
	"context"
	"database/sql"
)

// VerifyRepeatableRead 参考答案：RR 隔离级别下，事务 A 的两次读都基于同一快照，
// 看不到事务 B 已提交的修改，因此两次读到的值相同（可重复读）。
func VerifyRepeatableRead(ctx context.Context, db1, db2 *sql.DB) (int, int, error) {
	// 建表 + 初始数据
	if _, err := db1.ExecContext(ctx, `DROP TABLE IF EXISTS t_isolation`); err != nil {
		return 0, 0, err
	}
	if _, err := db1.ExecContext(ctx, `CREATE TABLE t_isolation (id INT PRIMARY KEY, v INT)`); err != nil {
		return 0, 0, err
	}
	if _, err := db1.ExecContext(ctx, `INSERT INTO t_isolation (id, v) VALUES (1, 100)`); err != nil {
		return 0, 0, err
	}

	// 事务 A：RR 隔离级别
	tx, err := db1.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// 第一次读
	var first int
	if err := tx.QueryRowContext(ctx, `SELECT v FROM t_isolation WHERE id = 1`).Scan(&first); err != nil {
		return 0, 0, err
	}

	// 事务 B：修改并提交
	if _, err := db2.ExecContext(ctx, `UPDATE t_isolation SET v = 200 WHERE id = 1`); err != nil {
		return 0, 0, err
	}

	// 第二次读：RR 下基于同一快照，仍读到旧值
	var second int
	if err := tx.QueryRowContext(ctx, `SELECT v FROM t_isolation WHERE id = 1`).Scan(&second); err != nil {
		return 0, 0, err
	}

	return first, second, nil
}
