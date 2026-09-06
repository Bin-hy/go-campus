package geox

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// AddShop 把门店加入 shops 地理索引（lon 经度，lat 纬度）。
func AddShop(ctx context.Context, c *redis.Client, name string, lon, lat float64) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Nearby 查询坐标 (lon, lat) 附近 radiusKm 公里内最近的 count 家门店，
// 按距离从近到远返回店名。
func Nearby(ctx context.Context, c *redis.Client, lon, lat, radiusKm float64, count int) ([]string, error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
