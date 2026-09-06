package stringx

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheUser 参考答案。
func CacheUser(ctx context.Context, c *redis.Client, id int64, name string) (string, time.Duration, error) {
	data, err := json.Marshal(map[string]any{"id": id, "name": name})
	if err != nil {
		return "", 0, err
	}
	key := fmt.Sprintf("user:%d", id)
	if err := c.Set(ctx, key, data, 60*time.Second).Err(); err != nil {
		return "", 0, err
	}
	jsonStr, err := c.Get(ctx, key).Result()
	if err != nil {
		return "", 0, err
	}
	ttl, err := c.TTL(ctx, key).Result()
	if err != nil {
		return "", 0, err
	}
	return jsonStr, ttl, nil
}

// IncrViews 参考答案。
func IncrViews(ctx context.Context, c *redis.Client, articleID int64, n int64) (int64, error) {
	return c.IncrBy(ctx, fmt.Sprintf("article:%d:views", articleID), n).Result()
}
