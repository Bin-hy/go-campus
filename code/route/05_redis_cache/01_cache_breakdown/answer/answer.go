//go:build ignore

package answer

import (
	"context"
	"errors"
	"time"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key string, val string, ttl time.Duration) error
	SetNX(ctx context.Context, key string, val string, ttl time.Duration) (bool, error)
	Del(ctx context.Context, key string) error
}

type Loader func(ctx context.Context, key string) (string, error)

var ErrRebuildTimeout = errors.New("缓存重建超时")

// GetWithMutex 参考答案：SETNX 抢锁 + 持锁重建 + 自旋等待
func GetWithMutex(ctx context.Context, c Cache, loader Loader, key string) (string, error) {
	if v, ok, err := c.Get(ctx, key); err != nil {
		return "", err
	} else if ok {
		return v, nil
	}

	lockKey := "lock:" + key
	got, err := c.SetNX(ctx, lockKey, "1", 5*time.Second)
	if err != nil {
		return "", err
	}

	if got {
		defer c.Del(ctx, lockKey)
		val, err := loader(ctx, key)
		if err != nil {
			return "", err
		}
		if err := c.Set(ctx, key, val, 30*time.Second); err != nil {
			return "", err
		}
		return val, nil
	}

	for i := 0; i < 5; i++ {
		time.Sleep(20 * time.Millisecond)
		if v, ok, err := c.Get(ctx, key); err == nil && ok {
			return v, nil
		}
	}
	return "", ErrRebuildTimeout
}
