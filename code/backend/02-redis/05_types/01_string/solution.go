package stringx

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheUser 把用户信息（id、name）序列化成 JSON 写入缓存 key user:{id}，
// 过期时间 60 秒，然后读回 JSON 字符串和剩余 TTL。
func CacheUser(ctx context.Context, c *redis.Client, id int64, name string) (jsonStr string, ttl time.Duration, err error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// IncrViews 给文章 article:{id}:views 的阅读量计数器原子加 n，返回增加后的最新值。
func IncrViews(ctx context.Context, c *redis.Client, articleID int64, n int64) (int64, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
