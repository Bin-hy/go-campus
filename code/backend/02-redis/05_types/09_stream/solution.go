package streamx

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// PublishOrderEvent 往 order:events 追加一条订单事件，
// 字段包含 orderID 和 type，返回自动生成的消息 ID。
func PublishOrderEvent(ctx context.Context, c *redis.Client, orderID int64, eventType string) (string, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// EnsureGroup 为 order:events 从头（0）创建消费组；
// 组已存在（BUSYGROUP）时返回 nil。
func EnsureGroup(ctx context.Context, c *redis.Client, group string) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// ConsumeOne 以组内消费者 consumer 的身份从 order:events 读一条新消息，
// 确认（XACK）后返回消息 ID 和字段 map。
// 2 秒内没有新消息返回 redis.Nil 错误。
func ConsumeOne(ctx context.Context, c *redis.Client, group, consumer string) (string, map[string]string, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
