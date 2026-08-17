package order

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

// ProduceConsumeOrdered 参考答案。
func ProduceConsumeOrdered(ctx context.Context, broker, topic string, seq []string) ([]string, error) {
	w := &kafka.Writer{
		Addr:         kafka.TCP(broker),
		Topic:        topic,
		RequiredAcks: kafka.RequireAll,
		Balancer:     &kafka.Hash{}, // 单分区下顺序生产
	}
	defer w.Close()
	for _, s := range seq {
		if err := w.WriteMessages(ctx, kafka.Message{Value: []byte(s)}); err != nil {
			return nil, err
		}
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker}, Topic: topic, GroupID: "g-order",
		MinBytes: 1, MaxBytes: 10e6,
	})
	defer r.Close()

	got := []string{}
	for i := 0; i < len(seq); i++ {
		ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
		m, err := r.ReadMessage(ctx2)
		cancel()
		if err != nil {
			return nil, err
		}
		got = append(got, string(m.Value))
	}
	return got, nil
}
