package isolationlevels

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// LevelReport / LevelKey* 与题目目录 solution.go 中的定义保持一致，
// 此处重复声明是为了让参考答案包可以独立编译、独立验证。
type LevelReport struct {
	DirtyRead         bool
	NonRepeatableRead bool
	WriterBlocked     bool
}

const (
	LevelKeyRU           = "RU"
	LevelKeyRC           = "RC"
	LevelKeyRR           = "RR"
	LevelKeySerializable = "SERIALIZABLE"
)

// tblLevels 每轮剧本开始前都会 DROP + CREATE + 灌初始数据，保证探测互不影响。
const tblLevels = "t_isolevels"

func resetLevelTable(ctx context.Context, db *sql.DB, initial int) error {
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tblLevels); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE "+tblLevels+" (id INT PRIMARY KEY, v INT)"); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (id, v) VALUES (1, %d)", tblLevels, initial))
	return err
}

// probeDirtyRead 剧本：B 改不提交，A 读。
// RU 下 A 直接读到未提交的 200（脏读）；RC/RR 读到已提交的 100。
func probeDirtyRead(ctx context.Context, db1, db2 *sql.DB, level sql.IsolationLevel) (bool, error) {
	if err := resetLevelTable(ctx, db1, 100); err != nil {
		return false, err
	}
	txA, err := db1.BeginTx(ctx, &sql.TxOptions{Isolation: level})
	if err != nil {
		return false, err
	}
	defer txA.Rollback()

	txB, err := db2.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	if _, err := txB.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET v = 200 WHERE id = 1", tblLevels)); err != nil {
		txB.Rollback()
		return false, err
	}
	// 关键一步：B 还没提交，A 就发起普通 SELECT（快照读/无锁读）
	var v int
	err = txA.QueryRowContext(ctx, fmt.Sprintf("SELECT v FROM %s WHERE id = 1", tblLevels)).Scan(&v)
	if rbErr := txB.Rollback(); rbErr != nil {
		return false, rbErr
	}
	if err != nil {
		return false, err
	}
	// 读到 200 说明看到了 B 未提交的修改 = 脏读
	return v == 200, nil
}

// probeNonRepeatableRead 剧本：A 先读 100 → B 改成 200 并提交 → A 再读。
// RC/RU 每次读都重建 Read View，第二次读到 200 → 不可重复读；
// RR 复用第一次读的快照，第二次仍读到 100 → 可重复读。
func probeNonRepeatableRead(ctx context.Context, db1, db2 *sql.DB, level sql.IsolationLevel) (bool, error) {
	if err := resetLevelTable(ctx, db1, 100); err != nil {
		return false, err
	}
	txA, err := db1.BeginTx(ctx, &sql.TxOptions{Isolation: level})
	if err != nil {
		return false, err
	}
	defer txA.Rollback()

	var first, second int
	if err := txA.QueryRowContext(ctx, fmt.Sprintf("SELECT v FROM %s WHERE id = 1", tblLevels)).Scan(&first); err != nil {
		return false, err
	}
	if _, err := db2.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET v = 200 WHERE id = 1", tblLevels)); err != nil {
		return false, err
	}
	if err := txA.QueryRowContext(ctx, fmt.Sprintf("SELECT v FROM %s WHERE id = 1", tblLevels)).Scan(&second); err != nil {
		return false, err
	}
	return second != first, nil
}

// probeSerializableWriterBlocked 剧本：SERIALIZABLE 下 A 的普通 SELECT 也是
// 共享锁（等价 LOCK IN SHARE MODE），并发的 UPDATE 必须等 A 结束才能写。
// 这是 SERIALIZABLE 与 RR 最本质的区别：读写互斥，换来最强隔离。
func probeSerializableWriterBlocked(ctx context.Context, db1, db2 *sql.DB) (bool, error) {
	if err := resetLevelTable(ctx, db1, 100); err != nil {
		return false, err
	}
	txA, err := db1.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return false, err
	}
	defer txA.Rollback()

	// A 普通 SELECT（共享锁）
	var v int
	if err := txA.QueryRowContext(ctx, fmt.Sprintf("SELECT v FROM %s WHERE id = 1", tblLevels)).Scan(&v); err != nil {
		return false, err
	}

	done := make(chan error, 1)
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	go func() {
		_, err := db2.ExecContext(wctx, fmt.Sprintf("UPDATE %s SET v = 200 WHERE id = 1", tblLevels))
		done <- err
	}()

	blocked := false
	select {
	case err := <-done:
		// B 竟然立刻执行完了 → 说明 A 的读没有锁住它（不该发生）
		return false, fmt.Errorf("SERIALIZABLE 下并发 UPDATE 未被阻塞: %v", err)
	case <-time.After(400 * time.Millisecond):
		blocked = true
	}
	// 释放 A 的锁，让 B 的 UPDATE 完成，避免留下悬挂事务
	if err := txA.Commit(); err != nil {
		return false, err
	}
	if err := <-done; err != nil {
		return false, err
	}
	return blocked, nil
}

// CompareIsolationLevels 参考答案。
func CompareIsolationLevels(ctx context.Context, db1, db2 *sql.DB) (map[string]LevelReport, error) {
	results := make(map[string]LevelReport, 4)

	for _, lv := range []struct {
		key string
		iso sql.IsolationLevel
	}{
		{LevelKeyRU, sql.LevelReadUncommitted},
		{LevelKeyRC, sql.LevelReadCommitted},
		{LevelKeyRR, sql.LevelRepeatableRead},
	} {
		dirty, err := probeDirtyRead(ctx, db1, db2, lv.iso)
		if err != nil {
			return nil, err
		}
		nonRepeat, err := probeNonRepeatableRead(ctx, db1, db2, lv.iso)
		if err != nil {
			return nil, err
		}
		results[lv.key] = LevelReport{
			DirtyRead:         dirty,
			NonRepeatableRead: nonRepeat,
			WriterBlocked:     false, // MVCC 级别：普通 SELECT 不阻塞写者
		}
	}

	writerBlocked, err := probeSerializableWriterBlocked(ctx, db1, db2)
	if err != nil {
		return nil, err
	}
	// SERIALIZABLE 读也加共享锁：写者进不来，事务内读到的必然都是已提交且稳定的数据，
	// 因此不存在脏读/不可重复读（以牺牲写并发为代价）。
	results[LevelKeySerializable] = LevelReport{
		DirtyRead:         false,
		NonRepeatableRead: false,
		WriterBlocked:     writerBlocked,
	}
	return results, nil
}
