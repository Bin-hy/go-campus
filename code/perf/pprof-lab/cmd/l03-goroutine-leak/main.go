// L03 · goroutine 只增不减：Ticker 未停 + 没有退出信号
//
// 业务场景：每次调用 /api/task/start 都启动一个"后台任务"（例如转码任务的心跳/进度上报）。
//
// bug 模式每次 start 泄漏 3 个 goroutine：
//  1. time.Ticker 驱动的 goroutine，for range 永不 break，Ticker 也从不 Stop；
//  2. 阻塞在永远不会被 close 的 channel 上；
//  3. 阻塞在永远不会有人写入的 channel 上。
//
// fix 模式所有后台 goroutine 都挂在 context 上、用 WaitGroup 记账，
// /api/task/stop 会 cancel + Wait，可以当场看到 goroutine 数回落。
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"gocampus/perf/pprof-lab/internal/labkit"
)

var (
	addrFlag      = flag.String("addr", "127.0.0.1:18083", "业务监听地址")
	pprofAddrFlag = flag.String("pprof-addr", "127.0.0.1:19083", "pprof 管理监听地址（生产只绑内网）")
	fixFlag       = flag.Bool("fix", false, "使用修复实现（context 取消 + WaitGroup 收敛）")
)

const (
	leakPerStart   = 3 // bug 模式每次 start 泄漏的 goroutine 数
	workerPerTask  = 3 // fix 模式每个任务的 goroutine 数（与 bug 对齐，便于对比）
	maxStartPerReq = 100
)

// baselineGoroutines 记录进程启动时的 goroutine 数，作为"泄漏了多少"的参照。
var baselineGoroutines int64

// leakedTotal 累计 bug 模式泄漏的 goroutine 数（估算值，仅供观测）。
var leakedTotal atomic.Int64

// ────────────────────────────── bug 实现 ──────────────────────────────

// leakTask 启动一组永远不会退出的 goroutine。
func leakTask() {
	// ⚠️ 问题点 1：Ticker 从不 Stop，goroutine 也没有退出条件。
	// 这里的代价是双份的：goroutine 泄漏 + runtime timer 泄漏。
	go func() {
		t := time.NewTicker(10 * time.Millisecond)
		for range t.C { // 没有 case <-done，永不退出
		}
	}()

	// ⚠️ 问题点 2：等一个永远不会被 close 的 channel。
	done := make(chan struct{})
	go func() {
		<-done
	}()

	// ⚠️ 问题点 3：等一个永远不会被写入的 channel。
	wait := make(chan struct{}, 1)
	go func() {
		<-wait
	}()

	leakedTotal.Add(leakPerStart)
}

// ────────────────────────────── fix 实现 ──────────────────────────────

// taskManager 是修复实现：每个后台 goroutine 都受 context 控制，并用 WaitGroup 记账。
type taskManager struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu       sync.Mutex // 只保护下面两个计数字段
	started  int64
	workers  atomic.Int64 // 当前存活的后台 goroutine 数
	stopping bool
}

// newTaskManager 创建一个可反复 Start / Stop 的管理器。
func newTaskManager() *taskManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &taskManager{ctx: ctx, cancel: cancel}
}

// Start 启动 n 个任务，每个任务 workerPerTask 个 goroutine。
func (m *taskManager) Start(n int) {
	m.mu.Lock()
	if m.stopping {
		m.mu.Unlock()
		return
	}
	m.started += int64(n)
	m.mu.Unlock()

	// 把 ctx 作为参数传进去，避免 Stop 之后再 Start 时读到被替换的字段（数据竞争）。
	ctx := m.ctx
	for i := 0; i < n; i++ {
		for w := 0; w < workerPerTask; w++ {
			m.wg.Add(1)
			m.workers.Add(1)
			go m.worker(ctx, i, w)
		}
	}
}

// worker 是 ✅ 修复后的后台 goroutine：Ticker 显式 Stop，并时刻监听 ctx.Done()。
func (m *taskManager) worker(ctx context.Context, taskID, workerID int) {
	defer m.wg.Done()
	defer m.workers.Add(-1)

	t := time.NewTicker(10 * time.Millisecond)
	defer t.Stop() // ✅ 修复 1：Ticker 一定要 Stop

	for {
		select {
		case <-ctx.Done():
			// ✅ 修复 2：统一的退出信号，任务收到 cancel 立刻返回
			return
		case <-t.C:
			// 模拟周期性业务：上报进度 / 心跳
			_ = taskID
			_ = workerID
		}
	}
}

// Stop ✅ 修复 3：cancel + WaitGroup.Wait，确保 goroutine 真正收敛后再返回。
// 返回收敛前后的 goroutine 数，便于接口直接展示效果。
func (m *taskManager) Stop() (before, after int) {
	before = runtime.NumGoroutine()

	m.cancel()
	m.wg.Wait()

	// 让这个 manager 可以再次被 Start（生产里更推荐一个任务一个 ctx）
	m.mu.Lock()
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.mu.Unlock()

	return before, runtime.NumGoroutine()
}

// Close 是进程退出时的收敛入口（幂等，可被多次调用）。
func (m *taskManager) Close() {
	m.mu.Lock()
	m.stopping = true
	m.mu.Unlock()
	m.cancel()
	m.wg.Wait()
}

// Active 返回当前存活的后台 goroutine 数。
func (m *taskManager) Active() int64 { return m.workers.Load() }

// ────────────────────────────── HTTP ──────────────────────────────

// newMux 构造业务路由（pprof 在 19083，不在这里）。
func newMux(fix bool, mgr *taskManager) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/task/start", func(w http.ResponseWriter, r *http.Request) {
		n := labkit.QueryInt(r, "n", 1, 1, maxStartPerReq)

		if fix {
			mgr.Start(n)
			labkit.WriteJSON(w, map[string]any{
				"mode":       labkit.ModeName(true),
				"started":    n,
				"goroutines": runtime.NumGoroutine(),
				"active":     mgr.Active(),
				"baseline":   atomic.LoadInt64(&baselineGoroutines),
				"stop_hint":  "GET /api/task/stop 可收敛全部后台 goroutine",
				"note":       "全部 goroutine 都监听 ctx.Done()，Stop 时 cancel + Wait",
			})
			return
		}

		for i := 0; i < n; i++ {
			leakTask()
		}
		// 纯观测辅助：等一下让刚起的 goroutine 真正被调度，便于紧接着查 count
		time.Sleep(10 * time.Millisecond)

		labkit.WriteJSON(w, map[string]any{
			"mode":             labkit.ModeName(false),
			"started":          n,
			"leaked_per_start": leakPerStart,
			"leaked_total":     leakedTotal.Load(),
			"goroutines":       runtime.NumGoroutine(),
			"baseline":         atomic.LoadInt64(&baselineGoroutines),
			"note":             "⚠️ 这些 goroutine 没有任何退出条件，永远不会释放",
		})
	})

	mux.HandleFunc("/api/task/count", func(w http.ResponseWriter, r *http.Request) {
		labkit.WriteJSON(w, map[string]any{
			"mode":           labkit.ModeName(fix),
			"goroutines":     runtime.NumGoroutine(),
			"baseline":       atomic.LoadInt64(&baselineGoroutines),
			"active_workers": mgr.Active(),
			"leaked_total":   leakedTotal.Load(),
			"num_cpu":        runtime.NumCPU(),
		})
	})

	mux.HandleFunc("/api/task/stop", func(w http.ResponseWriter, r *http.Request) {
		if !fix {
			labkit.WriteJSON(w, map[string]any{
				"mode":       labkit.ModeName(false),
				"goroutines": runtime.NumGoroutine(),
				"stopped":    false,
				"note":       "⚠️ bug 模式没有任何退出信号，/api/task/stop 无法收敛；这正是泄漏的代价",
			})
			return
		}
		before, after := mgr.Stop()
		labkit.WriteJSON(w, map[string]any{
			"mode":              labkit.ModeName(true),
			"goroutines_before": before,
			"goroutines_after":  after,
			"released":          before - after,
			"baseline":          atomic.LoadInt64(&baselineGoroutines),
			"note":              "cancel + WaitGroup.Wait 后 goroutine 全部收敛",
		})
	})

	return mux
}

func main() {
	flag.Parse()
	atomic.StoreInt64(&baselineGoroutines, int64(runtime.NumGoroutine()))
	mgr := newTaskManager()

	labkit.Run(labkit.Config{
		BizAddr:   *addrFlag,
		PprofAddr: *pprofAddrFlag,
		Fix:       *fixFlag,
		Handler:   newMux(*fixFlag, mgr),
		BizNote:   "GET /api/task/start | GET /api/task/count | GET /api/task/stop",
		Problem:   "每次 start 泄漏 3 个 goroutine（Ticker 未停 / 阻塞在永不关闭的 channel）",
		FixNote:   "context 取消 + WaitGroup 收敛，/api/task/stop 可当场看到 goroutine 数回落",
		OnShutdown: func(_ context.Context) {
			if *fixFlag {
				before, after := mgr.Stop()
				log.Printf("[exit] 后台任务收敛: goroutines %d -> %d", before, after)
				return
			}
			log.Printf("[exit] bug 模式：%d 个泄漏的 goroutine 无法回收（进程退出才消失）", leakedTotal.Load())
		},
	})
}
