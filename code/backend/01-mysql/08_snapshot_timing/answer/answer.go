package snapshot

import (
	"context"
	"database/sql"
	"fmt"
)

const tblSnap = "t_snaptime"

// ProbeSnapshotTiming 参考答案。
func ProbeSnapshotTiming(ctx context.Context, db1, db2 *sql.DB) (int, error) {
	// 重建表：id=1 / id=2 初始都是 100
	if _, err := db1.ExecContext(ctx, "DROP TABLE IF EXISTS "+tblSnap); err != nil {
		return 0, err
	}
	if _, err := db1.ExecContext(ctx, "CREATE TABLE "+tblSnap+" (id INT PRIMARY KEY, v INT)"); err != nil {
		return 0, err
	}
	for _, id := range []int{1, 2} {
		if _, err := db1.ExecContext(ctx,
			fmt.Sprintf("INSERT INTO %s (id, v) VALUES (%d, 100)", tblSnap, id)); err != nil {
			return 0, err
		}
	}

	txA, err := db1.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return 0, err
	}
	defer txA.Rollback()

	// 第一步：当前读（FOR UPDATE），加锁读最新值，不建立快照 Read View
	var locked int
	if err := txA.QueryRowContext(ctx,
		fmt.Sprintf("SELECT v FROM %s WHERE id = 1 FOR UPDATE", tblSnap)).Scan(&locked); err != nil {
		return 0, err
	}

	// 第二步：B 改 id=2 并提交（id=2 未被 A 锁住，正常提交）
	if _, err := db2.ExecContext(ctx,
		fmt.Sprintf("UPDATE %s SET v = 200 WHERE id = 2", tblSnap)); err != nil {
		return 0, err
	}

	// 第三步：A 的第一条快照读。Read View 在【此刻】才建立，
	// B 的提交已经在视图之前完成 → 读到 200 而非 100。
	var value int
	if err := txA.QueryRowContext(ctx,
		fmt.Sprintf("SELECT v FROM %s WHERE id = 2", tblSnap)).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}
