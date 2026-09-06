package phantom

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

func TestCompareSnapshotPhantom(t *testing.T) {
	db1 := connect(t)
	db2 := connect(t)
	defer db1.Close()
	defer db2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	got, err := CompareSnapshotPhantom(ctx, db1, db2)
	if err != nil {
		t.Fatalf("CompareSnapshotPhantom 失败: %v", err)
	}

	if got.RRBefore != 2 || got.RRAfter != 2 {
		t.Errorf("RR 快照读应免疫幻读：before=%d after=%d（都应命中 2 行）", got.RRBefore, got.RRAfter)
	}
	if got.RCBefore != 2 || got.RCAfter != 3 {
		t.Errorf("RC 快照读应出现幻读：before=%d after=%d（期望 2 → 3）", got.RCBefore, got.RCAfter)
	}
	t.Logf("快照读下的幻读：RR %d → %d（稳定），RC %d → %d（多出一行，幻读）",
		got.RRBefore, got.RRAfter, got.RCBefore, got.RCAfter)
}
