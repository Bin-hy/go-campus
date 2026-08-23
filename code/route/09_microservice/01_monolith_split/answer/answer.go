//go:build ignore

package answer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

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

func (r CreateTaskReq) validate() error {
	if r.IDempotencyKey == "" || r.UserID == "" || r.Title == "" || r.VideoURL == "" {
		return ErrInvalidRequest
	}
	return nil
}

type TaskStore interface {
	Save(ctx context.Context, t *Task) error
	Get(ctx context.Context, id string) (*Task, error)
	UpdateStatus(ctx context.Context, id, status, resultURL string) error
}

type Idempotency interface {
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

type Publisher interface {
	Publish(ctx context.Context, topic string, msg []byte) error
}

// ClipApp 参考答案：用例层只依赖接口（依赖倒置 + 构造器注入）
type ClipApp struct {
	store   TaskStore
	idem    Idempotency
	pub     Publisher
	idemTTL time.Duration
}

func NewClipApp(store TaskStore, idem Idempotency, pub Publisher, idemTTL time.Duration) *ClipApp {
	return &ClipApp{store: store, idem: idem, pub: pub, idemTTL: idemTTL}
}

func (a *ClipApp) CreateTask(ctx context.Context, req CreateTaskReq) (*Task, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	ok, err := a.idem.Claim(ctx, "idem:"+req.IDempotencyKey, a.idemTTL)
	if err != nil {
		return nil, fmt.Errorf("claim idempotency: %w", err)
	}
	if !ok {
		return nil, ErrDuplicate
	}
	task := &Task{
		ID:       req.IDempotencyKey,
		UserID:   req.UserID,
		Title:    req.Title,
		VideoURL: req.VideoURL,
		Status:   StatusPending,
	}
	if err := a.store.Save(ctx, task); err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}
	body, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshal task: %w", err)
	}
	if err := a.pub.Publish(ctx, "clip-task", body); err != nil {
		// 发布失败不 rollback：pending 状态 + worker 重扫补偿
		return nil, fmt.Errorf("publish task: %w", err)
	}
	return task, nil
}

func (a *ClipApp) ProcessResult(ctx context.Context, taskID, resultURL string) error {
	cur, err := a.store.Get(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	if cur.Status == StatusDone || cur.Status == StatusFailed {
		return nil // 消费幂等：终态不覆盖
	}
	if err := a.store.UpdateStatus(ctx, taskID, StatusDone, resultURL); err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}
