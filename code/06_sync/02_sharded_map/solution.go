package sharded_map

const numShards = 16

// ShardedMap 分片锁 Map
type ShardedMap struct {
	// TODO: 定义你的字段
}

// NewShardedMap 创建分片 Map
func NewShardedMap() *ShardedMap {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Set 设置 key-value
func (m *ShardedMap) Set(key string, value interface{}) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Get 获取 key 对应的 value
func (m *ShardedMap) Get(key string) (interface{}, bool) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Delete 删除 key
func (m *ShardedMap) Delete(key string) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Len 返回所有分片的元素总数
func (m *ShardedMap) Len() int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
