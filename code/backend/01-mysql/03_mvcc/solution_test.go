package mvcc

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const dsn = "root:root123@tcp(127.0.0.1:3306)/demo?parseTime=true"

func TestSnapshotVsCurrentRead(t *testing.T) {
	db1, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db1.Close()
	db2, _ := sql.Open("mysql", dsn)
	defer db2.Close()
	if err := db1.Ping(); err != nil {
		t.Fatalf("ping MySQL 失败（确认已 docker compose up -d mysql）: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	snapshot, current, err := SnapshotVsCurrentRead(ctx, db1, db2)
	if err != nil {
		t.Fatalf("SnapshotVsCurrentRead 失败: %v", err)
	}

	if snapshot != 10 {
		t.Errorf("快照读应为旧值 10，实际 %d", snapshot)
	}
	if current != 99 {
		t.Errorf("当前读应为最新值 99，实际 %d", current)
	}
	t.Logf("MVCC 验证通过：快照读=%d（旧值），当前读=%d（最新值）", snapshot, current)
}
