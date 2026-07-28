package generic_lru

import "testing"

func TestGenericLRU_IntString(t *testing.T) {
	cache := NewGenericLRU[int, string](2)

	cache.Put(1, "one")
	cache.Put(2, "two")

	v, ok := cache.Get(1)
	if !ok || v != "one" {
		t.Errorf("Get(1) = (%q, %v)，期望 (one, true)", v, ok)
	}

	cache.Put(3, "three") // 淘汰 2
	_, ok = cache.Get(2)
	if ok {
		t.Error("key=2 应被淘汰")
	}
}

func TestGenericLRU_StringStruct(t *testing.T) {
	type User struct {
		Name string
		Age  int
	}

	cache := NewGenericLRU[string, User](3)
	cache.Put("alice", User{"Alice", 30})
	cache.Put("bob", User{"Bob", 25})

	u, ok := cache.Get("alice")
	if !ok || u.Name != "Alice" || u.Age != 30 {
		t.Errorf("Get(alice) 结果错误: %+v", u)
	}
}

func TestGenericLRU_Len(t *testing.T) {
	cache := NewGenericLRU[string, int](5)
	if cache.Len() != 0 {
		t.Errorf("空缓存 Len 应为0")
	}

	cache.Put("a", 1)
	cache.Put("b", 2)
	if cache.Len() != 2 {
		t.Errorf("Len 应为2，得到 %d", cache.Len())
	}
}

func TestGenericLRU_Delete(t *testing.T) {
	cache := NewGenericLRU[int, int](5)
	cache.Put(1, 100)
	cache.Put(2, 200)

	cache.Delete(1)
	_, ok := cache.Get(1)
	if ok {
		t.Error("删除后不应存在")
	}
	if cache.Len() != 1 {
		t.Errorf("删除后 Len 应为1")
	}

	// 删除不存在的 key 不应 panic
	cache.Delete(999)
}

func TestGenericLRU_Update(t *testing.T) {
	cache := NewGenericLRU[string, int](2)
	cache.Put("x", 1)
	cache.Put("x", 2) // 更新

	v, _ := cache.Get("x")
	if v != 2 {
		t.Errorf("更新后应为2，得到 %d", v)
	}
	if cache.Len() != 1 {
		t.Errorf("更新不应增加条目数")
	}
}

func TestGenericLRU_EvictionOrder(t *testing.T) {
	cache := NewGenericLRU[int, int](3)
	cache.Put(1, 1)
	cache.Put(2, 2)
	cache.Put(3, 3)
	cache.Get(1) // 1 变最新
	cache.Put(4, 4) // 淘汰 2

	if _, ok := cache.Get(2); ok {
		t.Error("2 应被淘汰")
	}
	if _, ok := cache.Get(1); !ok {
		t.Error("1 不应被淘汰（刚被访问过）")
	}
}

func TestGenericLRU_ZeroValue(t *testing.T) {
	cache := NewGenericLRU[string, int](2)

	v, ok := cache.Get("missing")
	if ok {
		t.Error("不存在的 key 应返回 false")
	}
	if v != 0 {
		t.Errorf("不存在时应返回零值，得到 %d", v)
	}
}
