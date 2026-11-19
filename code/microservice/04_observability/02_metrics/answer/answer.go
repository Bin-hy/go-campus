package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// 参考答案 / 可运行实现。
//
// 指标设计遵循 RED：Rate（请求数）、Errors（错误码）、Duration（耗时）。
// 标签只放低基数维度（method/code），绝不放 user_id/request_id。

// Metrics 一组 gRPC 服务端指标。
type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inflight *prometheus.GaugeVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_server_requests_total",
			Help: "Total number of gRPC requests.",
		}, []string{"method", "code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "grpc_server_handling_seconds",
			Help: "gRPC handler latency.",
			// 分位数必须从桶算：0.005 ~ 10s，覆盖大多数 RPC。
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

// UnaryServerInterceptor 统一埋点；业务 handler 不用写一行指标代码。
func (m *Metrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		m.inflight.WithLabelValues(info.FullMethod).Inc()
		defer m.inflight.WithLabelValues(info.FullMethod).Dec()

		resp, err := handler(ctx, req)

		code := status.Code(err).String()
		m.requests.WithLabelValues(info.FullMethod, code).Inc()
		m.duration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

// Handler 暴露 /metrics。
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

// HandlerFor 用于自定义 registry（测试/多进程隔离时更常用）。
func HandlerFor(reg prometheus.Gatherer) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
