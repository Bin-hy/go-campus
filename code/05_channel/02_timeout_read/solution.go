package timeout_read

import (
	"errors"
	"time"
)

// ErrTimeout 超时错误
var ErrTimeout = errors.New("read timeout")

// ReadWithTimeout 从 channel 读取一个值，超时返回错误
func ReadWithTimeout(ch <-chan int, timeout time.Duration) (int, error) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// ReadMultipleWithTimeout 从 channel 读取最多 n 个值
// 超时后立即返回已读取的值（可能不足 n 个）
func ReadMultipleWithTimeout(ch <-chan int, n int, timeout time.Duration) []int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// FirstResult 并发执行多个函数，返回第一个完成的结果
// 保证至少传入一个函数
func FirstResult(fns ...func() int) int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
