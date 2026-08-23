package distributed_lock

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fakeEtcd：内存版 etcd（模拟租约到期 / 续租 / CAS），供离线测试
// ---------------------------------------------------------------------------

type entry struct {
	value   string
	leaseID int64
	ttl     time.Duration
}

type fakeEtcd struct {
	mu        sync.Mutex
	data      map[string]*entry
	keepAlive map[int64]time.Time // leaseID -> 最近一次续租时间
	ttls      map[int64]time.Duration
	leaseSeq  int64
	closed    bool

	gen    int           // 代际：stopAllKeepAlives 时 +1
	stopCh chan struct{} // 关闭即通知当前代际的续租协程退出
}

func newFakeEtcd() *fakeEtcd {
	return &fakeEtcd{
		data:      map[string]*entry{},
		keepAlive: map[int64]time.Time{},
		ttls:      map[int64]time.Duration{},
		stopCh:    make(chan struct{}),
	}
}

// 条目是否已过期（租约续租停止超过 ttl 即过期，模拟"持有者崩溃"）
func (f *fakeEtcd) expired(e *entry) bool {
	last, ok := f.keepAlive[e.leaseID]
	return ok && time.Since(last) > e.ttl
}

func (f *fakeEtcd) drop(key string) {
	e := f.data[key]
	delete(f.data, key)
	if e != nil {
		delete(f.keepAlive, e.leaseID)
	}
}

func (f *fakeEtcd) Put(_ context.Context, key, value string, leaseID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return errClosed
	}
	f.data[key] = &entry{value: value, leaseID: leaseID, ttl: f.ttls[leaseID]}
	return nil
}

func (f *fakeEtcd) Get(_ context.Context, key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return "", false, errClosed
	}
	e, ok := f.data[key]
	if !ok || f.expired(e) {
		if ok {
			f.drop(key)
		}
		return "", false, nil
	}
	return e.value, true, nil
}

func (f *fakeEtcd) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return errClosed
	}
	f.drop(key)
	return nil
}

// PutIfNotExists 仅当 key 不存在（或已过期）时写入，返回是否写入成功
func (f *fakeEtcd) PutIfNotExists(_ context.Context, key, value string, leaseID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return false, errClosed
	}
	if e, ok := f.data[key]; ok && !f.expired(e) {
		return false, nil // 已被别人持有
	}
	f.drop(key)
	f.data[key] = &entry{value: value, leaseID: leaseID, ttl: f.ttls[leaseID]}
	return true, nil
}

// DeleteIfValue 仅当 key 当前值 == value 时删除（CAS 防误删）
func (f *fakeEtcd) DeleteIfValue(_ context.Context, key, value string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return false, errClosed
	}
	e, ok := f.data[key]
	if ok && !f.expired(e) && e.value == value {
		f.drop(key)
		return true, nil
	}
	return false, nil
}

func (f *fakeEtcd) LeaseGrant(_ context.Context, ttl time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, errClosed
	}
	f.leaseSeq++
	f.keepAlive[f.leaseSeq] = time.Now() // 创建即"续租"，避免立刻过期
	f.ttls[f.leaseSeq] = ttl
	return f.leaseSeq, nil
}

func (f *fakeEtcd) LeaseRevoke(_ context.Context, leaseID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return errClosed
	}
	delete(f.keepAlive, leaseID)
	delete(f.ttls, leaseID)
	for k, e := range f.data {
		if e.leaseID == leaseID {
			delete(f.data, k)
		}
	}
	return nil
}

func (f *fakeEtcd) LeaseKeepAlive(ctx context.Context, leaseID int64) (chan struct{}, error) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil, errClosed
	}
	gen := f.gen
	localStop := f.stopCh
	ttl := f.ttls[leaseID]
	f.mu.Unlock()

	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(ttl / 4)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				f.touch(leaseID)
			case <-stop:
				return
			case <-localStop:
				if f.currentGen() != gen { // 本代际被 stopAllKeepAlives 叫停
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return stop, nil
}

func (f *fakeEtcd) touch(leaseID int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.keepAlive[leaseID]; ok {
		f.keepAlive[leaseID] = time.Now()
	}
}

func (f *fakeEtcd) currentGen() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gen
}

// stopAllKeepAlives 模拟"所有持有者崩溃/断连"：停止全部续租，但存储仍可用
func (f *fakeEtcd) stopAllKeepAlives() {
	f.mu.Lock()
	f.gen++
	close(f.stopCh)
	f.stopCh = make(chan struct{})
	f.mu.Unlock()
}

func (f *fakeEtcd) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	f.gen++
	close(f.stopCh)
	return nil
}

func (f *fakeEtcd) len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.data)
}

var errClosed = errors.New("fake etcd closed")

// ---------------------------------------------------------------------------
// 测试
// ---------------------------------------------------------------------------

func TestTryLock_Exclusive(t *testing.T) {
	client := newFakeEtcd()
	ctx := context.Background()

	l1 := NewDistributedLock(client, "/locks/task", "node-1", time.Second)
	l2 := NewDistributedLock(client, "/locks/task", "node-2", time.Second)

	ok, err := l1.TryLock(ctx)
	if err != nil || !ok {
		t.Fatalf("l1 TryLock = %v, %v; want true, nil", ok, err)
	}
	ok, err = l2.TryLock(ctx)
	if err != nil || ok {
		t.Fatalf("l2 TryLock = %v, %v; want false, nil（l1 已持有）", ok, err)
	}
	if err := l1.Unlock(); err != nil {
		t.Fatalf("l1 Unlock: %v", err)
	}
	ok, err = l2.TryLock(ctx)
	if err != nil || !ok {
		t.Fatalf("l2 TryLock after unlock = %v, %v; want true", ok, err)
	}
	if err := l2.Unlock(); err != nil {
		t.Fatalf("l2 Unlock: %v", err)
	}
	if n := client.len(); n != 0 {
		t.Fatalf("锁释放后 etcd 仍残留 %d 个 key", n)
	}
}

func TestLock_MutualExclusion(t *testing.T) {
	client := newFakeEtcd()
	ctx := context.Background()

	var (
		cur, max int32
		wg       sync.WaitGroup
	)
	const n = 10
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l := NewDistributedLock(client, "/locks/task", "node-"+string(rune('a'+i)), 2*time.Second)
			if err := l.Lock(ctx); err != nil {
				t.Errorf("Lock: %v", err)
				return
			}
			c := atomic.AddInt32(&cur, 1)
			for {
				m := atomic.LoadInt32(&max)
				if c <= m || atomic.CompareAndSwapInt32(&max, m, c) {
					break
				}
			}
			time.Sleep(15 * time.Millisecond) // 模拟临界区
			atomic.AddInt32(&cur, -1)
			if err := l.Unlock(); err != nil {
				t.Errorf("Unlock: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if m := atomic.LoadInt32(&max); m != 1 {
		t.Fatalf("临界区最大并发 = %d, want 1（互斥被破坏）", m)
	}
}

// 持锁期间续租生效（超过 TTL 仍持有）；持有者崩溃（续租停止）后锁自动释放
func TestLease_ExpiryAutoRelease(t *testing.T) {
	client := newFakeEtcd()
	ctx := context.Background()
	ttl := 250 * time.Millisecond

	l1 := NewDistributedLock(client, "/locks/leader", "leader-1", ttl)
	ok, err := l1.TryLock(ctx)
	if err != nil || !ok {
		t.Fatalf("l1 TryLock = %v, %v; want true", ok, err)
	}

	// 超过 TTL 仍持有：后台续租生效
	time.Sleep(ttl + 100*time.Millisecond)
	l2 := NewDistributedLock(client, "/locks/leader", "leader-2", ttl)
	ok, err = l2.TryLock(ctx)
	if err != nil || ok {
		t.Fatalf("续租中 l2 TryLock = %v, %v; want false（l1 仍在续租）", ok, err)
	}

	// 模拟 l1 崩溃：停止续租 → 租约过期 → 锁自动释放
	client.stopAllKeepAlives()
	time.Sleep(ttl + 150*time.Millisecond)
	ok, err = l2.TryLock(ctx)
	if err != nil || !ok {
		t.Fatalf("l1 崩溃后 l2 TryLock = %v, %v; want true（锁应自动释放）", ok, err)
	}
	_ = l2.Unlock()
}

// 锁"过期易主"后，旧持有者 Unlock 不能删掉新持有者的锁（CAS 防误删）
func TestUnlock_NonOwner(t *testing.T) {
	client := newFakeEtcd()
	ctx := context.Background()
	ttl := 250 * time.Millisecond

	l1 := NewDistributedLock(client, "/locks/task", "old-owner", ttl)
	if ok, _ := l1.TryLock(ctx); !ok {
		t.Fatal("l1 应拿到锁")
	}

	// l1 的租约过期（l1 毫不知情，仍认为持有）
	client.stopAllKeepAlives()
	time.Sleep(ttl + 150*time.Millisecond)

	// l2 抢到锁（易主）
	l2 := NewDistributedLock(client, "/locks/task", "new-owner", ttl)
	if ok, _ := l2.TryLock(ctx); !ok {
		t.Fatal("l2 应在 l1 过期后拿到锁")
	}

	// l1 迟到的 Unlock：CAS 失败，删不掉 l2 的锁
	if err := l1.Unlock(); err != nil {
		t.Fatalf("l1 Unlock: %v", err)
	}
	l3 := NewDistributedLock(client, "/locks/task", "third", ttl)
	if ok, _ := l3.TryLock(ctx); ok {
		t.Fatal("l3 不应拿到锁：l1 的迟到 Unlock 误删了 l2 的锁（CAS 未生效）")
	}
	_ = l2.Unlock()
	if ok, _ := l3.TryLock(ctx); !ok {
		t.Fatal("l2 释放后 l3 应能拿到锁")
	}
	_ = l3.Unlock()
}

func TestLock_BlockingAndContextCancel(t *testing.T) {
	client := newFakeEtcd()
	l1 := NewDistributedLock(client, "/locks/task", "node-1", time.Second)
	if ok, _ := l1.TryLock(context.Background()); !ok {
		t.Fatal("l1 应拿到锁")
	}

	// l2 阻塞等待，150ms 超时后放弃
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	l2 := NewDistributedLock(client, "/locks/task", "node-2", time.Second)
	if err := l2.Lock(ctx); err != context.DeadlineExceeded {
		t.Fatalf("l2.Lock 超时 ctx = %v, want context.DeadlineExceeded", err)
	}

	// l1 释放后，l2 立刻能拿到
	if err := l1.Unlock(); err != nil {
		t.Fatal(err)
	}
	if err := l2.Lock(context.Background()); err != nil {
		t.Fatalf("l1 释放后 l2.Lock = %v, want nil", err)
	}
	_ = l2.Unlock()
}
