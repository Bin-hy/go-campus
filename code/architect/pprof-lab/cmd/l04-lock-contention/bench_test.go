package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

// sinkVal 吃掉返回值，防止编译器优化；用 atomic 写，保证 -race 下也干净。
var sinkVal int64

// benchCounter 是基准测试的公共驱动：构造对应实现后并行打同一个热点 key。
func benchCounter(b *testing.B, fix bool) {
	c := newCounter(fix)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var local int64
		for pb.Next() {
			local = c.Inc("hot")
		}
		atomic.StoreInt64(&sinkVal, local)
	})
}

// BenchmarkCounterBug 测量全局锁实现的并行吞吐。
// 运行：go test ./cmd/l04-lock-contention -bench=Counter -benchmem
func BenchmarkCounterBug(b *testing.B) { benchCounter(b, false) }

// BenchmarkCounterFix 测量分片锁 + atomic 实现的并行吞吐，与 Bug 版使用同一入口。
func BenchmarkCounterFix(b *testing.B) { benchCounter(b, true) }

// TestCounterModesAgree 保证两种实现的计数语义一致。
func TestCounterModesAgree(t *testing.T) {
	bugC, fixC := newCounter(false), newCounter(true)

	keys := []string{"hot", "warm", "hot", "cold", "hot"}
	for _, k := range keys {
		bv := bugC.Inc(k)
		fv := fixC.Inc(k)
		if bv != fv {
			t.Fatalf("key=%s 计数不一致: bug=%d fix=%d", k, bv, fv)
		}
	}
	if bugC.Total() != fixC.Total() {
		t.Fatalf("总数不一致: bug=%d fix=%d", bugC.Total(), fixC.Total())
	}
	if bugC.Keys() != 3 || fixC.Keys() != 3 {
		t.Fatalf("key 数应为 3: bug=%d fix=%d", bugC.Keys(), fixC.Keys())
	}
}

// TestCounterConcurrent 并发压同一个 key，确认两种实现都不会丢计数。
func TestCounterConcurrent(t *testing.T) {
	const (
		workers = 8
		each    = 50
	)
	for _, fix := range []bool{false, true} {
		c := newCounter(fix)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < each; i++ {
					c.Inc("hot")
				}
			}()
		}
		wg.Wait()

		want := int64(workers * each)
		if got := c.Total(); got != want {
			t.Fatalf("mode=%s 计数丢失: got=%d want=%d", labkitMode(fix), got, want)
		}
	}
}

// labkitMode 只是给测试日志用的模式名。
func labkitMode(fix bool) string {
	if fix {
		return "fix"
	}
	return "bug"
}
