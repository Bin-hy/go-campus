package registry_lb

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var ErrNoInstance = errors.New("no available instance")

// Registry 是内存版服务注册中心：
// 注册 + 心跳续租 + TTL 过期剔除 + 发现。
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
	// TODO: 实现你的代码
	panic("not implemented")
}

// Heartbeat 续租；实例不存在时返回错误（防止"幽灵实例"复活）。
func (r *Registry) Heartbeat(service, addr string) error {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Deregister 主动下线。
func (r *Registry) Deregister(service, addr string) {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Discover 返回健康（未过期）实例，按地址排序保证可预测。
func (r *Registry) Discover(service string) []string {
	// TODO: 实现你的代码（超过 TTL 未心跳的实例应剔除）
	panic("not implemented")
}

// RoundRobin 无状态轮询。
type RoundRobin struct {
	next atomic.Uint64
}

func (rr *RoundRobin) Pick(addrs []string) (string, error) {
	// TODO: 实现你的代码（空列表返回 ErrNoInstance）
	panic("not implemented")
}

// P2C（Power of Two Choices）：随机抽两个，选负载低的那个。
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

// Begin 标记一个在途请求；调用方必须在完成后 Done。
func (p *P2C) Begin(addr string) {
	// TODO: 实现你的代码
	panic("not implemented")
}

func (p *P2C) Done(addr string) {
	// TODO: 实现你的代码
	panic("not implemented")
}

func (p *P2C) Load(addr string) int64 {
	// TODO: 实现你的代码
	panic("not implemented")
}

func (p *P2C) Pick(addrs []string) (string, error) {
	// TODO: 实现你的代码（随机抽两个不同下标，选在途更少的）
	panic("not implemented")
}
