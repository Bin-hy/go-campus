package order

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

func TestProduceConsumeOrdered(t *testing.T) {
	topic := uniqueName("t-order")
	ensureTopic(t, topic)

	seq := []string{"1", "2", "3", "4", "5"}
	got, err := ProduceConsumeOrdered(context.Background(), broker, topic, seq)
	if err != nil {
		t.Fatalf("ProduceConsumeOrdered 失败: %v", err)
	}
	for i, s := range seq {
		if got[i] != s {
			t.Fatalf("顺序被破坏：第 %d 条应为 %s 实际 %s（全序 %v）", i, s, got[i], got)
		}
	}
	t.Logf("顺序消费验证通过：单分区内 %v", got)
}
