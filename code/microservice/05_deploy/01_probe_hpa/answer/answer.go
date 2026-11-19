package probe_hpa

import (
	"math"
	"net/http"
	"sync"
)

// 参考答案 / 可运行实现。

// ProbeState 管理 startup / readiness / liveness 三种探针共享的状态。
//
//	startup   → 进程启动、依赖初始化完成前，一直失败（保护慢启动）
//	readiness → 失败时 K8s 把 Pod 从 Service endpoints 摘掉，不接新流量
//	liveness  → 失败时 K8s 杀掉容器重启（只用于"死了就救不回来"的场景）
type ProbeState struct {
	mu       sync.RWMutex
	started  bool
	live     bool
	ready    bool
	draining bool
}

func NewProbeState() *ProbeState {
	return &ProbeState{live: true}
}

// MarkStarted 依赖初始化完成（数据库连接、缓存预热、配置加载）。
func (p *ProbeState) MarkStarted() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started = true
	p.ready = true
}

// SetReady 依赖健康变化时调用（如注册中心断开 → false）。
func (p *ProbeState) SetReady(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ready = v
}

// MarkNotLive 表示不可恢复的故障，允许 liveness 重启容器。
func (p *ProbeState) MarkNotLive() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.live = false
}

// Drain 收到 SIGTERM：先摘 readiness，等在途请求处理完。
func (p *ProbeState) Drain() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.draining = true
	p.ready = false
}

func (p *ProbeState) Started() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.started
}

// Ready 只有"启动完成 + 依赖健康 + 未停机"才返回 true。
func (p *ProbeState) Ready() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.started && p.ready && !p.draining
}

// Live 停机过程中仍返回 true：否则 K8s 会误杀正在优雅退出的 Pod。
func (p *ProbeState) Live() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.live
}

func (p *ProbeState) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.Started() {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (p *ProbeState) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.Ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (p *ProbeState) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.Live() {
			http.Error(w, "not live", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// HPAConfig 对应 HorizontalPodAutoscaler 的关键字段。
type HPAConfig struct {
	MinReplicas      int32
	MaxReplicas      int32
	TargetCPUPercent float64 // 目标利用率，例如 50 表示 50%
	Tolerance        float64 // 容忍度，K8s 默认 0.1（±10%）
	ScaleUpLimit     int32   // 单次最多增加多少副本（0 表示不限）
	ScaleDownLimit   int32   // 单次最多减少多少副本（0 表示不限）
}

// DesiredReplicas 实现 HPA 的核心公式：
//
//	desired = ceil(current * currentMetric / targetMetric)
//
// 细节：
//   - 利用率在 target 的 ±tolerance 内不动（防抖动）；
//   - 结果夹在 [min, max]；
//   - 单次扩缩有步长上限（K8s 的 scaleUp/scaleDown 策略）。
func DesiredReplicas(current int32, cpuPercent float64, cfg HPAConfig) int32 {
	if current < 1 {
		current = 1
	}
	if cfg.TargetCPUPercent <= 0 {
		return clamp(current, cfg.MinReplicas, cfg.MaxReplicas)
	}
	ratio := cpuPercent / cfg.TargetCPUPercent
	if math.Abs(ratio-1) <= cfg.Tolerance {
		return clamp(current, cfg.MinReplicas, cfg.MaxReplicas)
	}

	desired := int32(math.Ceil(float64(current) * ratio))
	if desired < current && cfg.ScaleDownLimit > 0 && current-desired > cfg.ScaleDownLimit {
		desired = current - cfg.ScaleDownLimit
	}
	if desired > current && cfg.ScaleUpLimit > 0 && desired-current > cfg.ScaleUpLimit {
		desired = current + cfg.ScaleUpLimit
	}
	return clamp(desired, cfg.MinReplicas, cfg.MaxReplicas)
}

func clamp(v, min, max int32) int32 {
	if min > 0 && v < min {
		v = min
	}
	if max > 0 && v > max {
		v = max
	}
	return v
}
