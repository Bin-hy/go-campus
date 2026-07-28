package retry_wrapper

import "time"

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries  int           // 最大重试次数
	InitialWait time.Duration // 初始等待时间
	MaxWait     time.Duration // 最大等待时间
	Multiplier  float64       // 退避倍数（指数退避）
}

// DefaultConfig 默认重试配置
func DefaultConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     5 * time.Second,
		Multiplier:  2.0,
	}
}

// Retry 带指数退避的重试
// 返回最后一次执行的结果，如果全部失败返回最后的 error
func Retry(fn func() error, config RetryConfig) error {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// RetryWithResult 带返回值的重试
func RetryWithResult[T any](fn func() (T, error), config RetryConfig) (T, error) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// IsRetryable 判断错误是否可重试（实现 Retryable 接口检查）
type Retryable interface {
	IsRetryable() bool
}

// ShouldRetry 判断是否应该重试
// - 如果 err 实现了 Retryable 接口，用接口判断
// - 否则默认可重试
func ShouldRetry(err error) bool {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
