# Service 流量转发：Endpoints 维护 + 负载均衡

## 难度：⭐⭐ 中等

## 考点
- label selector 匹配一组 Pod → Endpoints（Pod IP 列表）
- readiness 摘除：不 Ready 的 Pod 不进 Endpoints，流量只发给健康 Pod
- kube-proxy 的两种转发策略：轮询（RoundRobin）/ 最少连接（LeastConnections）

## 题目描述

实现 Service 的两件核心事：

1. `BuildEndpoints(pods, selector)`：选出 labels 完全匹配 selector 且 `Ready` 的 Pod，返回 IP 列表（保持入参顺序）
2. `LoadBalancer`：两种转发策略——
   - `NewRoundRobin(ips)`：轮询，`Next()` 依次返回下一个 IP，循环往复；空列表返回 `""`
   - `NewLeastConn(pods)`：最少连接，`Next()` 返回当前连接数 `Conns` 最小的 Pod IP（平局按 IP 字典序取小），并把该 Pod 的 `Conns+1`（模拟新连接挂上）

## 函数签名

```go
type Pod struct {
    IP     string
    Labels map[string]string
    Ready  bool
    Conns  int // 当前连接数
}

func BuildEndpoints(pods []Pod, selector map[string]string) []string

type LoadBalancer interface {
    Next() string
}
func NewRoundRobin(ips []string) LoadBalancer
func NewLeastConn(pods []Pod) LoadBalancer
```

## 提示

- 匹配规则：selector 的每个 key-value 都必须在 Pod 的 Labels 中存在且相等
- RoundRobin 内部用游标记录当前位置；LeastConn 每次全量扫描找最小
- `Next()` 是纯内存操作；测试为单线程，锁可有可无

## 运行测试

```bash
cd code/backend/07-k8s/04_service_lb && go test -v
```
