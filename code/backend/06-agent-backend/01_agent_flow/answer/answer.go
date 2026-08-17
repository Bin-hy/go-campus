package agent_flow

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// AgentFlow 参考答案：完整闭环。
func AgentFlow(ctx context.Context, db *sql.DB, rdb *redis.Client, broker, topic, taskID string) (string, error) {
	// 1. 写 Redis 会话上下文
	if err := rdb.Set(ctx, "session:"+taskID, "帮我搜索 Go 后端资料", 0).Err(); err != nil {
		return "", err
	}

	// 2. 投递任务到 Kafka
	w := &kafka.Writer{Addr: kafka.TCP(broker), Topic: topic, RequiredAcks: kafka.RequireAll}
	if err := w.WriteMessages(ctx, kafka.Message{Key: []byte(taskID), Value: []byte(taskID)}); err != nil {
		return "", err
	}
	w.Close()

	// 3. 消费任务
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker}, Topic: topic, GroupID: "agent-worker-" + taskID,
		MinBytes: 1, MaxBytes: 10e6,
	})
	defer r.Close()
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	msg, err := r.ReadMessage(ctx2)
	cancel()
	if err != nil {
		return "", err
	}

	// 4. 读上下文决策
	session, _ := rdb.Get(ctx, "session:"+string(msg.Key)).Result()
	decision := "unknown-tool"
	if strings.Contains(session, "搜索") {
		decision = "search-tool"
	}

	// 5. 工具执行 + 写回 Redis + MySQL 落库
	result := "[" + decision + "] 结果：找到了 Go 后端资料"
	rdb.Set(ctx, "result:"+taskID, result, 0)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS task_results (
		task_id VARCHAR(64) PRIMARY KEY, result VARCHAR(255))`); err != nil {
		return "", err
	}
	if _, err := db.Exec(`INSERT INTO task_results (task_id, result) VALUES (?, ?)`, taskID, result); err != nil {
		return "", err
	}
	return result, nil
}
