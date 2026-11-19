# 熔断、限流与优雅停机

## 难度：⭐⭐⭐ 困难

## 考点
- 令牌桶：突发容量 + 匀速补充；时钟注入让限流可测
- 熔断三态：Closed → Open → Half-Open → Closed/Open
- 优雅停机门闸：停止接新请求 + 等待在途请求清零 + 超时兜底
- 真实对应：SIGTERM → readiness=false → preStop → GracefulStop

## 题目描述

1. `TokenBucket`：`NewTokenBucket(ratePerSec, burst)`，`Allow()` 原子地取一个令牌；
2. `Breaker`：连续失败 `MaxFailures` 次进入 Open；冷却 `Cooldown` 后进入 Half-Open 放一个探测；探测成功恢复 Closed，失败重新 Open；
3. `Drain`：`Acquire/Release` 跟踪在途；`Shutdown(ctx)` 停止接收并等待清零，超时返回 `ctx.Err()`。

## 函数签名

```go
func NewTokenBucket(ratePerSec float64, burst int) *TokenBucket
func (b *TokenBucket) Allow() bool
func (b *TokenBucket) SetClock(fn func() time.Time)

type BreakerConfig struct { MaxFailures int; Cooldown time.Duration }
func NewBreaker(cfg BreakerConfig) *Breaker
func (b *Breaker) Allow() bool
func (b *Breaker) Success()
func (b *Breaker) Failure()

type Drain struct{ /* ... */ }
func NewDrain() *Drain
func (d *Drain) Acquire() error
func (d *Drain) Release()
func (d *Drain) Shutdown(ctx context.Context) error
```

## 提示

1. 令牌桶用浮点 `tokens` 保存小数，`elapsed*rate` 补水。
2. Half-Open 只放**一个**探测：可以用状态转换本身控制（Allow 时从 Open 转 Half-Open）。
3. `Drain.zero` 每次归零后要重建 channel，否则第二次归零无法通知。
4. `Shutdown` 先 `StopAccepting()` 再等 channel，避免"边等边进新请求"。

## 验收

- [ ] 3 个突发令牌用完后拒绝；补 150ms（rate=10）后放行 1 个
- [ ] 连续 3 次失败 → Open；冷却后 Half-Open；探测成功 → Closed
- [ ] draining 后 `Acquire` 返回 `ErrDraining`
- [ ] 在途未清零时 Shutdown 阻塞；清零后返回 nil；超时返回 `DeadlineExceeded`
- [ ] `go test -race ./02_governance/03_resilience_graceful`

## 运行

```bash
cd code/microservice
go test -race -v ./02_governance/03_resilience_graceful
```

## 面试追问

1. **限流和熔断的区别？** 限流保护自己（控制入口速率）；熔断保护自己不被下游拖死（快速失败）。
2. **熔断的半开为什么只放一个请求？** 探测流量太大会再次压垮刚恢复的下游；单探针成功后再逐步放量。
3. **优雅停机为什么先 readiness=false 再停服务？** 让 K8s/LB 先把流量摘掉；直接 `Stop` 会丢掉"刚被转发但还没处理"的请求。
4. **`preStop` 里为什么要 sleep？** Endpoint 摘除是异步的，sleep 给 kube-proxy/Ingress 收敛时间。
