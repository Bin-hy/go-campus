package rate_limiter

// RateLimiter 令牌桶限流器
type RateLimiter struct {
	// TODO: 定义你的字段
}

// NewRateLimiter 创建限流器
// rate: 每秒产生的令牌数
// burst: 最大突发量（桶容量）
func NewRateLimiter(rate int, burst int) *RateLimiter {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Allow 非阻塞尝试获取一个令牌
// 有令牌返回 true，无令牌返回 false
func (r *RateLimiter) Allow() bool {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Wait 阻塞等待获取一个令牌
func (r *RateLimiter) Wait() {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Stop 停止限流器，释放后台 goroutine
func (r *RateLimiter) Stop() {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
