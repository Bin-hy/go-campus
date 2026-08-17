package produce_consume

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

func uniqueTopic(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// ensureTopic 显式创建单分区 topic 并等待就绪。
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
	time.Sleep(3 * time.Second) // 等待 broker 完成 topic 创建
}

func TestProduceConsume(t *testing.T) {
	topic := uniqueTopic("t-produce-consume")
	ensureTopic(t, topic)

	ctx := context.Background()
	msgs := []string{"a", "b", "c"}

	got, err := ProduceAndConsume(ctx, broker, topic, msgs)
	if err != nil {
		t.Fatalf("ProduceAndConsume 失败: %v", err)
	}
	for _, m := range msgs {
		if !got[m] {
			t.Errorf("未消费到消息 %q", m)
		}
	}
	t.Logf("生产消费验证通过：3 条消息全部消费")
}
