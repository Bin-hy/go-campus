# BinRag（docs-rag）工程化 / 部署 / 前端 / 评估 深度技术档案

> 只读分析，未修改任何文件。所有结论均标注 `文件路径:行号`。代码中不存在的内容明确写「代码中未找到」。
> 统计口径为本次实测：Go 测试文件 63 个 / 测试函数 405 个；Python 测试函数 86 个；前端 vitest 用例 37 个（`it(` 计数）。
> 沙箱限制说明：`go test` 无法执行（`go list` 访问 `~/Library/Caches/go-build` 被拒），测试数据为静态统计。

---

## 1. 双形态部署：Web（单二进制 + go:embed）与桌面（Wails v3）

### 1.1 共享代码的边界

两条启动路径共用同一个装配包 `internal/app`，这是"同源"的唯一手段：

| 层 | Web (`cmd/server`) | 桌面 (`cmd/desktop`) | 评估 (`cmd/eval`) |
|---|---|---|---|
| 配置解析 | `app.ParseConfigFlag` → `config.LoadConfig`（`cmd/server/main.go:20-21`） | 同左（`cmd/desktop/main.go:22-23`） | 自己实现同语义逻辑（`cmd/eval/main.go:44-51`） |
| 应用装配 | `app.New(cfg)`（`cmd/server/main.go:32`） | `app.New(cfg)`（`cmd/desktop/main.go:30`） | `app.AssembleEvalDeps(cfg)`（`cmd/eval/main.go:57`） |
| HTTP 路由 | `a.Router()`（`cmd/server/main.go:41`） | `a.Router()`（`cmd/desktop/main.go:42`） | 不构建 HTTP（`internal/app/app.go:361-392` 注释明示） |
| 前端产物 | `go:embed` 进二进制（`internal/webui/embed.go:11-12`） | 同一份 embed | 不影响 |

`internal/app/app.go:1-3` 的包注释直接写明设计意图：「Web 形态（cmd/server）与桌面形态（cmd/desktop）共用本包，保证两条启动路径行为一致」。

**关键点**：桌面版**不复用** Wails 的 IPC/binding 机制，而是把完整后端当本地服务跑，窗口用 URL 加载。`cmd/desktop/main.go:1-3` 注释：「前端与 Web 版完全同源，无代理层、不依赖 Wails 专有 API」。因此前端代码里没有任何 `wails.*` 运行时依赖，构建产物对两种形态是**字节级同一份**。

### 1.2 桌面内嵌后端随机端口实现

```
cmd/desktop/main.go:36-50
36  // 内嵌 HTTP 服务：仅监听本机随机端口
37  ln, err := net.Listen("tcp", "127.0.0.1:0")
38  if err != nil { ... os.Exit(1) }
42  server := &http.Server{Handler: a.Router()}
43  go func() {
44      if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) { ... }
48  }()
49  addr := "http://" + ln.Addr().String() + "/"
50  slog.Info("内嵌服务已启动", "addr", addr)
```

细节要点：

1. **先 `net.Listen` 拿端口再 `server.Serve(ln)`**，而不是 `server.ListenAndServe()`。这是随机端口方案的必需形态——必须先绑定才能通过 `ln.Addr()` 拿到内核分配的端口号给窗口用。`cmd/server/main.go:39-42,51` 走的是另一条路：`Addr: fmt.Sprintf(":%d", cfg.Server.Port)` + `ListenAndServe()`。
2. **`127.0.0.1:0` 绑定**：随机端口 + 仅本机回环，不暴露到局域网。注意 Web 版 `Addr` 是 `":8085"`，即 `0.0.0.0`（`cmd/server/main.go:40`），对外可达。二者监听语义不同。
3. **桌面形态完全忽略 `cfg.Server.Port`**：代码中未找到桌面路径读取 `cfg.Server.Port` 的任何位置（`cmd/desktop/main.go` 全文 87 行无 `cfg.Server` 引用）。
4. **窗口 URL 取自监听结果**（`cmd/desktop/main.go:80`：`URL: addr`），窗口配置 `1280×800`，`MinWidth 960 / MinHeight 640`，背景色 `RGB(245,247,250)`（`cmd/desktop/main.go:73-81`）。
5. **macOS 生命周期**：`ApplicationShouldTerminateAfterLastWindowClosed: true`（`cmd/desktop/main.go:56-58`），关最后一个窗口即退出应用，触发 `OnShutdown`。

### 1.3 优雅退出 / 信号处理

**Web 侧**（`cmd/server/main.go:57-68`）：

```go
58  quit := make(chan os.Signal, 1)
59  signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
60  <-quit
61  slog.Info("收到退出信号，开始优雅关停")
63  shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
65  if err := server.Shutdown(shutdownCtx); err != nil { slog.Warn("HTTP 关停超时", "err", err) }
```
- 捕获 `SIGINT` + `SIGTERM`（覆盖 Ctrl-C 与容器 `docker stop`），超时 10s。
- 资源释放靠 `defer a.Close()`（`cmd/server/main.go:37`）。

**`App.Close()` 的释放顺序**（`internal/app/app.go:281-291`）：
```go
282  a.worker.Shutdown()          // 停入库 worker，等当前任务
283-287 if a.auditSink != nil { 5s ctx; a.auditSink.Shutdown(ctx) }  // flush MCP 审计
288  a.cancel()                   // 取消 root ctx
289  a.st.Close()                 // 关 PostgreSQL
```
注意 `App.Close()` **不关 vectorstore**（`vs.Close()` 只在装配失败路径被调用，如 `internal/app/app.go:131, 174`）。Qdrant gRPC 连接依赖进程退出回收。

**⚠️ 注释与代码不一致（可被挑刺）**：`cmd/server/main.go:57` 注释写「优雅关停：先停 worker（等待当前任务），再关 HTTP」，但实际执行顺序是 `server.Shutdown`（第 65 行）先跑，`defer a.Close()`（第 37 行注册，main 返回时才执行）后跑。即**先关 HTTP，再停 worker**。注释方向反了。

**桌面侧**（`cmd/desktop/main.go:59-70`）：
```go
59  OnShutdown: func() {
61      ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
63      if err := server.Shutdown(ctx); err != nil { slog.Warn("内嵌 HTTP 关停超时", ...) }
66      if err := a.Close(); err != nil { slog.Warn("后端资源释放异常", ...) }
69      slog.Info("桌面应用已退出")
70  },
```
- 桌面侧**没有 signal.Notify**：代码中未找到 `cmd/desktop` 对 `os.Signal` 的处理。`kill -TERM <pid>` 或 Ctrl-C 启动的桌面进程会直接终止，不走 `OnShutdown` → worker 任务与审计日志不 flush。
- 桌面侧 `a.Close()` 只在 `OnShutdown` 里调用，**没有 `defer`**：若 `wailsApp.Run()` 之前失败（第 83-86 行路径），后端资源不会被释放。

### 1.4 端口与配置来源

**统一优先级**（三处同构实现）：

| 优先级 | 来源 | 证据 |
|---|---|---|
| 1 | 命令行 `-c` / `--config`（同一变量两别名，解析失败静默忽略） | `internal/app/app.go:295-305` |
| 2 | 环境变量 `BINRAG_CONFIG` | `internal/config/config.go:330-332`、`internal/app/app.go:259-261` |
| 3 | 默认 `./configs/config.yaml` | `internal/config/config.go:333-335` |

- `cmd/eval` 自己实现了同语义解析（`cmd/eval/main.go:44-50`），未复用 `ParseConfigFlag`——因为它还要解析 `flag` 包的其他参数，共用会有 flag 冲突。
- **local 覆盖文件自动合并**：`LoadConfig` 会尝试读 `<主文件名>.local.yaml` 并二次 `yaml.Unmarshal` 覆盖（`internal/config/config.go:347-358, 367-378`）。这是"本地密钥不进主配置"的机制。`.gitignore:11-13` 忽略 `*.local.*`，但放行 `!*.local.yaml.example`。
- 服务端口：`cfg.Server.Port`（默认 8080，`internal/config/config.go:525-527`；示例配置为 8085，`configs/config.yaml:238`）。桌面形态不读该值。
- 评测服务地址：`eval.service_url` + `eval.internal_token`（`internal/config/config.go:35-41`），空则代理组 503（`internal/api/proxy_eval.go:48-51`）。

---

## 2. 评估体系

### 2.1 Go 侧 `cmd/eval`（自研 CLI 评估）

**入口参数**（`cmd/eval/main.go:21-30`）：

| 参数 | 默认 | 说明 |
|---|---|---|
| `-c` | 空 → `BINRAG_CONFIG` → `configs/config.yaml` | 配置路径 |
| `-d` | **必填**（空则报错退出，`cmd/eval/main.go:37-41`） | 数据集 .json/.jsonl |
| `-m` | `full` | `retrieve` / `qa` / `full` |
| `-k` | `1,3,5` | Recall@K 的 K 列表 |
| `-j` | 空 | Judge 模型（空=用 llm 配置默认） |
| `-n` | `2` | 样本并发数 |
| `-o` | 空=stdout | 报告路径；`.json` 后缀→JSON，否则 text（`cmd/eval/main.go:124-130`） |
| `-v` | false | 详细日志；否则 `slog.SetLogLoggerLevel(LevelError)`（`cmd/eval/main.go:33-35`） |

**数据集模型**（`internal/eval/dataset.go:14-25`）：
- `EvalSample{question, answer(可选), expected_ids(必填数组), kb_id(可选)}`
- 校验规则（`internal/eval/dataset.go:79-95`）：样本数 > 0；`question` 去空白非空；**`expected_ids` 必须非 nil（允许空数组，`null` 报错）**。
- 加载：`.json` 整体反序列化为 `Dataset{name, samples}`；`.jsonl` 逐行解析，`scanner.Buffer(make([]byte, 1MB), 1MB)` 允许大行（`internal/eval/dataset.go:57-76`）；未知扩展名报错（`:42`）。
- `name` 为空时取文件名 basename（`internal/eval/dataset.go:47-49`）。

**Recall@K 计算式**（两层）：

单样本判定（`internal/eval/evaluator.go:125-160`）：
1. `maxK = max(KValues)`，**只发一次检索请求** `TopK: maxK`（`:125-132`）；
2. 取回结果 ID 列表 `res.Retrieved`（`:137-139`）；
3. 对每个 K：取 `Retrieved[:K]` 做集合，若 `ExpectedIDs` 中**任一**命中则该 K 记 `true`（`:144-160`，`break` 语义 = 多期望片段只算一次命中，不是标准 Recall 的分数式）。

汇总（`internal/eval/report.go:38-63`）：
```
Recall@K = 命中样本数 / 有效样本数
有效样本 = 非 Error 且 len(Sample.ExpectedIDs) > 0      （:52-54）
valid == 0 时不写该 K 的键（:60-62），报告里省略
```
即**分母剔除错误样本与无期望片段样本**，与常见实现（分母取全量）不同。

**LLM-as-Judge 的 Prompt 与评分维度**（`internal/eval/judge.go`）：

准确性（`judge.go:14-21`），四档评分锚点：
```
你是一个严格的 RAG 问答质量评审员。请根据「标准答案」评判「模型回答」的准确性。
评分标准（0-10）：
- 9-10：完全正确，覆盖标准答案所有关键点
- 6-8：基本正确，有少量遗漏或轻微偏差
- 3-5：部分正确，存在明显错误或遗漏关键点
- 0-2：完全错误或答非所问
只输出 JSON：{"score": <0-10 的整数>}，不要输出其他内容。
```
用户消息模板：`"问题：%s\n\n标准答案：%s\n\n模型回答：%s"`（`judge.go:35`）。

忠实度（`judge.go:23-28`），二值判定：
```
你是一个严格的 RAG 忠实度评审员。请判断「模型回答」是否完全基于提供的「引用资料」，没有编造或臆测。
- 回答内容全部能在引用资料中找到依据 → {"faithful": true}
- 回答包含引用资料中没有的信息、或与引用资料矛盾 → {"faithful": false}
只输出 JSON：{"faithful": <true 或 false>}，不要输出其他内容。
```
引用资料只喂 **filename + heading**（`judge.go:58-63`），**不喂正文** `Content`——所以 Go 侧忠实度实际判的是"回答是否与来源标题/文件名所暗示的主题一致"，判别力弱于正文比对。

调用约束：`temperature = 0.0`（`judge.go:31, 86`）；`WithModel(model)` 仅在非空时追加（`judge.go:87-89`）；JSON 容错提取首个 `{` 到末个 `}`（`judge.go:94-100`）；评分越界（<0 或 >10）报错（`judge.go:46-48`）。

**三种模式的分派**（`internal/eval/evaluator.go:48-69`）：
- `retrieve` → `runRetrieve`：只检索 + Recall（`:73-80`）
- `qa` → `runQA`：检索 + 问答，收集 Answer/Sources（不评 LLM 指标，`:83-95`）
- `full` → `runFull`：检索 + 问答 + `evalJudge`（`:98-115`）
- 未知模式返回 `modeError`（`:242-250`）

**并发与容错**（`internal/eval/evaluator.go:204-239`）：`sem := make(chan struct{}, concurrency)` 信号量；每样本 goroutine 独立 `recover()` 记 panic 日志不 crash 进程（`:218-223`）；`ctx.Done()` 双重检查（入信号量前 `:224-228`，执行前 `:230-234`）。

**会话隔离**：`evalAsk` 的 sessionID 是 `fmt.Sprintf("eval-%d-%s", sampleIndex, truncate(s.Question, 20))`（`internal/eval/evaluator.go:166`），注释说明「sampleIndex 保证 sessionID 唯一，避免前缀相同样本历史互污」。

**报告字段**（`internal/eval/report.go:26-35`）：
```go
Report{
  DatasetName, Mode, TotalSamples, ErrorCount,
  RecallByK     map[int]float64 `json:"recall_by_k,omitempty"`
  AvgAccuracy   *float64        `json:"avg_accuracy,omitempty"`   // 非 nil 评分均值
  FaithfulRatio *float64        `json:"faithful_ratio,omitempty"` // true 占比
  Results       []EvalResult
}
```
`EvalResult` 结构见 `report.go:13-23`（`Recall map[int]bool`、`Accuracy *float64`、`Faithful *bool`、`Error string`）。指针语义 = "未评"与"评了 0 分"可区分。
`ComputeMetrics` 中 `AvgAccuracy` 只对非 nil 求均值（`:64-76`），`FaithfulRatio` 只对非 nil 计数（`:78-90`）。
文本报告用插入排序输出 K（`report.go:148-160`），回答截断按 rune 防中文断 UTF-8（`report.go:163-169`）。

**评估依赖装配**（`internal/app/app.go:361-392`）：`AssembleEvalDeps` 只建 embedder → vectorstore(+EnsureCollection) → bm25(CJK unigram tokenizer) → reranker → retriever → engine → llm，**不连 PostgreSQL、不起 worker、不建 HTTP 路由**（`:359-360` 注释）。`Closer` 只关 vectorstore（`:383-385`）。

### 2.2 Python RAGAS 服务在架构中的位置

**是独立服务**：`services/ragas-eval`，FastAPI + uv，端口 8090，**不发布宿主端口**（`docker-compose.yml:77-78` 注释 + `:85-112` 无 `ports:`）。

**调用链（四条数据流，`docs/35-ragas评测/architect-design.md:60-96`）**：
1. **控制流**：前端 → Go `Auth` 中间件 → Go 反向代理 → Python。Go 是纯透传代理（`internal/api/proxy_eval.go:45-84`）。
2. **采集流**：Python worker 内 `BinRagClient` 带**评测专用 API Key** 回调 Go `POST /api/v1/chat?include_contexts=true`（`services/ragas-eval/src/ragas_eval/core/collector.py:68-109`）。
3. **评测流**：Python → Judge LLM / Embedding（OpenAI 兼容端点）。
4. **持久化流**：全部落 SQLite，前端读报告只打存储。

**Go 侧代理实现**（`internal/api/proxy_eval.go`）：
- 常量：`X-Eval-Internal-Token`（`:16`）、`/api/v1/eval/health`（`:19`）、`/tasks`（`:22`）。
- `Available()` 判定：`service_url` 与 `internal_token` 均非空（`internal/config/config.go:41`），否则 503「评测服务未配置」（`proxy_eval.go:48-51`）。
- `Director` 只改 scheme/host + 注入 token（`:69-75`），路径/查询串/Body 原样。
- 唯一业务逻辑：`POST /tasks` 解析 body 取 `kb_id` 做**越权校验**，越权/不存在返 404（`:88-109`）。body 读出后必须 `io.NopCloser(bytes.NewReader(body))` 复原（`:95`）。
- body 非合法 JSON 时不拦截，交 Python 返 400/422（`:98-101`，注释「纯透传原则」）。
- `ErrorHandler` 上游不可达返 `CodeBadGateway`（`:76-81`）。
- 路由挂载 `v1.Any("/eval/*path", h.EvalProxy)`（`internal/api/router.go:141`）；`/api/v1/eval/health` 豁免 Go 鉴权用**包装中间件**实现而非单独路由（`router.go:97-105`，注释解释：Gin 不允许静态路由与 `/eval/*path` 通配共存）。

### 2.3 限流与队列设计、并发与重试

**三层并发限额**（`services/ragas-eval/src/ragas_eval/config.py:60-64` + `docs/35-ragas评测/architect-design.md §4.3`）：

| 层 | 配置项 | 默认 | 上限/钳制 | 生效位置 |
|---|---|---|---|---|
| 任务级 | `max_running_tasks` | 2 | — | worker 数 = 该值（`core/queue.py:64-65`） |
| 排队 | `max_queue_size` | 16 | 满 → `QueueFullError` → 429 | `core/queue.py:83-90` |
| 样本采集 | `task.sample_concurrency` | 4 | `max(1, min(v,8))` 双处钳制 | API `api/routes_tasks.py:105`；Semaphore `core/collector.py:164` |
| 批次 | `eval_batch_size` | 8 | — | `core/runner.py:158, 191` |
| LLM 速率 | `judge_rpm` | 60 | 令牌桶 | `llm/factory.py:73, 99` |

**队列实现**（`core/queue.py:33-122`）：进程内 `asyncio.Queue[str]`，存的是 task_id 字符串；worker pool 由 `asyncio.create_task(self._worker_loop())` 起（`:64-65`）。取消用 `dict[str, asyncio.Event]` 取消标志（`:53`），**协作式、样本边界生效**，正在进行的 LLM 调用不硬断（`:92-94` 注释 + architect-design 状态图）。空任务/非 pending 状态任务静默跳过（`:127-132`）。

**三阶段状态机**（`core/queue.py:124-285`）：
```
pending ──▶ collecting ──▶ evaluating ──▶ completed
   │            │               │
   ▼            ▼               ▼
canceled ◀──（样本边界协作取消）
任意运行态 ──▶ failed
collecting/evaluating ──▶ pending（仅启动恢复路径，断点续跑）
```
迁移表集中定义在 `core/lifecycle.py:19-34`，非法迁移抛 `IllegalTransitionError`（`:37-54`）。完成判定 `ok + failed >= total`，允许部分样本失败（`lifecycle.py:67-73`）。

**启动恢复 / 断点续跑**：`start()` 先 `reset_interrupted_tasks()` 把 collecting/evaluating 重置为 pending，再 `list_resumable_task_ids()` 全部重入队（`core/queue.py:60-68`）；样本表有 `UNIQUE(task_id, idx)`（`store/db.py:61`），已 `ok` 样本跳过（`core/queue.py:169`）。

**任务级 deadline**：`time.monotonic()` 差值超 `task_deadline`（默认 7200s）→ 迁 failed（`core/queue.py:159-160, 177-179, 253-255`）。

**重试策略（三处，退避序列不同）**：

| 调用方 | 触发重试 | 退避序列 | 不重试 | 证据 |
|---|---|---|---|---|
| 采集 Go chat | 网络错误 + 5xx | 1s / 4s（共 2 次） | 4xx（kb 不存在等）直接记样本失败 | `core/collector.py:25, 90-109` |
| Judge LLM | 网络错误 + 429 + 5xx | 1s / 2s / 4s（共 3 次） | 非 429 的 4xx（配置错误） | `llm/factory.py:29, 95-115` |
| Judge JSON 解析 | 解析失败 | 无退避，立即重试 ≤2 次 | — | `llm/factory.py:31, 117-131` |

底层隐式重试被显式关闭：`max_retries=0`（`llm/factory.py:167`），注释「重试由微服务统一控制」。

**批内失败降级**（`core/runner.py:207-242`）：`ragas.evaluate` 整批抛异常 → 逐样本重试一次 → 仍失败记 `error`（不阻断任务）。`ragas.evaluate` 是同步阻塞 API，放 `asyncio.to_thread` 避免卡事件循环（`core/runner.py:102-138`）。

**RPM 令牌桶**（`llm/ratelimit.py:13-48`）：容量默认 = rpm（允许瞬时突发 ≤ 一分钟配额），补充速率 `rpm/60` 每秒，`asyncio.Lock` + 计算等待时长 sleep。注释声明语义对齐 Go 侧 `rate.NewLimiter`。

**注入取消到 runner**：`should_cancel=lambda: self._is_cancelled(task_id) or deadline_exceeded()`（`core/queue.py:235`），在**批次边界**检查（`core/runner.py:192-195`）。

### 2.4 两套评估的关系与差异

| 维度 | Go `internal/eval` / `cmd/eval` | Python RAGAS 服务 |
|---|---|---|
| 调用形态 | 进程内直调 retriever/rag.Engine（`internal/app/app.go:361-392`） | 黑盒 HTTP，回调 Go `/api/v1/chat?include_contexts=true`（`core/collector.py:74-96`） |
| 指标 | Recall@K（检索）+ 准确性 0-10 + 忠实度布尔 | faithfulness / answer_relevancy / context_precision / context_recall（0~1） |
| 数据集 | 同一份 `EvalSample` 格式（Go `internal/eval/dataset.go:14-19`；Python `core/dataset.py:25-31`，注释「对齐 Go EvalSample 结构」） | 同左，**两侧不互相依赖**（`docs/35-ragas评测/spec.md:11`） |
| 校验口径 | Go `Validate`（`dataset.go:79-95`） | Python `_validate_sample` 逐条对齐 Go 文案（`core/dataset.py:50-69`，注释逐条标「对齐 Go」） |
| 任务化 | 无（一次性 CLI，输出即结束） | 有（异步任务 + 队列 + 报告持久化 + 幂等 + 取消 + 重启恢复） |
| 并发 | 样本级信号量（`evaluator.go:212`） | 三层限额（`config.py:60-64`） |
| 可复现 | 温度 0（`judge.go:31`） | 温度 0 + `config_snapshot`（dataset_hash / judge_model / binrag_config_hash / ragas_version / prompt_version，`api/routes_tasks.py:90-96`） |
| 并存约束 | spec N5 明确「不改动 internal/eval 既有行为；两套体系口径差异在文档中明示，**报告不直接互比**」（`docs/35-ragas评测/spec.md:40`） | — |

**为什么 Recall@K 留在 Go**：需要进程内访问 retriever（`docs/35-ragas评测/spec.md:11`：「`internal/eval` 保留 Recall@K 检索指标（需进程内访问 retriever）」）。而 RAGAS 天然黑盒、可走 HTTP，所以拆成独立服务。

**为了 RAGAS 新增的 Go 出口**：`rag.Source` 原本无 `content` 字段 → 采用 `include_contexts` 评测专用参数（`docs/35-ragas评测/architect-design.md:22`「硬约束 2」）。实现：`AskOptions.IncludeContexts`（`internal/rag/engine.go:68-70, 113-115`）、触发点 `handler_chat.go:100, 159, 206-208`、填充逻辑 `internal/rag/context.go:83`。注释明确「评测采集专用出口，默认不填充」。

### 2.5 接口路径与数据模型

**Python 端点前缀统一 `/api/v1/eval`**（`api/main.py:61-62`，注释「与 Go 代理路径完全一致，纯透传无需改写」）。

| # | 方法 | 路径 | 关键行为 | 证据 |
|---|---|---|---|---|
| 1 | GET | `/health` | status/version/checks(db/binrag_api/judge_llm/queue)；degraded 仍 **HTTP 200** | `main.py:66-101` |
| 2 | GET | `/healthz` | 裸探活，**仅 db 挂才 503** | `main.py:103-110` |
| 3 | GET | `/datasets` | `{items,total}` | `routes_datasets.py:26-30` |
| 4 | POST | `/datasets` | multipart，≤10MB(400)，格式非法 422 带行号 | `routes_datasets.py:33-57` |
| 5 | GET | `/datasets/{id}/preview` | limit 1-100 默认 5；field_stats | `routes_datasets.py:60-92` |
| 6 | DELETE | `/datasets/{id}` | 被任务引用 409 | `routes_datasets.py:95-104` |
| 7 | GET | `/judge-models` | 配置驱动，不硬编码 | `routes_tasks.py:32-47` |
| 8 | POST | `/tasks` | 201；预检依赖 503；未知指标 400；幂等 200；队列满 429 | `routes_tasks.py:50-117` |
| 9 | GET | `/tasks` | status/kb_id/page/page_size(1-100)；非法 status 400 | `routes_tasks.py:120-141` |
| 10 | GET | `/tasks/{id}` | 轮询主接口，纯 SQLite 读 | `routes_tasks.py:144-150` |
| 11 | POST | `/tasks/{id}/cancel` | 仅运行态可取消，其余 409 | `routes_tasks.py:153-172` |
| 12 | DELETE | `/tasks/{id}` | 运行中需先取消 409 | `routes_tasks.py:175-184` |
| 13 | GET | `/tasks/{id}/report` | 未完成 409；metric_lt / sort / 分页 | `routes_reports.py:93-127` |
| 14 | GET | `/tasks/{id}/report/samples/{sid}` | 含 contexts 正文与 reason | `routes_reports.py:130-143` |
| 15 | GET | `/reports?task_ids=` | ≤8 个任务的汇总批量对比 | `routes_reports.py:146-176` |

（表列 15 项；`docs/35-ragas评测/验收报告.md:10` 记为「14 个端点」，二者对 health/healthz 的计数口径不同。）

**响应包装**统一 `{"code":0,"message":"ok","data":...}`（`api/middleware.py:34-44`），失败 code 与 HTTP 状态一致；异常处理器两级（`AppError` → 其 code；未捕获 → 500 + request_id，`middleware.py:78-102`）。所有响应回显 `X-Request-ID`（`middleware.py:57-75`）。

**鉴权**：`InternalTokenMiddleware` 校验 `x-eval-internal-token`，**配置为空令牌时同样拒绝**（不容忍裸奔，`middleware.py:63-71`）；豁免路径 `frozenset({"/api/v1/eval/health", "/healthz"})`（`middleware.py:19`）。

**SQLite 数据模型**（`store/db.py:17-74`，`SCHEMA_VERSION = 1`，`user_version` 版本化迁移 `:96-106`）：

| 表 | 关键列 | 约束/索引 |
|---|---|---|
| `datasets` | id, name, source_format, content_hash, sample_count, with_reference_count, **raw_blob**(原文), created_at | 索引 `idx_datasets_hash(content_hash)` |
| `tasks` | id, name, dataset_id→datasets, kb_id, judge_model, metrics_json, sample_concurrency, strategy, status, progress_json, error_message, **idempotency_key UNIQUE**, config_snapshot_json, created_at/started_at/finished_at | `idx_tasks_status`、`idx_tasks_created(created_at DESC)` |
| `samples` | id, task_id→tasks ON DELETE CASCADE, idx, question, reference, kb_id, expected_ids_json, answer, contexts_json, scores_json, status, error | **`UNIQUE(task_id, idx)`** ← 断点续跑幂等基石；`idx_samples_task(task_id, idx)` |
| `reports` | task_id PK→tasks CASCADE, summary_json, created_at | 任务完成时一次性算好，读报告不扫样本表 |

连接 PRAGMA：`journal_mode=WAL` + `foreign_keys=ON` + `synchronous=NORMAL`（`store/db.py:85-93`）。

**报告 summary 结构**（`core/runner.py:261-321`）：每指标 `{mean, coverage, valid_samples}`；`coverage = 有效样本/总样本`；`parse_failure_rate` 与 `reliable = rate <= 0.05`；`prompt_version`（当前 `zh-v2`，`core/prompts.py:26`）；`total_samples` / `error_samples`。`context_precision` 若有样本缺 reference 则附加 `degraded: true`（`:302-309`）；`context_recall` 缺 reference 记 `{"score": None, "reason": "N/A（缺标准答案）"}`（`:183-188`）并在 note 中说明（`:310-311`）。

**降级表**（`core/runner.py:60-68`）：有 reference → 全部四指标；无 reference → 剔除 `context_recall`，`context_precision` 换 `LLMContextPrecisionWithoutReference`（`:90-95`）。

---

## 3. 容器化与 CI/CD

### 3.1 `Dockerfile` / `Dockerfile.deploy` 多阶段（两文件内容高度一致）

两者除文件头注释外**逐行相同**（`Dockerfile:1-94` vs `Dockerfile.deploy:1-95`），差异仅在说明文字与 `docker build -f Dockerfile.deploy` 的用法注释（`Dockerfile.deploy:16-22`）。

**阶段 1 `frontend`**（`Dockerfile:29-49`）：
- 基础镜像 `node:22`（**glibc 版，非 alpine**）。理由在 `:25-28` 注释：vite 8 依赖 Rust rolldown 与 napi-rs 原生绑定（如 `@napi-rs/lzma` 仅有 `-gnu` 变体），alpine(musl) 下加载失败 → `pnpm build` 退出 1。
- 先只拷 `package.json pnpm-lock.yaml pnpm-workspace.yaml` + 两个子包 `package.json` 再 `pnpm install --frozen-lockfile`（`:36-41`）——**分层缓存**，源码变更不触发依赖重装。
- `npm install -g pnpm@10.33.0` 与根 `package.json` 的 `packageManager` 字段保持一致（`package.json:3`）。
- `COPY frontend/ ./frontend/`（**必须保持 `./frontend/` 子目录**，`:44-47` 注释解释了若用 `frontend/ ./` 会散落并覆盖根 package.json，导致 `pnpm --filter` 把 root 纳入 scope 而报 `vue-tsc: not found`）。
- `RUN pnpm --filter binrag-frontend build`——只构建前端包，跳过 `official` 站点。

**阶段 2 `backend`**（`Dockerfile:52-69`）：
- `golang:1.26-alpine`；先 `COPY go.mod go.sum` + `go mod download` 单独分层（`:57-58`）。
- `COPY . .` 后**用阶段 1 产物覆盖** `./internal/webui/dist`（`:64`）——这是 go:embed 的关键联结点。
- `ARG TARGETARCH=amd64` + `CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath -ldflags="-w -s"`（`:67-69`）。「无 CGO 可交叉编译」——桌面形态因 Wails 需要 CGO，**不进容器镜像**。

**阶段 3 runtime**（`Dockerfile:72-94`）：
- `alpine:3.20` + `ca-certificates tzdata ffmpeg`（ffmpeg 是视频抽帧运行时依赖，`:74-76`）。
- 固定 uid 1000 便于挂载卷授权（`addgroup -S -g 1000 binrag` / `adduser -S -u 1000`，`:77-78`）。
- **镜像不含任何配置/数据**：只 `COPY --from=backend /out/binrag-server ./binrag-server`（`:86`）；`.dockerignore` 整体排除 `configs`、`deploy`、`data`（`.dockerignore:15-16, 31`）。
- `RUN mkdir -p /app/configs /app/data/uploads && chown -R binrag:binrag /app`（`:88`）。
- `USER binrag`（`:90`）、`EXPOSE 8085`（`:92`）、`CMD ["./binrag-server", "-c", "/app/configs/config.yaml"]`（`:94`）。
- **fail-fast 设计**：未挂载配置时服务启动即失败，避免误用默认值运行（`:82-85` 注释；实现于 `internal/config/config.go:337-340` 读文件失败直接返回 error → `cmd/server/main.go:22-25` `os.Exit(1)`）。

**体积优化手段汇总**（均为代码/注释可依）：多阶段（丢弃 node_modules 与 Go 工具链）、`CGO_ENABLED=0`、`-trimpath`、`-ldflags="-w -s"`（去符号表与 DWARF）、runtime 仅 alpine + 一个二进制、`.dockerignore` 排除 node_modules/dist/configs/deploy/data。

**镜像大小**：仓库代码与文档中**未记载**任何镜像体积数字，不做推测。

### 3.2 ragas-eval 镜像（`services/ragas-eval/Dockerfile`）

- 阶段 1 `ghcr.io/astral-sh/uv:python3.11-bookworm`：先 `COPY pyproject.toml uv.lock` + `uv sync --frozen --no-dev --no-install-project`（分层缓存，`:13-14`），再 `COPY src` + `uv sync --frozen --no-dev`（`:17-18`）。`--frozen` 保证 uv.lock 锁定可复现。
- 阶段 2 `python:3.11-slim-bookworm`：`useradd -m -u 1000 appuser`（`:24`），只拷 `.venv`（`:29`），`ENV PATH="/app/.venv/bin:$PATH" PYTHONUNBUFFERED=1 EVAL_LISTEN_PORT=8090 EVAL_DB_PATH=/data/eval.db`（`:30-35`），`mkdir -p /data && chown appuser`（`:38`），`USER appuser`，`EXPOSE 8090`，`CMD uvicorn ragas_eval.main:app --host 0.0.0.0 --port ${EVAL_LISTEN_PORT}`（`:44`）。
- **该 Dockerfile 内无 HEALTHCHECK**——健康检查由 compose 提供。

### 3.3 Compose 各服务的依赖与卷

**`docker-compose.yml`（一键构建 + 全栈，项目名 `binrag`，`:23`）**：

| 服务 | 镜像/构建 | 端口 | 依赖 | 卷 | healthcheck |
|---|---|---|---|---|---|
| `postgres` | `postgres:16` | 默认不发布（注释掉的 5433，`:28-30`） | — | `pg_data:/var/lib/postgresql/data` | `pg_isready -U binrag`，5s/3s/10 次（`:37-41`） |
| `qdrant` | `qdrant/qdrant:latest` | 不发布 | — | `qdrant_data:/qdrant/storage` | **无** |
| `binrag-server` | `build: . / Dockerfile`，tag `ghcr.io/bin-hy/documentsrag:latest` | `8085:8085` | postgres `service_healthy`、qdrant `service_started` | config 只读文件挂载 + `data_uploads:/app/data` | `wget -q -O /dev/null http://127.0.0.1:8085/swagger/index.html`，30s/5s/3 次/start 10s（`:69-74`） |
| `ragas-eval` | `build: ./services/ragas-eval` | **不发布**（仅内网） | binrag-server `service_healthy`（`:103-105`） | `eval_data:/data` | `python -c urllib.request.urlopen('http://127.0.0.1:8090/healthz')`，30s/5s/3 次/start 15s（`:106-111`） |

命名卷 4 个：`pg_data` / `qdrant_data` / `data_uploads` / `eval_data`（`:114-118`）。
配置文件挂载：`./deploy/configs/config.docker.yaml:/app/configs/config.yaml:ro`（`:61`）——只读 + 缺失即启动失败。
ragas-eval 环境变量全部从 `.env` 注入（`RAGAS_EVAL_API_KEY` / `EVAL_INTERNAL_TOKEN` / `EVAL_JUDGE_*` / `EVAL_EMBED_*`，`:88-100`），其中 `EVAL_BINRAG_BASE_URL=http://binrag-server:8085`（`:90`）注释标「回调 Go 的内网地址」。前置条件在 `:79-84` 注释写明 3 条。

> **⚠️ 健康检查设计缺陷**：`binrag-server` 用 `/swagger/index.html` 当健康探针（`:70`）。该路由公开（`internal/api/router.go:84`），但探针语义是"HTTP 服务在"——**不校验 PostgreSQL / Qdrant 连通性**。而 `ragas-eval` 的 `depends_on: condition: service_healthy` 依赖这个探针，意味着 Go 后端起来了但依赖挂了时，评测服务仍会启动并 `preflight` 失败返 503。Go 侧**没有 `/healthz` 或 `/health` 端点**。

**`docker-compose.dev.yml`（`binrag-dev`，`:14`）**：只起 qdrant(`6333/6334`) + postgres(`5432`) 供宿主机跑后端/前端，**不构建也不运行 BinRag 自身**（`:8-9` 注释「开发时进程跑在宿主机，便于热重载调试」）。
**`docker-compose.local.yml`（32 行）**：qdrant + postgres(`5485:5432`，账号 `binhy`/`meiyoumima`)；**无 `name:` 项目名**——与其他 compose 文件不一致（`:1`）。
**`docker-compose.prod.yml`（`binrag-prod`，`:19`）**：`image: ghcr.io/bin-hy/documentsrag:${BINRAG_IMAGE_TAG:-latest}`（`:49`），不现场构建；**不包含 ragas-eval 服务**。
**`docker-compose.prod.local.yml`**：与 prod.yml 同构，仅配置挂载指向 `config.docker.local.yaml`（`:54`）；同样**不含 ragas-eval**。

> **⚠️ 缺口**：生产部署 compose 均未纳入 ragas-eval，只有一键构建的 `docker-compose.yml` 有。

### 3.4 GitHub Actions job 结构

**`ci.yml`（push main + PR，`permissions: contents: read`）** —— 单 job `build-and-test`，步骤严格有序（`:13-75`）：
1. checkout / setup-go 1.26(cache) / pnpm action / setup-node 22(cache: pnpm)；
2. **gofmt 检查**：`gofmt -l .` 非空即 fail 并打印文件清单（`:32-40`）；
3. `pnpm install --frozen-lockfile`（`:44`）；
4. `pnpm --filter binrag-frontend build`（含 `vue-tsc --noEmit`，`:47`）；
5. `pnpm --filter binrag-frontend test`（vitest，`:50`，注释「SSE 解析 / 流式降级状态机」）；
6. **安装 Wails Linux 系统依赖** `libgtk-4-dev libwebkitgtk-6.0-dev libglib2.0-dev libsoup-3.0-dev`（`:55-62`）——因 `cmd/desktop` 引入 Wails v3（CGO），`go build/vet/test ./...` 需要 pkg-config 能校验到这些库；
7. `go build ./...` → `go vet ./...` → `go test ./...`（`:64-72`）；
8. **选择性 race**：`go test -race ./internal/store/... ./internal/task/... ./internal/api/... ./internal/eval/...`（`:74-75`）。

顺序关键点：**前端 build 必须早于 Go build**——因为 `go:embed` 需要 `internal/webui/dist` 存在（`.gitignore:35-37` 只提交占位 `index.html`，assets 全部被忽略，实测 `git ls-files internal/webui/dist` 仅 1 条）。

**`docker-publish.yml`（push main / push `v*` / workflow_dispatch）**：
- 权限 `contents: read` + `packages: write`（`:25-27`）。
- 步骤：checkout → `docker/setup-qemu-action@v3` → `docker/setup-buildx-action@v3` → `docker/login-action@v3`（`GITHUB_TOKEN`）→ `docker/metadata-action@v5` → `docker/build-push-action@v6`。
- 标签规则（`:58-63`）：`type=ref,event=branch`、`type=sha,prefix=sha-`、semver <span v-pre>`{{version}}`/`{{major}}.{{minor}}`</span>、`type=raw,value=latest,enable=`<span v-pre>`{{is_default_branch}}`</span>。
- 多架构 `platforms: linux/amd64,linux/arm64`（`:70`），层缓存 `cache-from/to: type=gha,mode=max`（`:75-76`）。
- **只构建根 `Dockerfile`**（`:69`），**不构建 `services/ragas-eval`**。

**`release.yml`（push `v*`，`permissions: contents: write`）** —— 4 个 job：
1. `web-cross-build`：matrix 5 目标 `linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64`（`:22-38`），`fail-fast: false`；先建前端 → `CGO_ENABLED=0 go build ... -o dist-web/binrag-server${EXT} ./cmd/server`（`:65-66`）→ 打包 `二进制 + configs/config.yaml + README.md + start.sh|start.bat`（`:68-80`）→ `tar.gz`（windows 为 zip）→ `upload-artifact`。
2. `desktop-macos`：`macos-latest` 本机构建（Wails 需 CGO）；`go build -o bin/BinRag ./cmd/desktop`（`:121`）→ 组装 `.app` + `build/darwin/Info.plist` → `codesign --force --deep -s -`（**adhoc 签名**，`:128`）→ `hdiutil create -format UDZO` 生成 dmg（含示例配置与 README，`:130-137`）。
3. `desktop-windows`：`windows-latest` + `choco install mingw -y`（`:166-167`，MinGW 是 webview 的 CGO 依赖）→ `go build -o bin/binrag-desktop.exe ./cmd/desktop` → `Compress-Archive` 打 zip。
4. `release`：`needs: [web-cross-build, desktop-macos, desktop-windows]`（`:195`）→ `download-artifact pattern: binrag-* merge-multiple: true` → `softprops/action-gh-release@v2` + `generate_release_notes: true`。

> **⚠️ CI/CD 缺口**：
> - **Python 评测服务完全没有 CI**：`grep -rn "services/ragas-eval" .github/` 结果为空；CI 中无 `pytest` / `ruff` / `uv` 步骤。86 个 pytest 用例与 ruff 只在本地跑（`docs/35-ragas评测/验收报告.md:39`）。
> - **`docker-publish` 不依赖 `ci`**：无 `needs:`，镜像可在测试失败时照样推送。
> - **CI 未跑 `go test -race ./internal/config/... ./internal/webui/... ./internal/rag/...`**。
> - **前端无 lint**：`frontend/package.json:6-10` 仅有 `dev/build/preview/test`，无 eslint。

---

## 4. 可观测性

### 4.1 日志格式与级别

- **统一 `log/slog`**，**全部使用默认 handler（未自定义）**：全仓库 `grep "slog.SetDefault\|slog.New(\|TextHandler\|JSONHandler"` 仅命中 `cmd/eval/main.go:34` 的 `slog.SetLogLoggerLevel`。即：默认 text handler 输出到 stderr，键值对形态 `time=... level=INFO msg="HTTP 请求" method=GET path=...`。
- **唯一的级别控制**：`cmd/eval` 在非 `-v` 时 `slog.SetLogLoggerLevel(slog.LevelError)`（`cmd/eval/main.go:33-35`）。`cmd/server` / `cmd/desktop` **没有日志级别配置项**——即生产环境无法通过配置调低日志级别（可被挑刺）。
- **中文 message + 中英混杂 key**（风格不统一）：如 `"加载配置失败"`（`cmd/server/main.go:23`）、`"HTTP 服务已启动"`（`:45`）、但 key 有的中文有的英文：`"耗时ms"`（`internal/api/middleware.go:96`）vs `"method"/"path"/"status"`（`:93-95`）。
- **请求日志中间件**（`internal/api/middleware.go:88-99`）：记录 method / path / status / 耗时ms。**不记录** request_id、用户身份、客户端 IP、User-Agent。
- **启动日志**（`cmd/server/main.go:44-50`）：addr / worker / storage / auth 四项。桌面版：`"内嵌服务已启动" addr=...`（`cmd/desktop/main.go:50`）。

### 4.2 metrics / trace

**代码中未找到**：`grep -rn "prometheus\|opentelemetry\|/metrics\|otel" --include=*.go .` → 0 命中。无 Prometheus exporter、无 OpenTelemetry、无 /metrics 端点、无 tracing。

**唯一的"指标"性质出口**是评测服务的 `health.checks.queue`（`services/ragas-eval/src/ragas_eval/main.py:94`，`queue.stats()` 返回 `{running, queued}`，`core/queue.py:100-102`）与 `JudgeStats{total_calls, parse_failures, retry_events}`（`llm/factory.py:41-53`）。

### 4.3 审计日志

**只覆盖 MCP**，不覆盖普通 API：
- `mcp.AuditSink` 异步审计（`internal/mcp/audit.go:17-56`）：`Submit` 非阻塞投递到 buffered channel（默认 1024 容量，`internal/app/app.go:184`），**队列满则丢弃并 warn**，绝不阻塞主请求；后台 worker 写库（`audit.go:89`，失败仅 warn）。
- 参数截断到 `paramLimit`（默认 2000 字符）并记录截断前原始长度（`audit.go:56-60`）；结构体**无 Secret/Token 字段**（`audit.go:21` 注释）。
- 关闭时 `Shutdown(ctx)` 停收 → flush → 退出（`internal/app/app.go:283-287`，5s 超时；`internal/app/app.go:54-55` 注释「plan D8」）。
- 仅在 `cfg.Server.MCP.Enabled` 时创建（`internal/app/app.go:183-184`），默认**关闭**（`configs/config.yaml:255`，`internal/config/config.go:540` 注释「Enabled 零值即 false，安全默认」）。
- 唯一写入点：`internal/mcp/tools.go:124`。落表 `mcp_audit_logs`（`internal/store/schema.go:105`）。

### 4.4 启动自检

`app.New` 是**强前置校验的组合**，任一失败即 `os.Exit(1)`：

| 顺序 | 自检 | 失败处置 | 证据 |
|---|---|---|---|
| 1 | 配置加载 + `Validate()`（OIDC public_url/providers、多媒体 base_url、frame_strategy=scene 需 vision_embedding、eval.service_url 合法性） | exit 1 | `internal/config/config.go:612-687`；`cmd/server/main.go:21-25` |
| 2 | PostgreSQL 连接 | exit 1 | `internal/app/app.go:64-68` |
| 3 | `st.Migrate()`（CREATE TABLE IF NOT EXISTS + `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` 追加迁移） | exit 1 | `internal/app/app.go:70-74`；`internal/store/schema.go:121-125` |
| 4 | bootstrap API Key 种子（仅表空时，`sha256` 存 hash，日志 Warn 提示移除配置） | exit 1 | `internal/app/app.go:76-80, 308-332` |
| 5 | Embedder 初始化 | exit 1 | `internal/app/app.go:83-88` |
| 6 | Qdrant 连接 + **`EnsureCollection`（首次启动自动建集合）** | exit 1 | `internal/app/app.go:89-100` |
| 7 | `BuildRuntime`（LLM/reranker/retriever/engine） | exit 1 | `internal/app/app.go:127-133` |
| 8 | **`auth.NewManager`：OIDC discovery 失败即装配失败** | exit 1 | `internal/app/app.go:143-150`（注释明示） |
| 9 | `webui.Register`（embed FS 读取） | exit 1 | `internal/app/app.go:173-179` |

**失败时资源清理是逐路径手写的**（`cancel()` + `st.Close()` + `vs.Close()` + `worker.Shutdown()`，见 `:66-68, 71-74, 84-88, 90-94, 129-133, 145-149, 174-178`）——**没有统一的 cleanup 栈**，容易漏。逐条核对未发现确凿泄漏，但手写结构本身是挑刺点。

**Go 侧无 health 端点**：`grep -rn "healthz\|/health\|HealthCheck" internal/api/` 仅命中 `proxy_eval.go:19`（代理转发用的路径常量）。Web 形态没有独立的健康检查接口，容器探针只能打 `/swagger/index.html`。

**Python 侧健康检查**：`/health` 分项 + `/healthz` 裸探活（见 §2.5）；`judge_llm` 探活结果缓存 60s 防打满供应商（`main.py:86-93`，`PingCache` 实现于 `llm/factory.py:193-209`）；degraded 仍返 200 的理由：依赖不可用不代表进程异常（`main.py:70-71`）。

---

## 5. IDE / 开发体验

### 5.1 Taskfile 任务清单（`Taskfile.yml`，v3，47 行）

声明：`APP_NAME: BinRag`、`BIN_DIR: bin`、`BUNDLE_ID: com.binrag.app`（`:4-7`）。注释说明可用 `wails3 task <name>` 或 `task <name>` 执行（`:1`）。

| 任务 | desc | 命令 | 依赖 |
|---|---|---|---|
| `build` | 构建桌面应用（前端产物 + Go 二进制） | 调 `build:frontend` → `build:binary` | — |
| `build:frontend` | 前端产物（输出 `internal/webui/dist`，被 go:embed 打包） | `cd frontend && npm run build` | — |
| `build:binary` | 编译桌面入口（macOS） | `go build -trimpath -o bin/BinRag ./cmd/desktop` | — |
| `run` | 本地运行桌面应用（默认 `configs/config.local.yaml`） | `go run ./cmd/desktop -c configs/config.local.yaml` | — |
| `package` | 组装 macOS .app（含 adhoc 签名） | rm/mkdir/cp 二进制与 `build/darwin/Info.plist` + `codesign --force --deep -s -` | `deps: build` |
| `package:dmg` | 组装 .dmg 磁盘映像 | `hdiutil create -format UDZO` | `deps: package` |

**Taskfile 只覆盖桌面形态**：**没有** `test` / `lint` / `dev` / `deps` / `docker` 任务（明显的补强点）。Web 形态的构建靠 `Dockerfile` 或手工 `pnpm build && go build ./cmd/server`。

### 5.2 热重载

- **前端热重载**：`pnpm dev`（`frontend/package.json:7` → `vite`），根 `package.json:8` 提供 `dev:frontend` 快捷方式；Vite server 端口 **5173**，并配 `/api` 代理到 `http://127.0.0.1:8085`（`frontend/vite.config.ts:9-17`，注释「本地开发代理到后端服务（configs/config.local.yaml 端口）」）。
- **后端热重载**：**代码中未找到**。无 `.air.toml`、无 `nodemon`、无 `reflex`/`fresh` 配置；只能 `go run ./cmd/server -c configs/config.local.yaml` 手工重启。
- **配置热重载（运行时不重启）**：`ConfigManager` + `atomic.Pointer[RuntimeComponents]`。`app.New` 创建 `components` 原子指针并 `Store(rtComp)`（`internal/app/app.go:137-138`），`cfgMgr` 由 `config.NewConfigManager(cfgFile(cfg), cfg)` 构造（`:140`）；API 层通过 `Rebuild` 回调重建（`:156-158`）；`Engine` 每次请求 `components.Load()` 取当前引擎（`:159-165`）。
  - `rebuildComponents` 构建失败时**保持旧组件（回滚）**：`BuildRuntime` 返回 error 即 return，不 `Store`（`internal/app/rebuild.go:61-73`）。
  - `rebuildComponents` 是**自由函数**而非 App 方法，注释说明「供 New 中的 Rebuild 闭包和 App 方法共用，消除重复逻辑」（`rebuild.go:59-60`）。
  - 启动级组件（`vs` / `bm25` / `history`）**不随配置重建**，复用传入（`rebuild.go:27-28` 注释；`:41`）。
  - 桌面形态无认证路由（`oidc.public_url` 注释「桌面版不适用，保持 false」，`configs/config.yaml:267`）。

### 5.3 本地依赖启动方式

三条路径：

1. **只起依赖（推荐）**：`docker compose -f docker-compose.dev.yml up -d`（`:3`）→ qdrant 6333/6334 + postgres 5432 暴露到宿主机；项目名 `binrag-dev` 与部署环境卷/网络隔离（`:11`）。
2. **本机已有 Postgres/Qdrant**：直接 `go run ./cmd/server -c configs/config.local.yaml`（`Taskfile.yml:29` 是桌面版；Web 版需手写）。
3. **本地评测服务**：`EVAL_*` 环境变量 + `uv run uvicorn ragas_eval.main:app --port 8090`，Go 侧配 `eval.service_url=http://localhost:8090`（`docs/35-ragas评测/验收报告.md:63-66`）。

**desktop 的 `run` 任务硬编码 `configs/config.local.yaml`**（`Taskfile.yml:29`），需先自行拷 `configs/config.local-copy.local.yaml`（144 行模板）。

---

## 6. 前端关键实现

### 6.1 SSE 客户端：解析与"重连"

**为什么手写**（`frontend/src/api/chat.ts:2-3` 注释）：「EventSource 无法携带 Authorization 头，也无法用 AbortController 主动停止」。所以用 `fetch` + `ReadableStream`。

**凭据附加**（`chat.ts:7-10`）：`bearerCredential() = getStoredToken() || getStoredApiKey()`——**会话 JWT 优先 / API Key 兜底**；注释说明「SSE 不走 axios 拦截器，需手动附加」。这造成凭据逻辑在 `client.ts` 与 `chat.ts` 两处重复。

**请求**（`chat.ts:23-32`）：POST `/api/v1/chat`，头含 `Accept: text/event-stream`，body 为 `ChatRequest`，`signal` 透传。

**非 2xx 降级**（`chat.ts:34-43`）：
```ts
if (!resp.ok || !resp.body) {
  let message = `请求失败（HTTP ${resp.status}）`
  try { const body = await resp.json(); message = body?.message ?? message } catch { }
  throw new Error(message)
}
```
即**优先后端 message，回退 HTTP 状态码**。

**逐行解析循环**（`chat.ts:45-79`）：
```ts
45  const reader = resp.body.getReader()
46  const decoder = new TextDecoder()
47  let buffer = ''
48  let currentEvent = ''
51  for (;;) {
52    const { done, value } = await reader.read()
53    if (done) break
54    buffer += decoder.decode(value, { stream: true })   // stream:true 处理跨 chunk 多字节
57    while ((newlineIndex = buffer.indexOf('\n')) >= 0) {
58      const line = buffer.slice(0, newlineIndex).replace(/\r$/, '')   // 兼容 CRLF
59      buffer = buffer.slice(newlineIndex + 1)
61      if (line.startsWith('event:')) { currentEvent = line.slice(6).trim(); continue }
65      if (!line.startsWith('data:')) continue
67      const payload = line.slice(5).trim()
68      if (!payload) continue
71      try { ev = toEvent(currentEvent, JSON.parse(payload)) } catch { continue }  // 非 JSON data 行跳过
76      onEvent(ev)
77      if (ev.type === 'done') return      // done 立即返回，不再读流
```
要点：
- **粘包/半包处理**：字符串缓冲 + `indexOf('\n')` 切行，跨 `read()` 的半个事件保留在 `buffer` 里。**但事件之间靠空行分隔**——实现里空行只是"不匹配 event:/data: 而被 continue"，没有按 `\n\n` 分帧，也没有在事件结束处重置 `currentEvent`。**语义上 `currentEvent` 是"粘性"的**：若某事件只有 `data:` 没有 `event:` 行，会沿用上一个事件名。
- **`TextDecoder` 未做收尾 flush**：循环结束（`done`）时不清空 `buffer` 残留——若服务端在最后一行后不补 `\n`，最后一个事件会丢失。当前 Go 侧 `c.SSEvent` 总是补 `\n\n`（`internal/api/handler_chat.go:179-189` + 每事件 `c.Writer.Flush()`），所以实际不触发。
- **无重连**：代码中未找到重连/`retry:` 处理/`Last-Event-ID`。**"重连"在实现里不存在**——降级方式是"标记错误 + 用户手动重发"（`stores/chat.ts:167-173`）。

**事件映射**（`chat.ts:83-99`）：`thinking`→`{type:'thinking',step}`、`sources`→`{sources: data ?? []}`、`chunk`→`{content: data?.content ?? ''}`、`error`→`{message: data?.message ?? '未知错误'}`、`done`/default→`{type:'done'}`。与 Go 侧 `c.SSEvent` 名称一一对应（`internal/api/handler_chat.go:179-189`：thinking/sources/chunk/done/error）。

### 6.2 流式中断降级状态机（真实代码）

**状态**（`frontend/src/stores/chat.ts:42-49`）：`streaming: boolean`、`abortController: AbortController | null`、`messages: LocalMessage[]`（助手消息带 `sources?` / `thinking?` / `error?`，`:11-17`）。

**转移条件全部在 `send()` 里**（`stores/chat.ts:113-185`）：

| 阶段 | 行号 | 动作 |
|---|---|---|
| 入口守卫 | `:114` | `if (!question.trim() \|\| this.streaming) return` —— 流式中重复发送直接丢弃 |
| 建会话 | `:115-119` | 无 activeSessionId 则 `newSession(kbId)`；`effectiveKb = activeSession?.kbId ?? kbId` |
| 占位 | `:121-122` | push user 消息 + push **空的 assistant 消息**（`content:''`,`sources:[]`,`thinking:[]`） |
| 置流式 | `:123-124` | `streaming = true`；`abortController = new AbortController()` |
| 会话标题 | `:126-132` | 首条消息前 20 字为标题（`title === '新会话'` 才改） |
| 记下标 | `:134-135` | `assistantIndex = messages.length - 1`；`let hasContent = false` |

**事件驱动转移**（`:145-164` 的 switch）：
- `thinking` → `messages[assistantIndex].thinking?.push(ev.step)`（**不置 hasContent**）
- `sources` → 覆盖 `messages[assistantIndex].sources = ev.sources`
- `chunk` → **`hasContent = true`** + `content += ev.content`
- `error` → `error = true`；`content = ev.message || '生成回答失败'`（**覆盖式赋值，会丢弃已收到的部分内容**）
- `done` → `break`（无额外动作）

**异常与终止转移**（`:167-184`）：
```ts
167  } catch (err) {
168    const aborted = (err as Error)?.name === 'AbortError'
169    if (!aborted) {
170      this.messages[assistantIndex].error = true
171      this.messages[assistantIndex].content =
172        this.messages[assistantIndex].content || '请求失败，请检查网络或稍后重试'   // 有部分内容则保留
173    }
174  } finally {
175    this.streaming = false            // 无条件复位（AC2/AC3/AC5/AC6）
176    this.abortController = null
177    if (!hasContent && !this.messages[assistantIndex].error) {
178      this.messages[assistantIndex].content = '（无回答）'   // 空流占位（AC5）
179    }
180-183  meta.updatedAt = now; saveSessions(...)
184  }
```

**停止**（`:188-190`）：`stop() { this.abortController?.abort() }` —— 触发 `fetch` 抛 `AbortError`，走 `:168-169` 分支：**不标 error，保留已输出内容**。

**与 spec 的对应**（`docs/11-前端流式中断降级/spec.md:23-29`，F1-F7）：F1↔`:157-160`；F2↔`:168-169` + `store.stop`；F3↔`:169-173`；F4↔`chat.ts:34-43`；F5↔`:177-179`；F6↔`:174-176`；F7↔`:114` 守卫 + `switchSession` 中 `if (this.streaming) this.stop()`（`:75`）。

**单测覆盖**：`stores/chat.test.ts` 12 个 `it(`，标题即「chatStore 降级状态机」（`:1, :23`）；`api/chat.test.ts` 6 个 `it(`。

**已知不一致（可挑刺）**：`error` 事件（`:157-160`）**覆盖** content，而 `catch` 分支（`:171-172`）**保留**已有内容；F3 要求"若已有部分内容则保留并提示不完整"，F1 未明确覆盖语义——两条路径行为不同。

### 6.3 Markdown 渲染与引用卡片

**MarkdownRenderer.vue**（114 行）：
- `marked.parse(content, { async:false, breaks:true, gfm:true })`（`:31-35`）→ **DOMPurify.sanitize** 消毒（`:30`）→ `v-html`（`:42`），并带 eslint-disable 注释。
- **highlight.js 按需注册 8 种语言**：javascript/typescript/python/go/bash/json/xml/sql（`:6-24`），样式 `highlight.js/styles/github-dark.css`（`:15`）。
- 用 `computed` 缓存渲染结果（`:28`），流式增量时每 chunk 全量重渲染——未做增量/防抖。

**SourceCard.vue**（205 行）：
- 卡片列表：序号徽章 + 图标 + 文件名 + heading + `score.toFixed(2)`（`:62-77`）。
- 点击 → `openChunk(src)`：`getChunk(src.id)` 拉 chunk 详情（`:36`），失败则关弹窗 + `ElMessage.error`（`:49-52`）。
- **类型推断 + 专用阅读器路由**（`:20-27` `inferFileType`）：`source_type==='video'|'audio'` 优先，否则按扩展名 `.pdf`→pdf、`.md/.markdown`→markdown；`resolveViewer(t)` 存在才设 `fileType`（`:39`）。
- **定位信息透传**（`:41-47`）：`{page: page_number, startMs, endMs, anchor, heading}` → `DocumentViewer`。
- 无专用阅读器时回退 <span v-pre>`<pre>{{ detail.content }}</pre>`</span>（`:95`）。
- 阅读器目录：`components/viewer/{DocumentViewer,PdfViewer,MarkdownViewer,VideoViewer,AudioViewer,ViewerRegistry}` — PDF 用 `pdfjs-dist`、音频用 `wavesurfer.js`（`frontend/package.json`）。

### 6.4 图表 / 上传交互要点

**图表**（`utils/evalChart.ts` 186 行 + `components/eval/EvalChart.vue`）：
- 三个 option 工厂：`buildRadarOption`（雷达，`:55-94`）、`buildDistributionOption`（5 桶分布并列柱，`:110-147`）、`buildCompareBarOption`（对比分组柱，`:150-186`）。
- **颜色全部读 CSS 变量**（`--br-*` / `--el-*`），带 fallback（`cssVar`，`:8-12`）；对比色板亮/暗两套（`:15-22`）。不加载 ECharts 内置 dark 主题。
- **分桶**：`bucketize` 每 0.2 一桶共 5 桶，`Math.min(4, Math.max(0, Math.floor(s*5)))`（`:99-107`），NaN 跳过。
- `EvalChart.vue`：**按需注册** `echarts/core` 的 CanvasRenderer + Bar/Radar + Grid/Legend/Tooltip（`:19-23`）；**`MutationObserver` 监听 `html.dark` class 变化重建 option**（`:35-41`，卸载时 disconnect `:37-39`）；`computed` 求值 `makeOption(isDark)` 让响应式依赖自动追踪（`:28`）。
- **懒加载**：`defineAsyncComponent(() => import('.../EvalChart.vue'))`（`views/eval/EvalDetailView.vue:24`）——`docs/35-ragas评测/验收报告.md:40` 记「echarts 独立 chunk（532KB）不进主包」（文档记载值，非本次实测）。

**上传交互（文档）** `UploadPanel.vue`：
- **动态格式过滤以后端 `supported-types` 为准**，`accept` 由后端返回的 supported 扩展名拼出（`:24-27`）；后端不可用时兜底 7 个文本扩展名（`:12, :20-22`），拉取失败静默保留兜底（`:58-64`）。
- 不支持的文件前端预筛 + Warn 提示（`:66-76`）；`unsupportedHints` 把"已认识但能力未配置"的类型分组，并把后端 reason 翻译成人话（`:36-56`）。
- 拖拽 + 点击两种入口，`:auto-upload="false"` 手动控制（`:105-113`）。

**上传交互（评测数据集）** `views/eval/EvalNewView.vue`：
- **前端三重预检**：扩展名 `.json/.jsonl`（`:130-134`）、大小 ≤10MB（`:135-138`）、**只解析首条结构**（jsonl 取首个非空行，json 取 `samples[0]`），校验 `question` 为 string（`:139-158`）。
- **幂等键**：`const idempotencyKey = ref(crypto.randomUUID())`，表单会话内稳定，提交成功后重新生成（`:34`，配合 `api/eval.ts:63-73` 发 `Idempotency-Key` 头）。
- **无 reference 禁用 context_recall**：`recallDisabled = with_reference_count === 0`，watch 到禁用即从 metrics 剔除该指标（`:55-62`）。
- **表单草稿**：`watch(form, {deep:true})` 变化即存 store，提交成功清空（`:94-105`；store 侧 `stores/eval.ts:149-162` 含 `draftFromTask` 支持"重新发起"）。

### 6.5 构建产物位置与打包方式

- **Vite outDir 直接指向 Go embed 目录**：`outDir: '../internal/webui/dist'` + `emptyOutDir: true`（`frontend/vite.config.ts:6-8`），注释「后端单一二进制托管前端」。
- build 脚本含类型检查：`"build": "vue-tsc --noEmit && vite build"`（`frontend/package.json:8`）。
- **产物提交策略**：`.gitignore:35-37` 忽略 `internal/webui/dist/*` 但 `!internal/webui/dist/index.html`——**只提交占位 index.html 保证 go:embed 可编译**。实测 `git ls-files internal/webui/dist` 返回 1 条。这解释了 CI 中「前端 build 必须先于 go build」的顺序约束（`ci.yml:42` 注释）。
- 实测当前 checkout 的 dist：`index.html`（1815 字节）+ `assets/` 46 个文件（含 `index-BYcT8OWj.js`、`index-DuF6E25F.css`）。
- **打包方式**：Go `//go:embed all:dist`（`internal/webui/embed.go:11-12`）→ `fs.Sub(distFS, "dist")`（`:15-17`）→ 二进制内嵌。因为 embed FS **不支持 `c.File`**，`serveIndex` 必须 `fs.ReadFile` 读内容后 `c.Data` 写回（`internal/webui/router.go:39-48` 注释明示）。
- 根 `package.json` 是 pnpm workspace（`packages: [frontend, official]`，`pnpm-workspace.yaml:1-3`），`packageManager: pnpm@10.33.0`。

### 6.6 前端 API 封装与状态管理

- **统一 axios 实例**（`api/client.ts:40-43`）：`baseURL: '/'`、`timeout: 60000`。
- 请求拦截：会话 JWT 优先，无则 API Key（`:46-54`）。
- 响应拦截：**业务码解包**——`code !== 0` 直接 reject 成 Error(message)（`:57-64`）；**401 → `clearCredentials()` + 跳 `/login`**（`:65-74`）。
- `request<T>(config)` 返回 `resp.data.data`（`:78-81`）。
- **路由守卫**：`getStoredToken().length > 0 || getStoredApiKey().length > 0` 即视为已登录；`meta.public` 豁免（`router/index.ts` 守卫段）。
- **评测中心路由顺序敏感**：`/eval/new`、`/eval/compare` 必须先于 `/eval/:id` 注册（`router/index.ts:31` 注释）。
- 全部页面 `defineAsyncComponent` 懒加载。
- **轮询状态机**（`stores/eval.ts:164-311`）：链式 `setTimeout` 退避——前 30s 每 2s、30s~5min 每 5s、之后每 15s（`:22-27, :197-202`）；**上一轮响应回来后才排下一轮**避免慢请求堆叠（`:239-245`）；`visibilitychange` 隐藏清定时器、恢复立即拉一次（`:285-304`，模块级单例监听器 `:33`）；终态自动停 + `completed` 顺带拉报告（`:212-221`）；**连续失败 3 次暂停并提示 + 手动重试**（`:222-235, :178-184`）；列表页 10s 低频轮询且"无活动任务自动停"（`:249-281`）。
- 状态管理用 Pinia：`stores/{chat,doc,kb,eval,auth}.ts`。`stores/chat.ts` 把会话索引持久化到 `localStorage`（`binrag_sessions`，`:7, 30-39`）；**thinking 不持久化**（`:10` 注释「N4：仅在当轮流式过程中累积，历史加载不含」）；`switchSession` 直接从后端拉历史并解析 sources JSON 字符串（`:79-93`，`parseSources` 容错 `:20-28`）。

---

## 7. 配置清单（`configs/*.yaml` 全量 + 代码默认值）

configs 目录共 3 个文件：`config.yaml`（307 行，示例主配置，随发布包分发，`release.yml:73`）、`config.local.yaml`（199 行，本地覆盖，gitignore 忽略）、`config.local-copy.local.yaml`（144 行，模板）。`deploy/configs/` 另有 `config.docker.yaml`（10406 字节）、`config.docker.local.yaml`、`config.local.yaml.example`。

**默认值列未标"示例值"的即代码 `applyDefaults` 的兜底值**（`internal/config/config.go:380-605`）。「必填」判定依据 = 代码 `Validate` 报错 或 运行时 `Available()==false` 才报错。

### embedder
| 项 | 示例值 | 代码默认 | 必填 |
|---|---|---|---|
| `embedder.provider` | `openai` | 无默认（空即按 provider 分支） | 是（否则 Embedder 初始化失败，`internal/app/app.go:83-88`） |
| `embedder.base_url` | `https://xxx` | 无 | 是 |
| `embedder.api_key` | `sk-xxx` | 无 | 是 |
| `embedder.model` | `text-embedding-v4` | 无 | 是 |
| `embedder.dimension` | `1024` | **1536**（`config.go:390-392`） | 否 |
| `embedder.batch_size` | `10` | **100**（`config.go:381-383`） | 否 |
| `embedder.max_retries` | `3` | **3**（`:384-386`） | 否 |
| `embedder.qps` | `10` | **10**（`:387-389`） | 否 |

### vectorstore
| 项 | 示例值 | 默认 | 必填 |
|---|---|---|---|
| `vectorstore.host` | `localhost:6334` | 无 | 是 |
| `vectorstore.collection_name` | `binrag` | 无 | 是 |
| `vectorstore.dimension` | `1024` | **= embedder.dimension**（`:396-398`） | 否 |
| `vectorstore.distance` | `cosine` | **cosine**（`:393-395`） | 否 |

### loader / chunker
| 项 | 示例值 | 默认 | 必填 |
|---|---|---|---|
| `loader.min_readable_chars` | `20` | **20**（`:521-523`） | 否 |
| `chunker.strategy` | `recursive` | 无（`fixed/recursive/heading`） | 是 |
| `chunker.chunk_size` | `500` | **512**（`:399-401`） | 否 |
| `chunker.chunk_overlap` | `50` | 50（`<0` 时，`:402-404`） | 否 |
| `chunker.heading_level` | `2` | **2**（`:405-407`） | 否 |

### retriever / reranker
| 项 | 示例值 | 默认 | 必填 |
|---|---|---|---|
| `retriever.top_k` | `10` | **10**（`:409-411`） | 否 |
| `retriever.rrf_k` | `60` | **60**（`:412-414`） | 否 |
| `retriever.vector_weight` | `0.7` | **0.7**（`:415-417`） | 否（注释要求两者和为 1） |
| `retriever.bm25_weight` | `0.3` | **0.3**（`:418-420`） | 否 |
| `retriever.enable_bm25` | `true` | 零值 false | 否 |
| `retriever.enable_reranker` | `true` | 零值 false | 否 |
| `retriever.multi_query_concurrency` | — | **3**（`:421-423`） | 否 |
| `reranker.base_url` | `https://xxx` | 无 | 是 |
| `reranker.api_key` | `sk-xxx` | 无 | 是 |
| `reranker.model` | `bge-reranker-v2-m3` | 无 | 是 |
| `reranker.mode` | `api` | 无（`api`/`llm`/`ollama`） | 是 |
| `reranker.top_n` | `5` | **5**（`:425-427`） | 否 |
| `reranker.max_retries` | `3` | **3**（`:428-430`） | 否 |
| `reranker.qps` | `10` | **10**（`:431-433`） | 否 |
| `reranker.llm_prompt_template` | `""` | 空=内置模板 | 否 |
| `reranker.llm_temperature` | `0` | 零值 0 | 否 |

### llm / rag
| 项 | 示例值 | 默认 | 必填 |
|---|---|---|---|
| `llm.base_url` | `https://xxx` | 无 | 是 |
| `llm.api_key` | `sk-xxx` | 无（本地模型可空） | 否 |
| `llm.model` | `gpt-4o` | 无 | 是 |
| `llm.temperature` | `0.7` | **0.7**（`==0` 时，`:444-446`） | 否 |
| `llm.max_tokens` | `2048` | **2048**（`:447-449`） | 否 |
| `llm.max_retries` | `3` | **3**（`:435-437`） | 否 |
| `llm.qps` | `10` | **10**（`:438-440`） | 否 |
| `llm.timeout` | `60` | **60**（`:441-443`） | 否 |
| `rag.top_k` | `5` | **5**（`:464-466`） | 否 |
| `rag.max_context_tokens` | `2048` | **2048**（`:467-469`） | 否 |
| `rag.max_chunks` | `5` | **5**（`:470-472`） | 否 |
| `rag.enable_rewrite` | `true` | **true**（指针 nil→true，`:473-476`） | 否 |
| `rag.multi_query_enabled` | `false` | nil→false（`MultiQueryOn` `:233-236`） | 否 |
| `rag.multi_query_count` | `3` | **3**（`:484-486`） | 否 |
| `rag.multi_query_concurrency` | `3` | **3**（`:487-489`） | 否 |
| `rag.decomposition_enabled` | `false` | nil→false（`:238-241`） | 否 |
| `rag.decomposition_mode` | `parallel` | **parallel**（`:491-493`） | 否 |
| `rag.decomposition_max_sub` | `5` | **5**（`:494-496`） | 否 |
| `rag.step_back_enabled` | `false` | nil→false（`:243-246`） | 否 |
| `rag.routing_enabled` | `false` | nil→false（`:248-251`） | 否 |
| `rag.routing_fallback` | `multi_query` | **multi_query**（`:498-500`） | 否 |
| `rag.hyde_enabled` | `false` | nil→false（`:253-256`） | 否 |
| `rag.hyde_skip_simple` | `true` | nil→true（`:258-261`） | 否 |
| `rag.strategy.query` | `multi` | **multi**（`:502-504`） | 否 |
| `rag.strategy.fusion` | `rrf` | **rrf**（`:505-507`） | 否 |
| `rag.strategy.decomposition` | `off` | **off**（`:508-510`） | 否 |
| `rag.strategy.step_back` | `off` | **off**（`:511-513`） | 否 |
| `rag.strategy.hyde` | `off` | **off**（`:514-516`） | 否 |
| `rag.strategy.routing` | `auto` | **off**（`:517-519`，示例值与默认不同） | 否 |
| `rag.strategy.thinking` | `on` | 空=继承 | 否 |
| `rag.strategy.data_sources` | `[]` | 空=仅 vector_store（私有性默认，`configs/config.yaml:202-205`） | 否 |
| `rag.history_capacity` | `50` | **50**（`:477-479`） | 否 |
| `rag.history_limit` | `10` | **10**（`:480-482`） | 否 |
| `rag.system_prompt_path` / `context_template_path` / `rewrite_template_path` | `""` | 空=内置模板 | 否 |
| `rag.multi_query_template_path` / `decomposition_template_path` / `step_back_template_path` / `routing_template_path` / `hyde_template_path` | 未在示例配置中出现 | 空=内置 | 否（结构体有字段，`config.go:206, 211, 212, 217, 218`） |

### web_search / postgres / server
| 项 | 示例值 | 默认 | 必填 |
|---|---|---|---|
| `web_search.provider` | `bocha` | **bocha**（`:451-453`） | 否 |
| `web_search.base_url` | `""` | 空=官方地址 | 否 |
| `web_search.api_key` | `""` | 空=未就绪（`Available()==false`，`config.go:50`） | 否 |
| `web_search.count` | `5` | **5**（`:454-456`） | 否 |
| `web_search.timeout` | `30` | **30**（`:457-459`） | 否 |
| `web_search.qps` | `1` | **1**（`:460-462`） | 否 |
| `postgres.dsn` | `postgres://binrag:binrag@localhost:5432/binrag` | 无 | **是**（连接失败即 exit 1，`internal/app/app.go:64-68`） |
| `server.port` | `8085` | **8080**（`:525-527`） | 否 |
| `server.file_storage_dir` | `./data/uploads` | **./data/uploads**（`:528-530`） | 否 |
| `server.upload_max_size_mb` | `1024` | **50**（`:531-533`） | 否 |
| `server.worker_count` | `5` | **2**（`:534-536`） | 否 |
| `server.task_max_retries` | `3` | **3**（`:537-539`） | 否 |
| `server.auth_enabled` | `true` | 零值 false | 否 |
| `server.bootstrap_api_key` | `""` | 空=不种子（`:309-311`） | 否 |
| `server.rate_limit_qps` | `0` | 0=不限制（`internal/api/middleware.go:116-119`） | 否 |
| `server.mcp.enabled` | `false` | false（`:540` 注释「安全默认」） | 否 |
| `server.mcp.path` | `/mcp` | **/mcp**（`:541-543`） | 否 |
| `server.mcp.audit_param_limit` | `2000` | **2000**（`:544-546`） | 否 |

### oidc / multimedia / eval
| 项 | 示例值 | 默认 | 必填 |
|---|---|---|---|
| `oidc.enabled` | `false` | false | 否 |
| `oidc.public_url` | `https://rag.example.com` | 无 | **enabled=true 时必填**（`config.go:614-617`） |
| `oidc.jwt_secret` | `""` | 空=启动时随机生成（重启后旧会话失效，`configs/config.yaml:269`） | 否 |
| `oidc.jwt_expire_minutes` | `1200` | **120**（`:548-550`；示例值与默认不同） | 否 |
| `oidc.providers[].name` | `github` / `company` | 无 | **是**（须匹配 `^[a-zA-Z0-9_-]+$`、不重复，`config.go:621-631`） |
| `oidc.providers[].type` | `oauth2` / `oidc` | **oidc**（`:553-555`） | 否 |
| `oidc.providers[].display_name` | `GitHub` | **= name**（`:556-558`） | 否 |
| `oidc.providers[].client_id` | `""` | 无 | **是**（`config.go:632-634`） |
| `oidc.providers[].client_secret` | `""` | 无 | **是**（`:635-637`） |
| `oidc.providers[].issuer` | `""` | 无 | **type=oidc 时必填**（`:640-642`） |
| `oidc.providers[].scope` | 注释掉 | github→`["read:user"]`；其他→`["openid","profile","email"]`（`:559-565`） | 否 |
| `oidc.providers[].redirect_url` | 注释掉 | 空=按 type 拼默认 | 否（非空须合法 URL，`:650-654`） |
| `oidc.providers[].permissive_sub` | 未在示例中 | 零值 false | 否（字段存在，`config.go:322`） |
| `multimedia.vision.provider` | `openai_compat` | **openai_compat**（`:571-573`） | 否 |
| `multimedia.vision.base_url` | `""` | 空=官方地址 | 否（非空须合法 URL，`:658-667`） |
| `multimedia.vision.api_key` | `""` | 空=能力不可用（图片/视频上传 400 拒绝） | 否 |
| `multimedia.vision.model` | `""` | 无 | 否 |
| `multimedia.vision.timeout` | `30` | **30**（`:574-576`） | 否 |
| `multimedia.speech.provider` | `openai_compat` | **openai_compat**（`:577-579`） | 否 |
| `multimedia.speech.{base_url,api_key,model}` | `""` | 空=音频能力不可用 | 否 |
| `multimedia.speech.timeout` | `30` | **30**（`:580-582`） | 否 |
| `multimedia.frame_interval_sec` | `10` | **10**（`:568-570`，兼容项） | 否 |
| `multimedia.video.frame_strategy` | `fixed` | **fixed**（`:584-586`） | 否（仅 fixed/scene，`:669-673`） |
| `multimedia.video.frame_interval_sec` | `10` | **= 顶层 frame_interval_sec**（`:587-589`，video 优先） | 否 |
| `multimedia.video.scene.sample_fps` | `2` | **2**（`:590-592`） | 否 |
| `multimedia.video.scene.similarity_threshold` | `0.85` | **0.85**（`:593-595`） | 否 |
| `multimedia.video.scene.min_scene_duration_ms` | `3000` | **3000**（`:596-598`） | 否 |
| `multimedia.video.vision_embedding.provider` | `openai_compat` | **openai_compat**（`:599-601`） | 否 |
| `multimedia.video.vision_embedding.api_key` | `""` | 无 | **frame_strategy=scene 时必填**（`config.go:674-676`） |
| `multimedia.video.vision_embedding.timeout` | `30` | **30**（`:602-604`） | 否 |
| `eval.service_url` | `""` | 空=未配置 | 否（非空须合法 URL，`:678-682`）；**留空则 `/api/v1/eval/*` 全组 503** |
| `eval.internal_token` | `""` | 空=未配置 | 否；**与 Python 侧 `EVAL_INTERNAL_TOKEN` 必须一致** |

> **⚠️ 部署模板缺段（实测）**：`configs/config.yaml`（307 行）含 **14 个**顶层段（含 `multimedia:` 与 `eval:`）；而 `deploy/configs/config.docker.yaml`（10406 字节）只有 **12 个**顶层段，**完全没有 `multimedia:` 与 `eval:`**。而 `docker-compose.yml:83` 的注释却要求「在 config.docker.yaml 中配置 eval_service_url 与 eval_internal_token」——即模板本身没给这两段，用户必须手工补，否则 `/api/v1/eval/*` 全组 503（`Eval.Available()==false`），且多媒体能力全部不可用。这是一个**真实的模板与文档不一致缺陷**。

**评测服务（Python）配置**（`services/ragas-eval/src/ragas_eval/config.py:19-79`，全部支持 `EVAL_` 前缀环境变量覆盖，`:29`）：

| 项 | 默认 | 必填 |
|---|---|---|
| `listen_port` | 8090 | 否 |
| `binrag_base_url` | `http://localhost:8085` | 否 |
| `binrag_api_key` | `""` | **是**（`config.py:22-25` 文档注明必填） |
| `internal_token` | `""` | **是**（空则所有非豁免端点 401，`:42`） |
| `judge_base_url` | `https://api.openai.com/v1` | 否 |
| `judge_api_key` | `""` | **是** |
| `judge_model` | `gpt-4o-mini` | 否 |
| `judge_timeout` | 60.0 | 否 |
| `judge_max_tokens` | 2048 | 否 |
| `embed_base_url` | `https://api.openai.com/v1` | 否 |
| `embed_api_key` | `""` | **是** |
| `embed_model` | `text-embedding-3-small` | 否（**必须与索引进库同源**，`:51` 注释） |
| `embed_batch_size` | 64 | 否 |
| `db_path` | `./data/eval.db` | 否 |
| `max_running_tasks` | 2 | 否 |
| `max_queue_size` | 16 | 否 |
| `sample_concurrency` | 4 | 否（校验器钳制 [1,8]，`:75-79`；**但该全局项实际未被任何代码读取**，见 §10） |
| `judge_rpm` | 60 | 否 |
| `eval_batch_size` | 8 | 否 |
| `task_deadline` | 7200 | 否 |
| `sample_soft_timeout` | 300 | 否（**代码中未被使用**） |
| `retention_days` | 0 | 否（**代码中未被使用**，`:70` 注释「预留」） |
| `max_dataset_bytes` | 10MB | 否 |

---

## 8. 测试与质量

### 8.1 Go 单测分布（实测）

- **测试文件 63 个**，**测试函数 405 个**（`grep -c "^func Test"`）。
- 按包（测试函数数）：

| 包 | 数量 | 包 | 数量 |
|---|---|---|---|
| `internal/rag` | 85 | `internal/config` | 16 |
| `internal/api` | 68 | **`internal/eval`** | **15** |
| `internal/loader` | 36 | `internal/chunker` | 15 |
| `internal/auth` | 30 | `internal/llm` | 13 |
| `internal/retriever` | 26 | `internal/reranker` | 9 |
| `internal/multimedia` | 21 | `internal/task` | 7 |
| `internal/mcp` | 20 | `internal/pipeline` | 6 |
| `internal/store` | 18 | `internal/datasource` | 6 |
| `internal/search` | 5 | **`internal/webui`** | **4** |
| `internal/embedding` | 4 | **`cmd/server`** | **1** |

- `internal/eval` 两个测试文件共 15 个用例：`dataset_test.go`、`evaluator_test.go`。
- **`internal/app` 无任何测试文件**（`ls internal/app/*_test.go` 无结果）——装配与优雅退出路径零覆盖。
- `internal/webui` 4 个用例（`router_test.go:22, 37, 52, 67`）：根路径 index、SPA 深链回退、API 未匹配返 JSON 404、assets 缺失不回退 index。
- `cmd/desktop` **无测试**；`cmd/eval` **无测试**。

### 8.2 前端单测

vitest（`frontend/vitest.config.ts`：`environment: 'jsdom'`、`globals: true`），4 个测试文件 **37 个用例**：

| 文件 | 用例数 | 覆盖 |
|---|---|---|
| `src/api/chat.test.ts` | 6 | SSE 解析：正常流顺序、error 事件、非 JSON data 行跳过、非 2xx 取后端 message、非 2xx 无 JSON 回退状态码、AbortError |
| `src/stores/chat.test.ts` | 12 | chatStore 降级状态机（mock chatStream 驱动事件序列） |
| `src/utils/evalScore.test.ts` | 13 | 评测分数工具 |
| `src/components/ThinkingPanel.test.ts` | 6 | 思考链路面板 |

与文档一致：`docs/35-ragas评测/验收报告.md:40` 记「vitest 37/37」。

### 8.3 Python 单测

13 个测试文件，**86 个测试函数**（`grep -c "^def test_\|^async def test_"`）：
`test_collector(188行) / test_config / test_lifecycle / test_llm(141) / test_middleware(124) / test_prompts(132) / test_queue(166) / test_routes_datasets(124) / test_routes_reports(213) / test_routes_tasks(166) / test_runner(176) / test_store(147)` + `conftest.py(223)`。
配置：`asyncio_mode = "auto"`、`testpaths = ["tests"]`（`pyproject.toml:61-64`）。
mock 手段：`respx`（httpx mock）、可注入 `evaluate_fn` double（`core/runner.py:148-165`）、`dependency_overrides` 跳过预检（`api/deps.py:51-56` 注释）。

### 8.4 CI 是否跑测试

**Go：跑**（`ci.yml:71-72` `go test ./...` + `:74-75` 选择性 `-race`）。
**前端：跑**（`ci.yml:49-50` `pnpm --filter binrag-frontend test`）。
**Python：不跑**（CI 中无任何 Python/uv/pytest 步骤；`.github/workflows/*.yml` 中 `services/ragas-eval` 出现 0 次）。

### 8.5 其他质量门

- **gofmt 强制**（`ci.yml:32-40`，非空即 fail 并打印清单）。
- **`go vet ./...`**（`ci.yml:67-68`）。
- **前端类型检查进 build 步骤**（`vue-tsc --noEmit && vite build`，`frontend/package.json:8`），CI 因此隐式做了类型检查。
- **Python ruff**：配置存在且严格（`select = ["E","W","F","I","UP","B","ASYNC"]`，`pyproject.toml:48-59`），但**不在 CI 中执行**。
- **无前端 lint**（无 eslint 配置与脚本）。
- **无覆盖率门槛**、无 SonarQube/CodeQL、无依赖漏洞扫描。
- 文档记载：`docs/35-ragas评测/验收报告.md:38-40` 记「Go build/test/vet 三项 exit 0（20 个包全 ok）」「Python 86 pytest 全过、ruff 全过」「前端 pnpm build 通过、vitest 37/37」——为文档记载值；本次因沙箱限制未能执行 `go test`。

---

## 9. 面试官最可能深挖的 10 个点

**Q1：「同一份前端产物怎么同时服务 Web 和桌面？桌面怎么知道后端在哪？」**
依据：`internal/app/app.go:1-3`（两形态共用装配包）；`cmd/desktop/main.go:37`（`net.Listen("tcp","127.0.0.1:0")` 先绑定拿随机端口）；`cmd/desktop/main.go:49`（`addr := "http://" + ln.Addr().String() + "/"`）；`cmd/desktop/main.go:80`（`URL: addr`）；`internal/webui/embed.go:11-12`（`//go:embed all:dist`）。**关键论证**：必须"先 Listen 再 Serve"才能读端口——不能用 `ListenAndServe()`。

**Q2：「go:embed 一个 SPA，深浅链和 /api 404 怎么区分？」**
依据：`internal/webui/router.go:24`（`/` → serveIndex）；`:25-28`（`/assets/*filepath` → FileServer + `Cache-Control: public, max-age=86400`）；`:29-35`（`NoRoute`：`/api/` 前缀或非 GET → JSON 404，否则回退 index）；`:39-48`（**embed FS 不可用 `c.File`，必须 `fs.ReadFile` + `c.Data`**）；测试 `internal/webui/router_test.go:37-65`。

**Q3：「优雅退出怎么保证不丢任务？退出顺序是什么？」**
依据：`cmd/server/main.go:58-60`（SIGINT/SIGTERM）；`:63-67`（10s 超时 `server.Shutdown`）；`internal/app/app.go:281-291`（`Close()` 内序：worker.Shutdown → auditSink.Shutdown(5s) → cancel → st.Close）。**面试官会追**：桌面侧无 `signal.Notify`，且 `App.Close()` 未 `vs.Close()`。

**Q4：「Recall@K 的分母是什么？为什么这么定？」**
依据：`internal/eval/report.go:49-63`（`valid` 剔除 `Error != ""` 与 `len(ExpectedIDs)==0`）；`internal/eval/evaluator.go:144-160`（任一期望命中即算命中，多期望不做分数式）；`:125-132`（**只发一次 `TopK=maxK` 检索**）。

**Q5：「LLM-as-Judge 怎么保证可复现和输出可解析？」**
依据：`internal/eval/judge.go:31`（`zeroTemp = 0.0`）+ `:86`；`:95-100`（首个 `{` 到末个 `}` 容错提取）；`:46-48`（评分越界报错）；`:58-63`（**忠实度只喂 filename+heading，不喂正文** ← 最容易被追问的弱点）。

**Q6：「RAGAS 服务为什么是独立进程？和 Go 侧评估怎么分工？」**
依据：`docs/35-ragas评测/spec.md:11`（Recall@K 需进程内 retriever，故留 Go；RAGAS 天然黑盒可走 HTTP）；`spec.md:40`（N5：报告不直接互比）；`internal/app/app.go:361-392`（`AssembleEvalDeps` 刻意不连 PG/不起 worker/不建 HTTP）；`internal/rag/engine.go:68-70, 113-115` + `internal/rag/context.go:22, 83` + `internal/api/handler_chat.go:206-208`（为 RAGAS 新增的 `include_contexts` 出口）。

**Q7：「三层并发限额是哪三层？队列满了怎么办？任务取消是硬断还是协作？」**
依据：`config.py:60-64`（任务级 2 / 排队 16 / 样本级 clamp[1,8]）+ 令牌桶 60rpm；`core/queue.py:83-90`；`api/routes_tasks.py:113-116`（队列满 → **回滚删除刚落库的任务骨架** → 429）；`core/queue.py:53, 92-94`（`asyncio.Event` 协作式取消，**样本边界生效，进行中的 LLM 不硬断**）；`core/runner.py:192-195`。

**Q8：「评测服务重启后任务怎么办？」**
依据：`core/queue.py:60-68`（`reset_interrupted_tasks` + `list_resumable_task_ids` 重入队）；`store/db.py:61`（`UNIQUE(task_id, idx)` 是断点续跑幂等基石）；`core/queue.py:169`（pending 样本才采集）；`core/queue.py:229-231`（评测前再查一次取消/DB 状态，**避免翻活已取消任务**）。

**Q9：「Go 代理只是透传，为什么要解析 body？」**
依据：`internal/api/proxy_eval.go:59-65, 88-109`（**唯一业务逻辑**：`POST /tasks` 解析 `kb_id` 做越权校验，越权 404）；`:95`（读 body 后必须 `io.NopCloser(bytes.NewReader(body))` 复原供透传）；`:98-101`（非合法 JSON 不拦截，注释「纯透传原则」）；`internal/api/router.go:97-105`（`/eval/health` 豁免鉴权用**包装中间件**，因 Gin 不允许静态路由与 `/eval/*path` 通配共存）。

**Q10：「SSE 为什么不用 EventSource？前端断流怎么降级？」**
依据：`frontend/src/api/chat.ts:2-3`（EventSource 无法带 Authorization 头、无法 AbortController 停止）；`:45-79`（手写行解析 + 粘包缓冲 + `TextDecoder({stream:true})` + `done` 即 return）；`:34-43`（非 2xx 优先后端 message）；`frontend/src/stores/chat.ts:145-184`（事件驱动转移 + `finally` 无条件复位 streaming + `!hasContent && !error → '（无回答）'`）；`:188-190`（`stop()` → abort → `AbortError` 分支保留部分内容不标错）。**加分点**：诚实说明"无自动重连"。

---

## 10. 可被挑刺的缺陷（诚实清单）

### 10.1 工程化 / 部署

1. **注释与代码不符**：`cmd/server/main.go:57` 注释「先停 worker，再关 HTTP」，实际 `server.Shutdown`（:65）先于 `defer a.Close()`（:37）。
2. **桌面形态无信号处理**：`cmd/desktop/main.go` 无 `os/signal` 导入，`kill -TERM` / Ctrl-C 绕过 `OnShutdown`，worker 与审计不 flush。
3. **桌面形态 `a.Close()` 无 `defer`**（`:66` 只在 `OnShutdown` 内），`wailsApp.Run()` 提前失败则资源不释放。
4. **桌面形态完全忽略 `server.port`**：无文档说明该配置在桌面下无效。
5. **`App.Close()` 不关 vectorstore**：`vs.Close()` 只在装配失败路径调用（`internal/app/app.go:131, 174`）。
6. **装配失败清理逐路径手写**（7 处组合，`internal/app/app.go:66-178`），无统一 cleanup 栈，易漏。
7. **Go 无健康检查端点**：探针只能打 `/swagger/index.html`（`docker-compose.yml:70`），**不校验 PG/Qdrant 连通性**；而 `ragas-eval` 的 `depends_on: service_healthy` 依赖该探针（`:103-105`）。
8. **生产 compose 不含 ragas-eval**（`docker-compose.prod.yml`、`docker-compose.prod.local.yml`）。
9. **`docker-compose.local.yml` 缺 `name:` 项目名**（32 行全文），与其余三文件不一致。
10. **`config.docker.yaml` 模板缺 `multimedia:` 与 `eval:` 两段**（实测顶层段 12 个 vs `configs/config.yaml` 14 个），而 `docker-compose.yml:83` 却要求在其中配置 `eval_service_url`/`eval_internal_token` —— 模板与文档不一致，用户必须手工补。
11. **`Dockerfile` 与 `Dockerfile.deploy` 逐行重复**（各 ~95 行），改一处忘另一处风险高。
12. **release.yml 的 `.app` 是 adhoc 签名**（`:128` `codesign -s -`），Gatekeeper 会拦，无公证步骤。

### 10.2 CI/CD

13. **Python 评测服务零 CI**（无 pytest / ruff / `uv lock --check` / 镜像构建）。
14. **`docker-publish.yml` 无 `needs: ci`**（`:29-33`），`release.yml` 同样不依赖 CI。
15. **CI race 只覆盖 4 个包**（`ci.yml:75`），`internal/rag`（85 个测试，含并发检索）不跑 race。
16. **无前端 lint**；**无覆盖率门槛**；**无依赖漏洞扫描 / CodeQL**。
17. **`cache-to: type=gha,mode=max`**（`docker-publish.yml:76`）无过期策略，易占满 GHA 配额。
18. **`release.yml` 三个 job 各重复一次「前端 build」**，无 artifact 复用。

### 10.3 评估体系

19. **Python 配置死代码**：`sample_soft_timeout`（`config.py:69`）与 `retention_days`（`:70`）**在 `src/` 中零引用**——文档承诺的"单样本软超时"实际未实现。
20. **`EVAL_SAMPLE_CONCURRENCY` 全局配置无效**：`config.py:63` 的 `sample_concurrency` 除被 validator 钳制（`:75-79`）外无读取点；实际生效的是**每任务请求体**的值（`api/routes_tasks.py:105` → `core/queue.py:166, 172`）。
21. **Judge 重试次数不可配置**：`llm/factory.py:95` `for backoff in (0.0, *_RETRY_BACKOFFS)` 硬编码 3 次，无对应设置项。
22. **Go 侧忠实度判定只喂 filename/heading 不喂正文**（`internal/eval/judge.go:58-63`），判别力弱。
23. **Go 侧 Judge 丢弃理由**：只解析 `score`/`faithful`；Python 侧 `scores_json` 保留 `{score, reason}`（`store/models.py:92-93`）——两套能力不对称。
24. **两套评估并发模型不可比**：Go 默认并发 2（`cmd/eval/main.go:27`）vs Python 四层限额——耗时结构不同，性能数据不可互参。
25. **`cmd/eval` 不自动创建 `-o` 的父目录**（`cmd/eval/main.go:89` 直接 `os.Create`）。
26. **`parse_failure_rate` 分母口径含糊**（`core/runner.py:288, 315`）：未请求的指标完全不计入 `judged_cells`，不同指标组合下分母不同，跨任务对比失真。
27. **`_filter_sort_paginate` 在 Python 侧全量排序分页**（`api/routes_reports.py:57-90`，注释「样本规模千级内」），无上限保护。

### 10.4 前端

28. **SSE 解析无 `currentEvent` 重置**（`api/chat.ts:48, 61-62`），`currentEvent` 粘性——只有 `data:` 的事件会沿用上一个事件名。
29. **SSE 无自动重连、无 `Last-Event-ID`、无 `retry:` 处理**（代码中未找到），降级完全依赖用户手动重发。
30. **`TextDecoder` 未做收尾 flush**（`chat.ts:54`），最后一行不带换行会丢事件（当前 Go 侧总补 `\n\n`，未触发）。
31. **`error` 事件覆盖已有内容，`catch` 分支保留已有内容**（`stores/chat.ts:157-160` vs `:171-172`）——同为失败，行为不一致。
32. **凭据逻辑重复两处**（`api/client.ts:46-54` 与 `api/chat.ts:7-10`）。
33. **`saveSessions` 写 `localStorage` 无 try/catch**（`stores/chat.ts:37-39`），配额满抛未捕获异常。
34. **Markdown 流式渲染每 chunk 全量 `marked.parse` + `DOMPurify.sanitize`**（`MarkdownRenderer.vue:28-37`），长回答 O(n²)。
35. **`EvalChart` 的 `MutationObserver` 监听整个 `<html>` 的 class**（`components/eval/EvalChart.vue:35-41`），任何 html class 变动触发全部图表重算。
36. **任务列表 `page_size: 100` 一次拉全量做客户端筛选**（`stores/eval.ts:30`），超 100 静默截断。
37. **无前端 E2E 测试**，验收报告亦承认 AC8 浏览器走查未完成（`docs/35-ragas评测/验收报告.md:50`）。
38. **`frontend/src/api/eval.ts:38` 手动设 `Content-Type: multipart/form-data`**（axios 会自动加 boundary），反模式。

### 10.5 可观测性

39. **无 metrics / tracing / `/metrics`**（全仓库 grep 零命中），无法量化 RAG 延迟、检索命中率、token 消耗。
40. **`cmd/server` / `cmd/desktop` 无日志级别配置项**，生产无法调低级别。
41. **slog 用默认 text handler**，无 JSON 结构化输出。
42. **key 命名中英混杂**（`"耗时ms"` vs `"method"`，`internal/api/middleware.go:92-97`）。
43. **请求日志不含 request_id / 用户身份 / 客户端 IP**（`middleware.go:88-99`），无法串联一次请求的多条日志，无普通 API 审计（审计只覆盖 MCP）。
44. **审计队列满即丢弃且只 warn**（`internal/mcp/audit.go:17-21`），无丢弃计数上报——静默丢审计是合规风险。
45. **Python 侧无结构化日志 / 无 metrics / 无 access log**；`JudgeStats` 只在内存（`llm/factory.py:80`）且**未在任何 API 暴露**（`/health` 只给 `judge_llm: ok/error`，`main.py:91`），进程重启即丢。

### 10.6 测试

46. **`internal/app`（392 行装配 + 优雅退出核心）零测试**；`cmd/desktop`、`cmd/eval` 零测试——恰好是本次档案重点的三个区域。
47. **前端 SSE 测试用"一次性 close 的 ReadableStream"**（`api/chat.test.ts:10-16`），**不覆盖跨 chunk 粘包**——而粘包正是手写解析器的核心风险点。
48. **无 SSE 中断降级的前端集成/E2E 验证**，只有 store 级 mock 单测。
49. **Go 侧 race 覆盖不全**（同 #15），`internal/rag` 85 个测试含并发不跑 race。

---

## 附：关键「代码中未找到」清单

- 桌面形态对 `cfg.Server.Port` 的任何读取（`cmd/desktop/main.go` 全文）。
- Go 侧 `/health`、`/healthz`、`/metrics` 端点。
- Prometheus / OpenTelemetry / tracing 相关代码。
- 前端 SSE 重连、`Last-Event-ID`、`retry:` 处理。
- 后端热重载工具配置（`.air.toml` / nodemon / reflex）。
- Taskfile 中的 `test` / `lint` / `dev` / `docker` 任务。
- CI 中任何 Python 相关步骤；CI 中任何 `services/ragas-eval` 引用。
- `sample_soft_timeout`、`retention_days`、`Settings.sample_concurrency` 在 Python `src/` 中的消费点。
- 镜像体积、构建耗时、runtime 内存等任何性能数字（代码/文档中均未记载）——按要求不推测。
