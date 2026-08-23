package clip_service_capstone

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fakes（生产对应 MySQL / Redis / Kafka / etcd / gRPC）
// ---------------------------------------------------------------------------

type fakeStore struct {
	mu    sync.Mutex
	tasks map[string]*Task
}

func newFakeStore() *fakeStore { return &fakeStore{tasks: map[string]*Task{}} }

func (f *fakeStore) seed(t *Task) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tasks[t.ID] = t
}

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

func (f *fakeStore) UpdateStatus(_ context.Context, id string, st Status, resultURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tasks[id]
	if !ok {
		return errors.New("task not found")
	}
	t.Status = st
	t.ResultURL = resultURL
	return nil
}

func (f *fakeStore) ListByStatus(_ context.Context, st Status) ([]*Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*Task
	for _, t := range f.tasks {
		if t.Status == st {
			cp := *t
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeStore) status(id string) Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tasks[id]; ok {
		return t.Status
	}
	return ""
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

type pubRec struct {
	topic string
	msg   []byte
}

type fakeBroker struct {
	mu   sync.Mutex
	pubs []pubRec
	fail atomic.Bool
}

func newFakeBroker() *fakeBroker { return &fakeBroker{} }

func (f *fakeBroker) Publish(_ context.Context, topic string, msg []byte) error {
	if f.fail.Load() {
		return errors.New("kafka down")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pubs = append(f.pubs, pubRec{topic: topic, msg: append([]byte{}, msg...)})
	return nil
}

func (f *fakeBroker) count(topic string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, p := range f.pubs {
		if p.topic == topic {
			n++
		}
	}
	return n
}

type fakeRegistry struct {
	mu           sync.Mutex
	registered   map[string]string
	deregistered []string
	discover     []string
}

func newFakeRegistry() *fakeRegistry { return &fakeRegistry{registered: map[string]string{}} }

func (f *fakeRegistry) Register(_ context.Context, key, value string, _ time.Duration) (func(), error) {
	f.mu.Lock()
	f.registered[key] = value
	f.mu.Unlock()
	return func() {
		f.mu.Lock()
		delete(f.registered, key)
		f.deregistered = append(f.deregistered, key)
		f.mu.Unlock()
	}, nil
}

func (f *fakeRegistry) Discover(_ context.Context, _ string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.discover...), nil
}

func (f *fakeRegistry) isRegistered(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.registered[key]
	return ok
}

func (f *fakeRegistry) wasDeregistered(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range f.deregistered {
		if k == key {
			return true
		}
	}
	return false
}

// fakeLock：第一个 TryLock 者拿到锁（选主）
type fakeLock struct {
	mu    sync.Mutex
	owner string
}

func (f *fakeLock) TryLock(_ context.Context, key string, _ time.Duration) (func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.owner != "" {
		return nil, nil // 已被别人持有 → follower
	}
	f.owner = key
	return func() {
		f.mu.Lock()
		if f.owner == key {
			f.owner = ""
		}
		f.mu.Unlock()
	}, nil
}

// fakeTransport：可替换行为 + 按地址计数
type fakeTransport struct {
	mu     sync.Mutex
	fn     func(ctx context.Context, addr, method string, req any) (any, error)
	counts map[string]int
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{counts: map[string]int{}}
}

func (f *fakeTransport) call(ctx context.Context, addr, method string, req any) (any, error) {
	f.mu.Lock()
	f.counts[addr]++
	fn := f.fn
	f.mu.Unlock()
	if fn == nil {
		return "https://cdn/result.mp4", nil
	}
	return fn(ctx, addr, method, req)
}

func (f *fakeTransport) count(addr string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[addr]
}

// ---------------------------------------------------------------------------
// 工具
// ---------------------------------------------------------------------------

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("条件在 %v 内未满足", timeout)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func baseOpts() Options {
	return Options{
		WorkerCount:      2,
		RegisterKey:      "/services/clip/10.0.0.1:8080",
		RegisterValue:    "clip-api",
		InferencePrefix:  "/services/inference",
		TTL:              time.Minute,
		IdempotencyTTL:   time.Minute,
		InferenceTimeout: time.Second,
		InferenceRetry:   2,
		InboxSize:        64,
	}
}

// ---------------------------------------------------------------------------
// 测试
// ---------------------------------------------------------------------------

func TestCreateTask_HappyPath(t *testing.T) {
	store, idem, broker := newFakeStore(), newFakeIdem(), newFakeBroker()
	svc := NewClipService(store, idem, broker, newFakeRegistry(), &fakeLock{}, newFakeTransport().call, baseOpts())

	task, err := svc.CreateTask(context.Background(), CreateTaskReq{
		ID: "t-1", IDempotencyKey: "k-1", UserID: "u-1", VideoURL: "https://cdn/a.mp4", Title: "混剪",
	})
	if err != nil {
		t.Fatalf("CreateTask err = %v", err)
	}
	if task.Status != StatusPending {
		t.Fatalf("status = %s, want pending", task.Status)
	}
	if store.status("t-1") != StatusPending {
		t.Fatal("任务应落库为 pending")
	}
	if n := broker.count(topicClipTask); n != 1 {
		t.Fatalf("publish count = %d, want 1", n)
	}
}

func TestCreateTask_DuplicateAndInvalid(t *testing.T) {
	store, idem, broker := newFakeStore(), newFakeIdem(), newFakeBroker()
	svc := NewClipService(store, idem, broker, newFakeRegistry(), &fakeLock{}, newFakeTransport().call, baseOpts())

	req := CreateTaskReq{ID: "t-2", IDempotencyKey: "k-2", UserID: "u", VideoURL: "v", Title: "t"}
	if _, err := svc.CreateTask(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateTask(context.Background(), req); err != ErrDuplicate {
		t.Fatalf("第二次 err = %v, want ErrDuplicate", err)
	}
	if n := broker.count(topicClipTask); n != 1 {
		t.Fatalf("publish count = %d, want 1（重复创建不重复发布）", n)
	}

	// 校验失败：零副作用
	before := broker.count(topicClipTask)
	if _, err := svc.CreateTask(context.Background(), CreateTaskReq{ID: "bad"}); err != ErrInvalidRequest {
		t.Fatalf("err = %v, want ErrInvalidRequest", err)
	}
	if broker.count(topicClipTask) != before {
		t.Fatal("校验失败不应发布")
	}
}

func TestHandleIncoming_Success(t *testing.T) {
	store := newFakeStore()
	store.seed(&Task{ID: "t-3", Status: StatusPending})
	registry := newFakeRegistry()
	registry.discover = []string{"inf-1"}
	transport := newFakeTransport()
	svc := NewClipService(store, newFakeIdem(), newFakeBroker(), registry, &fakeLock{}, transport.call, baseOpts())

	err := svc.HandleIncoming(context.Background(), mustJSON(t, Task{ID: "t-3"}))
	if err != nil {
		t.Fatalf("HandleIncoming err = %v", err)
	}
	got, _ := store.Get(context.Background(), "t-3")
	if got.Status != StatusDone || got.ResultURL != "https://cdn/result.mp4" {
		t.Fatalf("got = %+v, want done + resultURL", got)
	}
	if n := transport.count("inf-1"); n != 1 {
		t.Fatalf("inference calls = %d, want 1", n)
	}
}

func TestHandleIncoming_RetryThenSuccess(t *testing.T) {
	store := newFakeStore()
	store.seed(&Task{ID: "t-4", Status: StatusPending})
	registry := newFakeRegistry()
	registry.discover = []string{"inf-1", "inf-2"}
	transport := newFakeTransport()
	first := true
	transport.fn = func(_ context.Context, addr, _ string, _ any) (any, error) {
		if first {
			first = false
			return nil, errors.New("inference down: " + addr)
		}
		return "https://cdn/retry.mp4", nil
	}
	svc := NewClipService(store, newFakeIdem(), newFakeBroker(), registry, &fakeLock{}, transport.call, baseOpts())

	if err := svc.HandleIncoming(context.Background(), mustJSON(t, Task{ID: "t-4"})); err != nil {
		t.Fatalf("HandleIncoming err = %v（应重试成功）", err)
	}
	got, _ := store.Get(context.Background(), "t-4")
	if got.Status != StatusDone || got.ResultURL != "https://cdn/retry.mp4" {
		t.Fatalf("got = %+v, want done + retry.mp4", got)
	}
	// 轮询换实例重试：inf-1 失败 1 次，inf-2 成功 1 次
	if transport.count("inf-1") != 1 || transport.count("inf-2") != 1 {
		t.Fatalf("calls inf-1=%d inf-2=%d, want 1/1", transport.count("inf-1"), transport.count("inf-2"))
	}
}

func TestHandleIncoming_FailureSetsFailed(t *testing.T) {
	store := newFakeStore()
	store.seed(&Task{ID: "t-5", Status: StatusPending})
	registry := newFakeRegistry()
	registry.discover = []string{"inf-1"}
	transport := newFakeTransport()
	transport.fn = func(_ context.Context, _ string, _ string, _ any) (any, error) {
		return nil, errors.New("always down")
	}
	svc := NewClipService(store, newFakeIdem(), newFakeBroker(), registry, &fakeLock{}, transport.call, baseOpts())

	if err := svc.HandleIncoming(context.Background(), mustJSON(t, Task{ID: "t-5"})); err == nil {
		t.Fatal("推理一直失败应返回错误")
	}
	got, _ := store.Get(context.Background(), "t-5")
	if got.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if n := transport.count("inf-1"); n != baseOpts().InferenceRetry {
		t.Fatalf("inference calls = %d, want %d（重试耗尽）", n, baseOpts().InferenceRetry)
	}
}

// 消费幂等：已 done 的任务重投 → 直接跳过，不再调推理
func TestHandleIncoming_SkipDone(t *testing.T) {
	store := newFakeStore()
	store.seed(&Task{ID: "t-6", Status: StatusDone, ResultURL: "https://cdn/x.mp4"})
	registry := newFakeRegistry()
	registry.discover = []string{"inf-1"}
	transport := newFakeTransport()
	svc := NewClipService(store, newFakeIdem(), newFakeBroker(), registry, &fakeLock{}, transport.call, baseOpts())

	if err := svc.HandleIncoming(context.Background(), mustJSON(t, Task{ID: "t-6"})); err != nil {
		t.Fatalf("err = %v", err)
	}
	if n := transport.count("inf-1"); n != 0 {
		t.Fatalf("inference calls = %d, want 0（终态跳过）", n)
	}
	got, _ := store.Get(context.Background(), "t-6")
	if got.ResultURL != "https://cdn/x.mp4" {
		t.Fatalf("resultURL 被覆盖 = %s", got.ResultURL)
	}
}

// worker 池：3 worker 并发消费 6 条任务全部完成；关闭后 SubmitIncoming 返回 ErrClosed
func TestWorkerPool_AndGracefulShutdown(t *testing.T) {
	store := newFakeStore()
	registry := newFakeRegistry()
	registry.discover = []string{"inf-1"}
	transport := newFakeTransport()
	transport.fn = func(_ context.Context, _ string, _ string, _ any) (any, error) {
		time.Sleep(20 * time.Millisecond) // 模拟推理耗时
		return "https://cdn/w.mp4", nil
	}
	opts := baseOpts()
	opts.WorkerCount = 3
	svc := NewClipService(store, newFakeIdem(), newFakeBroker(), registry, &fakeLock{}, transport.call, opts)

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start err = %v", err)
	}
	ids := []string{"w-1", "w-2", "w-3", "w-4", "w-5", "w-6"}
	for _, id := range ids {
		store.seed(&Task{ID: id, Status: StatusPending})
	}
	for _, id := range ids {
		if err := svc.SubmitIncoming(context.Background(), mustJSON(t, Task{ID: id})); err != nil {
			t.Fatalf("SubmitIncoming err = %v", err)
		}
	}
	waitUntil(t, 5*time.Second, func() bool {
		for _, id := range ids {
			if store.status(id) != StatusDone {
				return false
			}
		}
		return true
	})
	if n := transport.count("inf-1"); n != 6 {
		t.Fatalf("inference calls = %d, want 6", n)
	}

	if err := svc.Close(); err != nil {
		t.Fatalf("Close err = %v", err)
	}
	if err := svc.Close(); err != nil { // 幂等
		t.Fatalf("第二次 Close err = %v", err)
	}
	if err := svc.SubmitIncoming(context.Background(), mustJSON(t, Task{ID: "w-7"})); err != ErrClosed {
		t.Fatalf("关闭后 SubmitIncoming err = %v, want ErrClosed", err)
	}
}

func TestLeaderElection_AndMaintenance(t *testing.T) {
	lock := &fakeLock{}
	storeA, storeB := newFakeStore(), newFakeStore()
	brokerA, brokerB := newFakeBroker(), newFakeBroker()

	svcA := NewClipService(storeA, newFakeIdem(), brokerA, newFakeRegistry(), lock, newFakeTransport().call, baseOpts())
	svcB := NewClipService(storeB, newFakeIdem(), brokerB, newFakeRegistry(), lock, newFakeTransport().call, baseOpts())

	if err := svcA.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svcB.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !svcA.IsLeader() || svcB.IsLeader() {
		t.Fatalf("leader: A=%v B=%v, want A leader / B follower", svcA.IsLeader(), svcB.IsLeader())
	}

	// 只有 leader 能跑维护：重投 stuck 的 pending 任务
	storeA.seed(&Task{ID: "stuck-1", Status: StatusPending})
	storeA.seed(&Task{ID: "stuck-2", Status: StatusPending})
	if err := svcA.RunMaintenance(context.Background()); err != nil {
		t.Fatalf("leader RunMaintenance err = %v", err)
	}
	if n := brokerA.count(topicClipTask); n != 2 {
		t.Fatalf("leader 重投 %d 条, want 2", n)
	}
	if err := svcB.RunMaintenance(context.Background()); err != ErrNotLeader {
		t.Fatalf("follower RunMaintenance err = %v, want ErrNotLeader", err)
	}
	if n := brokerB.count(topicClipTask); n != 0 {
		t.Fatalf("follower 不应重投，实际 %d 条", n)
	}
	_ = svcA.Close()
	_ = svcB.Close()
}

// 注册生命周期：Start 注册本服务，Close 反注册（优雅下线）
func TestRegistry_Lifecycle(t *testing.T) {
	store := newFakeStore()
	registry := newFakeRegistry()
	svc := NewClipService(store, newFakeIdem(), newFakeBroker(), registry, &fakeLock{}, newFakeTransport().call, baseOpts())

	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !registry.isRegistered("/services/clip/10.0.0.1:8080") {
		t.Fatal("Start 后本服务应已注册")
	}
	if err := svc.Close(); err != nil {
		t.Fatal(err)
	}
	if !registry.wasDeregistered("/services/clip/10.0.0.1:8080") {
		t.Fatal("Close 后本服务应已反注册（优雅下线）")
	}
}
