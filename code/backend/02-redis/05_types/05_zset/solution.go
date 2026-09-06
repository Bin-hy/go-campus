package zsetx

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// AddHot 给热搜榜 hot:search 中的关键词原子加 delta 热度。
func AddHot(ctx context.Context, c *redis.Client, keyword string, delta float64) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// TopN 返回热搜榜前 n 名（分数从高到低），结果带分数。
func TopN(ctx context.Context, c *redis.Client, n int64) ([]redis.Z, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// AddDelayTask 把任务加入延迟队列 delay:queue，score 为执行时间戳（毫秒）。
func AddDelayTask(ctx context.Context, c *redis.Client, task string, runAt time.Time) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// PopDueTasks 取出所有 score <= now 的到期任务，从队列中删除后返回任务列表。
func PopDueTasks(ctx context.Context, c *redis.Client, now time.Time) ([]string, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
