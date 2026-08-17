package agent_flow

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

const (
	broker   = "localhost:9092"
	redisAddr = "127.0.0.1:6379"
	mysqlDSN  = "root:root123@tcp(127.0.0.1:3306)/demo?parseTime=true"
)

func ensureTopic(t *testing.T, topic string) {
	t.Helper()
	conn, err := kafka.Dial("tcp", broker)
	if err != nil {
		t.Fatalf("连接 broker 失败（确认已 docker compose up -d kafka）: %v", err)
	}
	defer conn.Close()
	controller, _ := conn.Controller()
	cconn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		t.Fatal(err)
	}
	defer cconn.Close()
	_ = cconn.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1})
	time.Sleep(3 * time.Second)
}

func TestAgentFlow(t *testing.T) {
	ctx := context.Background()

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping MySQL 失败（确认已 docker compose up -d mysql）: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("连接 Redis 失败: %v", err)
	}

	topic := fmt.Sprintf("agent-tasks-%d", time.Now().UnixNano())
	ensureTopic(t, topic)

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	result, err := AgentFlow(ctx, db, rdb, broker, topic, taskID)
	if err != nil {
		t.Fatalf("AgentFlow 失败: %v", err)
	}

	if !contains(result, "search-tool") {
		t.Errorf("决策应选 search-tool，实际结果 %q", result)
	}

	var got string
	if err := db.QueryRow(`SELECT result FROM task_results WHERE task_id = ?`, taskID).Scan(&got); err != nil {
		t.Fatalf("MySQL 落库查询失败: %v", err)
	}
	mem, _ := rdb.Get(ctx, "result:"+taskID).Result()
	if got != result || mem != result {
		t.Fatalf("闭环不一致：MySQL=%q Redis=%q 期望=%q", got, mem, result)
	}
	t.Logf("Agent Backend 闭环验证通过：任务 %s → 决策 search-tool，结果已写 Redis + MySQL", taskID)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && len(s) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
