package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Metrics 一组 gRPC 服务端指标。
type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inflight *prometheus.GaugeVec
}

// NewMetrics 注册 RED 三件套：
//   - grpc_server_requests_total{method,code}（Counter）
//   - grpc_server_handling_seconds{method}（Histogram，5ms~10s）
//   - grpc_server_inflight_requests{method}（Gauge）
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_server_requests_total",
			Help: "Total number of gRPC requests.",
		}, []string{"method", "code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_server_handling_seconds",
			Help:    "gRPC handler latency.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}, []string{"method"}),
		inflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "grpc_server_inflight_requests",
			Help: "Current in-flight gRPC requests.",
		}, []string{"method"}),
	}
	reg.MustRegister(m.requests, m.duration, m.inflight)
	return m
}

// UnaryServerInterceptor 统一埋点：计时、取状态码、维护在途。
func (m *Metrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Handler 暴露 /metrics。
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

// HandlerFor 用于自定义 registry。
func HandlerFor(reg prometheus.Gatherer) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
