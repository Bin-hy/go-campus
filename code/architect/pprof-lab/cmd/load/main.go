// cmd/load 是本实验包自带的极简压测器（零第三方依赖，不需要 wrk / ab）。
//
// 固定并发 worker 打同一个 URL，输出 QPS、平均延迟、P50 / P95 / P99、
// 成功 / 失败数、状态码与错误分布。
//
// 用法：
//
//	go run ./cmd/load -url='http://127.0.0.1:18081/api/render?n=2000' -c=50 -d=20s
//
// 设计要点（这些细节不注意，压测结果会失真）：
//   - MaxIdleConnsPerHost 默认只有 2：并发 50 时会不停重建 TCP 连接，
//     测出来的"延迟"其实是握手成本，所以这里显式调到并发数；
//   - 每个 worker 自己攒延迟样本，结束后再合并：热路径上不加锁；
//   - -d 到点只停止"投放新请求"，在途请求让它正常跑完，
//     否则收尾时的主动取消会被记成失败，也会打断服务端正在处理的请求。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// usage 打印用法示例。
const usage = `用法示例：
  go run ./cmd/load -url='http://127.0.0.1:18081/api/render?n=2000' -c=50 -d=20s
  go run ./cmd/load -url='http://127.0.0.1:18083/api/task/start' -c=5 -d=5s -timeout=5s
`

// collector 收集成功 / 失败、状态码与错误分布。
type collector struct {
	ok   atomic.Int64
	fail atomic.Int64

	mu     sync.Mutex
	status map[int]int64
	errs   map[string]int64
}

func newCollector() *collector {
	return &collector{status: make(map[int]int64), errs: make(map[string]int64)}
}

// recordOK 记一次成功的请求。
func (c *collector) recordOK(code int) {
	c.ok.Add(1)
	c.mu.Lock()
	c.status[code]++
	c.mu.Unlock()
}

// recordFail 记一次失败的请求。
func (c *collector) recordFail(err error) {
	c.fail.Add(1)
	key := classify(err)
	c.mu.Lock()
	c.errs[key]++
	c.mu.Unlock()
}

// classify 把错误归类，方便看"到底是超时还是连接被拒"。
func classify(err error) string {
	if err == nil {
		return "unknown"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout(deadline)"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timeout(net)"
	}
	msg := err.Error()
	msg = strings.ReplaceAll(msg, "\n", " ")
	// 去掉 http 客户端错误前缀里冗长的 URL（每个错误都带一大段，看不清重点）
	if i := strings.LastIndex(msg, "://"); i >= 0 {
		if j := strings.Index(msg[i:], ": "); j >= 0 {
			msg = msg[:i] + "..." + msg[i+j:]
		}
	}
	if len(msg) > 100 {
		msg = msg[:100] + "..."
	}
	return msg
}

// percentile 计算分位数，p 取 0~100，sorted 必须已升序。
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// row 打印一行对齐的 "标签: 值"。
func row(label, value string) {
	fmt.Printf("  %-12s %s\n", label+":", value)
}

// printKVTable 打印一个对齐的两列表格（比如状态码 / 错误分布）。
func printKVTable(title string, kv map[string]int64) {
	if len(kv) == 0 {
		return
	}
	type pair struct {
		k string
		v int64
	}
	pairs := make([]pair, 0, len(kv))
	for k, v := range kv {
		pairs = append(pairs, pair{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	fmt.Printf("\n%s\n", title)
	for _, p := range pairs {
		fmt.Printf("  %-10s %8d\n", p.k, p.v)
	}
}

// startProgress 每 5 秒打印一次进度，避免长时间压测看起来像卡死。返回停止函数。
func startProgress(col *collector, start time.Time) func() {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				elapsed := time.Since(start)
				total := col.ok.Load() + col.fail.Load()
				fmt.Printf("  [进度] 已跑 %.0fs  请求 %d  实时 QPS %.0f  失败 %d\n",
					elapsed.Seconds(), total, float64(total)/elapsed.Seconds(), col.fail.Load())
			}
		}
	}()
	return func() {
		close(done)
		wg.Wait()
	}
}

func main() {
	targetURL := flag.String("url", "", "压测目标 URL（可带 query）")
	conc := flag.Int("c", 50, "并发 worker 数")
	dur := flag.Duration("d", 20*time.Second, "压测时长")
	timeout := flag.Duration("timeout", 10*time.Second, "单请求超时")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *targetURL == "" {
		fmt.Fprint(os.Stderr, "❌ 必须指定 -url\n\n")
		flag.Usage()
		os.Exit(2)
	}
	if *conc <= 0 {
		*conc = 1
	}
	if *dur <= 0 {
		*dur = 5 * time.Second
	}
	if *timeout <= 0 {
		*timeout = 10 * time.Second
	}

	tr := &http.Transport{
		MaxIdleConns:        *conc * 2,
		MaxIdleConnsPerHost: *conc, // 默认只有 2，压测时必须调大，否则测的是 TCP 握手
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true, // 关掉 Accept-Encoding，避免压缩开销影响对比
	}
	client := &http.Client{Transport: tr, Timeout: *timeout}

	fmt.Printf("\n===== pprof-lab 压测 =====\n")
	row("目标", *targetURL)
	row("并发 / 时长", fmt.Sprintf("%d / %s", *conc, *dur))
	row("单请求超时", timeout.String())
	fmt.Println()

	col := newCollector()
	// ctx 只用于"停止投放新请求"；在途请求交给 client.Timeout 管，让它自然跑完。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deadline := time.Now().Add(*dur)
	latencies := make([][]time.Duration, *conc)
	var wg sync.WaitGroup

	start := time.Now()
	stopProgress := startProgress(col, start)

	for i := 0; i < *conc; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			local := make([]time.Duration, 0, 4096)
			for time.Now().Before(deadline) && ctx.Err() == nil {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, *targetURL, nil)
				if err != nil {
					col.recordFail(err)
					break
				}
				t0 := time.Now()
				resp, err := client.Do(req)
				if err != nil {
					// 压测收尾时的主动取消不算失败
					if ctx.Err() != nil {
						break
					}
					col.recordFail(err)
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				d := time.Since(t0)

				if resp.StatusCode >= 400 {
					col.recordFail(fmt.Errorf("http %d", resp.StatusCode))
					continue
				}
				col.recordOK(resp.StatusCode)
				local = append(local, d)
			}
			latencies[id] = local
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	cancel()
	stopProgress()

	// 合并所有 worker 的样本再排序
	var all []time.Duration
	for _, l := range latencies {
		all = append(all, l...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })

	ok, fail := col.ok.Load(), col.fail.Load()
	total := ok + fail
	qps := float64(total) / elapsed.Seconds()

	fmt.Printf("\n===== 结果 =====\n")
	row("实际耗时", fmt.Sprintf("%.2fs", elapsed.Seconds()))
	row("总请求数", fmt.Sprintf("%d", total))
	row("成功 / 失败", fmt.Sprintf("%d / %d", ok, fail))
	row("QPS", fmt.Sprintf("%.2f", qps))
	row("成功率", fmt.Sprintf("%.2f%%", ratio(ok, total)))

	if len(all) == 0 {
		fmt.Println("\n  ⚠️ 没有任何成功请求：先确认服务是否启动、URL 是否正确、端口是否被占用。")
		printKVTable("错误分布", snapshotErrors(col))
		fmt.Println()
		return
	}

	var sum time.Duration
	for _, d := range all {
		sum += d
	}
	row("平均延迟", avg(sum, len(all)))
	row("P50", percentile(all, 50).String())
	row("P95", percentile(all, 95).String())
	row("P99", percentile(all, 99).String())
	row("最快 / 最慢", all[0].String()+" / "+all[len(all)-1].String())

	printKVTable("状态码分布", snapshotStatus(col))
	printKVTable("错误分布", snapshotErrors(col))

	fmt.Printf("\n提示：把上面这组数字抄进对应样例 README 的「预期对比」表；\n")
	fmt.Printf("     再换 -fix 模式跑一遍同样的命令，就是最直观的对比。\n")
}

// avg 计算平均延迟。
func avg(sum time.Duration, n int) string {
	if n == 0 {
		return "0s"
	}
	return (sum / time.Duration(n)).String()
}

// ratio 计算百分比。
func ratio(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

// snapshotStatus 复制一份状态码统计。
func snapshotStatus(c *collector) map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.status))
	for k, v := range c.status {
		out[fmt.Sprintf("%d", k)] = v
	}
	return out
}

// snapshotErrors 复制一份错误统计。
func snapshotErrors(c *collector) map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.errs))
	for k, v := range c.errs {
		out[k] = v
	}
	return out
}
