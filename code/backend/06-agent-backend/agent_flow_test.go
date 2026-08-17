// T40 实验：Agent Backend 综合 demo —— 串起 Redis / Kafka / MySQL 三组件，
// 演示「写上下文 → 投任务 → 消费决策 → 工具执行 → 写回 Redis + MySQL」闭环。
package agent_backend

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
	mysqlDSN = "root:root123@tcp(127.0.0.1:3306)/demo?parseTime=true"
	broker   = "localhost:9092"
	redisAddr = "127.0.0.1:6379"
)

func ensureTopic(t *testing.T, topic string) {
	t.Helper()
	conn, err := kafka.Dial("tcp", broker)
	if err != nil {
		t.Fatalf("连接 broker 失败: %v", err)
	}
	defer conn.Close()
	controller, _ := conn.Controller()
	cconn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		t.Fatal(err)
	}
	defer cconn.Close()
	_ = cconn.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1})
}

// TestAgentFlow 演示完整闭环：Redis 记忆 → Kafka 分发 → 决策 → 工具 → MySQL 落库。
func TestAgentFlow(t *testing.T) {
	ctx := context.Background()

	// --- MySQL：结果表 ---
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	defer db.Close()
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS task_results (
		task_id VARCHAR(64) PRIMARY KEY, result VARCHAR(255))`)

	// --- Redis：会话记忆 ---
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("连接 Redis 失败: %v", err)
	}
	defer rdb.Close()

	// --- Kafka：任务队列 ---
	topic := fmt.Sprintf("agent-tasks-%d", time.Now().UnixNano())
	ensureTopic(t, topic)
	writer := &kafka.Writer{Addr: kafka.TCP(broker), Topic: topic, RequiredAcks: kafka.RequireAll}
	defer writer.Close()

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())

	// 1. 网关：写会话上下文到 Redis + 投递任务到 Kafka
	if err := rdb.Set(ctx, "session:"+taskID, "帮我搜索 Go 后端资料", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteMessages(ctx, kafka.Message{Key: []byte(taskID), Value: []byte(taskID)}); err != nil {
		t.Fatalf("投递任务失败: %v", err)
	}

	// 2. Worker：消费任务
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker}, Topic: topic, GroupID: "agent-worker",
		CommitInterval: 0,
	})
	defer reader.Close()
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	msg, err := reader.ReadMessage(ctx2)
	cancel()
	if err != nil {
		t.Fatalf("消费任务失败: %v", err)
	}

	// 3. 读 Redis 上下文 + 决策（if-else 模拟 Agent 决策）
	session, _ := rdb.Get(ctx, "session:"+string(msg.Key)).Result()
	decision := "unknown-tool"
	if contains(session, "搜索") {
		decision = "search-tool"
	}

	// 4. 工具执行（模拟）
	result := fmt.Sprintf("[%s] 结果：找到了 Go 后端资料", decision)

	// 5. 写回 Redis（更新记忆）+ MySQL 落库
	rdb.Set(ctx, "result:"+taskID, result, 0)
	if _, err := db.Exec(`INSERT INTO task_results (task_id, result) VALUES (?, ?)`, taskID, result); err != nil {
		t.Fatalf("MySQL 落库失败: %v", err)
	}

	// 6. 验证闭环
	var got string
	if err := db.QueryRow(`SELECT result FROM task_results WHERE task_id = ?`, taskID).Scan(&got); err != nil {
		t.Fatalf("查询 MySQL 结果失败: %v", err)
	}
	mem, _ := rdb.Get(ctx, "result:"+taskID).Result()

	if got != result || mem != result {
		t.Fatalf("闭环不一致：MySQL=%q Redis=%q 期望=%q", got, mem, result)
	}
	t.Logf("Agent Backend 闭环验证通过：任务 %s → 决策 %s → 结果已写 Redis + MySQL", taskID, decision)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
