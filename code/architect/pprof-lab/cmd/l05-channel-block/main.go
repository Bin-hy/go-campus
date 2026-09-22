// L05 · channel 阻塞：无缓冲 channel + 单 worker 串行
//
// 业务场景：素材处理流水线。请求投递一个 job（5 万个整数的统计），等待结果。
//
// bug 模式用「无缓冲 channel + 只有 1 个 worker」：整条流水线串行，
// 调用方阻塞在 chansend、worker 阻塞在 chanrecv，pprof 的 block profile
// 会把 runtime.chansend / runtime.chanrecv 直接顶到最上面。
//
// fix 模式改成「有缓冲 channel + worker pool」：多个 job 真正并行，
// 并且用 done channel + WaitGroup 保证退出时 worker 能收敛。
package main

import (
	"context"
	"flag"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"gocampus/perf/pprof-lab/internal/labkit"
)

var (
	addrFlag      = flag.String("addr", "127.0.0.1:18085", "业务监听地址")
	pprofAddrFlag = flag.String("pprof-addr", "127.0.0.1:19085", "pprof 管理监听地址（生产只绑内网）")
	fixFlag       = flag.Bool("fix", false, "使用修复实现（有缓冲 channel + worker pool）")
)

const (
	jobElems   = 200000 // 每个 job 处理 20 万个样本值：约几百微秒真实 CPU 工作
	sampleSize = 400000 // 共享样本表大小（只分配一次）
	chanBuf    = 4096   // ✅ 修复：channel 缓冲大小
)

// job 是一次流水线任务。
type job struct {
	id   int
	data []int64
}

// result 是流水线输出。
type result struct {
	id   int
	sum  int64
	max  int64
	hash uint64
}

// sharedSample 是一份共享的只读样本表（模拟启动时加载好的特征 / 素材库）。
// job 只持有它的切片头，所以"提交 job"这一步本身不分配内存——
// 这样压测时看到的开销就都来自流水线调度与计算本身，对比才干净。
var sharedSample = func() []int64 {
	s := make([]int64, sampleSize)
	for i := range s {
		s[i] = int64((i*7)%1000) + 1
	}
	return s
}()

// buildJob 从共享样本表里取一个滑动窗口（不同 job 的数据不同）。
func buildJob(id int) job {
	off := (id * 997) % (sampleSize - jobElems)
	return job{id: id, data: sharedSample[off : off+jobElems]}
}

// compute 是流水线里的真实计算：滚动哈希（FNV-1a）+ 求和 + 求最大值。
func compute(j job) result {
	var sum, max int64
	h := uint64(14695981039346656037)
	for _, v := range j.data {
		h = (h ^ uint64(v)) * 1099511628211
		sum += v
		if v > max {
			max = v
		}
	}
	return result{id: j.id, sum: sum, max: max, hash: h}
}

// pipeline 是两种实现的公共接口。
type pipeline interface {
	// Submit 投递一个 job 并等待结果；ctx 取消时返回错误。
	Submit(ctx context.Context, j job) (result, error)
	// Close 停止流水线并收敛 worker。
	Close()
}

// ────────────────────────────── bug 实现 ──────────────────────────────

// serialPipeline ⚠️ 问题点：无缓冲 channel + 单 worker，整条流水线串行。
type serialPipeline struct {
	in  chan job
	out chan result
}

func newSerialPipeline() *serialPipeline {
	p := &serialPipeline{
		in:  make(chan job),    // ⚠️ 无缓冲：投递必须等 worker 空出手来
		out: make(chan result), // ⚠️ 无缓冲：worker 必须等调用方来取
	}
	go p.worker()
	return p
}

// worker ⚠️ 问题点：整条流水线只有 1 个 goroutine，所有 job 串行执行；
// 而且它阻塞在无缓冲的 p.out 上，调用方一慢，整个流水线就停摆。
//
// 额外代价：如果调用方先超时退出（客户端断开 / 上游超时），worker 会永久阻塞在
// `p.out <- ...` 上——既泄漏 goroutine，也让流水线彻底失去处理能力。
func (p *serialPipeline) worker() {
	for j := range p.in {
		p.out <- compute(j)
	}
}

// Submit ⚠️ 问题点：两次无缓冲 channel 往返，一次请求要经历两次阻塞。
func (p *serialPipeline) Submit(ctx context.Context, j job) (result, error) {
	select {
	case p.in <- j:
	case <-ctx.Done():
		return result{}, ctx.Err()
	}
	select {
	case r := <-p.out:
		return r, nil
	case <-ctx.Done():
		return result{}, ctx.Err()
	}
}

// Close 关掉入口 channel 让 worker 的 range 结束。
// 注意：必须在确认没有在途 Submit 之后调用，否则会 panic: send on closed channel。
func (p *serialPipeline) Close() { close(p.in) }

// ────────────────────────────── fix 实现 ──────────────────────────────

// poolPipeline ✅ 修复：有缓冲 channel + worker pool。
type poolPipeline struct {
	in   chan job
	out  chan result
	done chan struct{}
	wg   sync.WaitGroup
}

func newPoolPipeline(workers int) *poolPipeline {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	p := &poolPipeline{
		in:   make(chan job, chanBuf),    // ✅ 缓冲：短时间内可以连续投递，不必等 worker
		out:  make(chan result, chanBuf), // ✅ 缓冲：worker 不会因为调用方慢而卡住
		done: make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

// worker ✅ 修复：多个 worker 并行消费同一个 channel，天然负载均衡。
func (p *poolPipeline) worker() {
	defer p.wg.Done()
	for j := range p.in {
		r := compute(j)
		select {
		case p.out <- r:
		case <-p.done:
			// ✅ 修复：退出信号，worker 不会被缓冲写阻塞住
			return
		}
	}
}

// Submit ✅ 修复：channel 有缓冲，大多数情况下投递与接收都不阻塞。
func (p *poolPipeline) Submit(ctx context.Context, j job) (result, error) {
	select {
	case p.in <- j:
	case <-ctx.Done():
		return result{}, ctx.Err()
	}
	select {
	case r := <-p.out:
		return r, nil
	case <-ctx.Done():
		return result{}, ctx.Err()
	}
}

// Close ✅ 修复：close(in) 让空闲 worker 的 range 结束，close(done) 让正在写结果的
// worker 立刻返回，最后 WaitGroup.Wait 确认全部收敛。
// 同样必须在没有在途 Submit 时调用。
func (p *poolPipeline) Close() {
	close(p.done)
	close(p.in)
	p.wg.Wait()
}

// ────────────────────────────── HTTP ──────────────────────────────

// newMux 构造业务路由（pprof 在 19085，不在这里）。
func newMux(fix bool, p pipeline, seq *atomic.Int64) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/pipeline", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// 用请求自带的 ctx：客户端断开时能及时取消，避免白干活
		res, err := p.Submit(r.Context(), buildJob(int(seq.Add(1))))
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		labkit.WriteJSON(w, map[string]any{
			"mode":       labkit.ModeName(fix),
			"job":        res.id,
			"sum":        res.sum,
			"max":        res.max,
			"hash":       res.hash,
			"elems":      jobElems,
			"elapsed_ms": float64(time.Since(start).Microseconds()) / 1000,
		})
	})
	return mux
}

func main() {
	flag.Parse()

	seq := &atomic.Int64{}
	var p pipeline
	if *fixFlag {
		p = newPoolPipeline(runtime.NumCPU())
	} else {
		p = newSerialPipeline()
	}

	labkit.Run(labkit.Config{
		BizAddr:   *addrFlag,
		PprofAddr: *pprofAddrFlag,
		Fix:       *fixFlag,
		Handler:   newMux(*fixFlag, p, seq),
		BizNote:   "GET /api/pipeline",
		Problem:   "无缓冲 channel + 单 worker，投递与取结果各阻塞一次，整条流水线串行",
		FixNote:   "有缓冲 channel + worker pool，多 job 真并行，退出时 done + WaitGroup 收敛",
		OnShutdown: func(_ context.Context) {
			p.Close()
		},
	})
}
