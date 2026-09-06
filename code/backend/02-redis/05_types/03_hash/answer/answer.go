package hashx

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// SaveProfile 参考答案。
func SaveProfile(ctx context.Context, c *redis.Client, id int64, name, city string, age int) error {
	key := fmt.Sprintf("user:%d", id)
	return c.HSet(ctx, key, "name", name, "city", city, "age", age).Err()
}

// IncrAge 参考答案。
func IncrAge(ctx context.Context, c *redis.Client, id int64) (int64, map[string]string, error) {
	key := fmt.Sprintf("user:%d", id)
	age, err := c.HIncrBy(ctx, key, "age", 1).Result()
	if err != nil {
		return 0, nil, err
	}
	fields, err := c.HGetAll(ctx, key).Result()
	if err != nil {
		return 0, nil, err
	}
	return age, fields, nil
}

// AddToCart 参考答案。
func AddToCart(ctx context.Context, c *redis.Client, uid int64, sku string, delta int64) (int64, error) {
	key := fmt.Sprintf("cart:%d", uid)
	return c.HIncrBy(ctx, key, sku, delta).Result()
}
