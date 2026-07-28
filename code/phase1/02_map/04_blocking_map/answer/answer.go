//go:build ignore

package answer

import (
	"errors"
	"sync"
	"time"
)

var ErrTimeout = errors.New("get timeout")

type waiter struct {
	ch chan struct{}
}

type BlockingMap struct {
	mu      sync.RWMutex
	data    map[string]interface{}
	waiters map[string][]*waiter
}

func NewBlockingMap() *BlockingMap {
	return &BlockingMap{
		data:    make(map[string]interface{}),
		waiters: make(map[string][]*waiter),
	}
}

func (m *BlockingMap) Put(key string, value interface{}) {
	m.mu.Lock()
	m.data[key] = value
	// 通知所有等待该 key 的 goroutine
	if ws, ok := m.waiters[key]; ok {
		for _, w := range ws {
			close(w.ch)
		}
		delete(m.waiters, key)
	}
	m.mu.Unlock()
}

func (m *BlockingMap) Get(key string, timeout time.Duration) (interface{}, error) {
	m.mu.RLock()
	if v, ok := m.data[key]; ok {
		m.mu.RUnlock()
		return v, nil
	}
	m.mu.RUnlock()

	// key 不存在，注册等待
	w := &waiter{ch: make(chan struct{})}
	m.mu.Lock()
	// double check
	if v, ok := m.data[key]; ok {
		m.mu.Unlock()
		return v, nil
	}
	m.waiters[key] = append(m.waiters[key], w)
	m.mu.Unlock()

	// 阻塞等待
	select {
	case <-w.ch:
		m.mu.RLock()
		v := m.data[key]
		m.mu.RUnlock()
		return v, nil
	case <-time.After(timeout):
		// 清理 waiter（可选优化）
		return nil, ErrTimeout
	}
}
