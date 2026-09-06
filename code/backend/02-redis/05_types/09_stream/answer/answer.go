package streamx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const streamKey = "order:events"

// PublishOrderEvent 参考答案。
func PublishOrderEvent(ctx context.Context, c *redis.Client, orderID int64, eventType string) (string, error) {
	return c.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{"orderID": orderID, "type": eventType},
	}).Result()
}

// EnsureGroup 参考答案。
func EnsureGroup(ctx context.Context, c *redis.Client, group string) error {
	err := c.XGroupCreate(ctx, streamKey, group, "0").Err()
	if err != nil && strings.Contains(err.Error(), "BUSYGROUP") {
		return nil // 组已存在，幂等
	}
	return err
}

// ConsumeOne 参考答案。
func ConsumeOne(ctx context.Context, c *redis.Client, group, consumer string) (string, map[string]string, error) {
	res, err := c.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{streamKey, ">"},
		Count:    1,
		Block:    2 * time.Second,
	}).Result()
	if err != nil {
		return "", nil, err // 超时无消息时 err 为 redis.Nil
	}
	msg := res[0].Messages[0]

	fields := make(map[string]string, len(msg.Values))
	for k, v := range msg.Values {
		fields[k] = fmt.Sprint(v)
	}
	if err := c.XAck(ctx, streamKey, group, msg.ID).Err(); err != nil {
		return "", nil, err
	}
	return msg.ID, fields, nil
}
