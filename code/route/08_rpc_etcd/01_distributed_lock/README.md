# etcd 语义分布式锁（租约 + 续租 + CAS 防误删）

## 难度：⭐⭐⭐ 中等偏难

## 考点
- etcd 核心机制：Lease 租约（TTL + keepalive 续租）、事务 CAS（原子占坑 / 条件删除）
- 分布式锁三要素：**原子加锁、防误删、防死锁**
- 崩溃自动释放：持有者崩溃 = 续租停止 = 租约过期 = 锁自动释放
- 并发正确性：多节点互斥、并发安全（data race 检测）

## 题目描述

用 `Etcd` 接口抽象真实 etcd（生产实现为 `go.etcd.io/etcd/client/v3`，本练习离线可测），实现文档 8.2.5 / 8.2.6 / 8.4.2 的**分布式锁**：

1. **TryLock**：非阻塞尝试加锁——先 `LeaseGrant` 创建租约，再 `PutIfNotExists`（对应真实 etcd 的 `Txn(If CreateRevision(key)==0, Then Put(key, token, WithLease))`）**原子占坑**；抢到后启动 `LeaseKeepAlive` 后台续租；
2. **Lock**：阻塞加锁——循环 TryLock，直到成功或 `ctx` 取消/超时；
3. **Unlock**：释放锁——先停止续租，再用 `DeleteIfValue`（对应真实 etcd 的 `Txn(If Value(key)==token, Then Delete)`）**CAS 删除**：只有 token 匹配才删，防止"删了别人的锁"。

选主场景（文档 8.4.3）就是"所有候选节点抢同一把锁 + Watch"：本题先把锁本身写对，选主是它的直接应用。

## 函数签名

```go
// Etcd 抽象 etcd 客户端（生产用 clientv3，见 README 末尾对照表）
type Etcd interface {
	Put(ctx context.Context, key, value string, leaseID int64) error
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	Delete(ctx context.Context, key string) error
	PutIfNotExists(ctx context.Context, key, value string, leaseID int64) (bool, error)
	DeleteIfValue(ctx context.Context, key, value string) (bool, error)
	LeaseGrant(ctx context.Context, ttl time.Duration) (int64, error)
	LeaseRevoke(ctx context.Context, leaseID int64) error
	LeaseKeepAlive(ctx context.Context, leaseID int64) (stop chan struct{}, err error)
	Close() error
}

type DistributedLock struct{ /* 自行设计 */ }

func NewDistributedLock(client Etcd, key, token string, ttl time.Duration) *DistributedLock
func (l *DistributedLock) TryLock(ctx context.Context) (bool, error)
func (l *DistributedLock) Lock(ctx context.Context) error
func (l *DistributedLock) Unlock() error
```

## 提示
1. **原子占坑**：`PutIfNotExists` 返回 `true` 才算抢到锁；返回 `false` 说明别人持有——**记得 `LeaseRevoke` 自己刚创建的租约**，否则每次重试都泄漏一个租约；
2. **续租**：`LeaseKeepAlive` 返回的 `stop` 通道在 `Unlock` 时 close；续租由 fake 后台线程模拟，你只管"启动它 + 关闭它"；
3. **防误删**：`Unlock` 用 `DeleteIfValue(key, token)`，**不要**用裸 `Delete`——token 不匹配说明锁已易主，删了会破坏互斥；
4. **可重入**：`TryLock` 里如果自己已持有（`held == true`），直接返回成功（简化实现）；
5. **并发安全**：`DistributedLock` 内部字段（held/leaseID/stopKeepAlive）会被 TryLock/Unlock 并发访问，加 `sync.Mutex` 保护，跑 `go test -race` 验证。

## 与真实 etcd 对照（背下这段，面试加分）

| 本题接口 | 真实 clientv3 | 说明 |
| --- | --- | --- |
| `PutIfNotExists` | `cli.Txn(ctx).If(Compare(CreateRevision(key),"=",0)).Then(OpPut(key,val,WithLease(id)))` | 原子占坑：CreateRevision==0 表示 key 不存在 |
| `DeleteIfValue` | `cli.Txn(ctx).If(Compare(Value(key),"=",token)).Then(OpDelete(key))` | CAS 释放：值匹配才删 |
| `LeaseGrant/KeepAlive/Revoke` | `cli.Lease.Grant / KeepAlive / Revoke` | 租约三件套 |
| 封装好的现成实现 | `concurrency.NewSession + concurrency.NewMutex` | 生产直接用，原理就是本题 |

## 验收
- [ ] 20 个 goroutine 争抢同一把锁，临界区**同时最多 1 个**进入（`TestLock_MutualExclusion`）
- [ ] 持锁期间超过 TTL 仍持有（后台续租生效，`TestLease_ExpiryAutoRelease` 前半段）
- [ ] 模拟持有者崩溃（停止续租）后，锁在 TTL 内**自动释放**（同测试后半段）
- [ ] 锁被"过期易主"后，旧持有者 Unlock **删不掉新持有者的锁**（`TestUnlock_NonOwner`）
- [ ] `go test -race ./08_rpc_etcd/01_distributed_lock -v` 无 data race
