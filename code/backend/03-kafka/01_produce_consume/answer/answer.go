package produce_consume

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

// ProduceAndConsume 参考答案。
func ProduceAndConsume(ctx context.Context, broker, topic string, msgs []string) (map[string]bool, error) {
	w := &kafka.Writer{
		Addr:         kafka.TCP(broker),
		Topic:        topic,
		RequiredAcks: kafka.RequireAll,
	}
	defer w.Close()

	for _, m := range msgs {
		if err := w.WriteMessages(ctx, kafka.Message{Value: []byte(m)}); err != nil {
			return nil, err
		}
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{broker},
		Topic:    topic,
		GroupID:  "g",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer r.Close()

	got := map[string]bool{}
	for i := 0; i < len(msgs); i++ {
		ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
		m, err := r.ReadMessage(ctx2)
		cancel()
		if err != nil {
			return nil, err
		}
		got[string(m.Value)] = true
	}
	return got, nil
}
