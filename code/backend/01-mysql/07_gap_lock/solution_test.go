package gaplock

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const dsn = "root:root123@tcp(127.0.0.1:3306)/demo?parseTime=true"

func connect(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("连接 MySQL 失败（确认已 docker compose up -d mysql）: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping MySQL 失败: %v", err)
	}
	return db
}

func TestCompareGapLock(t *testing.T) {
	db1 := connect(t)
	db2 := connect(t)
	defer db1.Close()
	defer db2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	got, err := CompareGapLock(ctx, db1, db2)
	if err != nil {
		t.Fatalf("CompareGapLock 失败: %v", err)
	}

	if !got.RRInsertBlocked {
		t.Errorf("RR 应有 next-key/间隙锁堵住并发插入，实际未阻塞")
	}
	if got.RCInsertBlocked {
		t.Errorf("RC 不应有间隙锁，并发插入不应被阻塞")
	}
	if got.RCPhantomRows != 3 {
		t.Errorf("RC 再次当前读应看到幻影行（3 行），实际 %d 行", got.RCPhantomRows)
	}
	t.Logf("间隙锁对比：RR 插入被阻塞=%v；RC 插入被阻塞=%v 且再次当前读=%d 行（出现幻影行）",
		got.RRInsertBlocked, got.RCInsertBlocked, got.RCPhantomRows)
}
