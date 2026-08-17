# 限流：令牌桶与滑动窗口

## 难度：⭐⭐⭐ 困难

## 考点
- 令牌桶（允许突发）
- 滑动窗口（平滑限流）

## 题目描述

实现两种限流器：

- `TokenBucket`：容量 `capacity`，每秒补充 `rate` 个令牌，`Allow()` 取一个令牌，有则放行、无则拒绝（允许突发）
- `SlidingWindow`：最近 `window` 内最多放行 `limit` 个请求，`Allow()` 判断是否放行（平滑）

## 函数签名

```go
type TokenBucket struct { ... }
func NewTokenBucket(capacity, rate float64) *TokenBucket
func (b *TokenBucket) Allow() bool

type SlidingWindow struct { ... }
func NewSlidingWindow(window time.Duration, limit int) *SlidingWindow
func (w *SlidingWindow) Allow() bool
```

## 提示

- 令牌桶：`Allow` 时先按 elapsed 时间补充令牌（不超过 capacity），有令牌就减一返回 true
- 滑动窗口：`Allow` 时淘汰 window 之外的旧时间戳，剩余数 < limit 才放行并记录当前时间戳

## 运行测试

```bash
cd code/backend/04-microservice/02_ratelimit && go test -v
```
