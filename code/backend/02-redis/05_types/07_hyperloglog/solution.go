package hllx

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// AddUV 记录页面 uv:{page} 的访问用户，可一次添加多个。
func AddUV(ctx context.Context, c *redis.Client, page string, users ...string) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// CountUV 返回页面 uv:{page} 的 UV 估算值。
func CountUV(ctx context.Context, c *redis.Client, page string) (int64, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// MergeUV 把多个页面的 UV 合并到 uv:{dest}，返回合并后的 UV 估算值。
// 跨页面的同一用户只算一次。
func MergeUV(ctx context.Context, c *redis.Client, dest string, pages ...string) (int64, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
