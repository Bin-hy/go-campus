# 单体拆分重构：把"上帝函数"拆成有边界的模块（依赖注入 + 仓储模式）

## 难度：⭐⭐⭐ 中等偏难

## 考点
- **边界识别**：一个函数里混了几种职责（校验 / DB / 缓存 / 队列 / 日志）？该归哪个模块？
- **依赖倒置 + 构造器注入**：用例层（app）只依赖接口，不依赖具体实现
- 仓储模式（Repository）、幂等键、事件发布
- context 贯穿、错误包装（`%w`）、并发安全

## 题目描述

下面是"拆之前"的 AI 剪辑单体代码——`createTask` 把**校验、直连 MySQL、Redis SETNX、Kafka 发布、打日志**全部内联在一个函数里（大厂管这叫"上帝函数"，是拆分的头号改造对象）：

```go
// ---- 拆之前的单体（一坨）----
func createTask(userID, title, videoURL, idempotencyKey string) (*Task, error) {
	if userID == "" || title == "" || videoURL == "" || idempotencyKey == "" {
		log.Printf("[monolith] invalid request user=%s", userID)
		return nil, errors.New("invalid request")
	}
	// 直连 MySQL
	db, _ := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/clip")
	row := db.QueryRow("SELECT 1 FROM idem_keys WHERE k=?", idempotencyKey)
	var one int
	if err := row.Scan(&one); err == nil {
		return nil, errors.New("duplicate")
	}
	// 写幂等键
	db.Exec("INSERT INTO idem_keys(k) VALUES(?)", idempotencyKey)
	// 直连 Redis
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	ok, _ := rdb.SetNX(context.Background(), "idem:"+idempotencyKey, 1, time.Hour).Result()
	if !ok {
		return nil, errors.New("duplicate")
	}
	// 直连 Kafka
	w := kafka.NewWriter(kafka.WriterConfig{Brokers: []string{"127.0.0.1:9092"}, Topic: "clip-task"})
	task := &Task{ID: idempotencyKey, UserID: userID, Title: title, VideoURL: videoURL, Status: "pending"}
	w.WriteMessages(context.Background(), kafka.Message{Value: mustJSON(task)})
	// 直连 MySQL 落库
	db.Exec("INSERT INTO tasks(id,user_id,title,video_url,status) VALUES(?,?,?,?,?)",
		task.ID, task.UserID, task.Title, task.VideoURL, task.Status)
	log.Printf("[monolith] task created %s", task.ID)
	return task, nil
}
```

**问题（先想清楚再动手）**：
1. 这个函数混了几种职责？各自应该归到哪个模块（domain / store / cache / broker / app）？
2. 如果"素材服务"要独立部署，这段代码里哪部分会被一起拆走？为什么边界要画在"职责"上而不是"代码行"上？
3. 直接 `sql.Open` / `redis.NewClient` / `kafka.NewWriter` 写死在内联，有什么问题？（换 DB、单测、并行开发各踩一脚）

**任务**：用**接口 + 构造器注入**重构为有边界的四层，行为与单体一致：

| 层 | 职责 | 接口 |
| --- | --- | --- |
| `domain` | Task 实体、状态常量、请求校验 | `Task` / `CreateTaskReq` |
| `store` | 任务持久化（生产 MySQL） | `TaskStore` |
| `idem` | 幂等键（生产 Redis SETNX） | `Idempotency` |
| `broker` | 异步发布（生产 Kafka） | `Publisher` |
| `app` | 用例编排：CreateTask / ProcessResult，**只依赖接口** | `ClipApp` |

## 函数签名

```go
// ---------- domain ----------
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
	IDempotencyKey string // 幂等键：同一键只允许创建一次
	UserID         string
	Title          string
	VideoURL       string
}

var (
	ErrInvalidRequest = errors.New("invalid task request")
	ErrDuplicate      = errors.New("duplicate idempotency key")
)

// ---------- 基础设施接口（生产：MySQL / Redis / Kafka） ----------
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

// ---------- app（用例层，只依赖接口 = 依赖倒置） ----------
type ClipApp struct{ /* 自行设计 */ }

func NewClipApp(store TaskStore, idem Idempotency, pub Publisher, idemTTL time.Duration) *ClipApp
func (a *ClipApp) CreateTask(ctx context.Context, req CreateTaskReq) (*Task, error)
func (a *ClipApp) ProcessResult(ctx context.Context, taskID, resultURL string) error
```

## 提示
1. **CreateTask 流程（顺序很重要）**：先校验 → `idem.Claim("idem:"+key)` 幂等占坑 → `store.Save(pending)` → `pub.Publish("clip-task", json)` → 返回；任何一步失败都要想清楚"已产生的副作用怎么补偿"（发布失败时任务已落库 pending——生产靠 worker 重扫补偿，注释说明即可）；
2. **ProcessResult（worker 回调）必须幂等**：先 `store.Get` 查当前状态，已终态（done/failed）直接返回，不覆盖 resultURL——Kafka at-least-once 重投会重复回调，不幂等就会把已完成的任务状态打回 processing；
3. **构造器注入**：`NewClipApp` 收接口，**不要**在 `ClipApp` 内部 `sql.Open` / `redis.NewClient` / `kafka.NewWriter`（这就是"依赖倒置"，单体里写死实现是拆不掉的第一原因）；
4. **并发安全**：fake 实现里 map 要加锁；`go test -race` 验证；
5. **错误包装**：跨层错误用 `fmt.Errorf("...: %w", err)`，保留错误链，别吞错误。

## 与真实工程对照（面试加分）
- 这就是大厂拆分的第一步：**先在代码层把边界画出来（接口），再谈部署拆分**——代码边界不清，拆出来的"服务"只是把一坨搬进另一个进程；
- `ClipApp` 只依赖接口 → 单测全用 fake，不需要 MySQL/Redis/Kafka 环境（本练习就是这样）；生产实现各自在 `store/mysql.go`、`idem/redis.go`、`broker/kafka.go` 里，互不干扰，这就是"每个服务独占自己的数据/中间件"的雏形（Database-per-service 见文档 Day 9 9.4）。

## 验收
- [ ] `CreateTask` 校验失败：**零副作用**（不 Claim、不 Save、不 Publish）
- [ ] 相同幂等键第二次创建 → `ErrDuplicate`，**不重复发布**（Publish 只 1 次）
- [ ] `ProcessResult` 幂等：done 后再次回调，resultURL 不被覆盖
- [ ] `go test -race ./09_microservice/01_monolith_split -v` 无 data race
- [ ] 追问：发布失败但任务已落库，怎么补偿？答：pending 状态 + worker 定期重扫重投（at-least-once + 消费幂等），而不是在发布前 rollback 落库
