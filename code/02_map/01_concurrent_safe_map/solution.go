package concurrent_safe_map

// SafeMap 是一个并发安全的 map
// 请选择合适的字段实现并发安全
type SafeMap struct {
	// TODO: 定义你的字段
}

// NewSafeMap 创建一个新的并发安全 Map
func NewSafeMap() *SafeMap {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Set 设置 key-value 对
func (m *SafeMap) Set(key string, value int) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Get 获取 key 对应的 value
// 返回 (value, 是否存在)
func (m *SafeMap) Get(key string) (int, bool) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Delete 删除指定 key
func (m *SafeMap) Delete(key string) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Len 返回 map 中的元素个数
func (m *SafeMap) Len() int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Range 遍历所有 key-value 对
// fn 返回 false 时停止遍历
func (m *SafeMap) Range(fn func(key string, value int) bool) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
