package lock

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestDistributedLock(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer c.Close()
	ctx := context.Background()
	if err := c.Ping(ctx).Err(); err != nil {
		t.Fatalf("连接 Redis 失败（确认已 docker compose up -d redis）: %v", err)
	}
	c.FlushDB(ctx)

	const key = "lock:order"

	// A 加锁成功
	okA, err := AcquireLock(ctx, c, key, "A-owner")
	if err != nil {
		t.Fatal(err)
	}
	if !okA {
		t.Fatal("A 应加锁成功")
	}

	// B 加锁失败（锁已被 A 持有）
	okB, err := AcquireLock(ctx, c, key, "B-owner")
	if err != nil {
		t.Fatal(err)
	}
	if okB {
		t.Fatal("B 不应加锁成功（A 未释放）")
	}

	// B 用错误 value 解锁：应失败，不误删 A 的锁
	released, err := ReleaseLock(ctx, c, key, "B-owner")
	if err != nil {
		t.Fatal(err)
	}
	if released {
		t.Fatal("B 用错误 value 不应解锁成功")
	}

	// A 用正确 value 解锁：成功
	released, err = ReleaseLock(ctx, c, key, "A-owner")
	if err != nil {
		t.Fatal(err)
	}
	if !released {
		t.Fatal("A 应用正确 value 解锁成功")
	}
	t.Log("分布式锁验证通过：SET NX EX 互斥 + Lua 安全解锁")
}
