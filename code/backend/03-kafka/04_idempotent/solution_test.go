package idempotent

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

func TestConsumeIdempotent(t *testing.T) {
	topic := uniqueName("t-idempotent")
	ensureTopic(t, topic)

	handled, err := ConsumeIdempotent(context.Background(), broker, topic)
	if err != nil {
		t.Fatalf("ConsumeIdempotent 失败: %v", err)
	}
	if handled != 1 {
		t.Errorf("幂等消费应只处理 1 次，实际 %d 次", handled)
	}
	t.Logf("幂等消费验证通过：重复消息（同 key）只处理 1 次")
}
