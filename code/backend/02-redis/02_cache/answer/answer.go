package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// CachePenetration 参考答案。
func CachePenetration(ctx context.Context, c *redis.Client) (int, error) {
	const key = "item:notexist"
	dbHit := 0

	get := func() {
		if _, err := c.Get(ctx, key).Result(); err == nil {
			return // 命中缓存（含空值）
		}
		dbHit++
		// 模拟 DB：key 不存在，缓存空值（短 TTL）
		c.Set(ctx, key, "", 10*time.Second)
	}

	get() // 第一次：打 DB
	get() // 第二次：命中空值缓存，不打 DB
	return dbHit, nil
}
