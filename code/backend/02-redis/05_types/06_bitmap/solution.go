package bitmapx

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Sign 用户 uid 在 sign:{uid}:{month} 的第 day 天签到（offset = day-1）。
func Sign(ctx context.Context, c *redis.Client, uid int64, month string, day int) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// SignCount 统计用户 uid 在 month 月的签到总天数。
func SignCount(ctx context.Context, c *redis.Client, uid int64, month string) (int64, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// MarkActive 把用户 uid 标记为 date 当天活跃（active:{date}，offset = uid）。
func MarkActive(ctx context.Context, c *redis.Client, date string, uid int64) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// ActiveCount 统计 date 当天的活跃用户数。
func ActiveCount(ctx context.Context, c *redis.Client, date string) (int64, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// BothActiveCount 统计 date1 和 date2 两天都活跃的用户数。
func BothActiveCount(ctx context.Context, c *redis.Client, date1, date2 string) (int64, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
