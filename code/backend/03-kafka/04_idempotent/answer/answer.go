package idempotent

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

// ConsumeIdempotent 参考答案：用 map 去重表，同 key 只处理一次。
func ConsumeIdempotent(ctx context.Context, broker, topic string) (int, error) {
	w := &kafka.Writer{Addr: kafka.TCP(broker), Topic: topic, RequiredAcks: kafka.RequireAll}
	defer w.Close()
	// 生产两条同 key 消息（模拟重复）
	if err := w.WriteMessages(ctx, kafka.Message{Key: []byte("order-1"), Value: []byte("pay")}); err != nil {
		return 0, err
	}
	if err := w.WriteMessages(ctx, kafka.Message{Key: []byte("order-1"), Value: []byte("pay")}); err != nil {
		return 0, err
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker}, Topic: topic, GroupID: "g-idempotent",
		MinBytes: 1, MaxBytes: 10e6,
	})
	defer r.Close()

	seen := map[string]bool{} // 去重表
	handled := 0
	for i := 0; i < 2; i++ {
		ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
		m, err := r.ReadMessage(ctx2)
		cancel()
		if err != nil {
			return 0, err
		}
		id := string(m.Key)
		if !seen[id] {
			seen[id] = true
			handled++
		}
	}
	return handled, nil
}
