# Rate Limiter 令牌桶限流器

## 难度：⭐⭐⭐ 困难

## 考点
- 令牌桶算法（Token Bucket）
- time.Ticker 定时补充令牌
- channel 实现非阻塞令牌获取
- 并发安全

## 题目描述

实现一个令牌桶限流器：
1. 以固定速率生成令牌（每 interval 时间生成一个）
2. 桶有最大容量 burst，令牌数不超过 burst
3. `Allow()` — 尝试获取一个令牌，成功返回 true，失败返回 false（非阻塞）
4. `Wait()` — 阻塞等待获取一个令牌
5. `Stop()` — 停止限流器，释放资源

## 函数签名

```go
type RateLimiter struct { ... }

func NewRateLimiter(rate int, burst int) *RateLimiter
func (r *RateLimiter) Allow() bool
func (r *RateLimiter) Wait()
func (r *RateLimiter) Stop()
```

参数说明：
- rate: 每秒产生的令牌数
- burst: 桶的最大容量（允许的突发请求数）

## 提示
1. 用 buffered channel（容量=burst）存储令牌
2. 后台 goroutine 用 time.Ticker 定时向 channel 发送令牌
3. Allow 用 select + default 实现非阻塞尝试
4. Wait 直接从 channel 接收（阻塞直到有令牌）
5. Stop 时停止 Ticker 并关闭通知 channel
