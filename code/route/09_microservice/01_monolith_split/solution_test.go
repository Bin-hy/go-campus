package monolith_split

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fakes（生产对应 MySQL / Redis / Kafka）
// ---------------------------------------------------------------------------

type fakeStore struct {
	mu    sync.Mutex
	tasks map[string]*Task
}

func newFakeStore() *fakeStore { return &fakeStore{tasks: map[string]*Task{}} }

func (f *fakeStore) Save(_ context.Context, t *Task) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *t
	f.tasks[t.ID] = &cp
	return nil
}

func (f *fakeStore) Get(_ context.Context, id string) (*Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[id]
	if !ok {
		return nil, errors.New("task not found")
	}
	cp := *t
	return &cp, nil
}

func (f *fakeStore) UpdateStatus(_ context.Context, id, status, resultURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[id]
	if !ok {
		return errors.New("task not found")
	}
	t.Status = status
	t.ResultURL = resultURL
	return nil
}

func (f *fakeStore) len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.tasks)
}

type fakeIdem struct {
	mu   sync.Mutex
	keys map[string]bool
}

func newFakeIdem() *fakeIdem { return &fakeIdem{keys: map[string]bool{}} }

func (f *fakeIdem) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.keys[key] {
		return false, nil
	}
	f.keys[key] = true
	return true, nil
}

type fakePub struct {
	mu      sync.Mutex
	topics  map[string]int // topic -> 发布次数
	lastMsg []byte
	fail    atomic.Bool
}

func newFakePub() *fakePub { return &fakePub{topics: map[string]int{}} }

func (f *fakePub) Publish(_ context.Context, topic string, msg []byte) error {
	if f.fail.Load() {
		return errors.New("kafka down")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.topics[topic]++
	f.lastMsg = append([]byte{}, msg...)
	return nil
}

func (f *fakePub) count(topic string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.topics[topic]
}

// ---------------------------------------------------------------------------
// 测试
// ---------------------------------------------------------------------------

func TestCreateTask_HappyPath(t *testing.T) {
	store, idem, pub := newFakeStore(), newFakeIdem(), newFakePub()
	app := NewClipApp(store, idem, pub, time.Minute)

	task, err := app.CreateTask(context.Background(), CreateTaskReq{
		IDempotencyKey: "task-1",
		UserID:         "u-1",
		Title:          "婚礼混剪",
		VideoURL:       "https://cdn/a.mp4",
	})
	if err != nil {
		t.Fatalf("CreateTask err = %v", err)
	}
	if task.Status != StatusPending {
		t.Fatalf("status = %s, want pending", task.Status)
	}
	if store.len() != 1 {
		t.Fatalf("store len = %d, want 1", store.len())
	}
	if n := pub.count("clip-task"); n != 1 {
		t.Fatalf("publish count = %d, want 1", n)
	}
	if len(pub.lastMsg) == 0 {
		t.Fatal("publish 的消息不应为空")
	}
}

func TestCreateTask_InvalidNoSideEffect(t *testing.T) {
	store, idem, pub := newFakeStore(), newFakeIdem(), newFakePub()
	app := NewClipApp(store, idem, pub, time.Minute)

	_, err := app.CreateTask(context.Background(), CreateTaskReq{
		IDempotencyKey: "task-bad",
		Title:          "缺 UserID 和 VideoURL",
	})
	if err != ErrInvalidRequest {
		t.Fatalf("err = %v, want ErrInvalidRequest", err)
	}
	// 校验失败 = 零副作用
	if store.len() != 0 {
		t.Fatalf("校验失败却落库了 %d 条", store.len())
	}
	if n := pub.count("clip-task"); n != 0 {
		t.Fatalf("校验失败却发布了 %d 条", n)
	}
	if len(idem.keys) != 0 {
		t.Fatalf("校验失败却占用了 %d 个幂等键", len(idem.keys))
	}
}

func TestCreateTask_DuplicateIdempotency(t *testing.T) {
	store, idem, pub := newFakeStore(), newFakeIdem(), newFakePub()
	app := NewClipApp(store, idem, pub, time.Minute)

	req := CreateTaskReq{IDempotencyKey: "task-dup", UserID: "u", Title: "t", VideoURL: "v"}
	if _, err := app.CreateTask(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateTask(context.Background(), req); err != ErrDuplicate {
		t.Fatalf("第二次创建 err = %v, want ErrDuplicate", err)
	}
	if n := pub.count("clip-task"); n != 1 {
		t.Fatalf("publish count = %d, want 1（重复创建不得重复发布）", n)
	}
	if store.len() != 1 {
		t.Fatalf("store len = %d, want 1", store.len())
	}
}

func TestCreateTask_PublishFail(t *testing.T) {
	store, idem, pub := newFakeStore(), newFakeIdem(), newFakePub()
	app := NewClipApp(store, idem, pub, time.Minute)
	pub.fail.Store(true)

	_, err := app.CreateTask(context.Background(), CreateTaskReq{
		IDempotencyKey: "task-pubfail", UserID: "u", Title: "t", VideoURL: "v",
	})
	if err == nil {
		t.Fatal("发布失败应返回错误")
	}
	// 任务已落库 pending：靠 worker 重扫补偿（不 rollback，见 README）
	if store.len() != 1 {
		t.Fatalf("store len = %d, want 1（任务应已落库，等待补偿）", store.len())
	}
	got, _ := store.Get(context.Background(), "task-pubfail")
	if got.Status != StatusPending {
		t.Fatalf("status = %s, want pending（等待重扫补偿）", got.Status)
	}
}

func TestProcessResult_Idempotent(t *testing.T) {
	store, idem, pub := newFakeStore(), newFakeIdem(), newFakePub()
	app := NewClipApp(store, idem, pub, time.Minute)

	// 先创建（pending）
	req := CreateTaskReq{IDempotencyKey: "task-p", UserID: "u", Title: "t", VideoURL: "v"}
	if _, err := app.CreateTask(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	// 第一次回调：pending → done
	if err := app.ProcessResult(context.Background(), "task-p", "https://cdn/out.mp4"); err != nil {
		t.Fatalf("ProcessResult err = %v", err)
	}
	// 第二次回调（Kafka 重投）：终态不覆盖 resultURL
	if err := app.ProcessResult(context.Background(), "task-p", "https://cdn/evil.mp4"); err != nil {
		t.Fatalf("第二次 ProcessResult err = %v", err)
	}
	got, _ := store.Get(context.Background(), "task-p")
	if got.ResultURL != "https://cdn/out.mp4" {
		t.Fatalf("resultURL = %s, want 首次回调的值（幂等未生效）", got.ResultURL)
	}
	if got.Status != StatusDone {
		t.Fatalf("status = %s, want done", got.Status)
	}
}

// 并发创建同一幂等键：只能成功 1 个，发布 1 次
func TestCreateTask_ConcurrentSameKey(t *testing.T) {
	store, idem, pub := newFakeStore(), newFakeIdem(), newFakePub()
	app := NewClipApp(store, idem, pub, time.Minute)

	const n = 20
	var wg sync.WaitGroup
	var created, dup int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := app.CreateTask(context.Background(), CreateTaskReq{
				IDempotencyKey: "task-race", UserID: "u", Title: "t", VideoURL: "v",
			})
			switch {
			case err == nil:
				atomic.AddInt32(&created, 1)
			case errors.Is(err, ErrDuplicate):
				atomic.AddInt32(&dup, 1)
			default:
				t.Errorf("unexpected err: %v", err)
			}
		}()
	}
	wg.Wait()
	if created != 1 {
		t.Fatalf("成功创建 %d 次, want 1", created)
	}
	if int(created+dup) != n {
		t.Fatalf("created+dup = %d, want %d", created+dup, n)
	}
	if p := pub.count("clip-task"); p != 1 {
		t.Fatalf("publish count = %d, want 1", p)
	}
}
