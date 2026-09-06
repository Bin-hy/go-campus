package hllx

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// AddUV 参考答案。
func AddUV(ctx context.Context, c *redis.Client, page string, users ...string) error {
	vals := make([]any, len(users))
	for i, u := range users {
		vals[i] = u
	}
	return c.PFAdd(ctx, fmt.Sprintf("uv:%s", page), vals...).Err()
}

// CountUV 参考答案。
func CountUV(ctx context.Context, c *redis.Client, page string) (int64, error) {
	return c.PFCount(ctx, fmt.Sprintf("uv:%s", page)).Result()
}

// MergeUV 参考答案。
func MergeUV(ctx context.Context, c *redis.Client, dest string, pages ...string) (int64, error) {
	sources := make([]string, len(pages))
	for i, p := range pages {
		sources[i] = fmt.Sprintf("uv:%s", p)
	}
	destKey := fmt.Sprintf("uv:%s", dest)
	if err := c.PFMerge(ctx, destKey, sources...).Err(); err != nil {
		return 0, err
	}
	return c.PFCount(ctx, destKey).Result()
}
