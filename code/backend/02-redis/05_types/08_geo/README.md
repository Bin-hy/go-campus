# 08 Geo：附近门店搜索

## 难度：⭐ 入门

## 考点
- `GEOADD`：存经纬度（经度在前！）
- `GEOSEARCH ... BYRADIUS ... ASC COUNT n`：半径搜索 + 距离排序
- 理解 Geo 底层是 ZSet（GeoHash → score）

## 题目描述

1. `AddShop`：把门店 name 的坐标（lon 经度, lat 纬度）加入 `shops`。
2. `Nearby`：查询坐标 (lon, lat) 附近 radiusKm 公里内最近的 count 家门店，按距离升序返回店名。

## 函数签名

```go
func AddShop(ctx context.Context, c *redis.Client, name string, lon, lat float64) error
func Nearby(ctx context.Context, c *redis.Client, lon, lat, radiusKm float64, count int) ([]string, error)
```

## 提示

1. `c.GeoAdd(ctx, key, &redis.GeoLocation{Name: name, Longitude: lon, Latitude: lat})`
2. `c.GeoSearch(ctx, key, &redis.GeoSearchQuery{...})`，设置 `Radius`、`RadiusUnit: "km"`、`Sort: "ASC"`、`Count: count`
3. 深圳参考坐标：南山 (113.930, 22.533)，福田 (114.055, 22.541)，罗湖 (114.131, 22.548)，广州 (113.264, 23.129)
4. 延伸：`GeoSearchLocation` 可以带回距离和坐标（`WithDist: true`）

## 运行测试

```bash
cd code/backend && docker compose up -d redis
cd 02-redis/05_types/08_geo && go test -v
```
