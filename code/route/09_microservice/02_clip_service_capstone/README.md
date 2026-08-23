# AI 剪辑任务服务：生产级微服务骨架（综合实战）

## 难度：⭐⭐⭐⭐⭐ 最难（大厂生产形态，混合多个大章节）

> 这是后端专题的**毕业设计**：把 Day 1（并发/Context）、Day 5（Redis 幂等）、Day 6（Kafka 异步）、Day 8（etcd 注册/发现/选主、RPC 超时重试治理）、Day 9（仓储模式/依赖注入/消费幂等/优雅关闭）**全部塞进一个服务**。它的形状就是大厂一个真实微服务的形状：接口抽象基础设施 + 用例编排 + 并发 worker + 全链路治理。

## 考点（混合学习清单）
- **并发**：worker 池消费 inbox、channel 关闭语义、`sync.WaitGroup` 优雅关闭
- **Context**：请求级超时、worker 处理超时、取消传播
- **etcd**：服务注册 + 租约 + 优雅下线（`ServiceRegistry`）、选主（`DistributedLock`）
- **Redis**：幂等键（`IdempotencyStore.Claim`）
- **MySQL**：仓储模式 + 状态机流转（`TaskStore`）
- **Kafka**：异步发布 + at-least-once 消费幂等（`TaskBroker`）
- **RPC 治理**：注册中心发现实例 + 单次调用超时 + 失败重试（`callInference`）
- **工程**：错误包装（`%w`）、构造器注入、幂等关闭、测试全 fake 无外部依赖

## 题目描述

实现 `ClipService`——"用户提交剪辑任务 → 异步 worker 处理 → 调推理服务 → 回写结果"的完整服务骨架：

1. **Start**：把本服务注册到注册中心（带租约 + 返回反注册函数）→ 启动 N 个 worker 并发消费 inbox → 尝试抢 `/leader/clip-scheduler` 锁（抢到者 = leader，只有 leader 能跑维护任务）；
2. **CreateTask**：校验 → 幂等占坑（`Claim`）→ 落库 pending → 发布到 `clip-task` topic → 返回任务（重复幂等键返回 `ErrDuplicate`，不重复发布）；
3. **SubmitIncoming**：broker 回调入口（生产里由 Kafka consumer group 触发），入队给 worker；服务已关闭返回 `ErrClosed`；
4. **HandleIncoming**（worker 处理逻辑）：**消费幂等**（终态跳过，Kafka 重投安全）→ 置 processing → `callInference`（从注册中心发现推理实例，带超时 + 重试）→ 成功置 done + resultURL，失败置 failed；
5. **RunMaintenance**：仅 leader 可执行——把长期 stuck 的 pending 任务重新发布（重投补偿）；
6. **Close**：反注册 → 释放 leader 锁 → 停 worker → 等排空（幂等，可重复调用）。

## 函数签名

```go
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
	ID             string // 任务 ID
	IDempotencyKey string // 幂等键（与 ID 可不同）
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

// ---------- 基础设施接口（生产：MySQL / Redis / Kafka / etcd / gRPC） ----------
type TaskStore interface {
	Save(ctx context.Context, t *Task) error
	Get(ctx context.Context, id string) (*Task, error)
	UpdateStatus(ctx context.Context, id string, st Status, resultURL string) error
	ListByStatus(ctx context.Context, st Status) ([]*Task, error) // 维护任务用
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
	RegisterKey      string // 本服务注册 key（含地址，如 /services/clip/10.0.0.1:8080）
	RegisterValue    string // 本服务元数据
	InferencePrefix  string // 注册中心里推理服务的前缀
	TTL              time.Duration
	IdempotencyTTL   time.Duration
	InferenceTimeout time.Duration
	InferenceRetry   int
	InboxSize        int
}

type ClipService struct{ /* 自行设计 */ }

func NewClipService(store TaskStore, idem IdempotencyStore, broker TaskBroker,
	registry ServiceRegistry, lock DistributedLock, transport InferenceTransport,
	opts Options) *ClipService

func (s *ClipService) Start(ctx context.Context) error
func (s *ClipService) CreateTask(ctx context.Context, req CreateTaskReq) (*Task, error)
func (s *ClipService) SubmitIncoming(ctx context.Context, raw []byte) error
func (s *ClipService) HandleIncoming(ctx context.Context, raw []byte) error
func (s *ClipService) RunMaintenance(ctx context.Context) error
func (s *ClipService) IsLeader() bool
func (s *ClipService) Close() error
```

## 提示
1. **Start 的注册失败要返回错误**（注册不了就别上线）；选主失败**不是错误**——当 follower 即可（follower 靠 watch 在 leader 挂了以后抢锁接任，见文档 8.4.3）；
2. **workerLoop**：`for { select { case raw := <-inbox: 处理; case <-stop: return } }`；处理包一层 `context.WithTimeout(30s)` 防单条任务卡死整个 worker；
3. **SubmitIncoming**：`select` 同时监听 inbox 可写 / `stop` 已关（返回 `ErrClosed`）/ `ctx.Done()`；
4. **Close 顺序**：先反注册 + 释放锁，再 `close(stop)`，最后 `wg.Wait()` 等 worker 排空；用 `sync.Once` 或 `closed` 标志保证幂等；
5. **callInference**：`Discover(InferencePrefix)` → 无实例返回 `ErrNoInference` → 循环重试（`attempt % len(addrs)` 轮询换实例）→ 每次 `context.WithTimeout(InferenceTimeout)` 包住 transport 调用，`cancel()` 防泄漏——这就是 Day 8 治理模式的现场复用；
6. **消费幂等**：`HandleIncoming` 开头 `store.Get` 看状态，`done/failed` 直接 return nil（Kafka at-least-once 重投安全）；
7. **错误包装**：跨层 `fmt.Errorf("...: %w", err)`，测试用 `errors.Is` 断言。

## 与真实工程对照（这一段就是大厂代码的样子）
- `Start` 里"注册 + 选主 + 起 worker"对应生产 Go 服务的 `main()` 三件套：etcd 注册、leader 选举、协程池；
- `callInference` 对应生产 gRPC 客户端拦截器链（超时 + 重试 + 熔断）的**最小内联版**；
- `RunMaintenance` 对应"定时重扫补偿"（对账系统），是 at-least-once 语义闭环的最后一块；
- 全部依赖都是接口 → 测试零外部依赖（本练习的 fake），生产替换为 MySQL/Redis/Kafka/etcd/gRPC 实现即可。

## 验收
- [ ] `CreateTask` 幂等：同幂等键第二次 → `ErrDuplicate`，发布只 1 次；校验失败零副作用
- [ ] `HandleIncoming` 成功链路：pending → processing → done + resultURL，推理调用 1 次
- [ ] 推理先失败后成功：重试后成功（transport 调用 2 次）；一直失败 → 置 failed 并返回错误
- [ ] 消费幂等：已 done 的任务再次投递 → 直接跳过，不再调推理
- [ ] worker 池：3 worker 并发消费 6 条任务全部处理完成，`-race` 无冲突
- [ ] 选主：两个服务同时 Start，只有一个 `IsLeader()`；非 leader 调 `RunMaintenance` 返回 `ErrNotLeader`；leader 的 `RunMaintenance` 会重投 stuck 任务
- [ ] 注册生命周期：Start 后注册中心能看到本服务 key；Close 后反注册
- [ ] 优雅关闭：Close 幂等；关闭后 `SubmitIncoming` 返回 `ErrClosed`；worker 排空后再退出
- [ ] `go test -race ./09_microservice/02_clip_service_capstone -v` 全部通过

## 追问链（面试连问三层）
1. 为什么 `CreateTask` 先 Claim 再落库？（幂等占坑要原子，先落库再查会重复发布）
2. 发布失败任务已落库怎么办？（pending + RunMaintenance 重扫重投 = at-least-once）
3. 重投会不会重复处理？（消费幂等：终态跳过 + 状态机，而不是"处理前删消息"）
4. leader 挂了谁接任？（follower watch 锁 key，租约过期 → 抢锁 → 接管，见 8.4.3）
5. `Close` 为什么先反注册再停 worker？（先摘流量，再排空在途任务——优雅下线的标准顺序）
