//go:build ignore

package answer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

type Task struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	VideoURL  string `json:"video_url"`
	Title     string `json:"title"`
	Status    Status `json:"status"`
	ResultURL string `json:"result_url,omitempty"`
}

type CreateTaskReq struct {
	ID             string
	IDempotencyKey string
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

type TaskStore interface {
	Save(ctx context.Context, t *Task) error
	Get(ctx context.Context, id string) (*Task, error)
	UpdateStatus(ctx context.Context, id string, st Status, resultURL string) error
	ListByStatus(ctx context.Context, st Status) ([]*Task, error)
}

type IdempotencyStore interface {
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

type TaskBroker interface {
	Publish(ctx context.Context, topic string, msg []byte) error
}

type ServiceRegistry interface {
	Register(ctx context.Context, key, value string, ttl time.Duration) (deregister func(), err error)
	Discover(ctx context.Context, prefix string) ([]string, error)
}

type DistributedLock interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (unlock func(), err error)
}

type InferenceTransport func(ctx context.Context, addr, method string, req any) (any, error)

type Options struct {
	WorkerCount      int
	RegisterKey      string
	RegisterValue    string
	InferencePrefix  string
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

// ClipService 参考答案：生产级微服务骨架（混合 Day1/5/6/8/9 全部知识点）
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

func (s *ClipService) workerLoop() {
	defer s.wg.Done()
	handle := func(raw []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), workerTimeout)
		err := s.HandleIncoming(ctx, raw)
		cancel()
		_ = err
	}
	for {
		select {
		case raw := <-s.inbox:
			handle(raw)
		case <-s.stop:
			for { // 排空剩余任务后退出
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

func (s *ClipService) SubmitIncoming(ctx context.Context, raw []byte) error {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return ErrClosed
	}
	select {
	case s.inbox <- raw:
		return nil
	case <-s.stop:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

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
		ID: req.ID, UserID: req.UserID, VideoURL: req.VideoURL,
		Title: req.Title, Status: StatusPending,
	}
	if err := s.store.Save(ctx, task); err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}
	body, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshal task: %w", err)
	}
	if err := s.broker.Publish(ctx, topicClipTask, body); err != nil {
		return nil, fmt.Errorf("publish task: %w", err)
	}
	return task, nil
}

func (s *ClipService) HandleIncoming(ctx context.Context, raw []byte) error {
	var task Task
	if err := json.Unmarshal(raw, &task); err != nil {
		return fmt.Errorf("unmarshal task: %w", err)
	}
	cur, err := s.store.Get(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	if cur.Status == StatusDone || cur.Status == StatusFailed {
		return nil // 消费幂等
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
		addr := addrs[attempt%len(addrs)]
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
		close(stop)
		s.wg.Wait()
	}
	return nil
}
