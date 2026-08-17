package idempotent

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Idempotent 参考答案。
func Idempotent(ctx context.Context, c *redis.Client) (bool, error) {
	const key = "order:12345"

	first, err := c.SetNX(ctx, key, "1", 0).Result() // 首次下单成功
	if err != nil {
		return false, err
	}
	_ = first

	dup, err := c.SetNX(ctx, key, "1", 0).Result() // 重复下单：key 已存在，SETNX 失败
	if err != nil {
		return false, err
	}
	return !dup, nil // 重复被拦截 = 第二次 SETNX 失败
}
