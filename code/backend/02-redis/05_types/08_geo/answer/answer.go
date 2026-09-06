package geox

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// AddShop 参考答案。
func AddShop(ctx context.Context, c *redis.Client, name string, lon, lat float64) error {
	return c.GeoAdd(ctx, "shops", &redis.GeoLocation{
		Name:      name,
		Longitude: lon,
		Latitude:  lat,
	}).Err()
}

// Nearby 参考答案。
func Nearby(ctx context.Context, c *redis.Client, lon, lat, radiusKm float64, count int) ([]string, error) {
	return c.GeoSearch(ctx, "shops", &redis.GeoSearchQuery{
		Longitude:  lon,
		Latitude:   lat,
		Radius:     radiusKm,
		RadiusUnit: "km",
		Sort:       "ASC",
		Count:      count,
	}).Result()
}
