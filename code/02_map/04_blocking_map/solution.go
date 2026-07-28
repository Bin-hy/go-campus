package blocking_map

import (
	"errors"
	"time"
)

// ErrTimeout 超时错误
var ErrTimeout = errors.New("get timeout")

// BlockingMap 支持阻塞读的并发安全 Map
type BlockingMap struct {
	// TODO: 定义你的字段
}

// NewBlockingMap 创建 BlockingMap
func NewBlockingMap() *BlockingMap {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Put 设置值，并通知所有等待该 key 的 goroutine
func (m *BlockingMap) Put(key string, value interface{}) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Get 获取值
// - 如果 key 已存在，立即返回
// - 如果 key 不存在，阻塞等待直到 Put 或超时
// - 超时返回 (nil, ErrTimeout)
func (m *BlockingMap) Get(key string, timeout time.Duration) (interface{}, error) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
