# 探针与 HPA：readiness / liveness / startup 与扩缩容公式

## 难度：⭐⭐ 中等

## 考点
- 三种探针的职责边界：startup 保护慢启动、readiness 摘流量、liveness 重启
- readiness ≠ liveness：依赖故障只能摘流量，不能重启所有实例
- 停机流程：readiness=false → 等待在途 → 退出；liveness 在停机期间必须仍为成功
- HPA 公式：`ceil(current * currentMetric / targetMetric)`
- 容忍度（默认 ±10%）防抖动；扩缩步长与稳定窗口

## 题目描述

1. `ProbeState`：`MarkStarted / SetReady / MarkNotLive / Drain`，以及三个 HTTP handler；
2. `DesiredReplicas(current, cpuPercent, cfg)`：实现 HPA 核心公式 + 容忍度 + 上下限 + 单次步长。

## 函数签名

```go
type ProbeState struct{ /* ... */ }
func NewProbeState() *ProbeState
func (p *ProbeState) MarkStarted()
func (p *ProbeState) SetReady(v bool)
func (p *ProbeState) MarkNotLive()
func (p *ProbeState) Drain()
func (p *ProbeState) StartupHandler() http.HandlerFunc
func (p *ProbeState) ReadinessHandler() http.HandlerFunc
func (p *ProbeState) LivenessHandler() http.HandlerFunc

type HPAConfig struct {
    MinReplicas, MaxReplicas int32
    TargetCPUPercent, Tolerance float64
    ScaleUpLimit, ScaleDownLimit int32
}
func DesiredReplicas(current int32, cpuPercent float64, cfg HPAConfig) int32
```

## 提示

1. readiness 必须是"启动完成 && 依赖健康 && 未停止接收"三者的与。
2. `Drain()` 只影响 readiness，不影响 liveness——否则 K8s 会在优雅停机时把 Pod 杀掉。
3. HPA 先算 `ratio`，容忍区间内直接返回 current；再向上取整；最后依次应用步长与 min/max。
4. 没有 CPU 指标（0）时应缩到 min，而不是除零。

## 验收

- [ ] 启动前 startup/readiness 失败，liveness 成功
- [ ] 依赖故障：readiness 失败、liveness 成功
- [ ] Drain 后 readiness 失败、liveness 仍成功
- [ ] HPA：4 副本 80% / target 50% → 7；54% → 4（容忍）；单次扩容上限 4 → 8
- [ ] `go test ./05_deploy/01_probe_hpa`

## 运行

```bash
cd code/microservice
go test -v ./05_deploy/01_probe_hpa
```

## 对照 lab/k8s

`lab/k8s/10-uservice.yaml` 里就是这套探针的真实 YAML：

```yaml
startupProbe:   { grpc: { port: 9001 }, failureThreshold: 30 }
readinessProbe: { grpc: { port: 9001 }, periodSeconds: 5 }
livenessProbe:  { grpc: { port: 9001 }, periodSeconds: 10 }
lifecycle:
  preStop: { exec: { command: ["/bin/sh", "-c", "sleep 5"] } }
```

## 面试追问

1. **liveness 探什么最合适？** 只探进程自身（能否响应），绝不探数据库/下游；下游挂了重启自己没用，还会引发雪崩。
2. **HPA 为什么有 15 秒/分钟的延迟？** 指标采集周期（metrics-server 15s）+ HPA 同步周期（15s）+ 稳定窗口。
3. **HPA 只看 CPU 够吗？** 生产常用多指标（QPS、队列长度、P99）；自定义指标走 Prometheus Adapter。
4. **滚动发布怎么保证不中断？** `maxUnavailable: 0` + `maxSurge: 1` + readiness + preStop + PDB。
