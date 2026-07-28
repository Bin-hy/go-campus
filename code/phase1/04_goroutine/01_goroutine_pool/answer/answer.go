//go:build ignore

package answer

import "sync"

type Pool struct {
	tasks chan func()
	wg    sync.WaitGroup
}

func NewPool(maxWorkers int) *Pool {
	p := &Pool{
		tasks: make(chan func(), 100),
	}

	for i := 0; i < maxWorkers; i++ {
		go func() {
			for task := range p.tasks {
				func() {
					defer func() { recover() }()
					task()
				}()
				p.wg.Done()
			}
		}()
	}

	return p
}

func (p *Pool) Submit(task func()) {
	p.wg.Add(1)
	p.tasks <- task
}

func (p *Pool) Wait() {
	p.wg.Wait()
	close(p.tasks)
}
