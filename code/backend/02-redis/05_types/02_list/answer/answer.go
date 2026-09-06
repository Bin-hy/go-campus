package listx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SubmitTask 参考答案。
func SubmitTask(ctx context.Context, c *redis.Client, task string) error {
	return c.LPush(ctx, "task:queue", task).Err()
}

// FetchTask 参考答案。
func FetchTask(ctx context.Context, c *redis.Client, timeout time.Duration) (string, error) {
	res, err := c.BRPop(ctx, timeout, "task:queue").Result()
	if err != nil {
		return "", err // 超时时 err 就是 redis.Nil
	}
	return res[1], nil // res = [key, value]
}

// PushFeed 参考答案。
func PushFeed(ctx context.Context, c *redis.Client, uid int64, postID string) ([]string, error) {
	key := fmt.Sprintf("feed:%d", uid)
	pipe := c.Pipeline()
	pipe.LPush(ctx, key, postID)
	pipe.LTrim(ctx, key, 0, 4) // 只留最新 5 条
	rangeCmd := pipe.LRange(ctx, key, 0, -1)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	return rangeCmd.Val(), nil
}
