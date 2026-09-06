# 05 Redis 9 种数据类型：动手练习

> 配套教程：[Redis 9 种数据类型与常见用法](../../../../docs/后端技术栈强化/02-redis/数据类型与常见用法.md)

每个目录是一个独立练习：`solution.go` 里手写实现 TODO，`go test -v` 验证，卡住再看 `answer/`。

| 目录 | 类型 | 练习内容 |
|------|------|---------|
| [01_string](./01_string) | String | 缓存用户 JSON + 文章阅读计数器 |
| [02_list](./02_list) | List | 阻塞式任务队列 + 最新动态时间线 |
| [03_hash](./03_hash) | Hash | 用户资料读写 + 购物车改数量 |
| [04_set](./04_set) | Set | 点赞去重 + 共同关注 |
| [05_zset](./05_zset) | ZSet | 热搜排行榜 + 延迟任务队列 |
| [06_bitmap](./06_bitmap) | Bitmap | 用户签到统计 + 日活与连续活跃 |
| [07_hyperloglog](./07_hyperloglog) | HyperLogLog | 页面 UV + 跨页合并去重 |
| [08_geo](./08_geo) | Geo | 附近门店搜索 |
| [09_stream](./09_stream) | Stream | 订单事件流 + 消费组消费与 ACK |

## 环境准备

```bash
cd code/backend && docker compose up -d redis   # Redis 7 @ 127.0.0.1:6379
```

## 建议顺序

先 1-5（基础五种，面试必问）→ 再 6-8（场景加分项）→ 最后 9（Stream，消息队列方向重点）。
