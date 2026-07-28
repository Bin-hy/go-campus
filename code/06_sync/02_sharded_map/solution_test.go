package sharded_map

import (
	"fmt"
	"sync"
	"testing"
)

func TestShardedMap_Basic(t *testing.T) {
	m := NewShardedMap()
	m.Set("hello", "world")
	m.Set("foo", 42)

	v, ok := m.Get("hello")
	if !ok || v != "world" {
		t.Errorf("Get(hello) = (%v, %v)，期望 (world, true)", v, ok)
	}

	v, ok = m.Get("foo")
	if !ok || v != 42 {
		t.Errorf("Get(foo) = (%v, %v)，期望 (42, true)", v, ok)
	}

	_, ok = m.Get("notexist")
	if ok {
		t.Error("不存在的 key 应返回 false")
	}
}

func TestShardedMap_Delete(t *testing.T) {
	m := NewShardedMap()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Delete("a")

	_, ok := m.Get("a")
	if ok {
		t.Error("删除后不应存在")
	}

	v, ok := m.Get("b")
	if !ok || v != 2 {
		t.Error("删除 a 不应影响 b")
	}
}

func TestShardedMap_Len(t *testing.T) {
	m := NewShardedMap()
	if m.Len() != 0 {
		t.Errorf("空 map 长度应为0，得到 %d", m.Len())
	}

	for i := 0; i < 100; i++ {
		m.Set(fmt.Sprintf("key%d", i), i)
	}
	if m.Len() != 100 {
		t.Errorf("期望长度100，得到 %d", m.Len())
	}

	m.Delete("key0")
	m.Delete("key99")
	if m.Len() != 98 {
		t.Errorf("删除后期望98，得到 %d", m.Len())
	}
}

func TestShardedMap_Concurrent(t *testing.T) {
	m := NewShardedMap()
	var wg sync.WaitGroup

	// 并发写入
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("g%d_k%d", i, j)
				m.Set(key, i*100+j)
			}
		}(i)
	}

	// 并发读取
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("g%d_k%d", i, j)
				m.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// 验证总数
	if m.Len() != 10000 {
		t.Errorf("期望10000个元素，得到 %d", m.Len())
	}
}

func TestShardedMap_Distribution(t *testing.T) {
	m := NewShardedMap()

	// 插入大量 key，验证分片分布相对均匀
	for i := 0; i < 1600; i++ {
		m.Set(fmt.Sprintf("key_%d", i), i)
	}

	if m.Len() != 1600 {
		t.Errorf("总数应为1600，得到 %d", m.Len())
	}
}

func BenchmarkShardedMap_Set(b *testing.B) {
	m := NewShardedMap()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Set(fmt.Sprintf("key%d", i), i)
			i++
		}
	})
}

func BenchmarkShardedMap_Get(b *testing.B) {
	m := NewShardedMap()
	for i := 0; i < 10000; i++ {
		m.Set(fmt.Sprintf("key%d", i), i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Get(fmt.Sprintf("key%d", i%10000))
			i++
		}
	})
}
