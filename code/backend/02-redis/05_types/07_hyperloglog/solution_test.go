package hllx

import (
	"context"
	"testing"

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

func TestUV(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	AddUV(ctx, c, "home", "u1", "u2", "u3")
	AddUV(ctx, c, "home", "u1") // 重复访问

	n, err := CountUV(ctx, c, "home")
	if err != nil || n != 3 {
		t.Fatalf("home UV 应为 3，实际 %d, err=%v", n, err)
	}

	AddUV(ctx, c, "detail", "u3", "u4")
	total, err := MergeUV(ctx, c, "all", "home", "detail")
	if err != nil {
		t.Fatalf("MergeUV 失败: %v", err)
	}
	if total != 4 {
		t.Fatalf("合并后 UV 应为 4（u3 跨页去重），实际 %d", total)
	}
}
