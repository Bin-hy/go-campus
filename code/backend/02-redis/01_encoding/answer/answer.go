package encoding

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// EncodingSwitch 参考答案。
func EncodingSwitch(ctx context.Context, c *redis.Client) (string, string, error) {
	if err := c.FlushDB(ctx).Err(); err != nil {
		return "", "", err
	}
	key := "myset"
	if err := c.SAdd(ctx, key, 1, 2, 3, 4, 5).Err(); err != nil {
		return "", "", err
	}
	before, err := c.Do(ctx, "OBJECT", "ENCODING", key).Text()
	if err != nil {
		return "", "", err
	}
	if err := c.SAdd(ctx, key, "hello").Err(); err != nil {
		return "", "", err
	}
	after, err := c.Do(ctx, "OBJECT", "ENCODING", key).Text()
	if err != nil {
		return "", "", err
	}
	return before, after, nil
}
