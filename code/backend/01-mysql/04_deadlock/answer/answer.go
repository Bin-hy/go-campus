package deadlock

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// TriggerDeadlock 参考答案：A 锁 id=1 再锁 id=2，B 锁 id=2 再锁 id=1，形成等待环。
func TriggerDeadlock(ctx context.Context, db1, db2 *sql.DB) (bool, error) {
	if _, err := db1.ExecContext(ctx, `DROP TABLE IF EXISTS t_deadlock`); err != nil {
		return false, err
	}
	if _, err := db1.ExecContext(ctx, `CREATE TABLE t_deadlock (id INT PRIMARY KEY, v INT)`); err != nil {
		return false, err
	}
	if _, err := db1.ExecContext(ctx, `INSERT INTO t_deadlock (id, v) VALUES (1, 10), (2, 20)`); err != nil {
		return false, err
	}

	tx1, err := db1.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx1.Rollback()
	tx2, err := db2.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx2.Rollback()

	// A 锁 id=1，B 锁 id=2
	if _, err := tx1.ExecContext(ctx, `UPDATE t_deadlock SET v = 11 WHERE id = 1`); err != nil {
		return false, err
	}
	if _, err := tx2.ExecContext(ctx, `UPDATE t_deadlock SET v = 22 WHERE id = 2`); err != nil {
		return false, err
	}

	// A 尝试锁 id=2（阻塞），放在 goroutine
	errA := make(chan error, 1)
	go func() {
		_, err := tx1.ExecContext(ctx, `UPDATE t_deadlock SET v = 12 WHERE id = 2`)
		errA <- err
	}()
	time.Sleep(200 * time.Millisecond) // 给 A 时间进入阻塞

	// B 尝试锁 id=1（阻塞）→ 成环 → 死锁，InnoDB 回滚一方
	_, errB := tx2.ExecContext(ctx, `UPDATE t_deadlock SET v = 21 WHERE id = 1`)
	errAVal := <-errA

	deadlock := func(err error) bool {
		return err != nil && strings.Contains(err.Error(), "Deadlock")
	}
	return deadlock(errAVal) || deadlock(errB), nil
}
