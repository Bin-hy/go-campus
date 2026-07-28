package lru_cache

// node 双向链表节点
type node struct {
	key, val   int
	prev, next *node
}

// LRUCache LRU 缓存结构
type LRUCache struct {
	// TODO: 定义你的字段
	// 提示：需要 capacity、当前大小、哈希表、双向链表的哨兵节点
}

// NewLRUCache 创建指定容量的 LRU 缓存
func NewLRUCache(capacity int) *LRUCache {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Get 获取 key 对应的 value，不存在返回 -1
// 存在时需要将该节点移到链表头部（标记为最近使用）
func (c *LRUCache) Get(key int) int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Put 插入或更新 key-value
// 容量满时淘汰最久未使用的（链表尾部）
func (c *LRUCache) Put(key, value int) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
