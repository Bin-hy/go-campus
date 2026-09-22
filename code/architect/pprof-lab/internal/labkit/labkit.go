// Package labkit 是本实验包所有样例共用的「启动骨架 + 小工具」。
//
// 它把「业务端口」和「pprof 管理端口」彻底分开：
//   - 业务 mux 跑在 -addr 上，只暴露业务端点；
//   - net/http/pprof 显式注册后跑在 -pprof-addr 上，只用于观测。
//
// ⚠️ 生产环境注意：pprof 会暴露进程内部信息（goroutine 栈、堆内容、命令行参数），
// 绝对不能挂在业务端口对公网开放。正确做法是：
//  1. 放到独立的管理端口（本实验包就是 1908N），只绑定 127.0.0.1 / 内网管理网卡；
//  2. 用安全组、K8s NetworkPolicy 限制访问来源，必要时加 basic auth 或 mTLS；
//  3. 需要长期观测就离线采集后集中存储，不要让公网直接访问 /debug/pprof。
package labkit

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

// shutdownTimeout 是优雅退出时等待在途请求的最长时间。
const shutdownTimeout = 5 * time.Second

// Config 描述一个样例程序的启动参数。
type Config struct {
	// BizAddr / PprofAddr 分别是业务监听地址与 pprof 管理监听地址。
	BizAddr   string
	PprofAddr string
	// Fix 为 true 时使用修复实现。
	Fix bool
	// Handler 是业务路由。
	Handler http.Handler
	// BizNote 是启动日志里的一行业务说明，例如 "GET /api/render?n=2000"。
	BizNote string
	// Problem / FixNote 分别是 bug 模式的问题描述与 fix 模式的修复手段描述。
	Problem string
	FixNote string
	// OnShutdown 可选：收到退出信号后先收敛后台 goroutine（L03 / L06 用它演示收敛）。
	OnShutdown func(ctx context.Context)
}

// ModeName 返回当前实现的英文名，用于日志和响应体。
func ModeName(fix bool) string {
	if fix {
		return "fix"
	}
	return "bug"
}

// ModeDesc 返回当前实现的中文名。
func ModeDesc(fix bool) string {
	if fix {
		return "fix 模式"
	}
	return "bug 模式"
}

// Run 启动业务 server 与 pprof server，阻塞直到收到 SIGINT / SIGTERM，然后优雅退出。
func Run(cfg Config) {
	// block / mutex profile 默认是关闭的，必须显式打开采样，
	// 否则 /debug/pprof/block 和 /debug/pprof/mutex 永远是空的（L04 / L05 要用）。
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	bizSrv := &http.Server{Addr: cfg.BizAddr, Handler: cfg.Handler, ReadHeaderTimeout: 5 * time.Second}
	pprofSrv := &http.Server{Addr: cfg.PprofAddr, Handler: pprofMux(), ReadHeaderTimeout: 5 * time.Second}

	log.Printf("[biz]   业务地址   http://%s   %s", cfg.BizAddr, cfg.BizNote)
	log.Printf("[pprof] pprof 地址 http://%s/debug/pprof/   (生产请放独立管理端口，且只绑定内网)", cfg.PprofAddr)
	if cfg.Fix {
		log.Printf("[mode]  %s ✅ %s", ModeDesc(true), cfg.FixNote)
	} else {
		log.Printf("[mode]  %s ⚠️  %s", ModeDesc(false), cfg.Problem)
	}

	go func() {
		if err := bizSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[biz] 监听 %s 失败: %v", cfg.BizAddr, err)
		}
	}()
	go func() {
		if err := pprofSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[pprof] 监听 %s 失败: %v", cfg.PprofAddr, err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Printf("[exit]  收到退出信号，开始优雅关闭（最多等 %s）...", shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// 先停业务入口，再收敛后台 goroutine，最后停 pprof（这样退出过程还能被观测到）。
	_ = bizSrv.Shutdown(shutdownCtx)
	if cfg.OnShutdown != nil {
		cfg.OnShutdown(shutdownCtx)
	}
	log.Printf("[exit]  当前 goroutine 数 = %d", runtime.NumGoroutine())
	_ = pprofSrv.Shutdown(shutdownCtx)
	log.Printf("[exit]  已退出")
}

// pprofMux 显式注册 net/http/pprof 的所有 handler。
//
// 这里刻意不用 `import _ "net/http/pprof"`（那会注册到 http.DefaultServeMux 上，
// 很容易不小心把它随业务端口暴露出去），而是显式挂到独立端口的 mux 上。
func pprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	// /debug/pprof/heap、/goroutine、/allocs、/block、/mutex 走子树匹配，由 pprof.Index 统一处理
	return mux
}

// QueryInt 读取整型 query 参数并收敛到 [min, max]；缺省或非法时返回 def。
func QueryInt(r *http.Request, key string, def, min, max int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// WriteJSON 以 JSON 返回响应体（状态码 200）。
func WriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	buf, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(buf)
}
