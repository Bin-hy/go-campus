package registry_lb

import (
	"errors"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// 参考答案 / 可运行实现。

var ErrNoInstance = errors.New("no available instance")

// Registry 是内存版服务注册中心：
// 注册 + 心跳续租 + TTL 过期剔除 + 发现。
// 生产对应 etcd 的 Lease + KeepAlive + Watch（见文档 07 章）。
type Registry struct {
	mu       sync.Mutex
	ttl      time.Duration
	now      func() time.Time
	services map[string]map[string]time.Time // service -> addr -> 最后心跳时间
}

func NewRegistry(ttl time.Duration) *Registry {
	return &Registry{
		ttl:      ttl,
		now:      time.Now,
		services: make(map[string]map[string]time.Time),
	}
}

// SetClock 注入时钟，便于测试 TTL 而不真的 sleep。
func (r *Registry) SetClock(fn func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = fn
}

// Register 注册实例；重复注册等价于一次心跳。
func (r *Registry) Register(service, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.services[service]
	if m == nil {
		m = make(map[string]time.Time)
		r.services[service] = m
	}
	m[addr] = r.now()
}

// Heartbeat 续租；实例不存在时返回错误（防止"幽灵实例"复活）。
func (r *Registry) Heartbeat(service, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.services[service]
	if m == nil {
		return errors.New("service not registered")
	}
	if _, ok := m[addr]; !ok {
		return errors.New("instance not registered")
	}
	m[addr] = r.now()
	return nil
}

// Deregister 主动下线：先摘流量再停进程。
func (r *Registry) Deregister(service, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.services[service], addr)
}

// Discover 返回健康（未过期）实例，按地址排序保证可预测。
func (r *Registry) Discover(service string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	m := r.services[service]
	out := make([]string, 0, len(m))
	for addr, lastSeen := range m {
		if now.Sub(lastSeen) >= r.ttl {
			delete(m, addr) // 惰性剔除：发现时顺手清理
			continue
		}
		out = append(out, addr)
	}
	sort.Strings(out)
	return out
}

// RoundRobin 无状态轮询。
type RoundRobin struct {
	next atomic.Uint64
}

func (rr *RoundRobin) Pick(addrs []string) (string, error) {
	if len(addrs) == 0 {
		return "", ErrNoInstance
	}
	i := rr.next.Add(1) - 1
	return addrs[i%uint64(len(addrs))], nil
}

// P2C（Power of Two Choices）：随机抽两个，选负载低的那个。
// 它比纯轮询更能感知真实负载，是 envoy/gRPC 生产常用的策略之一。
type P2C struct {
	mu   sync.Mutex
	rnd  *rand.Rand
	load map[string]*atomic.Int64
}

func NewP2C(seed int64) *P2C {
	return &P2C{
		rnd:  rand.New(rand.NewSource(seed)),
		load: make(map[string]*atomic.Int64),
	}
}

func (p *P2C) counter(addr string) *atomic.Int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.load[addr]
	if c == nil {
		c = &atomic.Int64{}
		p.load[addr] = c
	}
	return c
}

// Begin 标记一个在途请求；调用方必须在完成后 Done。
func (p *P2C) Begin(addr string) { p.counter(addr).Add(1) }
func (p *P2C) Done(addr string)  { p.counter(addr).Add(-1) }
func (p *P2C) Load(addr string) int64 {
	return p.counter(addr).Load()
}

func (p *P2C) Pick(addrs []string) (string, error) {
	if len(addrs) == 0 {
		return "", ErrNoInstance
	}
	if len(addrs) == 1 {
		return addrs[0], nil
	}
	p.mu.Lock()
	i := p.rnd.Intn(len(addrs))
	j := p.rnd.Intn(len(addrs) - 1)
	if j >= i {
		j++
	}
	p.mu.Unlock()

	a, b := addrs[i], addrs[j]
	if p.Load(a) <= p.Load(b) {
		return a, nil
	}
	return b, nil
}
