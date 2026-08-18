# 探针与自愈：liveness / readiness / startup 判定 + OOM 预警

## 难度：⭐⭐ 中等

## 考点
- 三种探针的分工：liveness 重启、readiness 摘流量（不重启）、startup 启动失败杀掉
- failureThreshold：最近连续 N 次失败才触发
- 资源限制：内存使用接近 limits 时预警（超限会被 OOMKilled）

## 题目描述

实现 Pod 健康判定引擎：

1. `JudgeProbe(kind, probe, history)`：给定探针类型、配置和最近 N 次探测结果（按时间顺序），判断是否触发——
   - 历史不足 `failureThreshold` 条 → 不触发（`ActionNone`）
   - 最近 `failureThreshold` 条全部失败 → 按类型触发：
     - `liveness` → `ActionRestart`（容器重启）
     - `readiness` → `ActionUnready`（从 Service 摘除，不重启）
     - `startup` → `ActionKill`（启动失败，杀掉重启）
2. `CheckOOM(memLimit, memUsageMiB)`：内存使用 ≥ limits 的 90% 返回 true（预警，接近 OOMKilled）

## 函数签名

```go
type Probe struct {
    FailureThreshold int // 连续失败多少次触发
}
type ProbeResult struct {
    OK bool
}
type ProbeAction int

const (
    ActionNone ProbeAction = iota
    ActionRestart // liveness 失败 → 重启容器
    ActionUnready // readiness 失败 → 摘出 Service
    ActionKill    // startup 失败 → 杀掉容器
)

func JudgeProbe(kind string, probe Probe, history []ProbeResult) ProbeAction
func CheckOOM(memLimit, memUsageMiB int64) bool
```

## 提示

- 只取 `history` 末尾 `failureThreshold` 条判断；不足则 None
- `CheckOOM` 用整数比较避免浮点：`memUsageMiB*10 >= memLimit*9`
- readiness 与 liveness 的区别是本题的灵魂：摘流量 ≠ 重启

## 运行测试

```bash
cd code/backend/07-k8s/05_probe && go test -v
```
