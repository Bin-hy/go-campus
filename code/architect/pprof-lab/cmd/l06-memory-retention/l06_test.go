package main

import (
	"bytes"
	"strconv"
	"testing"
	"time"
)

// TestMapStoreGrowsWithoutBound 验证 bug 模式确实只涨不降。
func TestMapStoreGrowsWithoutBound(t *testing.T) {
	s := newMapStore()
	defer s.Close()

	for i := 0; i < 50; i++ {
		if ev := s.Put("k"+strconv.Itoa(i), []byte("x")); ev != 0 {
			t.Fatalf("bug 模式不应该淘汰任何条目，实际淘汰 %d", ev)
		}
	}
	if got := s.Entries(); got != 50 {
		t.Fatalf("条目数应为 50，实际 %d", got)
	}
	if s.Capacity() != -1 || s.Evictions() != 0 {
		t.Fatalf("bug 模式应无上限且无淘汰: cap=%d evictions=%d", s.Capacity(), s.Evictions())
	}
	t.Logf("bug 模式: entries=%d（没有任何上界）", s.Entries())
}

// TestLRUEvictsByCapacity 验证 fix 模式按容量淘汰最久未使用的条目。
func TestLRUEvictsByCapacity(t *testing.T) {
	const (
		cap_  = 8
		total = 100
	)
	s := newLRUStore(lruOptions{maxEntries: cap_, maxBytes: 1 << 20, ttl: time.Minute, sweep: time.Hour})
	defer s.Close()

	for i := 0; i < total; i++ {
		s.Put("k"+strconv.Itoa(i), make([]byte, 1024))
	}

	if got := s.Entries(); got != cap_ {
		t.Fatalf("条目数应被限制在 %d，实际 %d", cap_, got)
	}
	if got := s.Evictions(); got != total-cap_ {
		t.Fatalf("淘汰数应为 %d，实际 %d", total-cap_, got)
	}
	if _, ok := s.Get("k99"); !ok {
		t.Fatal("最近写入的条目应该还在")
	}
	if _, ok := s.Get("k0"); ok {
		t.Fatal("最久未使用的条目应该已被淘汰")
	}
	if got := s.Bytes(); got != int64(cap_)*1024 {
		t.Fatalf("占用字节数应为 %d，实际 %d", cap_*1024, got)
	}
}

// TestLRUEvictsByBytes 验证字节上限同样生效（单条超大时也会被淘汰）。
func TestLRUEvictsByBytes(t *testing.T) {
	s := newLRUStore(lruOptions{maxEntries: 1000, maxBytes: 8 << 10, ttl: time.Minute, sweep: time.Hour})
	defer s.Close()

	for i := 0; i < 10; i++ {
		s.Put("k"+strconv.Itoa(i), make([]byte, 4<<10)) // 每条 4KB，最多放 2 条
	}
	if got := s.Entries(); got != 2 {
		t.Fatalf("按字节上限应只剩 2 条，实际 %d", got)
	}
	if got := s.Bytes(); got > 8<<10 {
		t.Fatalf("占用超限: %d", got)
	}
}

// TestLRUTTL 验证过期条目既不会被 Get 返回，也会被 janitor 清掉。
func TestLRUTTL(t *testing.T) {
	s := newLRUStore(lruOptions{
		maxEntries: 10,
		maxBytes:   1 << 20,
		ttl:        30 * time.Millisecond,
		sweep:      10 * time.Millisecond,
	})
	defer s.Close()

	s.Put("x", []byte("hello"))
	if _, ok := s.Get("x"); !ok {
		t.Fatal("刚写入就应该命中")
	}

	time.Sleep(80 * time.Millisecond)

	if _, ok := s.Get("x"); ok {
		t.Fatal("TTL 过期后不应该命中")
	}
	if got := s.Entries(); got != 0 {
		t.Fatalf("过期条目应被清理，实际还剩 %d 条", got)
	}
}

// TestStoreContent 两种实现写入的内容要能原样读出来（LRU 只是淘汰策略，不是改数据）。
func TestStoreContent(t *testing.T) {
	payload := bytes.Repeat([]byte("gocampus"), 128)

	for _, st := range []store{newMapStore(), newLRUStore(lruOptions{})} {
		st.Put("same-key", payload)
		got, ok := st.Get("same-key")
		if !ok {
			t.Fatalf("写入后应立即命中")
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("读出的内容与写入不一致")
		}
		st.Close()
	}
}
