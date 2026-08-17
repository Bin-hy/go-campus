package consistency

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheAside 参考答案：读缓存 miss 回源回填，写先更新 DB 再删缓存。
func CacheAside(ctx context.Context, c *redis.Client) (string, error) {
	const key = "item:1"
	db := map[string]string{key: "old-value"}

	read := func() string {
		if v, err := c.Get(ctx, key).Result(); err == nil {
			return v
		}
		v := db[key] // 回源 DB
		c.Set(ctx, key, v, 30*time.Second)
		return v
	}

	_ = read() // 首次读，回填 old-value

	// 写：Cache Aside —— 先更新 DB，再删缓存
	db[key] = "new-value"
	c.Del(ctx, key)

	// 再读：缓存已删，回源拿到 new-value
	return read(), nil
}
