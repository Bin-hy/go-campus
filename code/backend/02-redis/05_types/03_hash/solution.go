package hashx

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// SaveProfile 把用户资料写入 hash user:{id}，字段为 name、city、age。
func SaveProfile(ctx context.Context, c *redis.Client, id int64, name, city string, age int) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// IncrAge 给 user:{id} 的 age 字段原子加 1，返回新年龄和该用户的全部字段。
func IncrAge(ctx context.Context, c *redis.Client, id int64) (int64, map[string]string, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// AddToCart 给购物车 cart:{uid} 中商品 sku 的数量加 delta（delta 可为负数），
// 返回该商品当前数量。
func AddToCart(ctx context.Context, c *redis.Client, uid int64, sku string, delta int64) (int64, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
