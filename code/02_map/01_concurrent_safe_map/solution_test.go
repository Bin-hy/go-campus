package concurrent_safe_map

import (
	"sync"
	"testing"
)

func TestSafeMap_Basic(t *testing.T) {
	m := NewSafeMap()

	// Set and Get
	m.Set("hello", 1)
	m.Set("world", 2)

	v, ok := m.Get("hello")
	if !ok || v != 1 {
		t.Errorf("Get(hello): 期望 (1, true)，得到 (%d, %v)", v, ok)
	}

	v, ok = m.Get("world")
	if !ok || v != 2 {
		t.Errorf("Get(world): 期望 (2, true)，得到 (%d, %v)", v, ok)
	}

	// 不存在的 key
	v, ok = m.Get("notexist")
	if ok {
		t.Errorf("Get(notexist): 期望 (_, false)，得到 (%d, true)", v)
	}
}

func TestSafeMap_Update(t *testing.T) {
	m := NewSafeMap()

	m.Set("key", 100)
	m.Set("key", 200) // 更新

	v, ok := m.Get("key")
	if !ok || v != 200 {
		t.Errorf("更新后 Get(key): 期望 (200, true)，得到 (%d, %v)", v, ok)
	}
}

func TestSafeMap_Delete(t *testing.T) {
	m := NewSafeMap()

	m.Set("a", 1)
	m.Set("b", 2)
	m.Delete("a")

	_, ok := m.Get("a")
	if ok {
		t.Error("Delete 后 key 仍然存在")
	}

	v, ok := m.Get("b")
	if !ok || v != 2 {
		t.Error("Delete 影响了其他 key")
	}

	// 删除不存在的 key 不应 panic
	m.Delete("notexist")
}

func TestSafeMap_Len(t *testing.T) {
	m := NewSafeMap()

	if m.Len() != 0 {
		t.Errorf("空 map 长度应为0，得到 %d", m.Len())
	}

	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	if m.Len() != 3 {
		t.Errorf("期望长度3，得到 %d", m.Len())
	}

	m.Delete("b")
	if m.Len() != 2 {
		t.Errorf("删除后期望长度2，得到 %d", m.Len())
	}
}

func TestSafeMap_Range(t *testing.T) {
	m := NewSafeMap()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	collected := make(map[string]int)
	m.Range(func(key string, value int) bool {
		collected[key] = value
		return true
	})

	if len(collected) != 3 {
		t.Errorf("Range 应遍历3个元素，实际遍历了 %d 个", len(collected))
	}
}

func TestSafeMap_Range_EarlyStop(t *testing.T) {
	m := NewSafeMap()
	for i := 0; i < 10; i++ {
		m.Set(string(rune('a'+i)), i)
	}

	count := 0
	m.Range(func(key string, value int) bool {
		count++
		return count < 3 // 只遍历3个就停止
	})

	if count != 3 {
		t.Errorf("Range 提前停止失败：期望遍历3次，实际 %d 次", count)
	}
}

func TestSafeMap_Concurrent(t *testing.T) {
	m := NewSafeMap()
	var wg sync.WaitGroup

	// 100 个 goroutine 并发写
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Set(string(rune('A'+i%26)), i)
		}(i)
	}

	// 100 个 goroutine 并发读
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Get(string(rune('A' + i%26)))
		}(i)
	}

	// 50 个 goroutine 并发删除
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Delete(string(rune('A' + i%26)))
		}(i)
	}

	wg.Wait()

	// 不 panic 即为通过
	// 验证 Len 与实际一致
	count := 0
	m.Range(func(key string, value int) bool {
		count++
		return true
	})
	if count != m.Len() {
		t.Errorf("Len() 与 Range 计数不一致：Len=%d, Range=%d", m.Len(), count)
	}
}

func TestSafeMap_ConcurrentReadWrite(t *testing.T) {
	m := NewSafeMap()
	var wg sync.WaitGroup

	// 并发读写同一个 key
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			m.Set("counter", i)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			m.Get("counter")
		}
	}()

	wg.Wait()

	v, ok := m.Get("counter")
	if !ok {
		t.Error("并发写入后 key 应该存在")
	}
	if v != 9999 {
		t.Errorf("最终值应为9999，得到 %d", v)
	}
}
