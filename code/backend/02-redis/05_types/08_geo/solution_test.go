package geox

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

func TestNearby(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	AddShop(ctx, c, "nanshan-store", 113.930, 22.533)
	AddShop(ctx, c, "futian-store", 114.055, 22.541)
	AddShop(ctx, c, "luohu-store", 114.131, 22.548)
	AddShop(ctx, c, "guangzhou-store", 113.264, 23.129) // 远超半径

	// 站在南山，查 20km 内最近的 2 家
	near, err := Nearby(ctx, c, 113.930, 22.533, 20, 2)
	if err != nil {
		t.Fatalf("Nearby 失败: %v", err)
	}
	if len(near) != 2 {
		t.Fatalf("20km 内应只有 2 家（广州超出范围），实际 %v", near)
	}
	if near[0] != "nanshan-store" {
		t.Errorf("最近的应为 nanshan-store，实际 %s", near[0])
	}
	if near[1] != "futian-store" {
		t.Errorf("第二近的应为 futian-store，实际 %s", near[1])
	}
	t.Logf("附近门店: %v", near)
}
