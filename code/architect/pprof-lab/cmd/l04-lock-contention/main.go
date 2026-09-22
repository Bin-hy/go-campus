// L04 · 全局锁竞争：一把 mutex 保护一切，临界区还很长
//
// 业务场景：一个"热度计数器"，每个请求给某个 key（比如视频 ID）加一，
// 顺便更新统计信息、写一笔下游 IO。
//
// bug 模式用一把全局 mutex，并且把「统计计算」和「下游 IO 等待」都放在临界区里：
// 所有并发请求被强制串行，QPS 上限 = 1 / 临界区耗时。pprof 的 mutex profile
// 会直接给出 contentions 和 delay。
//
// fix 模式按 key 分片（64 个分片锁）+ atomic 计数，临界区缩到只剩 map 的一次读或建，
// 计算与 IO 全部移到锁外。
package main

import (
	"flag"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"gocampus/perf/pprof-lab/internal/labkit"
)

var (
	addrFlag      = flag.String("addr", "127.0.0.1:18084", "业务监听地址")
	pprofAddrFlag = flag.String("pprof-addr", "127.0.0.1:19084", "pprof 管理监听地址（生产只绑内网）")
	fixFlag       = flag.Bool("fix", false, "使用修复实现（分片锁 + atomic）")
)

const (
	workIters  = 200                   // 临界区里的真实计算量
	ioWait     = 30 * time.Microsecond // 模拟临界区里的下游 IO 等待（写 Redis / Kafka / 日志）
	shardCount = 64                    // ✅ 修复：分片数，取 2 的幂便于用位运算取模
)

// counter 是两种实现的公共接口，handler 与 benchmark 共用。
type counter interface {
	// Inc 把 key 的计数加一，返回该 key 的最新值。
	Inc(k string) int64
	// Total 返回所有 key 的计数总和。
	Total() int64
	// Keys 返回 key 的数量。
	Keys() int
}

// newCounter 按开关构造 bug / fix 实现。
func newCounter(fix bool) counter {
	if fix {
		return newFixCounter()
	}
	return newBugCounter()
}

// ────────────────────────────── bug 实现 ──────────────────────────────

// bugCounter ⚠️ 问题点：一把全局 mutex 保护整个计数器。
type bugCounter struct {
	mu  sync.Mutex
	m   map[string]int64
	seq int64 // 顺便维护的滚动校验和
}

func newBugCounter() *bugCounter {
	return &bugCounter{m: make(map[string]int64)}
}

// Inc ⚠️ 问题点：整个函数都在锁里，包括计算和 IO 等待。
func (c *bugCounter) Inc(k string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	// ⚠️ 问题点 1：把"顺手更新统计"的计算放进了临界区，锁被无谓地拉长。
	for i := 0; i < workIters; i++ {
		c.seq = c.seq*6364136223846793005 + 1442695040888963407
	}

	// ⚠️ 问题点 2：临界区里做 IO 等待（真实项目里很常见：锁内写 Redis / Kafka / 打同步日志）。
	// 所有并发请求被迫在这里排队，QPS 被 1/临界区耗时 卡死。
	time.Sleep(ioWait)

	c.m[k]++
	return c.m[k]
}

// Total 统计总和（读路径同样要抢同一把锁）。
func (c *bugCounter) Total() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	var sum int64
	for _, v := range c.m {
		sum += v
	}
	return sum
}

// Keys 返回 key 数量。
func (c *bugCounter) Keys() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// ────────────────────────────── fix 实现 ──────────────────────────────

// shard 是分片：每个分片有自己的锁和自己的 map，不同分片互不阻塞。
type shard struct {
	mu sync.Mutex
	m  map[string]*atomic.Int64
}

// fixCounter ✅ 修复：分片锁 + atomic 计数。
type fixCounter struct {
	shards [shardCount]shard
	total  atomic.Int64
	seq    atomic.Int64
}

func newFixCounter() *fixCounter {
	c := &fixCounter{}
	for i := range c.shards {
		c.shards[i].m = make(map[string]*atomic.Int64)
	}
	return c
}

// shardIndex 手写 FNV-1a 取模，零分配（用 hash/fnv 也可以，但手写更省一次接口调用）。
func shardIndex(k string) uint32 {
	const prime = 16777619
	var h uint32 = 2166136261
	for i := 0; i < len(k); i++ {
		h = (h ^ uint32(k[i])) * prime
	}
	return h % shardCount
}

// Inc ✅ 修复：临界区只剩 map 的一次读或建；计数走 atomic；计算与 IO 移到锁外。
func (c *fixCounter) Inc(k string) int64 {
	s := &c.shards[shardIndex(k)]

	// ✅ 修复 1：临界区缩到最小，并且按 key 分片，热点 key 只影响 1/64 的请求
	s.mu.Lock()
	v, ok := s.m[k]
	if !ok {
		v = new(atomic.Int64)
		s.m[k] = v
	}
	s.mu.Unlock()

	// ✅ 修复 2：自增本身用 atomic，不需要任何锁
	n := v.Add(1)
	c.total.Add(1)

	// ✅ 修复 3：原来塞在锁里的计算 / IO 等待全部搬到锁外，不再串行化所有请求
	var seq int64
	for i := 0; i < workIters; i++ {
		seq = seq*6364136223846793005 + 1442695040888963407
	}
	c.seq.Add(seq & 1)
	time.Sleep(ioWait)

	return n
}

// Total ✅ 修复：读路径不抢写路径的锁（这里遍历分片时每片短暂加锁）。
func (c *fixCounter) Total() int64 { return c.total.Load() }

// Keys 返回 key 数量。
func (c *fixCounter) Keys() int {
	n := 0
	for i := range c.shards {
		c.shards[i].mu.Lock()
		n += len(c.shards[i].m)
		c.shards[i].mu.Unlock()
	}
	return n
}

// ────────────────────────────── HTTP ──────────────────────────────

// newMux 构造业务路由（pprof 在 19084，不在这里）。
func newMux(fix bool, c counter) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/counter", func(w http.ResponseWriter, r *http.Request) {
		k := r.URL.Query().Get("k")
		if k == "" {
			k = "default"
		}
		start := time.Now()
		v := c.Inc(k)
		labkit.WriteJSON(w, map[string]any{
			"mode":       labkit.ModeName(fix),
			"key":        k,
			"value":      v,
			"total":      c.Total(),
			"keys":       c.Keys(),
			"shards":     shardCount,
			"elapsed_ms": float64(time.Since(start).Microseconds()) / 1000,
		})
	})
	return mux
}

func main() {
	flag.Parse()
	c := newCounter(*fixFlag)
	labkit.Run(labkit.Config{
		BizAddr:   *addrFlag,
		PprofAddr: *pprofAddrFlag,
		Fix:       *fixFlag,
		Handler:   newMux(*fixFlag, c),
		BizNote:   "GET /api/counter?k=hot",
		Problem:   "全局 mutex 且临界区里做计算 + IO 等待，所有请求被串行化",
		FixNote:   "64 个分片锁 + atomic 自增，临界区缩到最小，计算与 IO 移到锁外",
	})
}
