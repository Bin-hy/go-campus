package lru_cache

// LRUCache 实现 LRU 缓存
// 请定义合适的数据结构
type LRUCache struct {
	// TODO: 定义你的字段
	// 提示：需要 map、双向链表、容量
}

// NewLRUCache 创建指定容量的 LRU Cache
func NewLRUCache(capacity int) *LRUCache {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Get 获取 key 对应的 value，不存在返回 -1
// 访问后将该 key 标记为最近使用
func (c *LRUCache) Get(key int) int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Put 插入或更新 key-value
// 容量满时淘汰最久未使用的 key
func (c *LRUCache) Put(key int, value int) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
