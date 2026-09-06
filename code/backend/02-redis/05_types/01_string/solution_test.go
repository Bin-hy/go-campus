package stringx

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func connect(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("连接 Redis 失败（确认已 docker compose up -d redis）: %v", err)
	}
	if err := c.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("FlushDB 失败: %v", err)
	}
	return c
}

func TestCacheUser(t *testing.T) {
	c := connect(t)
	defer c.Close()

	got, ttl, err := CacheUser(context.Background(), c, 1, "Tom")
	if err != nil {
		t.Fatalf("CacheUser 失败: %v", err)
	}
	if !strings.Contains(got, `"Tom"`) {
		t.Errorf("缓存的 JSON 应包含用户名 Tom，实际 %s", got)
	}
	if ttl <= 0 || ttl > 60*time.Second {
		t.Errorf("TTL 应在 (0, 60s] 之间，实际 %v", ttl)
	}
	t.Logf("缓存内容: %s, TTL: %v", got, ttl)
}

func TestIncrViews(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	v, err := IncrViews(ctx, c, 1001, 3)
	if err != nil || v != 3 {
		t.Fatalf("首次加 3 应得 3，实际 %d, err=%v", v, err)
	}
	v, err = IncrViews(ctx, c, 1001, 2)
	if err != nil || v != 5 {
		t.Fatalf("再加 2 应得 5，实际 %d, err=%v", v, err)
	}
}
