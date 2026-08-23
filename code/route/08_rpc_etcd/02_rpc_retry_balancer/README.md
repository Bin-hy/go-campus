# RPC 客户端治理：负载均衡 + 超时 + 重试 + 熔断

## 难度：⭐⭐⭐⭐ 难（真实高并发治理核心）

## 考点
- 客户端负载均衡：round-robin 选实例（对应 gRPC resolver + balancer 思路）
- 单次调用超时：`context.WithTimeout` 派生（全链路 deadline 传播的第一步）
- 重试与指数退避 + jitter：只重试幂等请求，防**重试风暴/羊群效应**
- 熔断三态状态机：Closed → Open → Half-Open → 探测恢复
- 实例故障剔除：熔断的实例不再被选中

## 题目描述

用 `Registry` + `Transport` 两个接口抽象"服务注册中心 + 一次 RPC 调用"（生产实现为 etcd 注册中心 + gRPC 一元调用），实现文档 8.3.6 / 8.4.4 的 **RPC 客户端治理**：

1. **Call**：每次调用先 `Discover` 拿实例列表 → **过滤掉已熔断的实例** → round-robin 选一个 → 带超时调用；
2. **失败重试**：调用失败记录到该实例的熔断器，退避后重试下一个实例（最多 `MaxRetry` 次）；退避 = 指数增长 + 随机抖动（jitter），把重试打散；
3. **熔断**：某实例连续失败达 `FailThreshold` → 熔断 **Open**（不再选它，快速失败）；冷却 `Cooldown` 后进入 **Half-Open**，放一个探测请求——成功回 Closed 恢复，失败重新 Open。

## 函数签名

```go
// Registry 服务注册中心（生产为 etcd：注册/续租/Watch，这里只留发现）
type Registry interface {
	Discover(ctx context.Context) ([]string, error)
}

// Transport 向指定地址发起一次 RPC（生产为 grpc 一元调用）
type Transport func(ctx context.Context, addr, method string, req any) (any, error)

// Options 客户端治理参数
type Options struct {
	Timeout       time.Duration // 单次调用超时
	MaxRetry      int           // 最多重试次数
	BackoffBase   time.Duration // 指数退避基础值（1, 2, 4... 倍 + jitter）
	FailThreshold int           // 连续失败多少次触发熔断
	Cooldown      time.Duration // 熔断冷却期，之后进入 Half-Open 探测
}

type Caller struct{ /* 自行设计 */ }

func NewCaller(registry Registry, transport Transport, opts Options) *Caller
func (c *Caller) Call(ctx context.Context, method string, req any) (any, error)
```

## 提示
1. **熔断器按实例独立**：`sync.Map`（addr -> *breaker），每个 breaker 有 state（closed/open/halfOpen）、failures、openAt，用 `sync.Mutex` 保护；
2. **allow(addr)**：Open 且已过冷却期 → 转 Half-Open 放行（探测）；Open 未过冷却期 → 拒绝；closed/halfOpen → 放行；
3. **recordSuccess**：失败计数清零；Half-Open 探测成功 → 回 Closed；**recordFailure**：Half-Open 探测失败 → 回 Open（重置 openAt）；Closed 下连续失败达阈值 → Open；
4. **重试循环**：`for attempt := 0; attempt <= opts.MaxRetry; attempt++`，每次**重新 Discover + 过滤**（熔断的实例自动被剔除，模拟真实注册中心动态变化）；
5. **退避**：`backoff(base, attempt) = base * 2^attempt + rand(0, base)`——jitter 是防重试风暴的关键，别忘了；
6. **超时**：`context.WithTimeout(ctx, opts.Timeout)` 包住单次 transport 调用，`defer cancel()` 防泄漏；
7. **ctx 取消**：退避等待也要 `select { case <-ctx.Done(): ... }`，上层取消要立刻返回。

## 与真实框架对照（背下这段，面试加分）

| 本题 | gRPC-go | 说明 |
| --- | --- | --- |
| `Discover` | `Resolver`（etcd resolver 插件） | 从注册中心拿实例列表，Watch 增量更新 |
| round-robin | `balancer.RoundRobin`（Pick） | 客户端选路，无中心网关 |
| 单次超时 | `grpc.WithTimeout` / `context.WithTimeout` | 全链路 deadline 传播 |
| 重试 | `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize...)` + 拦截器 | 生产常自研拦截器做重试/退避 |
| 熔断 | `github.com/sony/gobreaker` / 自研拦截器 | Open/Half-Open/Closed 三态 |

## 验收
- [ ] 3 个健康实例 6 次调用：每个实例恰好被调用 2 次（轮询均匀，`TestCall_RoundRobinDistribution`）
- [ ] 首次调用失败：自动重试到其他实例并成功，总调用次数 = 2（`TestCall_RetryFallback`）
- [ ] 实例连续失败达阈值 → 熔断 Open：后续调用不再打它（计数不增长，直接 `ErrNoInstance`）
- [ ] 冷却期后放探测请求：成功则恢复（重新纳入轮询），失败则保持 Open
- [ ] 单次调用超过 Timeout 立即返回 `context.DeadlineExceeded`，不无限阻塞
- [ ] `go test -race ./08_rpc_etcd/02_rpc_retry_balancer -v` 无 data race
