# 分片锁 Map

## 难度：⭐⭐⭐ 困难

## 考点
- 降低锁粒度提高并发性能
- hash 分片策略
- 与单锁 map 的性能对比

## 题目描述

实现一个分片锁 Map（ShardedMap），通过将数据分散到多个分片（每个分片独立加锁）来减少锁竞争。

要求：
1. 固定 16 个分片
2. 用 key 的 hash 决定分片
3. 支持 Set、Get、Delete、Len 操作
4. 并发安全

## 函数签名

```go
type ShardedMap struct { ... }

func NewShardedMap() *ShardedMap
func (m *ShardedMap) Set(key string, value interface{})
func (m *ShardedMap) Get(key string) (interface{}, bool)
func (m *ShardedMap) Delete(key string)
func (m *ShardedMap) Len() int
```

## 提示
1. 16 个 shard，每个 shard 有自己的 sync.RWMutex + map
2. hash 函数可以用 fnv32：`hash/fnv` 包
3. 分片索引 = hash(key) % numShards
4. Len() 需要遍历所有分片加锁求和
