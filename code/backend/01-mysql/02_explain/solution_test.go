package explain

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const dsn = "root:root123@tcp(127.0.0.1:3306)/demo?parseTime=true"

func TestCompareExplain(t *testing.T) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping MySQL 失败（确认已 docker compose up -d mysql）: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	before, after, err := CompareExplain(ctx, db)
	if err != nil {
		t.Fatalf("CompareExplain 失败: %v", err)
	}

	if before != "ALL" {
		t.Errorf("无索引时 type 应为 ALL，实际 %s", before)
	}
	if after == "ALL" {
		t.Errorf("加索引后 type 不应再是 ALL，实际 %s", after)
	}
	t.Logf("EXPLAIN 优化验证通过：无索引 type=%s → 加索引 type=%s", before, after)
}
