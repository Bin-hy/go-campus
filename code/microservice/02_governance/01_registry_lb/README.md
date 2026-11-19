# 注册发现与客户端负载均衡：TTL、心跳、Round-Robin、P2C

## 难度：⭐⭐⭐ 困难

## 考点
- 注册中心模型：注册 → 心跳续租 → TTL 过期剔除 → 发现
- 惰性剔除 vs 后台扫描；时钟注入让 TTL 可测
- 客户端负载均衡：round-robin 与 P2C（Power of Two Choices）
- 真实对应：etcd Lease + KeepAlive + Watch；gRPC resolver + balancer

## 题目描述

实现内存注册中心与两种均衡器：

1. `Registry`：`Register / Heartbeat / Deregister / Discover`；超过 TTL 未心跳的实例在 `Discover` 时被剔除；
2. `RoundRobin`：无状态轮询，空列表返回 `ErrNoInstance`；
3. `P2C`：每次随机抽两个实例，选在途请求更少的那个；`Begin/Done` 维护负载。

## 函数签名

```go
type Registry struct{ /* ... */ }
func NewRegistry(ttl time.Duration) *Registry
func (r *Registry) SetClock(fn func() time.Time) // 测试用
func (r *Registry) Register(service, addr string)
func (r *Registry) Heartbeat(service, addr string) error
func (r *Registry) Deregister(service, addr string)
func (r *Registry) Discover(service string) []string

type RoundRobin struct{ /* ... */ }
func (rr *RoundRobin) Pick(addrs []string) (string, error)

type P2C struct{ /* ... */ }
func NewP2C(seed int64) *P2C
func (p *P2C) Begin(addr string)
func (p *P2C) Done(addr string)
func (p *P2C) Pick(addrs []string) (string, error)
```

## 提示

1. 心跳对"不存在的实例"要报错，否则已下线的实例会被"复活"。
2. Discover 返回排序后的列表，测试才稳定。
3. RoundRobin 用 `atomic.Uint64`，避免每次加锁。
4. P2C 抽两个**不同**的下标：`j := rnd.Intn(len-1); if j >= i { j++ }`。
5. P2C 只解决"选谁"，"负载"需要在请求前后 `Begin/Done` 配对。

## 验收

- [ ] 3 个实例，6 次轮询每个恰好 2 次
- [ ] 只有 1 个实例持续心跳，2 个 TTL 过期后被剔除
- [ ] 某实例 100 个在途请求时，P2C 不再选中它
- [ ] `go test -race ./02_governance/01_registry_lb`

## 运行

```bash
cd code/microservice
go test -race -v ./02_governance/01_registry_lb
```

## 面试追问

1. **为什么 gRPC 的 LB 在客户端做？** HTTP/2 长连接多路复用，连接数均衡 ≠ 流量均衡；客户端本地选路少一跳、无代理单点。
2. **etcd 注册发现怎么实现？** 实例创建 Lease（TTL），定时 KeepAlive；调用方 Watch 前缀增量更新本地列表；实例宕机租约到期自动删除。
3. **P2C 为什么比 round-robin 好？** 轮询对"慢实例"无感知，会把请求继续打到已经过载的节点；P2C 用少量状态换到近似 least-loaded，且避免"所有客户端同时选最小"的羊群效应。
4. **注册中心挂了怎么办？** 客户端缓存最后一次实例列表，继续用旧列表做 LB（fail-static）；恢复后 Watch 补齐。
