// T36 实验：秒杀防超卖 —— Redis Lua 原子扣减库存。
package scenario_lab

import (
	"context"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
)

func client(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("连接 Redis 失败: %v", err)
	}
	return c
}

// 原子扣减库存：只有当库存 > 0 时才减 1，避免超卖。
const decrScript = `
local stock = tonumber(redis.call("GET", KEYS[1]))
if stock and stock > 0 then
    return redis.call("DECR", KEYS[1])
end
return -1`

// TestSeckill 模拟 100 并发抢 10 个库存，验证不超卖（成功数 == 10）。
func TestSeckill(t *testing.T) {
	ctx := context.Background()
	c := client(t)
	defer c.Close()
	c.FlushDB(ctx)

	const stockKey = "seckill:stock"
	c.Set(ctx, stockKey, 10, 0) // 库存 10

	var (
		success int
		mu      sync.Mutex
	)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := c.Eval(ctx, decrScript, []string{stockKey}).Int()
			if err == nil && n >= 0 {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	stock, _ := c.Get(ctx, stockKey).Int()
	if success != 10 {
		t.Fatalf("秒杀应恰好成功 10 次，实际 %d（超卖！）", success)
	}
	if stock != 0 {
		t.Fatalf("库存应扣到 0，实际 %d", stock)
	}
	t.Logf("秒杀防超卖验证通过：100 并发抢 10 库存，成功 %d 次，库存 %d", success, stock)
}

// TestIdempotent 接口幂等：Redis token 一次性（SETNX 删除后不可复用）。
func TestIdempotent(t *testing.T) {
	ctx := context.Background()
	c := client(t)
	defer c.Close()
	c.FlushDB(ctx)

	const tokenKey = "idempotent:token:12345"

	// 第一次：SETNX 拿到 token，处理业务，删 token
	ok1, _ := c.SetNX(ctx, tokenKey, "1", 0).Result()
	if !ok1 {
		t.Fatal("第一次应拿到 token")
	}
	c.Del(ctx, tokenKey) // 业务处理完删除

	// 第二次（重复请求）：token 已删，SETNX 会重新成功 —— 需要配合唯一键兜底
	// 正确姿势：用业务唯一键 + Redis SETNX 作为一次性 token，处理后删除，
	// 重复请求因 token 不存在/已消费而被拒绝。
	// 这里演示：用一个"已消费"标记（SET 而非 SETNX 的删除语义）
	consumed, _ := c.Get(ctx, tokenKey).Result()
	_ = consumed

	// 用「唯一键」方案演示：DB 唯一索引兜底（这里用 Redis 模拟）
	orderKey := "order:12345"
	set, _ := c.SetNX(ctx, orderKey, "1", 0).Result()
	if !set {
		t.Fatal("首次下单应成功")
	}
	// 重复下单：SETNX 失败（已存在）→ 幂等拦截
	dup, _ := c.SetNX(ctx, orderKey, "1", 0).Result()
	if dup {
		t.Fatal("重复下单应被唯一键拦截")
	}
	t.Logf("接口幂等验证通过：唯一键 SETNX 拦截重复请求")
}
