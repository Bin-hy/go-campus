package zsetx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// AddHot 参考答案。
func AddHot(ctx context.Context, c *redis.Client, keyword string, delta float64) error {
	return c.ZIncrBy(ctx, "hot:search", delta, keyword).Err()
}

// TopN 参考答案。
func TopN(ctx context.Context, c *redis.Client, n int64) ([]redis.Z, error) {
	return c.ZRevRangeWithScores(ctx, "hot:search", 0, n-1).Result()
}

// AddDelayTask 参考答案。
func AddDelayTask(ctx context.Context, c *redis.Client, task string, runAt time.Time) error {
	return c.ZAdd(ctx, "delay:queue", redis.Z{
		Score:  float64(runAt.UnixMilli()),
		Member: task,
	}).Err()
}

// PopDueTasks 参考答案。
func PopDueTasks(ctx context.Context, c *redis.Client, now time.Time) ([]string, error) {
	key := "delay:queue"
	max := fmt.Sprintf("%d", now.UnixMilli())
	tasks, err := c.ZRangeByScore(ctx, key, &redis.ZRangeBy{Min: "-inf", Max: max}).Result()
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return tasks, nil
	}
	// 面试加分思考：先查后删在并发下可能重复消费，
	// 生产上应用 Lua 脚本把 ZRANGEBYSCORE + ZREM 合成原子操作。
	if err := c.ZRem(ctx, key, tasks).Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}
