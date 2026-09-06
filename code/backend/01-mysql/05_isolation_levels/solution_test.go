package isolationlevels

import (
	"context"
	"database/sql"
	"reflect"
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

func TestCompareIsolationLevels(t *testing.T) {
	db1 := connect(t)
	db2 := connect(t)
	defer db1.Close()
	defer db2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	got, err := CompareIsolationLevels(ctx, db1, db2)
	if err != nil {
		t.Fatalf("CompareIsolationLevels 失败: %v", err)
	}

	want := map[string]LevelReport{
		LevelKeyRU:           {DirtyRead: true, NonRepeatableRead: true, WriterBlocked: false},
		LevelKeyRC:           {DirtyRead: false, NonRepeatableRead: true, WriterBlocked: false},
		LevelKeyRR:           {DirtyRead: false, NonRepeatableRead: false, WriterBlocked: false},
		LevelKeySerializable: {DirtyRead: false, NonRepeatableRead: false, WriterBlocked: true},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("四种隔离级别的实测行为与标准不符：\n got: %+v\nwant: %+v", got, want)
	}
	for level, r := range got {
		t.Logf("隔离级别 %-12s → 脏读=%v 不可重复读=%v 读写互斥=%v", level, r.DirtyRead, r.NonRepeatableRead, r.WriterBlocked)
	}
}
