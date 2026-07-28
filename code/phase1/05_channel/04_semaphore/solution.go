package semaphore

import "time"

// Semaphore 计数信号量
type Semaphore struct {
	// TODO: 定义你的字段
}

// NewSemaphore 创建允许最多 n 个并发的信号量
func NewSemaphore(n int) *Semaphore {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Acquire 获取一个许可，无可用许可时阻塞
func (s *Semaphore) Acquire() {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// TryAcquire 尝试获取许可，超时返回 false
func (s *Semaphore) TryAcquire(timeout time.Duration) bool {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Release 释放一个许可
func (s *Semaphore) Release() {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Available 返回当前可用许可数
func (s *Semaphore) Available() int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
