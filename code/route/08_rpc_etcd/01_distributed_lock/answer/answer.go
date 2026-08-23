//go:build ignore

package answer

import (
	"context"
	"sync"
	"time"
)

// Etcd 抽象 etcd 客户端（生产用 go.etcd.io/etcd/client/v3）
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

// DistributedLock 参考答案：租约 + 原子占坑 + 后台续租 + CAS 释放
type DistributedLock struct {
	client Etcd
	key    string
	token  string
	ttl    time.Duration

	mu            sync.Mutex
	held          bool
	leaseID       int64
	stopKeepAlive chan struct{}
}

func NewDistributedLock(client Etcd, key, token string, ttl time.Duration) *DistributedLock {
	return &DistributedLock{client: client, key: key, token: token, ttl: ttl}
}

func (l *DistributedLock) TryLock(ctx context.Context) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held {
		return true, nil
	}

	leaseID, err := l.client.LeaseGrant(ctx, l.ttl)
	if err != nil {
		return false, err
	}
	ok, err := l.client.PutIfNotExists(ctx, l.key, l.token, leaseID) // 原子占坑
	if err != nil {
		_ = l.client.LeaseRevoke(context.Background(), leaseID)
		return false, err
	}
	if !ok { // 别人持有
		_ = l.client.LeaseRevoke(context.Background(), leaseID)
		return false, nil
	}

	stop, err := l.client.LeaseKeepAlive(ctx, leaseID) // 后台续租
	if err != nil {
		_ = l.client.LeaseRevoke(context.Background(), leaseID)
		return false, err
	}
	l.held, l.leaseID, l.stopKeepAlive = true, leaseID, stop
	return true, nil
}

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
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (l *DistributedLock) Unlock() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.held {
		return nil
	}
	if l.stopKeepAlive != nil {
		close(l.stopKeepAlive)
		l.stopKeepAlive = nil
	}
	ctx := context.Background()
	_, err := l.client.DeleteIfValue(ctx, l.key, l.token) // CAS：防误删他人锁
	_ = l.client.LeaseRevoke(ctx, l.leaseID)
	l.held = false
	return err
}
