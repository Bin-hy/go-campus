package seckill

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestSeckill(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer c.Close()
	ctx := context.Background()
	if err := c.Ping(ctx).Err(); err != nil {
		t.Fatalf("连接 Redis 失败（确认已 docker compose up -d redis）: %v", err)
	}
	c.FlushDB(ctx)

	success, err := Seckill(ctx, c)
	if err != nil {
		t.Fatalf("Seckill 失败: %v", err)
	}
	if success != 10 {
		t.Fatalf("秒杀应恰好成功 10 次，实际 %d（超卖！）", success)
	}
	stock, _ := c.Get(ctx, "seckill:stock").Int()
	if stock != 0 {
		t.Fatalf("库存应扣到 0，实际 %d", stock)
	}
	t.Logf("秒杀防超卖验证通过：100 并发抢 10 库存，成功 %d 次", success)
}
