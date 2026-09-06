package listx

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// SubmitTask 把任务名放入任务队列 task:queue（LPUSH）。
func SubmitTask(ctx context.Context, c *redis.Client, task string) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// FetchTask 用 BRPOP 阻塞地从 task:queue 取一个任务；
// 超时没有任务时返回 redis.Nil 错误。
func FetchTask(ctx context.Context, c *redis.Client, timeout time.Duration) (string, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// PushFeed 把 postID 加入用户 feed:{uid} 列表头部，只保留最新 5 条，
// 返回当前完整列表（最新在前）。
func PushFeed(ctx context.Context, c *redis.Client, uid int64, postID string) ([]string, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
