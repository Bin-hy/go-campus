package lru_cache

import "testing"

func TestLRUCache_Basic(t *testing.T) {
	cache := NewLRUCache(2)

	cache.Put(1, 1)
	cache.Put(2, 2)

	if v := cache.Get(1); v != 1 {
		t.Errorf("Get(1) = %d，期望 1", v)
	}

	// 容量满，淘汰 key=2（最久未使用）
	cache.Put(3, 3)

	if v := cache.Get(2); v != -1 {
		t.Errorf("Get(2) = %d，期望 -1（已淘汰）", v)
	}

	// 淘汰 key=1? 不，key=1 之前 Get 过，应淘汰 key=3
	// 等等，Put(3,3) 后 key=3 是最新的，key=1 比 key=3 旧
	// 所以 Put(4,4) 应淘汰 key=1
	cache.Put(4, 4)

	if v := cache.Get(1); v != -1 {
		t.Errorf("Get(1) = %d，期望 -1（已淘汰）", v)
	}
	if v := cache.Get(3); v != 3 {
		t.Errorf("Get(3) = %d，期望 3", v)
	}
	if v := cache.Get(4); v != 4 {
		t.Errorf("Get(4) = %d，期望 4", v)
	}
}

func TestLRUCache_Update(t *testing.T) {
	cache := NewLRUCache(2)

	cache.Put(1, 1)
	cache.Put(2, 2)
	cache.Put(1, 10) // 更新 key=1 的值，同时标记为最近使用

	if v := cache.Get(1); v != 10 {
		t.Errorf("更新后 Get(1) = %d，期望 10", v)
	}

	// 此时 key=1 最新，key=2 最旧
	cache.Put(3, 3) // 应淘汰 key=2

	if v := cache.Get(2); v != -1 {
		t.Errorf("Get(2) = %d，期望 -1", v)
	}
	if v := cache.Get(1); v != 10 {
		t.Errorf("Get(1) = %d，期望 10", v)
	}
}

func TestLRUCache_Capacity1(t *testing.T) {
	cache := NewLRUCache(1)

	cache.Put(1, 1)
	if v := cache.Get(1); v != 1 {
		t.Errorf("Get(1) = %d，期望 1", v)
	}

	cache.Put(2, 2) // 淘汰 key=1
	if v := cache.Get(1); v != -1 {
		t.Errorf("Get(1) = %d，期望 -1", v)
	}
	if v := cache.Get(2); v != 2 {
		t.Errorf("Get(2) = %d，期望 2", v)
	}
}

func TestLRUCache_GetNotExist(t *testing.T) {
	cache := NewLRUCache(3)

	if v := cache.Get(999); v != -1 {
		t.Errorf("空缓存 Get(999) = %d，期望 -1", v)
	}

	cache.Put(1, 1)
	if v := cache.Get(2); v != -1 {
		t.Errorf("Get(2) = %d，期望 -1", v)
	}
}

func TestLRUCache_EvictionOrder(t *testing.T) {
	cache := NewLRUCache(3)

	cache.Put(1, 1)
	cache.Put(2, 2)
	cache.Put(3, 3)
	// 顺序：3(最新) -> 2 -> 1(最旧)

	cache.Get(1) // 访问 key=1，变为最新
	// 顺序：1(最新) -> 3 -> 2(最旧)

	cache.Put(4, 4) // 淘汰 key=2
	if v := cache.Get(2); v != -1 {
		t.Errorf("Get(2) = %d，期望 -1（应被淘汰）", v)
	}
	if v := cache.Get(1); v != 1 {
		t.Errorf("Get(1) = %d，期望 1", v)
	}
	if v := cache.Get(3); v != 3 {
		t.Errorf("Get(3) = %d，期望 3", v)
	}
	if v := cache.Get(4); v != 4 {
		t.Errorf("Get(4) = %d，期望 4", v)
	}
}

func TestLRUCache_LargeCapacity(t *testing.T) {
	cache := NewLRUCache(1000)

	for i := 0; i < 1000; i++ {
		cache.Put(i, i*10)
	}

	// 所有 key 都在
	for i := 0; i < 1000; i++ {
		if v := cache.Get(i); v != i*10 {
			t.Errorf("Get(%d) = %d，期望 %d", i, v, i*10)
		}
	}

	// 插入第 1001 个，淘汰最旧的
	// 最旧的是 key=0（因为 Get 循环从 0 开始，最后 Get 的是 999）
	// 等等，Get 本身也会更新顺序... 所以 Get(0) 是第一个被访问的
	// Get 循环结束后，key=999 是最旧的（第一个被 Get 的），key=0 反而是比较旧的...
	// 不对：Get(0) 让 0 变最新，Get(1) 让 1 变最新... Get(999) 让 999 变最新
	// 所以 999 是最最新的，0 是最旧的
	cache.Put(9999, 9999)
	if v := cache.Get(0); v != -1 {
		t.Errorf("Get(0) = %d，期望 -1（应被淘汰）", v)
	}
	if v := cache.Get(9999); v != 9999 {
		t.Errorf("Get(9999) = %d，期望 9999", v)
	}
}

func TestLRUCache_SameKeyMultiplePut(t *testing.T) {
	cache := NewLRUCache(2)

	cache.Put(1, 1)
	cache.Put(1, 2)
	cache.Put(1, 3) // 多次更新同一个 key

	if v := cache.Get(1); v != 3 {
		t.Errorf("Get(1) = %d，期望 3", v)
	}

	// 容量应该还是只用了 1 个位置
	cache.Put(2, 2)
	// 此时 key=2 最新，key=1 次新（上面 Get(1) 比 Put(2) 早）
	cache.Put(3, 3) // 容量满，淘汰最旧的 key=1

	if v := cache.Get(1); v != -1 {
		t.Errorf("Get(1) = %d，期望 -1（应被淘汰）", v)
	}
	if v := cache.Get(3); v != 3 {
		t.Errorf("Get(3) = %d，期望 3", v)
	}
}
