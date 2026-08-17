package cache

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestCachePenetration(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer c.Close()
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("连接 Redis 失败（确认已 docker compose up -d redis）: %v", err)
	}
	c.FlushDB(context.Background())

	dbHits, err := CachePenetration(context.Background(), c)
	if err != nil {
		t.Fatalf("CachePenetration 失败: %v", err)
	}
	if dbHits != 1 {
		t.Errorf("空值缓存后应只打 1 次 DB，实际 %d 次", dbHits)
	}
	t.Logf("缓存穿透（空值缓存）验证通过：打 DB %d 次", dbHits)
}
