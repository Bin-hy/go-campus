package manual_commit

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

const broker = "localhost:9092"

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

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

func TestManualCommitAtLeastOnce(t *testing.T) {
	topic := uniqueName("t-manual-commit")
	group := uniqueName("g")
	ensureTopic(t, topic)

	again, err := ManualCommitAtLeastOnce(context.Background(), broker, topic, group)
	if err != nil {
		t.Fatalf("ManualCommitAtLeastOnce 失败: %v", err)
	}
	if again != "second" {
		t.Errorf("应重复读到 second，实际 %q", again)
	}
	t.Logf("手动提交 at-least-once 验证通过：second 未提交，重启后重复读到 %q", again)
}
