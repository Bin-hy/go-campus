package consistency

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestCacheAside(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer c.Close()
	ctx := context.Background()
	if err := c.Ping(ctx).Err(); err != nil {
		t.Fatalf("连接 Redis 失败（确认已 docker compose up -d redis）: %v", err)
	}
	c.FlushDB(ctx)

	afterWrite, err := CacheAside(ctx, c)
	if err != nil {
		t.Fatalf("CacheAside 失败: %v", err)
	}
	if afterWrite != "new-value" {
		t.Errorf("写后再读应为 new-value，实际 %s", afterWrite)
	}
	t.Logf("Cache Aside 验证通过：先更新 DB 再删缓存，读回源拿到 %s", afterWrite)
}
