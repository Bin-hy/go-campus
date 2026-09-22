// L07 · 逐个格式化 + 未预分配：导出接口 CPU 与分配双高
//
// 业务场景：剪辑任务的 CSV 导出。
//
// bug 模式逐行 fmt.Fprintf 直接写响应体：每一行都走一次反射格式化、分配临时字符串，
// 又是一次独立的小写入，行数一多 CPU 和分配全部爆掉。
//
// fix 模式用 bufio.Writer + 预分配 []byte + strconv.AppendXxx 手写编码，
// 并攒够 32KB 才批量写一次，整个导出过程几乎只在开头分配一次。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"gocampus/architect/pprof-lab/internal/labkit"
)

var (
	addrFlag      = flag.String("addr", "127.0.0.1:18087", "业务监听地址")
	pprofAddrFlag = flag.String("pprof-addr", "127.0.0.1:19087", "pprof 管理监听地址（生产只绑内网）")
	fixFlag       = flag.Bool("fix", false, "使用修复实现（bufio + 预分配 + 批量写）")
)

const (
	defaultRows = 20000 // /api/export 默认导出行数
	maxRows     = 200000
	flushBytes  = 32 << 10 // 攒够 32KB 显式批量写一次
	bufSize     = 64 << 10 // bufio 缓冲大小
)

// record 是一条导出记录（模拟剪辑任务的操作日志）。
type record struct {
	ID       int
	User     string
	Action   string
	Duration int
	Score    int
	TS       int64
}

var actions = []string{"import", "cut", "export", "render", "publish"}

// users 预先造好用户名表：造数据本身不应该分配内存，
// 否则会把"格式化"的开销和"造数据"的开销混在一起，对比就不干净了。
var users = func() []string {
	out := make([]string, 5000)
	for i := range out {
		out[i] = "u" + strconv.Itoa(i)
	}
	return out
}()

// buildRecord 生成第 i 条记录，bug / fix 共用（保证写出内容一致，且不产生分配）。
func buildRecord(i int) record {
	return record{
		ID:       i,
		User:     users[i%len(users)],
		Action:   actions[i%len(actions)],
		Duration: 200 + i%3000,
		Score:    i % 100,
		TS:       1735689600 + int64(i),
	}
}

// exportBug ⚠️ 问题点：逐行 fmt.Fprintf，没有缓冲、没有预分配。
func exportBug(w io.Writer, n int) (int, error) {
	total := 0
	for i := 0; i < n; i++ {
		r := buildRecord(i)
		// ⚠️ 问题点 1：fmt.Fprintf 走反射格式化——解析格式串、把参数装箱成 []any、
		// 再分配一个临时字符串，每行一次。
		// ⚠️ 问题点 2：每行都是一次独立的写，没有缓冲层把成百上千次小写合并成少数几次大写入。
		m, err := fmt.Fprintf(w, "%d,%s,%s,%d,%d,%d\n",
			r.ID, r.User, r.Action, r.Duration, r.Score, r.TS)
		total += m
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// exportFix ✅ 修复：bufio + 预分配 + 批量写 + 手写数字编码。
func exportFix(w io.Writer, n int) (int, error) {
	// ✅ 修复 1：显式缓冲层（64KB），把成千上万次小写变成少数几次大写入
	bw := bufio.NewWriterSize(w, bufSize)
	// ✅ 修复 2：预分配。整个导出过程只在开头分配这两块内存，之后全部复用。
	line := make([]byte, 0, 64)
	batch := make([]byte, 0, flushBytes)
	total := 0

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		m, err := bw.Write(batch)
		total += m
		batch = batch[:0]
		return err
	}

	for i := 0; i < n; i++ {
		r := buildRecord(i)

		// ✅ 修复 3：strconv.AppendXxx 直接追加进 []byte，零反射、零临时字符串
		line = line[:0]
		line = strconv.AppendInt(line, int64(r.ID), 10)
		line = append(line, ',')
		line = append(line, r.User...)
		line = append(line, ',')
		line = append(line, r.Action...)
		line = append(line, ',')
		line = strconv.AppendInt(line, int64(r.Duration), 10)
		line = append(line, ',')
		line = strconv.AppendInt(line, int64(r.Score), 10)
		line = append(line, ',')
		line = strconv.AppendInt(line, r.TS, 10)
		line = append(line, '\n')

		// ✅ 修复 4：批量写——攒够 32KB 才真正交给下一层
		batch = append(batch, line...)
		if len(batch) >= flushBytes {
			if err := flush(); err != nil {
				return total, err
			}
		}
	}
	if err := flush(); err != nil {
		return total, err
	}
	// ✅ 修复 5：最后 Flush，把 bufio 里剩下的字节交出去
	if err := bw.Flush(); err != nil {
		return total, err
	}
	return total, nil
}

// exportCSV 是两个实现的统一入口：handler 与测试都通过它切换 bug / fix。
func exportCSV(w io.Writer, n int, fix bool) (int, error) {
	if fix {
		return exportFix(w, n)
	}
	return exportBug(w, n)
}

// countingWriter 统计底层 Write 的调用次数与字节数。
//
// 这是回答「为什么慢」的关键仪器：bug 模式每行都是一次独立写，
// write_calls ≈ rows；fix 模式攒够 32KB 才写一次，write_calls 骤降到 bytes/32KB 量级。
// 生产上更稳的做法也是这个——包一层 io.Writer 计数器（约 10 行、零依赖），
// 比依赖 strace 权限更可靠。
type countingWriter struct {
	w     io.Writer
	calls int64
	bytes int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.calls++
	c.bytes += int64(n)
	return n, err
}

// newMux 构造业务路由（pprof 在 19087，不在这里）。
func newMux(fix bool) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/export", func(w http.ResponseWriter, r *http.Request) {
		n := labkit.QueryInt(r, "n", defaultRows, 1, maxRows)

		// stats=1：只做统计、不返回 CSV，用于量化"写得有多碎"。
		// 字段契约（文档 07-实战案例集 依赖它，勿改字段名）：
		// mode / rows / bytes / write_calls / avg_write_bytes / elapsed_ms
		if r.URL.Query().Get("stats") == "1" {
			cw := &countingWriter{w: io.Discard}
			start := time.Now()
			rows, err := exportCSV(cw, n, fix)
			elapsed := time.Since(start)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			avg := int64(0)
			if cw.calls > 0 {
				avg = cw.bytes / cw.calls
			}
			labkit.WriteJSON(w, map[string]any{
				"mode":            labkit.ModeName(fix),
				"rows":            n,
				"bytes":           cw.bytes,
				"write_calls":     cw.calls,
				"avg_write_bytes": avg,
				"elapsed_ms":      float64(elapsed.Microseconds()) / 1000,
			})
			_ = rows
			return
		}

		// 响应头必须在写 body 之前设置，写出去之后再改就无效了。
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("X-Mode", labkit.ModeName(fix))

		if _, err := exportCSV(w, n, fix); err != nil {
			// 客户端断开是压测里很常见的情况，不用大惊小怪
			return
		}
	})
	return mux
}

func main() {
	flag.Parse()
	labkit.Run(labkit.Config{
		BizAddr:   *addrFlag,
		PprofAddr: *pprofAddrFlag,
		Fix:       *fixFlag,
		Handler:   newMux(*fixFlag),
		BizNote:   "GET /api/export?n=20000（返回 CSV）| GET /api/export?stats=1（写次数统计）",
		Problem:   "逐行 fmt.Fprintf 直接写响应体，无缓冲、无预分配，反射格式化 + 大量临时字符串",
		FixNote:   "bufio.Writer + 预分配 []byte + strconv 追加 + 32KB 批量写",
	})
}
