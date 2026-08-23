package clip_service_capstone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ---------- 领域 ----------

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

// Task 剪辑任务实体（生产：MySQL tasks 表）
type Task struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	VideoURL  string `json:"video_url"`
	Title     string `json:"title"`
	Status    Status `json:"status"`
	ResultURL string `json:"result_url,omitempty"`
}

// CreateTaskReq 创建任务请求
type CreateTaskReq struct {
	ID             string
	IDempotencyKey string // 幂等键：同一键只允许创建一次
	UserID         string
	VideoURL       string
	Title          string
}

var (
	ErrInvalidRequest = errors.New("invalid clip task request")
	ErrDuplicate      = errors.New("duplicate idempotency key")
	ErrNoInference    = errors.New("no available inference instance")
	ErrNotLeader      = errors.New("not leader")
	ErrClosed         = errors.New("service closed")
)

func (r CreateTaskReq) validate() error {
	if r.ID == "" || r.IDempotencyKey == "" || r.UserID == "" || r.VideoURL == "" {
		return ErrInvalidRequest
	}
	return nil
}

// ---------- 基础设施接口（生产：MySQL / Redis / Kafka / etcd / gRPC） ----------

// TaskStore 任务仓储（生产 MySQL）
type TaskStore interface {
	Save(ctx context.Context, t *Task) error
	Get(ctx context.Context, id string) (*Task, error)
	UpdateStatus(ctx context.Context, id string, st Status, resultURL string) error
	ListByStatus(ctx context.Context, st Status) ([]*Task, error)
}

// IdempotencyStore 幂等键（生产 Redis SETNX）
type IdempotencyStore interface {
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// TaskBroker 异步消息（生产 Kafka）
type TaskBroker interface {
	Publish(ctx context.Context, topic string, msg []byte) error
}

// ServiceRegistry 服务注册中心（生产 etcd）
type ServiceRegistry interface {
	Register(ctx context.Context, key, value string, ttl time.Duration) (deregister func(), err error)
	Discover(ctx context.Context, prefix string) ([]string, error)
}

// DistributedLock 分布式锁 / 选主（生产 etcd Lease + Txn）
type DistributedLock interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (unlock func(), err error)
}

// InferenceTransport 一次推理 RPC（生产 gRPC 一元调用）
type InferenceTransport func(ctx context.Context, addr, method string, req any) (any, error)

// Options 服务配置
type Options struct {
	WorkerCount      int
	RegisterKey      string // 本服务注册 key（含地址）
	RegisterValue    string
	InferencePrefix  string // 推理服务在注册中心的 key 前缀
	TTL              time.Duration
	IdempotencyTTL   time.Duration
	InferenceTimeout time.Duration
	InferenceRetry   int
	InboxSize        int
}

const (
	topicClipTask = "clip-task"
	leaderKey     = "/leader/clip-scheduler"
	inferenceMtd  = "Inference.Generate"
	workerTimeout = 30 * time.Second
)

// ---------- ClipService ----------

// ClipService AI 剪辑任务服务：用例编排 + worker 池 + 注册/选主 + 治理
type ClipService struct {
	store     TaskStore
	idem      IdempotencyStore
	broker    TaskBroker
	registry  ServiceRegistry
	lock      DistributedLock
	transport InferenceTransport
	opts      Options

	mu           sync.Mutex
	started      bool
	closed       bool
	leader       bool
	leaderUnlock func()
	dereg        func()
	inbox        chan []byte
	stop         chan struct{}
	wg           sync.WaitGroup
}

func NewClipService(store TaskStore, idem IdempotencyStore, broker TaskBroker,
	registry ServiceRegistry, lock DistributedLock, transport InferenceTransport,
	opts Options) *ClipService {
	if opts.WorkerCount <= 0 {
		opts.WorkerCount = 1
	}
	if opts.InferenceRetry <= 0 {
		opts.InferenceRetry = 1
	}
	if opts.InboxSize <= 0 {
		opts.InboxSize = 64
	}
	return &ClipService{
		store: store, idem: idem, broker: broker,
		registry: registry, lock: lock, transport: transport,
		opts: opts,
	}
}

// Start 启动服务：注册到注册中心 → 尝试选主 → 启动 worker 池
// 注册失败必须报错（注册不了就别上线）；选主失败不是错误（当 follower）
func (s *ClipService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}

	dereg, err := s.registry.Register(ctx, s.opts.RegisterKey, s.opts.RegisterValue, s.opts.TTL)
	if err != nil {
		return fmt.Errorf("register to registry: %w", err)
	}
	s.dereg = dereg

	// 选主：抢到 /leader/clip-scheduler 就是 leader；抢不到就是 follower（watch 等接管）
	if unlock, err := s.lock.TryLock(ctx, leaderKey, s.opts.TTL); err == nil && unlock != nil {
		s.leader = true
		s.leaderUnlock = unlock
	}

	s.inbox = make(chan []byte, s.opts.InboxSize)
	s.stop = make(chan struct{})
	for i := 0; i < s.opts.WorkerCount; i++ {
		s.wg.Add(1)
		go s.workerLoop()
	}
	s.started = true
	return nil
}

// workerLoop 并发消费 inbox（生产里等价于 Kafka consumer group 的一个消费者）
func (s *ClipService) workerLoop() {
	defer s.wg.Done()
	handle := func(raw []byte) {
		// 单条任务加超时，防止一条卡死整个 worker
		ctx, cancel := context.WithTimeout(context.Background(), workerTimeout)
		err := s.HandleIncoming(ctx, raw)
		cancel()
		if err != nil {
			// 生产：记录指标 + 按策略重投（本骨架由 HandleIncoming 返回值暴露错误）
			_ = err
		}
	}
	for {
		select {
		case raw := <-s.inbox:
			handle(raw)
		case <-s.stop:
			// 优雅关闭：先排空 inbox 里剩余任务，再退出（在途任务全部处理完）
			for {
				select {
				case raw := <-s.inbox:
					handle(raw)
				default:
					return
				}
			}
		}
	}
}

// SubmitIncoming broker 回调入口：生产由 Kafka consumer 触发，这里入队给 worker
func (s *ClipService) SubmitIncoming(ctx context.Context, raw []byte) error {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return ErrClosed // 服务已关闭：不再接收新消息（确定性检查）
	}
	select {
	case s.inbox <- raw:
		return nil
	case <-s.stop:
		return ErrClosed // 关闭竞态兜底
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CreateTask 用户提交任务：校验 → 幂等占坑 → 落库 pending → 发布异步任务
func (s *ClipService) CreateTask(ctx context.Context, req CreateTaskReq) (*Task, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	ok, err := s.idem.Claim(ctx, "idem:"+req.IDempotencyKey, s.opts.IdempotencyTTL)
	if err != nil {
		return nil, fmt.Errorf("claim idempotency: %w", err)
	}
	if !ok {
		return nil, ErrDuplicate
	}

	task := &Task{
		ID:       req.ID,
		UserID:   req.UserID,
		VideoURL: req.VideoURL,
		Title:    req.Title,
		Status:   StatusPending,
	}
	if err := s.store.Save(ctx, task); err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}
	body, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshal task: %w", err)
	}
	if err := s.broker.Publish(ctx, topicClipTask, body); err != nil {
		// 发布失败不 rollback：pending + RunMaintenance 重扫重投（at-least-once）
		return nil, fmt.Errorf("publish task: %w", err)
	}
	return task, nil
}

// HandleIncoming worker 处理一条任务（消费幂等）：
// 终态跳过 → processing → callInference（发现 + 超时 + 重试）→ done/failed
func (s *ClipService) HandleIncoming(ctx context.Context, raw []byte) error {
	var task Task
	if err := json.Unmarshal(raw, &task); err != nil {
		return fmt.Errorf("unmarshal task: %w", err)
	}
	cur, err := s.store.Get(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	// 消费幂等：Kafka at-least-once 会重投，终态直接跳过
	if cur.Status == StatusDone || cur.Status == StatusFailed {
		return nil
	}
	if err := s.store.UpdateStatus(ctx, task.ID, StatusProcessing, ""); err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}

	url, err := s.callInference(ctx, cur)
	if err != nil {
		_ = s.store.UpdateStatus(ctx, task.ID, StatusFailed, "")
		return fmt.Errorf("inference: %w", err)
	}
	if err := s.store.UpdateStatus(ctx, task.ID, StatusDone, url); err != nil {
		return fmt.Errorf("mark done: %w", err)
	}
	return nil
}

// callInference 通过注册中心发现推理实例，带单次超时 + 失败重试（Day 8 治理模式内联）
func (s *ClipService) callInference(ctx context.Context, task *Task) (string, error) {
	addrs, err := s.registry.Discover(ctx, s.opts.InferencePrefix)
	if err != nil {
		return "", fmt.Errorf("discover inference: %w", err)
	}
	if len(addrs) == 0 {
		return "", ErrNoInference
	}
	var lastErr error
	for attempt := 0; attempt < s.opts.InferenceRetry; attempt++ {
		addr := addrs[attempt%len(addrs)] // 轮询换实例重试
		callCtx, cancel := context.WithTimeout(ctx, s.opts.InferenceTimeout)
		resp, err := s.transport(callCtx, addr, inferenceMtd, task)
		cancel()
		if err == nil {
			url, _ := resp.(string)
			return url, nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("inference failed after %d tries: %w", s.opts.InferenceRetry, lastErr)
}

// RunMaintenance 仅 leader 可执行：重投长期 stuck 的 pending 任务（对账补偿）
func (s *ClipService) RunMaintenance(ctx context.Context) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}
	stuck, err := s.store.ListByStatus(ctx, StatusPending)
	if err != nil {
		return fmt.Errorf("list stuck tasks: %w", err)
	}
	for _, t := range stuck {
		body, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("marshal task %s: %w", t.ID, err)
		}
		if err := s.broker.Publish(ctx, topicClipTask, body); err != nil {
			return fmt.Errorf("republish task %s: %w", t.ID, err)
		}
	}
	return nil
}

func (s *ClipService) IsLeader() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leader
}

// Close 优雅关闭：先反注册/释放锁（摘流量），再停 worker 等排空；幂等
func (s *ClipService) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if s.dereg != nil {
		s.dereg()
	}
	if s.leaderUnlock != nil {
		s.leaderUnlock()
	}
	stop := s.stop
	s.mu.Unlock()

	if stop != nil {
		close(stop) // worker 排空剩余任务后退出
		s.wg.Wait()
	}
	return nil
}
