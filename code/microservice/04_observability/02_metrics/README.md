# Prometheus 指标：gRPC 服务的 RED 埋点

## 难度：⭐⭐ 中等

## 考点
- Counter / Histogram / Gauge 的语义与选型
- RED 方法：Rate（请求量）、Errors（错误率）、Duration（延迟）
- 直方图桶的选取与分位数计算（`histogram_quantile`）
- 标签基数：只放 method/code，绝不放 user_id/request_id
- 拦截器统一埋点 + 自定义 Registry（测试隔离）

## 题目描述

实现 `Metrics`：

1. `grpc_server_requests_total{method,code}`：Counter；
2. `grpc_server_handling_seconds{method}`：Histogram，桶 5ms~10s；
3. `grpc_server_inflight_requests{method}`：Gauge；
4. `UnaryServerInterceptor()`：统一计时、取状态码、维护在途；
5. `HandlerFor(registry)`：暴露 `/metrics`。

## 函数签名

```go
type Metrics struct{ /* ... */ }
func NewMetrics(reg prometheus.Registerer) *Metrics
func (m *Metrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor
func HandlerFor(reg prometheus.Gatherer) http.Handler
```

## 提示

1. 用 `status.Code(err).String()` 取标签值，不要区分错误消息。
2. `reg.MustRegister(...)` 注册失败会 panic——同名指标重复注册要换成 `Register` 并容忍 `AlreadyRegisteredError`。
3. 测试里用 `prometheus.NewRegistry()` 隔离，避免全局默认 registry 互相污染。
4. 直方图的 `_count` / `_sum` 是免费送的，延迟均值 = sum/count。

## 验收

- [ ] 成功 1 次 → `{code="OK"}` = 1
- [ ] 参数错误 1 次 → `{code="InvalidArgument"}` = 1
- [ ] 3 次调用 → histogram sample_count = 3
- [ ] 请求结束后 inflight = 0
- [ ] `go test ./04_observability/02_metrics`

## 运行

```bash
cd code/microservice
go test -v ./04_observability/02_metrics
```

## 常用 PromQL

```promql
# 错误率
sum(rate(grpc_server_requests_total{code!="OK"}[5m]))
/
sum(rate(grpc_server_requests_total[5m]))

# P99 延迟
histogram_quantile(0.99,
  sum by (le, method) (rate(grpc_server_handling_seconds_bucket[5m])))

# 在途请求（背压信号）
sum by (method) (grpc_server_inflight_requests)
```

## 面试追问

1. **Counter 和 Gauge 的区别？** Counter 只增不减（重启归零，用 `rate`）；Gauge 可增可减（在途数、内存）。
2. **为什么延迟用 Histogram 而不是 Summary？** Histogram 可跨实例聚合（sum by le），Summary 的分位数不能合并。
3. **标签基数爆炸怎么防？** 禁止高基数字段进标签；method 用完整路径前先归一化（避免 `/user/123` 这种）。
4. **指标和日志/链路怎么分工？** 指标发现"有没有问题"，链路定位"哪个服务/哪一跳慢"，日志解释"为什么"。
