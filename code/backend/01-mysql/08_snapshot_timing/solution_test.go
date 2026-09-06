package snapshot

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

func TestProbeSnapshotTiming(t *testing.T) {
	db1 := connect(t)
	db2 := connect(t)
	defer db1.Close()
	defer db2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	got, err := ProbeSnapshotTiming(ctx, db1, db2)
	if err != nil {
		t.Fatalf("ProbeSnapshotTiming 失败: %v", err)
	}

	if got != 200 {
		t.Errorf("RR 快照在第一条【快照读】时才建立：B 已提交的 200 应可见，实际读到 %d（若为 100 说明把 BEGIN 误当成快照点）", got)
	}
	t.Logf("验证成功：事务第一条语句是当前读时不建 Read View，随后第一条快照读读到了 B 已提交的 200")
}
