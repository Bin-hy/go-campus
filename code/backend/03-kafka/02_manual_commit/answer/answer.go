package manual_commit

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

// ManualCommitAtLeastOnce 参考答案。
func ManualCommitAtLeastOnce(ctx context.Context, broker, topic, group string) (string, error) {
	w := &kafka.Writer{Addr: kafka.TCP(broker), Topic: topic, RequiredAcks: kafka.RequireAll}
	defer w.Close()
	if err := w.WriteMessages(ctx, kafka.Message{Value: []byte("first")}); err != nil {
		return "", err
	}
	if err := w.WriteMessages(ctx, kafka.Message{Value: []byte("second")}); err != nil {
		return "", err
	}

	newReader := func() *kafka.Reader {
		return kafka.NewReader(kafka.ReaderConfig{
			Brokers:        []string{broker},
			Topic:          topic,
			GroupID:        group,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: 0, // 禁用自动提交
		})
	}

	read := func(r *kafka.Reader) (kafka.Message, error) {
		ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return r.ReadMessage(ctx2)
	}

	r1 := newReader()
	m1, err := read(r1)
	if err != nil {
		return "", err
	}
	m2, err := read(r1)
	if err != nil {
		return "", err
	}
	if err := r1.CommitMessages(ctx, m1); err != nil { // 只提交 first
		return "", err
	}
	r1.Close()

	r2 := newReader()
	defer r2.Close()
	again, err := read(r2) // 从 first 之后重读，重复读到 second
	if err != nil {
		return "", err
	}
	_ = m2
	return string(again.Value), nil
}
