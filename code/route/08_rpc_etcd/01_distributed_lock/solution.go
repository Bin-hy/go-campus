package distributed_lock

import (
	"context"
	"sync"
	"time"
)

// Etcd 抽象 etcd 客户端（生产实现为 go.etcd.io/etcd/client/v3，对照表见 README）
type Etcd interface {
	// Put 写入 key=value，绑定 leaseID（0 表示不绑定）
	Put(ctx context.Context, key, value string, leaseID int64) error
	// Get 读取 key；key 不存在返回 ok=false
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	// Delete 删除 key
	Delete(ctx context.Context, key string) error
	// PutIfNotExists 仅当 key 不存在时写入（对应 etcd Txn: If CreateRevision(key)==0 Then Put）
	PutIfNotExists(ctx context.Context, key, value string, leaseID int64) (bool, error)
	// DeleteIfValue 仅当 key 当前值 == value 时删除（对应 etcd Txn: If Value(key)==value Then Delete）
	DeleteIfValue(ctx context.Context, key, value string) (bool, error)
	// LeaseGrant 创建 ttl 时长的租约，返回 leaseID
	LeaseGrant(ctx context.Context, ttl time.Duration) (int64, error)
	// LeaseRevoke 撤销租约，绑定该租约的 key 全部删除
	LeaseRevoke(ctx context.Context, leaseID int64) error
	// LeaseKeepAlive 开始后台续租，返回 stop 通道；关闭 stop 后停止续租
	LeaseKeepAlive(ctx context.Context, leaseID int64) (stop chan struct{}, err error)
	// Close 释放连接
	Close() error
}

// DistributedLock etcd 语义分布式锁：租约 + 原子占坑 + 后台续租 + CAS 释放
type DistributedLock struct {
	client Etcd
	key    string
	token  string // 本节点唯一标识：CAS 释放的依据，防"删了别人的锁"
	ttl    time.Duration

	mu            sync.Mutex // 保护下面三个字段（TryLock/Unlock 并发调用）
	held          bool
	leaseID       int64
	stopKeepAlive chan struct{}
}

// NewDistributedLock 创建一把锁；token 必须全局唯一（如 nodeID-uuid）
func NewDistributedLock(client Etcd, key, token string, ttl time.Duration) *DistributedLock {
	return &DistributedLock{client: client, key: key, token: token, ttl: ttl}
}

// TryLock 非阻塞尝试获取锁：
// 1) LeaseGrant 创建租约（key 绑定租约，崩溃时到期自动释放 = 防死锁）
// 2) PutIfNotExists 原子占坑（CreateRevision==0 的 CAS），失败则 Revoke 本次租约并返回 false
// 3) 成功则启动 LeaseKeepAlive 后台续租
func (l *DistributedLock) TryLock(ctx context.Context) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held {
		return true, nil // 本节点已持有（简化可重入）
	}

	leaseID, err := l.client.LeaseGrant(ctx, l.ttl)
	if err != nil {
		return false, err
	}
	ok, err := l.client.PutIfNotExists(ctx, l.key, l.token, leaseID)
	if err != nil {
		_ = l.client.LeaseRevoke(context.Background(), leaseID) // 失败要清理租约
		return false, err
	}
	if !ok { // 别人持有：释放本次租约，返回 false
		_ = l.client.LeaseRevoke(context.Background(), leaseID)
		return false, nil
	}

	stop, err := l.client.LeaseKeepAlive(ctx, leaseID) // 后台续租：崩溃即停，锁自动过期
	if err != nil {
		_ = l.client.LeaseRevoke(context.Background(), leaseID)
		return false, err
	}
	l.held, l.leaseID, l.stopKeepAlive = true, leaseID, stop
	return true, nil
}

// Lock 阻塞获取锁：轮询 TryLock，直到成功或 ctx 取消/超时
func (l *DistributedLock) Lock(ctx context.Context) error {
	for {
		ok, err := l.TryLock(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond): // 重试间隔
		}
	}
}

// Unlock 释放锁：先停止续租，再 CAS 删除（仅 token 匹配才删，防误删他人锁）
func (l *DistributedLock) Unlock() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.held {
		return nil
	}
	if l.stopKeepAlive != nil { // 停止续租（不 close nil / 不重复 close）
		close(l.stopKeepAlive)
		l.stopKeepAlive = nil
	}
	ctx := context.Background()
	_, err := l.client.DeleteIfValue(ctx, l.key, l.token) // CAS：值不匹配 = 锁已易主，删不掉
	_ = l.client.LeaseRevoke(ctx, l.leaseID)
	l.held = false
	return err
}
