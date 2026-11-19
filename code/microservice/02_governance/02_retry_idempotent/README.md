# 超时重试、指数退避与幂等：重试不重复扣款

## 难度：⭐⭐⭐⭐ 困难（生产治理核心）

## 考点
- 指数退避 + 抖动（jitter）：为什么必须抖动
- 只重试"可重试且幂等"的错误
- 重试预算：`ctx` 取消要能立刻中断退避等待
- 幂等键：同 key 只执行一次；并发同 key 用 singleflight 合并
- 失败不缓存、成功缓存 TTL：失败要允许同 key 重试
- 真实对应：gRPC retryPolicy / 自研拦截器 + Redis SETNX / MySQL 唯一索引

## 题目描述

1. `Do(ctx, policy, isRetryable, fn)`：最多 `MaxAttempts` 次，失败按指数退避，`ctx` 取消立即返回；
2. `Policy.Backoff(attempt)`：`base * 2^attempt`，封顶 `MaxDelay`，加 ±20% 抖动；
3. `IdempotencyStore.Do(key, fn)`：同 key 串行/并发都只执行一次，返回 `(value, deduplicated, error)`；失败不缓存，成功后缓存 TTL。

## 函数签名

```go
type Policy struct {
    MaxAttempts int
    BaseDelay   time.Duration
    MaxDelay    time.Duration
    Jitter      func() float64
}
func (p Policy) Backoff(attempt int) time.Duration
func Do(ctx context.Context, p Policy, isRetryable func(error) bool, fn func(context.Context) error) (attempts int, err error)

type IdempotencyStore struct{ /* ... */ }
func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore
func (s *IdempotencyStore) Do(key string, fn func() (any, error)) (any, bool, error)
```

## 提示

1. 退避等待用 `time.NewTimer` + `select { case <-ctx.Done(): }`，别直接 `time.Sleep`。
2. 幂等表的并发控制：先锁内查/占位，锁外执行；后来的请求 `<-done` 等第一个完成。
3. 失败要 `delete(entries, key)`，否则用户用同一个 key 重试会一直被"缓存失败"。
4. 返回值 `deduplicated=true` 是给调用方的信号：这次没有再扣款。

## 验收

- [ ] 前两次失败、第三次成功 → `attempts=3`
- [ ] 不可重试错误只调用 1 次
- [ ] ctx 在退避期间取消 → 立刻返回 `context.Canceled`
- [ ] 20 个 goroutine 同 key 并发 → 业务函数只执行 1 次
- [ ] 失败后同 key 可重试；TTL 过期后可再次执行
- [ ] `go test -race ./02_governance/02_retry_idempotent`

## 运行

```bash
cd code/microservice
go test -race -v ./02_governance/02_retry_idempotent
```

## 面试追问

1. **为什么重试必须抖动？** 否则所有客户端在同一时刻重试，故障瞬间流量翻倍（重试风暴），把"抖动"变成"雪崩"。
2. **幂等键放在哪一层？** 最靠近副作用的地方（扣款前查幂等表），且要和业务写入在同一事务里，否则仍有窗口。
3. **重试和幂等的关系？** 重试解决"可能失败"，幂等解决"重复执行安全"；没有幂等的重试是放大器。
4. **gRPC 原生重试够用吗？** `retryPolicy` 只按状态码重试，无法感知业务幂等；写接口建议应用层显式重试 + 幂等键。
