package monolith_split

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ---------- domain ----------

// Task 剪辑任务实体（domain 层：不依赖任何基础设施）
type Task struct {
	ID        string
	UserID    string
	Title     string
	VideoURL  string
	Status    string
	ResultURL string
}

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusDone       = "done"
	StatusFailed     = "failed"
)

// CreateTaskReq 创建任务请求（含幂等键）
type CreateTaskReq struct {
	IDempotencyKey string
	UserID         string
	Title          string
	VideoURL       string
}

var (
	ErrInvalidRequest = errors.New("invalid task request")
	ErrDuplicate      = errors.New("duplicate idempotency key")
)

// validate 校验入参（从单体里"拎出来"的第一段职责）
func (r CreateTaskReq) validate() error {
	if r.IDempotencyKey == "" || r.UserID == "" || r.Title == "" || r.VideoURL == "" {
		return ErrInvalidRequest
	}
	return nil
}

// ---------- 基础设施接口（生产：MySQL / Redis / Kafka；测试：fake） ----------

// TaskStore 任务仓储（生产 MySQL）
type TaskStore interface {
	Save(ctx context.Context, t *Task) error
	Get(ctx context.Context, id string) (*Task, error)
	UpdateStatus(ctx context.Context, id, status, resultURL string) error
}

// Idempotency 幂等键（生产 Redis SETNX）
type Idempotency interface {
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// Publisher 异步消息发布（生产 Kafka）
type Publisher interface {
	Publish(ctx context.Context, topic string, msg []byte) error
}

// ---------- app（用例层：只依赖接口 = 依赖倒置） ----------

// ClipApp 编排"创建任务"与"处理结果"两个用例，不关心存储/缓存/队列的具体实现
type ClipApp struct {
	store   TaskStore
	idem    Idempotency
	pub     Publisher
	idemTTL time.Duration
}

// NewClipApp 构造器注入：所有依赖以接口形式传入，绝不在内部 new 具体实现
func NewClipApp(store TaskStore, idem Idempotency, pub Publisher, idemTTL time.Duration) *ClipApp {
	return &ClipApp{store: store, idem: idem, pub: pub, idemTTL: idemTTL}
}

// CreateTask 用例：校验 → 幂等占坑 → 落库 pending → 发布异步任务
func (a *ClipApp) CreateTask(ctx context.Context, req CreateTaskReq) (*Task, error) {
	// 1) 校验（先校验，零副作用）
	if err := req.validate(); err != nil {
		return nil, err
	}
	// 2) 幂等占坑：同一幂等键只允许创建一次（生产 Redis SETNX）
	ok, err := a.idem.Claim(ctx, "idem:"+req.IDempotencyKey, a.idemTTL)
	if err != nil {
		return nil, fmt.Errorf("claim idempotency: %w", err)
	}
	if !ok {
		return nil, ErrDuplicate
	}
	// 3) 落库 pending
	task := &Task{
		ID:       req.IDempotencyKey, // 简化：任务 ID 复用幂等键；生产可各自独立
		UserID:   req.UserID,
		Title:    req.Title,
		VideoURL: req.VideoURL,
		Status:   StatusPending,
	}
	if err := a.store.Save(ctx, task); err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}
	// 4) 发布异步任务（Kafka clip-task topic）
	body, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshal task: %w", err)
	}
	if err := a.pub.Publish(ctx, "clip-task", body); err != nil {
		// 发布失败：任务已落库 pending —— 不 rollback，靠 worker 定期重扫补偿
		// （at-least-once + 消费幂等，见 README 验收追问）
		return nil, fmt.Errorf("publish task: %w", err)
	}
	return task, nil
}

// ProcessResult worker 处理完成后的回调：幂等更新，已终态不覆盖
func (a *ClipApp) ProcessResult(ctx context.Context, taskID, resultURL string) error {
	cur, err := a.store.Get(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	// 消费幂等：Kafka at-least-once 会重复回调，终态直接跳过
	if cur.Status == StatusDone || cur.Status == StatusFailed {
		return nil
	}
	if err := a.store.UpdateStatus(ctx, taskID, StatusDone, resultURL); err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}
