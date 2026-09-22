// L06 · 内存长期持有：全局 map 只涨不降
//
// 业务场景：素材缩略图的"缓存"。每次 /api/cache/put?kb=256&n=100 写入 n 条、
// 每条 kb KB 的数据。
//
// bug 模式就是一个全局 map[string][]byte：没有容量上限、没有 TTL、没有淘汰。
// 只要 key 不同就一直长，heap_inuse 只涨不降——注意这些内存**不是泄漏**（GC 看得见它们），
// 而是被业务代码一直强引用着，所以 GC 也无能为力。
//
// fix 模式换成带容量上限 + TTL 的 LRU（container/list 标准库实现）：
// 超出上限就淘汰最久未使用的条目，过期条目由后台 janitor 清理，占用有上界。
package main

import (
	"container/list"
	"context"
	"flag"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gocampus/perf/pprof-lab/internal/labkit"
)

var (
	addrFlag      = flag.String("addr", "127.0.0.1:18086", "业务监听地址")
	pprofAddrFlag = flag.String("pprof-addr", "127.0.0.1:19086", "pprof 管理监听地址（生产只绑内网）")
	fixFlag       = flag.Bool("fix", false, "使用修复实现（容量上限 + TTL 的 LRU）")
)

const (
	maxEntries = 256              // ✅ 修复：条目数上限
	maxBytes   = 32 << 20         // ✅ 修复：总字节上限 32MB
	ttl        = 30 * time.Second // ✅ 修复：条目存活时间
	sweepEvery = 5 * time.Second  // ✅ 修复：过期清理周期

	putMaxN  = 2000 // 单次 put 最多写多少条
	putMaxKB = 4096 // 单条最大多少 KB
)

// store 是两种缓存实现的公共接口。
type store interface {
	// Put 写入一条数据，返回本次被淘汰的条目数。
	Put(key string, val []byte) int
	// Get 读取一条数据（fix 模式会顺便检查 TTL 并刷新 LRU 位置）。
	Get(key string) ([]byte, bool)
	// Entries 当前条目数。
	Entries() int
	// Capacity 条目上限，-1 表示无上限。
	Capacity() int
	// Bytes 当前持有的大致字节数，-1 表示不统计。
	Bytes() int64
	// Evictions 累计淘汰条目数。
	Evictions() int64
	// Close 释放引用并收敛后台 goroutine。
	Close()
}

// ────────────────────────────── bug 实现 ──────────────────────────────

// mapStore ⚠️ 问题点：全局 map 直接持有 []byte，永不淘汰、没有 TTL、没有容量上限。
type mapStore struct {
	mu sync.RWMutex
	m  map[string][]byte
}

func newMapStore() *mapStore { return &mapStore{m: make(map[string][]byte)} }

// Put ⚠️ 问题点：只增不减。业务上"早就该过期"的数据依然被 map 强引用。
func (s *mapStore) Put(key string, val []byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = val
	return 0
}

// Get 读一条。
func (s *mapStore) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	return v, ok
}

// Entries 当前条目数。
func (s *mapStore) Entries() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}

// Capacity 无上限。
func (s *mapStore) Capacity() int { return -1 }

// Bytes bug 模式不统计占用（这也正是问题：没人知道它涨到多大了）。
func (s *mapStore) Bytes() int64 { return -1 }

// Evictions bug 模式永不淘汰。
func (s *mapStore) Evictions() int64 { return 0 }

// Close 清掉引用。可以借此对比"解除引用之后 heap 才会回落"。
func (s *mapStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = make(map[string][]byte)
}

// ────────────────────────────── fix 实现 ──────────────────────────────

// lruEntry 是 LRU 链表的节点。
type lruEntry struct {
	key      string
	val      []byte
	expireAt time.Time
}

// lruOptions 让容量 / TTL / 清理周期可配（单测用小值，跑起来更快）。
type lruOptions struct {
	maxEntries int
	maxBytes   int64
	ttl        time.Duration
	sweep      time.Duration
}

// lruStore ✅ 修复：container/list 做 LRU 链表 + map 做索引 + 容量上限 + TTL。
type lruStore struct {
	opt       lruOptions
	mu        sync.Mutex
	ll        *list.List // 队首是最新，队尾是最久未使用
	items     map[string]*list.Element
	bytes     int64
	evictions int64

	done chan struct{}
	wg   sync.WaitGroup
}

// newLRUStore 创建 LRU 缓存，0 值字段用默认常量补齐。
func newLRUStore(opt lruOptions) *lruStore {
	if opt.maxEntries <= 0 {
		opt.maxEntries = maxEntries
	}
	if opt.maxBytes <= 0 {
		opt.maxBytes = maxBytes
	}
	if opt.ttl <= 0 {
		opt.ttl = ttl
	}
	if opt.sweep <= 0 {
		opt.sweep = sweepEvery
	}
	s := &lruStore{
		opt:   opt,
		ll:    list.New(),
		items: make(map[string]*list.Element),
		done:  make(chan struct{}),
	}
	// ✅ 修复：后台定期清理过期条目，并且这个 goroutine 是可收敛的（对照 L03）
	s.wg.Add(1)
	go s.janitor()
	return s
}

// Put ✅ 修复：写入后立即按容量上限淘汰，保证占用有上界。
func (s *lruStore) Put(key string, val []byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if e, ok := s.items[key]; ok {
		ent := e.Value.(*lruEntry)
		s.bytes += int64(len(val)) - int64(len(ent.val))
		ent.val = val
		ent.expireAt = now.Add(s.opt.ttl)
		s.ll.MoveToFront(e)
	} else {
		ent := &lruEntry{key: key, val: val, expireAt: now.Add(s.opt.ttl)}
		s.items[key] = s.ll.PushFront(ent)
		s.bytes += int64(len(val))
	}

	// ✅ 修复 1：容量上限。超了就从尾部（最久未使用）开始淘汰。
	evicted := 0
	for s.ll.Len() > s.opt.maxEntries || s.bytes > s.opt.maxBytes {
		if !s.evictLocked(s.ll.Back()) {
			break // 链表已空，说明单条就超过预算，直接放弃
		}
		evicted++
	}
	return evicted
}

// Get ✅ 修复：读路径也检查 TTL，避免返回"已过期但还没被扫到"的数据。
func (s *lruStore) Get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.items[key]
	if !ok {
		return nil, false
	}
	ent := e.Value.(*lruEntry)
	if time.Now().After(ent.expireAt) {
		s.evictLocked(e)
		return nil, false
	}
	s.ll.MoveToFront(e)
	return ent.val, true
}

// evictLocked ✅ 修复 2：显式释放。Go 没有 free，"释放"就是解除所有引用：
// 从 map / 链表里摘掉，并把切片置 nil，让 GC 下一轮就能回收。
func (s *lruStore) evictLocked(e *list.Element) bool {
	if e == nil {
		return false
	}
	ent := e.Value.(*lruEntry)
	s.ll.Remove(e)
	delete(s.items, ent.key)
	s.bytes -= int64(len(ent.val))
	ent.val = nil
	s.evictions++
	return true
}

// janitor ✅ 修复 3：后台定期捞出过期条目；Close 时能干净退出。
func (s *lruStore) janitor() {
	defer s.wg.Done()
	t := time.NewTicker(s.opt.sweep)
	defer t.Stop() // ✅ Ticker 必须 Stop
	for {
		select {
		case <-s.done:
			return
		case now := <-t.C:
			s.evictExpired(now)
		}
	}
}

// evictExpired 从尾部往前扫，淘汰已过期条目。
func (s *lruStore) evictExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for e := s.ll.Back(); e != nil; {
		prev := e.Prev()
		if now.After(e.Value.(*lruEntry).expireAt) {
			s.evictLocked(e)
			n++
		}
		e = prev
	}
	return n
}

// Entries 当前条目数。
func (s *lruStore) Entries() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ll.Len()
}

// Capacity 条目上限。
func (s *lruStore) Capacity() int { return s.opt.maxEntries }

// Bytes 当前持有字节数。
func (s *lruStore) Bytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bytes
}

// Evictions 累计淘汰条目数。
func (s *lruStore) Evictions() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.evictions
}

// Close ✅ 修复 4：停后台 goroutine + 清空引用，heap 立刻可以回落。
func (s *lruStore) Close() {
	close(s.done)
	s.wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()
	for e := s.ll.Front(); e != nil; e = e.Next() {
		e.Value.(*lruEntry).val = nil
	}
	s.ll.Init()
	s.items = make(map[string]*list.Element)
	s.bytes = 0
}

// ────────────────────────────── HTTP ──────────────────────────────

// fillPattern 真正写满整块内存（不写满的话操作系统可能不会真正分配物理页）。
func fillPattern(b []byte, seed byte) {
	for i := range b {
		b[i] = seed + byte(i)
	}
}

// newMux 构造业务路由（pprof 在 19086，不在这里）。
func newMux(fix bool, st store, seq *atomic.Int64) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/cache/put", func(w http.ResponseWriter, r *http.Request) {
		kb := labkit.QueryInt(r, "kb", 64, 1, putMaxKB)
		n := labkit.QueryInt(r, "n", 1, 1, putMaxN)
		fixedKey := r.URL.Query().Get("key")

		var written, wantBytes, evicted int64
		for i := 0; i < n; i++ {
			key := fixedKey
			if key == "" {
				// 自动 key：每次都是新 key，正好用来暴露"永不淘汰"的问题
				key = "blob-" + strconv.FormatInt(seq.Add(1), 10)
			}
			// 每条都单独分配：共用同一块内存的话就看不出占用了
			val := make([]byte, kb<<10)
			fillPattern(val, byte(i))
			wantBytes += int64(len(val))
			evicted += int64(st.Put(key, val))
			written++
		}

		labkit.WriteJSON(w, map[string]any{
			"mode":          labkit.ModeName(fix),
			"kb_per_entry":  kb,
			"entries":       written,
			"written_mb":    float64(wantBytes) / (1 << 20),
			"evicted":       evicted,
			"cache_entries": st.Entries(),
			"cache_mb":      mb(st.Bytes()),
			"capacity":      st.Capacity(),
		})
	})

	mux.HandleFunc("/api/cache/stats", func(w http.ResponseWriter, r *http.Request) {
		var ms runtime.MemStats
		// 注意：ReadMemStats 会短暂 STW，生产环境别放在业务热路径上，只放管理/观测接口。
		runtime.ReadMemStats(&ms)
		labkit.WriteJSON(w, map[string]any{
			"mode":          labkit.ModeName(fix),
			"entries":       st.Entries(),
			"capacity":      st.Capacity(),
			"unbounded":     st.Capacity() < 0,
			"cache_mb":      mb(st.Bytes()),
			"evictions":     st.Evictions(),
			"heap_inuse_mb": ms.HeapInuse >> 20,
			"heap_alloc_mb": ms.HeapAlloc >> 20,
			"heap_objects":  ms.HeapObjects,
			"num_gc":        ms.NumGC,
			"goroutines":    runtime.NumGoroutine(),
		})
	})

	return mux
}

// mb 把字节数换算成 MB（-1 表示不统计，原样返回）。
func mb(b int64) float64 {
	if b < 0 {
		return -1
	}
	return float64(b) / (1 << 20)
}

func main() {
	flag.Parse()

	var st store
	if *fixFlag {
		st = newLRUStore(lruOptions{})
	} else {
		st = newMapStore()
	}
	seq := &atomic.Int64{}

	labkit.Run(labkit.Config{
		BizAddr:   *addrFlag,
		PprofAddr: *pprofAddrFlag,
		Fix:       *fixFlag,
		Handler:   newMux(*fixFlag, st, seq),
		BizNote:   "GET /api/cache/put?kb=256&n=100 | GET /api/cache/stats",
		Problem:   "全局 map[string][]byte 永不淘汰、没有 TTL，heap_inuse 只涨不降",
		FixNote:   "容量上限 + TTL 的 LRU（container/list），超限即淘汰并解除引用，占用有上界",
		OnShutdown: func(_ context.Context) {
			st.Close()
		},
	})
}
