// L02 · 每请求 MB 级分配：GC CPU 高、P99 抖
//
// 业务场景：缩略图生成。一张 4MB 的原图 → 2MB 中间结果 → 1MB 输出缩略图。
//
// bug 模式每次请求都 `make([]byte, 4MB/2MB/1MB)`，请求一多就变成
// 「分配 → 变垃圾 → GC → 再分配」的循环：CPU 大量花在 mallocgc / gcBgMarkWorker 上，
// P99 因为 GC assist 与 STW 抖动。
// fix 模式用 sync.Pool 复用这三块工作内存，稳态下分配量接近 0。
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"gocampus/architect/pprof-lab/internal/labkit"
)

var (
	addrFlag      = flag.String("addr", "127.0.0.1:18082", "业务监听地址")
	pprofAddrFlag = flag.String("pprof-addr", "127.0.0.1:19082", "pprof 管理监听地址（生产只绑内网）")
	fixFlag       = flag.Bool("fix", false, "使用修复实现（sync.Pool 复用缓冲区）")
)

const (
	rawBytes = 4 << 20 // 原图 4MB
	midBytes = 2 << 20 // 中间结果 2MB
	outBytes = 1 << 20 // 输出缩略图 1MB

	ringSize = 64      // 最近结果保留条数
	ringKeep = 8 << 10 // 每条保留的头部字节数（约 512KB 常驻）
)

// scratch 是修复模式复用的一组工作内存。
type scratch struct {
	raw []byte
	mid []byte
	out []byte
}

// scratchPool ⚠️ 注意：sync.Pool 里的对象可能在 GC 时被清掉，所以它只是
// "显著降低分配"而不是"永不分配"——这正是压测时 fix 模式仍会看到少量 alloc 的原因。
var scratchPool = sync.Pool{
	New: func() any {
		return &scratch{
			raw: make([]byte, rawBytes),
			mid: make([]byte, midBytes),
			out: make([]byte, outBytes),
		}
	},
}

// sampleRing 保留最近若干次结果的头部，模拟"最近缩略图"这类业务缓存。
// 它是有意保留的少量内存（约 512KB），不是问题本身。
var sampleRing = struct {
	mu   sync.Mutex
	bufs [ringSize][]byte
	idx  int
}{}

// pattern 是解码用的 4KB 数据块（真实解码里大量工作就是这种块拷贝）。
var pattern = func() []byte {
	p := make([]byte, 4<<10)
	for i := range p {
		p[i] = byte(i)
	}
	return p
}()

// transform 是缩略图生成的主体工作：解码（铺块）→ 缩放（取样）→ 编码（拷贝）。
//
// 真实图像处理这几步以内存拷贝为主，所以这里用 copy 而不是逐字节算术：
// 这样"分配"才会成为请求里最显著的成本——这正是 L02 要观察的东西。
// 这部分计算 bug / fix 完全相同，保证对比的是「分配」而不是「少干活」。
func transform(id int, raw, mid, out []byte) {
	// 解码：用 4KB 模式块铺满原图
	for off := 0; off < len(raw); off += len(pattern) {
		copy(raw[off:], pattern)
	}
	// 缩放：隔行取样
	half := len(mid) / 2
	copy(mid[:half], raw[:half])
	// 编码：取中间区域作为输出，覆盖 out 的每一段
	q := len(out) / 2
	copy(out[:q], mid[half-q:half])
	copy(out[q:], mid[half:half+q])
	// 让结果随 id 变化（便于比对两种实现的输出是否一致）
	seed := byte(id)
	out[0] = seed
	out[len(out)-1] = seed ^ 0xff
}

// checksum 抽样计算校验和（每 64 字节取 1 个），成本可忽略，
// 用来在两个模式之间比对结果是否一致。
func checksum(b []byte) uint32 {
	const prime = 16777619
	var h uint32 = 2166136261
	for i := 0; i < len(b); i += 64 {
		h = (h ^ uint32(b[i])) * prime
	}
	return h
}

// retainSample 把结果头部复制进环形缓冲，模拟"最近缩略图"缓存。
func retainSample(out []byte) {
	sampleRing.mu.Lock()
	defer sampleRing.mu.Unlock()
	b := sampleRing.bufs[sampleRing.idx]
	if b == nil {
		b = make([]byte, ringKeep)
		sampleRing.bufs[sampleRing.idx] = b
	}
	copy(b, out[:ringKeep])
	sampleRing.idx = (sampleRing.idx + 1) % ringSize
}

// renderThumb 是 bug / fix 共用的核心实现：生成缩略图字节。
//   - bug 模式：返回本次新分配的 out，pooled 为 nil；
//   - fix 模式：返回池内缓冲 out，并且 pooled 非 nil，**调用方用完必须归还**。
//
// handler 和 benchmark 都调用它，用 fix 参数切换实现。
func renderThumb(id int, fix bool) (out []byte, sum uint32, pooled *scratch) {
	if fix {
		s := scratchPool.Get().(*scratch)
		transform(id, s.raw, s.mid, s.out)
		retainSample(s.out)
		return s.out, checksum(s.out), s
	}

	// ⚠️ 问题点 1：每请求 4MB + 2MB + 1MB = 7MB 全新分配，用完立刻变垃圾。
	// 压测时这段就是 heap profile 里 runtime.mallocgc 的绝对大头。
	raw := make([]byte, rawBytes)
	mid := make([]byte, midBytes)
	outBuf := make([]byte, outBytes)
	transform(id, raw, mid, outBuf)
	retainSample(outBuf)
	return outBuf, checksum(outBuf), nil
}

// newMux 构造业务路由（pprof 在 19082，不在这里）。
func newMux(fix bool) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/thumb", func(w http.ResponseWriter, r *http.Request) {
		id := labkit.QueryInt(r, "id", 1, 0, 1<<20)
		start := time.Now()

		out, sum, pooled := renderThumb(id, fix)
		elapsed := time.Since(start)
		if pooled != nil {
			// ✅ 修复 2：响应体已经写出去（同步完成）后立刻归还，下一个请求直接复用
			defer scratchPool.Put(pooled)
		}

		// 注意：所有 header 必须在写 body 之前设置，否则不会生效。
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("X-Mode", labkit.ModeName(fix))
		w.Header().Set("X-Bytes", strconv.Itoa(len(out)))
		w.Header().Set("X-Checksum", strconv.FormatUint(uint64(sum), 10))
		w.Header().Set("X-Elapsed-Us", strconv.FormatInt(elapsed.Microseconds(), 10))
		if _, err := w.Write(out); err != nil {
			log.Printf("[thumb] id=%d 写入失败: %v", id, err)
		}
	})
	return mux
}

// startGCStatsLogger 每 every 秒打印一次堆与 GC 指标，方便肉眼看到 bug 模式的 GC 抖动。
// 返回 stop 函数：显式停 Ticker + 等 goroutine 退出（正确的收敛写法，可对照 L03）。
func startGCStatsLogger(every time.Duration) func() {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(every)
		defer t.Stop()
		var ms runtime.MemStats
		for {
			select {
			case <-done:
				return
			case <-t.C:
				runtime.ReadMemStats(&ms)
				log.Printf("[stats] heap_inuse=%dMB heap_objects=%d gc_cycles=%d gc_cpu=%.1f%% goroutines=%d",
					ms.HeapInuse>>20, ms.HeapObjects, ms.NumGC, ms.GCCPUFraction*100, runtime.NumGoroutine())
			}
		}
	}()
	return func() {
		close(done)
		wg.Wait()
	}
}

func main() {
	flag.Parse()
	stopStats := startGCStatsLogger(3 * time.Second)
	labkit.Run(labkit.Config{
		BizAddr:   *addrFlag,
		PprofAddr: *pprofAddrFlag,
		Fix:       *fixFlag,
		Handler:   newMux(*fixFlag),
		BizNote:   "GET /api/thumb?id=1（返回 1MB 二进制，响应头带 X-Elapsed-Us）",
		Problem:   "每请求分配 4MB+2MB+1MB 临时缓冲，GC CPU 高、P99 抖动",
		FixNote:   "sync.Pool 复用工作缓冲，稳态分配接近 0，GC 周期显著变少",
		OnShutdown: func(_ context.Context) {
			stopStats()
		},
	})
}
