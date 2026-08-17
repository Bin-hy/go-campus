package deadlock

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const dsn = "root:root123@tcp(127.0.0.1:3306)/demo?parseTime=true"

func TestTriggerDeadlock(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deadlocked, err := TriggerDeadlock(ctx, db1, db2)
	if err != nil {
		t.Fatalf("TriggerDeadlock 失败: %v", err)
	}
	if !deadlocked {
		t.Error("预期检测到死锁，实际未发生")
	}
	t.Logf("死锁复现验证通过：InnoDB 检测到死锁并回滚一方")
}
