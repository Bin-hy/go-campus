package lru_cache

import "testing"

// --- 测试用例 ---

func TestLRUCache_Basic(t *testing.T) {
	cache := NewLRUCache(2)

	cache.Put(1, 1)
	cache.Put(2, 2)

	if got := cache.Get(1); got != 1 {
		t.Errorf("Get(1) = %d, 期望 1", got)
	}

	cache.Put(3, 3) // 淘汰 key=2

	if got := cache.Get(2); got != -1 {
		t.Errorf("Get(2) = %d, 期望 -1（已被淘汰）", got)
	}

	cache.Put(4, 4) // 淘汰 key=1

	if got := cache.Get(1); got != -1 {
		t.Errorf("Get(1) = %d, 期望 -1（已被淘汰）", got)
	}
	if got := cache.Get(3); got != 3 {
		t.Errorf("Get(3) = %d, 期望 3", got)
	}
	if got := cache.Get(4); got != 4 {
		t.Errorf("Get(4) = %d, 期望 4", got)
	}
}

func TestLRUCache_Update(t *testing.T) {
	cache := NewLRUCache(2)

	cache.Put(1, 1)
	cache.Put(2, 2)
	cache.Put(1, 10) // 更新 key=1 的值，同时刷新为最近使用

	if got := cache.Get(1); got != 10 {
		t.Errorf("Get(1) = %d, 期望 10（更新后）", got)
	}

	cache.Put(3, 3) // 应淘汰 key=2（因为 key=1 刚被更新过）

	if got := cache.Get(2); got != -1 {
		t.Errorf("Get(2) = %d, 期望 -1（应被淘汰）", got)
	}
	if got := cache.Get(3); got != 3 {
		t.Errorf("Get(3) = %d, 期望 3", got)
	}
}

func TestLRUCache_CapacityOne(t *testing.T) {
	cache := NewLRUCache(1)

	cache.Put(1, 1)
	if got := cache.Get(1); got != 1 {
		t.Errorf("Get(1) = %d, 期望 1", got)
	}

	cache.Put(2, 2) // 淘汰 key=1
	if got := cache.Get(1); got != -1 {
		t.Errorf("Get(1) = %d, 期望 -1", got)
	}
	if got := cache.Get(2); got != 2 {
		t.Errorf("Get(2) = %d, 期望 2", got)
	}
}

func TestLRUCache_GetNotExist(t *testing.T) {
	cache := NewLRUCache(3)

	if got := cache.Get(999); got != -1 {
		t.Errorf("Get(999) = %d, 期望 -1（缓存为空）", got)
	}

	cache.Put(1, 1)
	if got := cache.Get(2); got != -1 {
		t.Errorf("Get(2) = %d, 期望 -1（不存在）", got)
	}
}

func TestLRUCache_GetRefreshOrder(t *testing.T) {
	cache := NewLRUCache(3)

	cache.Put(1, 1)
	cache.Put(2, 2)
	cache.Put(3, 3)

	cache.Get(1) // 刷新 key=1，此时顺序：2, 3, 1（2 最久）

	cache.Put(4, 4) // 应淘汰 key=2

	if got := cache.Get(2); got != -1 {
		t.Errorf("Get(2) = %d, 期望 -1（Get刷新顺序后2应被淘汰）", got)
	}
	if got := cache.Get(1); got != 1 {
		t.Errorf("Get(1) = %d, 期望 1", got)
	}
	if got := cache.Get(3); got != 3 {
		t.Errorf("Get(3) = %d, 期望 3", got)
	}
	if got := cache.Get(4); got != 4 {
		t.Errorf("Get(4) = %d, 期望 4", got)
	}
}

func TestLRUCache_ManyOperations(t *testing.T) {
	cache := NewLRUCache(2)

	cache.Put(2, 1)
	cache.Put(1, 1)
	cache.Put(2, 3) // 更新 key=2，刷新访问
	cache.Put(4, 1) // 淘汰 key=1

	if got := cache.Get(1); got != -1 {
		t.Errorf("Get(1) = %d, 期望 -1", got)
	}
	if got := cache.Get(2); got != 3 {
		t.Errorf("Get(2) = %d, 期望 3", got)
	}
}
