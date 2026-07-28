package goroutine_pool

// Pool 是一个固定大小的 goroutine 池
type Pool struct {
	// TODO: 定义你的字段
}

// NewPool 创建一个最多 maxWorkers 个 worker 的协程池
func NewPool(maxWorkers int) *Pool {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Submit 提交任务到池中执行
// 不应阻塞调用者（除非内部队列满）
func (p *Pool) Submit(task func()) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Wait 等待所有已提交的任务完成
func (p *Pool) Wait() {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
