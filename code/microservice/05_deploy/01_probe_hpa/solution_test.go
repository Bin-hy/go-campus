package probe_hpa

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func code(h http.HandlerFunc) int {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Code
}

func TestProbeLifecycle(t *testing.T) {
	p := NewProbeState()
	if code(p.StartupHandler()) != http.StatusServiceUnavailable {
		t.Fatal("未启动完成时 startup 应失败")
	}
	if code(p.ReadinessHandler()) != http.StatusServiceUnavailable {
		t.Fatal("未启动完成时 readiness 应失败")
	}
	if code(p.LivenessHandler()) != http.StatusOK {
		t.Fatal("启动过程中 liveness 应成功（不能自杀）")
	}

	p.MarkStarted()
	if code(p.StartupHandler()) != http.StatusOK || code(p.ReadinessHandler()) != http.StatusOK {
		t.Fatal("启动完成后探针应成功")
	}

	p.SetReady(false) // 依赖故障
	if code(p.ReadinessHandler()) != http.StatusServiceUnavailable {
		t.Fatal("依赖故障时 readiness 应失败")
	}
	if code(p.LivenessHandler()) != http.StatusOK {
		t.Fatal("依赖故障不应触发 liveness 重启")
	}

	p.SetReady(true)
	p.Drain() // 收到 SIGTERM
	if code(p.ReadinessHandler()) != http.StatusServiceUnavailable {
		t.Fatal("停机时 readiness 应立即失败，先摘流量")
	}
	if code(p.LivenessHandler()) != http.StatusOK {
		t.Fatal("停机过程中 liveness 必须保持成功，否则会被误杀")
	}
	t.Log("探针生命周期 OK")
}

func TestHPAFormula(t *testing.T) {
	cfg := HPAConfig{MinReplicas: 2, MaxReplicas: 10, TargetCPUPercent: 50, Tolerance: 0.1}
	cases := []struct {
		name    string
		current int32
		cpu     float64
		want    int32
	}{
		{"扩容", 4, 80, 7}, // ceil(4*1.6)=7
		{"缩容", 4, 20, 2}, // ceil(1.6)=2
		{"容忍区间内不动", 4, 54, 4},
		{"下限", 1, 1, 2},
		{"上限", 10, 100, 10},
	}
	for _, c := range cases {
		if got := DesiredReplicas(c.current, c.cpu, cfg); got != c.want {
			t.Errorf("%s: current=%d cpu=%.0f got=%d want=%d", c.name, c.current, c.cpu, got, c.want)
		}
	}
}

func TestHPAStabilizationSteps(t *testing.T) {
	cfg := HPAConfig{MinReplicas: 1, MaxReplicas: 50, TargetCPUPercent: 50, Tolerance: 0.1, ScaleUpLimit: 4, ScaleDownLimit: 1}

	if got := DesiredReplicas(4, 200, cfg); got != 8 {
		t.Fatalf("扩容应受单次步长限制（4 → 8），实际 %d", got)
	}
	if got := DesiredReplicas(10, 5, cfg); got != 9 {
		t.Fatalf("缩容每次最多减 1，实际 %d", got)
	}
}

func TestHPAZeroMetric(t *testing.T) {
	cfg := HPAConfig{MinReplicas: 2, MaxReplicas: 5, TargetCPUPercent: 50, Tolerance: 0.1}
	if got := DesiredReplicas(3, 0, cfg); got != 2 {
		t.Fatalf("无流量时应缩到 min，实际 %d", got)
	}
}
