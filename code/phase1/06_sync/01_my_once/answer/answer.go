//go:build ignore

package answer

import (
	"sync"
	"sync/atomic"
)

type MyOnce struct {
	done uint32
	mu   sync.Mutex
}

func (o *MyOnce) Do(f func()) {

	// 快判断，抢锁
	if atomic.LoadUint32(&o.done) == 1 {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	// 确保抢到锁之后的状态没有改变
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}

func (o *MyOnce) Done() bool {
	return atomic.LoadUint32(&o.done) == 1
}

func (o *MyOnce) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	atomic.StoreUint32(&o.done, 0)
}
