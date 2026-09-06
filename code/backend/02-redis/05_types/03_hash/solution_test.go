package hashx

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

func TestProfile(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	if err := SaveProfile(ctx, c, 1, "Tom", "Shenzhen", 20); err != nil {
		t.Fatalf("SaveProfile 失败: %v", err)
	}
	age, fields, err := IncrAge(ctx, c, 1)
	if err != nil {
		t.Fatalf("IncrAge 失败: %v", err)
	}
	if age != 21 {
		t.Errorf("年龄应为 21，实际 %d", age)
	}
	if fields["name"] != "Tom" || fields["city"] != "Shenzhen" || fields["age"] != "21" {
		t.Errorf("字段不符: %v", fields)
	}
}

func TestCart(t *testing.T) {
	c := connect(t)
	defer c.Close()
	ctx := context.Background()

	n, err := AddToCart(ctx, c, 1001, "sku:888", 2)
	if err != nil || n != 2 {
		t.Fatalf("加 2 件应为 2，实际 %d, err=%v", n, err)
	}
	n, err = AddToCart(ctx, c, 1001, "sku:888", 3)
	if err != nil || n != 5 {
		t.Fatalf("再加 3 件应为 5，实际 %d, err=%v", n, err)
	}
	n, err = AddToCart(ctx, c, 1001, "sku:888", -1)
	if err != nil || n != 4 {
		t.Fatalf("减 1 件应为 4，实际 %d, err=%v", n, err)
	}
}
