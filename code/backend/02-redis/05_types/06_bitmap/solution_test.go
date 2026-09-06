package bitmapx

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

func TestSign(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	for _, day := range []int{1, 2, 5, 15} {
		if err := Sign(ctx, c, 1001, "202510", day); err != nil {
			t.Fatalf("Sign(day=%d) 失败: %v", day, err)
		}
	}
	if err := Sign(ctx, c, 1001, "202510", 15); err != nil { // 重复签到
		t.Fatalf("重复 Sign 失败: %v", err)
	}
	n, err := SignCount(ctx, c, 1001, "202510")
	if err != nil || n != 4 {
		t.Fatalf("签到天数应为 4（重复签到不计），实际 %d, err=%v", n, err)
	}
}

func TestActive(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	for _, uid := range []int64{1, 2, 3} {
		MarkActive(ctx, c, "2025-10-14", uid)
	}
	for _, uid := range []int64{2, 3, 4} {
		MarkActive(ctx, c, "2025-10-15", uid)
	}

	n, err := ActiveCount(ctx, c, "2025-10-15")
	if err != nil || n != 3 {
		t.Fatalf("10-15 日活应为 3，实际 %d, err=%v", n, err)
	}
	n, err = BothActiveCount(ctx, c, "2025-10-14", "2025-10-15")
	if err != nil || n != 2 {
		t.Fatalf("两天都活跃应为 2（uid 2、3），实际 %d, err=%v", n, err)
	}
}
