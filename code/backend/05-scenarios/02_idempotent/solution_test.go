package idempotent

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestIdempotent(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer c.Close()
	ctx := context.Background()
	if err := c.Ping(ctx).Err(); err != nil {
		t.Fatalf("连接 Redis 失败（确认已 docker compose up -d redis）: %v", err)
	}
	c.FlushDB(ctx)

	blocked, err := Idempotent(ctx, c)
	if err != nil {
		t.Fatalf("Idempotent 失败: %v", err)
	}
	if !blocked {
		t.Error("重复下单应被唯一键拦截")
	}
	t.Log("接口幂等验证通过：唯一键 SETNX 拦截重复请求")
}
