package bitmapx

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Sign 参考答案。
func Sign(ctx context.Context, c *redis.Client, uid int64, month string, day int) error {
	key := fmt.Sprintf("sign:%d:%s", uid, month)
	return c.SetBit(ctx, key, int64(day-1), 1).Err()
}

// SignCount 参考答案。
func SignCount(ctx context.Context, c *redis.Client, uid int64, month string) (int64, error) {
	key := fmt.Sprintf("sign:%d:%s", uid, month)
	return c.BitCount(ctx, key, nil).Result()
}

// MarkActive 参考答案。
func MarkActive(ctx context.Context, c *redis.Client, date string, uid int64) error {
	return c.SetBit(ctx, fmt.Sprintf("active:%s", date), uid, 1).Err()
}

// ActiveCount 参考答案。
func ActiveCount(ctx context.Context, c *redis.Client, date string) (int64, error) {
	return c.BitCount(ctx, fmt.Sprintf("active:%s", date), nil).Result()
}

// BothActiveCount 参考答案。
func BothActiveCount(ctx context.Context, c *redis.Client, date1, date2 string) (int64, error) {
	dest := fmt.Sprintf("active:both:%s:%s", date1, date2)
	if err := c.BitOpAnd(ctx, dest, fmt.Sprintf("active:%s", date1), fmt.Sprintf("active:%s", date2)).Err(); err != nil {
		return 0, err
	}
	return c.BitCount(ctx, dest, nil).Result()
}
