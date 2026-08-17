package lock

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end`

func AcquireLock(ctx context.Context, c *redis.Client, key, val string) (bool, error) {
	return c.SetNX(ctx, key, val, 30*time.Second).Result()
}

func ReleaseLock(ctx context.Context, c *redis.Client, key, val string) (bool, error) {
	n, err := c.Eval(ctx, releaseScript, []string{key}, val).Int()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}
