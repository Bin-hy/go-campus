//go:build ignore

package answer

import (
	"hash/fnv"
	"sync"
)

const numShards = 16

type shard struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

type ShardedMap struct {
	shards [numShards]*shard
}

func NewShardedMap() *ShardedMap {
	m := &ShardedMap{}
	for i := range m.shards {
		m.shards[i] = &shard{data: make(map[string]interface{})}
	}
	return m
}

func (m *ShardedMap) getShard(key string) *shard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return m.shards[h.Sum32()%numShards]
}

func (m *ShardedMap) Set(key string, value interface{}) {
	s := m.getShard(key)
	s.mu.Lock()
	s.data[key] = value
	s.mu.Unlock()
}

func (m *ShardedMap) Get(key string) (interface{}, bool) {
	s := m.getShard(key)
	s.mu.RLock()
	v, ok := s.data[key]
	s.mu.RUnlock()
	return v, ok
}

func (m *ShardedMap) Delete(key string) {
	s := m.getShard(key)
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
}

func (m *ShardedMap) Len() int {
	total := 0
	for _, s := range m.shards {
		s.mu.RLock()
		total += len(s.data)
		s.mu.RUnlock()
	}
	return total
}
