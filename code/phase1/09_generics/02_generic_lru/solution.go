package generic_lru

// GenericLRU 泛型 LRU Cache
// K 必须是 comparable（可作为 map key）
// V 可以是任意类型
type GenericLRU[K comparable, V any] struct {
	// TODO: 定义你的字段
}

// NewGenericLRU 创建泛型 LRU Cache
func NewGenericLRU[K comparable, V any](capacity int) *GenericLRU[K, V] {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Get 获取值，不存在返回零值和 false
func (c *GenericLRU[K, V]) Get(key K) (V, bool) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Put 插入或更新，容量满时淘汰最久未使用的
func (c *GenericLRU[K, V]) Put(key K, value V) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Len 返回当前缓存条目数
func (c *GenericLRU[K, V]) Len() int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Delete 删除指定 key，不存在时不做任何操作
func (c *GenericLRU[K, V]) Delete(key K) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
