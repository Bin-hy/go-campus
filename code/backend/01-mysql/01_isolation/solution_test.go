package isolation

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

func TestRepeatableRead(t *testing.T) {
	db1 := connect(t)
	db2 := connect(t)
	defer db1.Close()
	defer db2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first, second, err := VerifyRepeatableRead(ctx, db1, db2)
	if err != nil {
		t.Fatalf("VerifyRepeatableRead 失败: %v", err)
	}

	if first != second {
		t.Errorf("RR 应可重复读：first=%d second=%d（第二次应看到 B 提交前的值）", first, second)
	}
	if first != 100 {
		t.Errorf("第一次读应为初始值 100，实际 %d", first)
	}
	t.Logf("可重复读验证通过：事务 A 两次都读到 %d（B 已提交 v=200 但 A 不可见）", first)
}
