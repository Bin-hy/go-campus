package seckill

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
)

const decrScript = `
local stock = tonumber(redis.call("GET", KEYS[1]))
if stock and stock > 0 then
    return redis.call("DECR", KEYS[1])
end
return -1`

// Seckill 参考答案。
func Seckill(ctx context.Context, c *redis.Client) (int, error) {
	const stockKey = "seckill:stock"
	if err := c.Set(ctx, stockKey, 10, 0).Err(); err != nil {
		return 0, err
	}

	var (
		success int
		mu      sync.Mutex
		wg      sync.WaitGroup
	)
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
	return success, nil
}
