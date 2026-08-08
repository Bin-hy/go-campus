package goroutine_pool

import (
	"sync"
)

// 如果要做到固定数量的goroutine，让它们不断从任务队列中取任务执行。
// 需要一个ch 存放待执行的任务
// 需考虑竞争锁问题

// Pool 是一个固定大小的 goroutine 池
type Pool struct {
	// TODO: 定义你的字段
	jobs      chan func()    // 任务队列
	taskWG    sync.WaitGroup // 等待任务完成
	workerWG  sync.WaitGroup // 等待worker退出
	closeOnce sync.Once      // 确保只关闭一次
	mu        sync.Mutex     // 保护任务计数器
	closed    bool           // 标记池是否已关闭
}

// NewPool 创建一个最多 maxWorkers 个 worker 的协程池
func NewPool(maxWorkers int) *Pool {
	// TODO: 在这里实现你的代码
	if maxWorkers <= 0 {
		panic("maxWorkers must be greater than 0")
	}
	p := &Pool{
		jobs:   make(chan func(), maxWorkers),
		closed: false,
	}

	for i := 0; i < maxWorkers; i++ {
		p.workerWG.Add(1)
		go func() {
			defer p.workerWG.Done() // 确保worker退出时调用Done

			for task := range p.jobs {
				func() {
					defer p.taskWG.Done()
					defer func() { recover() }() // 捕获panic，防止worker退出 避免没法defer到workerWG.Done导致一直阻塞
					task()
				}()

			}
		}()
	}

	return p
}

// Submit 提交任务到池中执行
// 不应阻塞调用者（除非内部队列满）
func (p *Pool) Submit(task func()) {
	// TODO: 在这里实现你的代码
	if task == nil {
		return
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}

	// 任务提交时，增加等待计数
	p.taskWG.Add(1)

	// 放入任务队列，由NewPool中的for worker: range p.jobs 取出执行
	p.jobs <- task
	p.mu.Unlock()

}

// Wait 等待所有已提交的任务完成
func (p *Pool) Wait() {
	// TODO: 在这里实现你的代码
	// panic("not implemented")

	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()

		p.taskWG.Wait()
		close(p.jobs) //Wait结束，直接关闭 jobs，此时range 会退出。
	})

	// 等待所有worker退出
	p.workerWG.Wait()
}
