// S3 Kafka 实验（T23-T26）：生产消费、手动提交、顺序、幂等。
package kafka_lab

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

// ensureTopic 显式创建单分区 topic。
func ensureTopic(t *testing.T, topic string) {
	t.Helper()
	conn, err := kafka.Dial("tcp", broker)
	if err != nil {
		t.Fatalf("连接 broker 失败: %v", err)
	}
	defer conn.Close()
	controller, err := conn.Controller()
	if err != nil {
		t.Fatalf("获取 controller 失败: %v", err)
	}
	cconn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		t.Fatalf("连接 controller 失败: %v", err)
	}
	defer cconn.Close()
	_ = cconn.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1})
}

// writeMsgs 带重试的写入：topic 创建是异步的，遇 Unknown Topic 等待后重试。
func writeMsgs(t *testing.T, w *kafka.Writer, ctx context.Context, msgs ...kafka.Message) {
	t.Helper()
	for i := 0; i < 15; i++ {
		if err := w.WriteMessages(ctx, msgs...); err == nil {
			return
		}
		time.Sleep(time.Second) // topic 尚未就绪，等待后重试
	}
	t.Fatalf("写入失败（重试 15 次仍失败）")
}

func newWriter(t *testing.T, topic string) *kafka.Writer {
	t.Helper()
	ensureTopic(t, topic)
	return &kafka.Writer{
		Addr:         kafka.TCP(broker),
		Topic:        topic,
		RequiredAcks: kafka.RequireAll, // acks=-1
		Balancer:     &kafka.Hash{},     // key 哈希分区
	}
}

func newReader(t *testing.T, topic, group string) *kafka.Reader {
	t.Helper()
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{broker},
		Topic:          topic,
		GroupID:        group,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0, // 禁用自动提交，手动控制 offset
	})
}

func readMsg(t *testing.T, ctx context.Context, r *kafka.Reader, label string) kafka.Message {
	t.Helper()
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	m, err := r.ReadMessage(ctx2)
	if err != nil {
		t.Fatalf("读 %s 失败: %v", label, err)
	}
	return m
}

// TestProduceConsume 生产 3 条消息并消费 3 条。
func TestProduceConsume(t *testing.T) {
	ctx := context.Background()
	topic := uniqueName("t-produce-consume")
	w := newWriter(t, topic)
	defer w.Close()

	msgs := []string{"a", "b", "c"}
	for _, m := range msgs {
		writeMsgs(t, w, ctx, kafka.Message{Value: []byte(m)})
	}

	r := newReader(t, topic, uniqueName("g"))
	defer r.Close()

	got := map[string]bool{}
	for i := 0; i < 3; i++ {
		m := readMsg(t, ctx, r, fmt.Sprintf("msg-%d", i))
		got[string(m.Value)] = true
	}
	for _, m := range msgs {
		if !got[m] {
			t.Fatalf("未消费到消息 %q", m)
		}
	}
	t.Logf("生产消费验证通过：3 条消息全部消费")
}

// TestManualCommitAtLeastOnce 手动提交：读两条但只提交第一条，第二条不提交（模拟崩溃），
// 重启后从上次提交位置重读，会重复读到第二条（at-least-once）。
func TestManualCommitAtLeastOnce(t *testing.T) {
	ctx := context.Background()
	topic := uniqueName("t-manual-commit")
	group := uniqueName("g")
	w := newWriter(t, topic)
	defer w.Close()
	writeMsgs(t, w, ctx, kafka.Message{Value: []byte("first")})
	writeMsgs(t, w, ctx, kafka.Message{Value: []byte("second")})

	r1 := newReader(t, topic, group)
	m1 := readMsg(t, ctx, r1, "first")
	m2 := readMsg(t, ctx, r1, "second")
	if err := r1.CommitMessages(ctx, m1); err != nil { // 只提交到 first
		t.Fatalf("提交失败: %v", err)
	}
	r1.Close()

	r2 := newReader(t, topic, group)
	defer r2.Close()
	again := readMsg(t, ctx, r2, "again")

	if string(again.Value) != string(m2.Value) {
		t.Fatalf("应重复读到第二条 %q，实际 %q", m2.Value, again.Value)
	}
	t.Logf("手动提交 at-least-once 验证通过：second 未提交，重启后重复读到 %q", again.Value)
}

// TestOrderPreserved 顺序：单分区内按生产顺序消费。
func TestOrderPreserved(t *testing.T) {
	ctx := context.Background()
	topic := uniqueName("t-order")
	w := newWriter(t, topic)
	defer w.Close()

	seq := []string{"1", "2", "3", "4", "5"}
	for _, s := range seq {
		writeMsgs(t, w, ctx, kafka.Message{Value: []byte(s)})
	}

	r := newReader(t, topic, uniqueName("g"))
	defer r.Close()

	got := []string{}
	for i := 0; i < len(seq); i++ {
		m := readMsg(t, ctx, r, fmt.Sprintf("seq-%d", i))
		got = append(got, string(m.Value))
	}
	for i, s := range seq {
		if got[i] != s {
			t.Fatalf("顺序被破坏：第 %d 条应为 %s 实际 %s（全序 %v）", i, s, got[i], got)
		}
	}
	t.Logf("顺序消费验证通过：单分区内 %v", got)
}

// TestIdempotentConsume 幂等：用去重表保证重复消息只处理一次。
func TestIdempotentConsume(t *testing.T) {
	ctx := context.Background()
	topic := uniqueName("t-idempotent")
	w := newWriter(t, topic)
	defer w.Close()

	writeMsgs(t, w, ctx, kafka.Message{Key: []byte("order-1"), Value: []byte("pay")})
	writeMsgs(t, w, ctx, kafka.Message{Key: []byte("order-1"), Value: []byte("pay")})

	r := newReader(t, topic, uniqueName("g"))
	defer r.Close()

	seen := map[string]bool{}
	handled := 0
	for i := 0; i < 2; i++ {
		m := readMsg(t, ctx, r, fmt.Sprintf("dup-%d", i))
		id := string(m.Key)
		if !seen[id] {
			seen[id] = true
			handled++
		}
	}
	if handled != 1 {
		t.Fatalf("幂等消费应只处理 1 次，实际 %d 次", handled)
	}
	t.Logf("幂等消费验证通过：重复消息（同 key）只处理 1 次")
}
