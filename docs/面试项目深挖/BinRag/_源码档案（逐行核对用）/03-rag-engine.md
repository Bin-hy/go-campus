# BinRag 「RAG 问答编排链路」深度技术档案（面试准备版）

> **阅读范围**：`internal/rag/*.go`（含 `_test.go`）、`internal/llm/llm.go`·`stream.go`、`internal/search/bocha.go`·`search.go`，并沿调用链补读 `internal/api/handler_chat.go`·`router.go`、`internal/config/config.go`、`internal/store/history.go`·`schema.go`、`internal/app/app.go`·`rebuild.go`、`internal/retriever/retriever.go`·`rrf.go`、`internal/datasource/*.go`。
> **约束**：全程只读分析，所有结论均标注 `文件:行号`；prompt 逐字引用；找不到的写「代码中未找到」，不编造默认值。

---

## 1. 问答全链路时序

### 1.0 入口分流

| 步骤 | 位置 | 说明 |
|---|---|---|
| 路由注册 | `internal/api/router.go:134` | `v1.POST("/chat", h.ChatDispatch)` |
| 分流 | `internal/api/router.go:164-169` | `isStreamRequest(c)` 判定流式与否 |
| 流式判定 | `internal/api/handler_chat.go:198-204` | `?stream=1` 或 `Accept` 含 `text/event-stream` |
| 非流式 | `internal/api/handler_chat.go:75` | `Chat` → `eng.Ask`（:107）→ `OK(c, result)`（:112） |
| 流式 | `internal/api/handler_chat.go:135` | `ChatStream` → `eng.StreamAsk`（:166）→ SSE 循环（:176-194） |

**请求入参**：`chatRequest`（`handler_chat.go:15-21`）= `session_id`(required)、`question`(required)、`kb_id`、`strategy`（请求级策略）、`enhanced`。

**Handler 侧前置步骤（Ask / StreamAsk 共用）**：

| # | 步骤 | 位置 | 可选性 |
|---|---|---|---|
| H1 | BindJSON 校验 | `handler_chat.go:77-80` / `137-140` | 必做 |
| H2 | 知识库范围解析 `resolveKBScope` | `handler_chat.go:28-57` / 调用点 `:88`、`:143` | 必做（越权防护） |
| H3 | 取配置快照 `cfgMgr.Get()` | `handler_chat.go:92-95`（流式 `:158` `h.cfgSnapshot()`） | 有 cfgMgr 才做 |
| H4 | 知识库级策略读取 `kbStrategy` | `handler_chat.go:116-130` / 调用 `:98`、`:157` | `kb_id` 非空才查库 |
| H5 | 组装 `AskOption` | `handler_chat.go:96-106` / `155-165` | `WithThinking(true)` **硬编码**（:99、:158） |

### 1.1 非流式 `Ask` 主链路（`internal/rag/engine.go:509-636`）

| 序 | 步骤（函数名） | 位置 | 串/并 | 可选性 |
|---|---|---|---|---|
| A1 | 解析 `AskOptions` | `engine.go:511-514` | 串 | 必做 |
| A2 | `sinkFor` 计算思考链路采集器 | `engine.go:516` → `148-157` | 串 | thinking 开关 |
| A3 | `effective` 三级策略合并 | `engine.go:518` → `207-258` | 串 | 必做 |
| A4 | `routeQuery` 路由判定（1 次 LLM） | `engine.go:524` → `routing.go:24-46` | 串 | **routing=auto** 才做 |
| A5 | `resolveDataSource` 数据源裁决 | `engine.go:534` → `470-498` | 串 | 仅 A4 成功分支 |
| A6 | `recordStep(StepRouting)` | `engine.go:543-547` | 串 | sink≠nil |
| A7 | strategy=direct → `ForceSingle` | `engine.go:549-551` | 串 | 仅 direct |
| A8 | `tryDecompose`（分解路径，含 2 次 LLM + N 路检索） | `engine.go:556-559` → `decompose.go:131-258` | 串（内部子问题检索**并发**） | routing 判定 decomposition |
| A9 | `tryMultiQuery`（多查询路径，1 次 LLM + N 路检索） | `engine.go:560-563` → `routing.go:135-192` | 串（内部 N 路**并发**） | routing 判定 multi_query |
| A10 | 常规分支：`tryDecompose`（Decomposition≠off）或 `tryStepBack`（StepBack=on） | `engine.go:569-579` | 串 | 二选一（互斥） |
| A11 | `prepare` 历史→改写→检索→组装 | `engine.go:585` → `821-1020` | 串 | 必做 |
| A12 | 空检索兜底 | `engine.go:591-609` | 串 | `len(sources)==0` |
| A12a | ↳ `forceWebSearchFallback`（系统强制联网） | `engine.go:594` → `314-351` | 串 | 仅 `o.Enhanced` 且有 web_search 工具 |
| A12b | ↳ `enhancedAnswer`（tool loop） | `engine.go:599` → `298-307`、`357-462` | 串（工具调用逐次串行） | 同上 |
| A12c | ↳ 直接返回 `noAnswerText` | `engine.go:607-608` | 串 | 其他情况 |
| A13 | 生成：`enhancedAnswer` 或 `llm.Generate` | `engine.go:613-629` | 串 | 必做 |
| A14 | `appendHistory` 落 user + assistant | `engine.go:632-633` → `814-818` | 串 | 必做 |
| A15 | `withThinking` 附思考链 | `engine.go:635` → `160-165` | 串 | thinking 开启 |

### 1.2 `prepare` 内部时序（`engine.go:821-1020`）

| 序 | 步骤 | 位置 | 串/并 | 可选性 |
|---|---|---|---|---|
| P1 | 策略再解析 + `ForceSingle` 覆盖 | `engine.go:826-834` | 串 | 必做 |
| P2 | 数据源白名单兜底校验 | `engine.go:836-840` | 串 | `AllowedDataSources` 非空 |
| P3 | `history.Get(sessionID, ragCfg.HistoryLimit)` | `engine.go:848` | 串 | 必做（失败即返回 error） |
| P4 | **多查询**：`multiQuery`（1 次 LLM，temp 0.1） | `engine.go:855-858` → `1070-1109` | 串 | `eff.Query == "multi"` |
| P4a | ↳ 失败降级：`rewriteQuery` | `engine.go:862-863` → `1023-1042` | 串 | 多查询失败且 rewrite 开 |
| P4b | ↳ `retriever.SearchMulti` 多路检索 | `engine.go:895-900` → `retriever.go:281-368` | 内部**并发**（`MultiQueryConcurrency`=3 信号量 `retriever.go:296`） | 多查询成功 |
| P5 | **单查询**：`rewriteQuery`（1 次 LLM，temp 0.1） | `engine.go:932-934` | 串 | `RewriteEnabled()` 且非 multi |
| P6 | 检索：数据源分支 / HyDE 分支 / 普通 `Search` | `engine.go:960-989` | 串 | 三选一 |
| P6a | ↳ `hydeSearch`（1 次 LLM + 1 次 Embed + 2 路串行检索 + RRF + Rerank） | `engine.go:980-981` → `routing.go:63-131` | **串行**（`routing.go:97` 后才 `:107`） | `shouldHyde` 且 `eff.Query != "multi"` |
| P7 | `buildContext` 上下文组装/截断 | `engine.go:996` → `context.go:35-81` | 串 | 必做 |
| P8 | `fillSourceContents` | `engine.go:997-999` | 串 | **仅 `IncludeContexts`**（评测出口） |
| P9 | `renderContext` 渲染 | `engine.go:1000` | 串 | 必做 |
| P10 | `recordStep(StepChunks)` | `engine.go:1006-1010` | 串 | sink≠nil |
| P11 | 组装 messages：system + history + user | `engine.go:1013-1017` | 串 | 必做 |

### 1.3 流式 `StreamAsk` 时序（`engine.go:639-811`）

- `out := make(chan StreamEvent)`（:640，**无缓冲**）→ 起 goroutine（:648）→ 立即返回 out（:810）。
- goroutine 内：`defer close(out)`（:649）→ sink 构造（:654-662，`TraceSinkFunc` 直接把 thinking 事件写通道）→ `effective`（:663）→ 与 Ask 同构的 routing/strategy 分流（:670-723）。
- **策略路径命中**（`strategyRes != nil`，:726-731）：发 `Sources` → 发**一次性** `Chunk`（整段答案）→ `Done`，**不做真流式**。
- 否则走 `prepare`（:737）→ 错误发 `EventError` 并 return（:738-741）。
- **增强模式**（:745-766）：`forceWebSearchFallback` + `runToolLoop` 静默跑完 → 一次性发 Sources/Chunk → 落历史 → Done（**同样非真流式**）。
- 普通路径：`sendEvent(Sources)`（:768）→ 空检索兜底（:771-777）→ `llm.StreamGenerate`（:779）→ 逐 chunk 转发（:786-798）→ `ctx.Err()` 检查（:801-803，**取消则不落历史、不发 Done**）→ 落历史（:805-806）→ `EventDone`（:807）。

### 1.4 事件顺序契约

`engine.go:30-40` 明确定义：**thinking×N → sources → chunk×N → done（或 error 终止）**。`EventError` 一旦发出即终止，不再有 chunk/done（`handler_chat.go:188-191` return）。

### 1.5 并发点汇总

| 并发点 | 位置 | 并发度 |
|---|---|---|
| 向量检索 ∥ BM25 检索 | `retriever.go:96-112` | 2 |
| 多路查询检索 | `retriever.go:301-328` | `retriever.MultiQueryConcurrency` 默认 3（`config.go:421-423`） |
| 分解子问题检索 | `decompose.go:179-201` | `RAG.MultiQueryConcurrency` 默认 3（`config.go:487-489`） |
| LLM 调用 | `llm.go:128` `rate.NewLimiter(QPS)` | QPS 默认 10 |
| Web 搜索 | `bocha.go:51` | QPS 默认 1 |

**其余全部串行**：路由判定、改写、分解判定/列表、Step-Back 判定、HyDE 生成、上下文组装、生成、落历史。

---

## 2. Query 改写与各增强策略

### 2.1 单查询改写（rewrite）

**开关配置项名**：
- 代码级：`RAGConfig.EnableRewrite *bool`（`config.go:202`），语义「nil 视为启用」→ `RewriteEnabled()`（`config.go:228-230`）
- YAML：`rag.enable_rewrite`（`config.go:202`；`configs/config.yaml:162` = `true`；`applyDefaults` 兜底 true，`config.go:473-476`）
- 生效门控：`eff.Query != "multi"` 才走单查询改写（`engine.go:932`），即 multi 模式下**单查询改写被多查询取代**。

**调用点**：`rewriteQuery`（`engine.go:1023-1042`），模型 = 全局 `cfg.LLM.Model`（`llm.go:306-309`，无 per-task 模型，`WithModel` 仅 `internal/eval/judge.go:88` 使用），温度 **0.1**（`engine.go:1031`），单条 user 消息。

**Prompt 原文（逐字，`internal/rag/prompt.go:24-32`）**：

```
将用户问题改写为自包含、适合检索的独立查询。结合对话历史消解指代（如「它」「这个」），保留关键信息，不要添加资料中不存在的内容。仅输出改写后的查询本身，不要任何解释。
{{if .History}}
对话历史：
{{- range .History}}
{{.Role}}: {{.Content}}
{{- end}}
{{end}}
用户问题：{{.Question}}
改写后的查询：
```

**多轮指代消解**：**有**。`rewriteQuery` 接收 `history []llm.Message`（`engine.go:1023`），模板以 <span v-pre>`{{.Role}}: {{.Content}}`</span> 渲染历史（`prompt.go:26-29`），prompt 明确要求「结合对话历史消解指代（如「它」「这个」）」。`prepare` 传的是 `history, err := e.history.Get(sessionID, ragCfg.HistoryLimit)`（`engine.go:848`）。

**失败降级**（`engine.go:934-953`）：
1. 模板渲染失败 → `fmt.Errorf("渲染改写提示失败: %w")`（`engine.go:1026`）
2. LLM 调用失败 → `fmt.Errorf("改写调用失败: %w")`（`engine.go:1034`）
3. 结果 `TrimSpace` 后为空 → `fmt.Errorf("改写结果为空")`（`engine.go:1038-1040`）
4. 任一失败 → `query` 保持原问题，记 `RewriteData{Fallback: true}` 思考步骤（`engine.go:936-942`），**不中断请求**

**改写结果只用于检索，不用于生成**：`query = rewritten`（`:944`）只进入 `retriever.Search` 的 `Query`；生成消息用 `question`（`engine.go:1016`）。测试断言：`TestAsk_FullChain`（`engine_test.go:218`、`:241-246`）。

### 2.2 多查询（Multi-Query）

| 维度 | 内容 |
|---|---|
| 触发条件 | `eff.Query == "multi"`（`engine.go:855`）。`eff.Query` 来自三级合并，默认 `"multi"`（`strategy.go:55`、`config.go:502-504`）；routing 判 direct 时被 `ForceSingle` 强制为 `"single"`（`engine.go:828-830`）；非向量数据源也强制 single（`engine.go:831-834`） |
| 开关配置项 | `rag.multi_query_enabled`（`*bool`，`config.go:203`，`MultiQueryOn()` `:233-235`，**nil 视为关闭**）→ 仅在 `effective` 的旧开关兜底分支里映射为 `Query="multi"`（`engine.go:221-226`）；模板 `rag.multi_query_template_path`；数量 `rag.multi_query_count`（默认 3，`config.go:484-486`）；并发 `rag.multi_query_concurrency`（默认 3，`:487-489`） |
| Prompt 原文（`prompt.go:35-43`） | 见下 |
| 模型/温度 | 全局模型，temp **0.1**（`engine.go:1082`） |
| 合并去重 | `multiQuery` 内部：以 `question` 打头 + 去重变体（`seen` map，过滤空串，`engine.go:1093-1104`）；变体 ≤1 个则报错「多查询变体为空」（:1105-1107）。跨路：`FuseMultiQuery` 按 ID 累加 `1/(k+rank+1)` 做 RRF（`rrf.go:65-106`）；`SearchMulti` 传入 `k=60`（`retriever.go:351`） |
| 额外延迟 | **+1 次 LLM** + N 路并发检索（N≤4）+ 融合 + 1 次整体 Rerank（`retriever.go:365`） |
| 失败降级 | 生成失败 / 非 JSON 数组 / 变体为空 → warn 后回落单查询改写（`engine.go:859-883`），再失败则用原问题 |

`defaultMultiQueryTemplate`（`internal/rag/prompt.go:35-43`）原文：

```
根据用户问题生成 {{.Count}} 个不同表达角度的检索查询变体，用于多路召回提升检索效果。变体应覆盖：同义改写、不同细节粒度、可能的隐含子主题。结合对话历史消解指代（如「它」「这个」）。只输出 JSON 数组字符串（如 ["变体1","变体2","变体3"]），不要任何解释或其他内容。
{{if .History}}
对话历史：
{{- range .History}}
{{.Role}}: {{.Content}}
{{- end}}
{{end}}
用户问题：{{.Question}}
查询变体：
```

> 注：`multiQuery` 在 `prepare` 路径下会传入 history（`engine.go:858`）；但 `tryMultiQuery`（routing 分流）传入的是 `nil`（`routing.go:138`）——**路由路径的多查询不做指代消解**。

### 2.3 问题分解（Decomposition）

| 维度 | 内容 |
|---|---|
| 触发条件 | ① routing 判定 `strategy == "decomposition"`（`engine.go:556-559`）；② 或 `eff.Decomposition != "off"`（`engine.go:570-573`）。均在数据源为 vector_store 时 |
| 开关配置项 | `rag.decomposition_enabled`（`*bool`，nil=关，`config.go:238-240`）；模式 `rag.decomposition_mode`（`off/parallel/sequential`，默认 `parallel`，`config.go:491-493`）；上限 `rag.decomposition_max_sub`（默认 5，`:494-496`）；判定模板 `rag.decomposition_template_path` |
| Prompt | 判定 `defaultDecomposeJudgeTemplate`（`prompt.go:46-50`）；列表 `defaultDecomposeListTemplate`（`prompt.go:53-55`，**不可配置**，`prompt.go:109` 明确注释「与判定共用一个路径会产生歧义」） |
| 模型/温度 | 全局模型，两次调用均 temp **0.0**（`decompose.go:36`、`:62`） |
| 执行 | `tryDecompose`（`decompose.go:131-258`）：判定 → `listSubQuestions`（去重/限长 `:71-88`）→ 逐子问题检索（`parallel` 并发信号量 `:179-201` / `sequential` 顺序 `:166-176`）→ 汇总 → 整体 Rerank **一次**（`:223`）→ `buildContext` → 一次综合生成（`:250`） |
| 合并去重 | 子问题层面去重（`decompose.go:72-84`）；**检索结果层面无去重**——直接 `append` 拼接（`:214-217`），依赖 rerank 重排 |
| 额外延迟 | **+2 次 LLM**（判定 + 列表）+ N 路检索 + 1 次 Rerank + 综合生成额外携带大上下文 |
| 失败降级 | 判定失败 / 不分解 → `(nil, false, err)`，调用方落常规路径（`engine.go:557-559`）；无检索结果 → `noAnswerText`（`decompose.go:218-221`） |

`defaultDecomposeJudgeTemplate`（`prompt.go:46-50`）原文：

```
判断以下问题是否需要分解为多个子问题分别检索。
需要分解：复合型问题（含多个独立信息需求）、需要对比分析、需要多步骤解答、问题复杂抽象。
不需要分解：简单事实查询、单一明确问题、定义类问题。
只输出 JSON：{"decompose": true 或 false, "reason": "简短理由"}，不要其他内容。
用户问题：{{.Question}}
```

`defaultDecomposeListTemplate`（`prompt.go:53-55`）原文：

```
将以下复杂问题分解为 {{.MaxSub}} 个以内、相互独立且可分别检索的子问题。每个子问题应能独立检索到相关信息。只输出 JSON 数组字符串（如 ["子问题1","子问题2"]），不要任何解释。
用户问题：{{.Question}}
子问题：
```

综合生成的 user 内容与常规不同（`decompose.go:248`）：`contextText + "\n\n用户问题：" + question + "\n\n请综合以上资料，全面回答该问题。"`

### 2.4 Step-Back

| 维度 | 内容 |
|---|---|
| 触发条件 | `eff.Decomposition == "off" && eff.StepBack == "on"`（`engine.go:570-578`，**与 Decomposition 互斥**，Decomposition 优先） |
| 开关配置项 | `rag.step_back_enabled`（`*bool`，nil=关，`config.go:242-245`）；模板 `rag.step_back_template_path` |
| Prompt | `defaultStepBackJudgeTemplate`（`prompt.go:58-63`） |
| 模型/温度 | 全局模型，temp **0.0**（`decompose.go:99`） |
| 执行 | `tryStepBack`（`decompose.go:261-344`）：判定 → 回退问题检索 + 原问题检索（**串行**，`:283`、`:288`）→ 合并 → 整体 Rerank 一次（`:310`）→ 生成 |
| 合并去重 | `allChunks := append(backChunks, origChunks...)`（`:305`）——**无 ID 去重** |
| 额外延迟 | **+1 次 LLM** + 1 次额外检索 + 1 次 Rerank |
| 失败降级 | 判定失败/不需要回退 → `(nil,false,err)` 落常规（`engine.go:575-577`） |

`defaultStepBackJudgeTemplate`（`prompt.go:58-63`）原文：

```
判断以下问题是否适合「先退一步检索更抽象/更广泛的信息，再精答」的策略。
适合：需要高层概念理解、时间序列/趋势（如「最近」「历年」）、多步推理、需要广泛背景的问题。
不适合：简单事实查询、定义类问题、需要实时数据的问题。
如果适合，生成一个更抽象、更通用的回退问题用于检索。
只输出 JSON：{"step_back": true 或 false, "question": "回退问题（step_back 为 true 时）"}，不要其他内容。
用户问题：{{.Question}}
```

### 2.5 HyDE

| 维度 | 内容 |
|---|---|
| 触发条件 | `shouldHyde(eff.HyDE, routeResult{Complexity: o.RouteComplexity}) && eff.Query != "multi"`（`engine.go:980`）。`shouldHyde` 要求 `effHyDE == "on"` **且** `e.embedder != nil`（`routing.go:51-59`） |
| 开关配置项 | `rag.hyde_enabled`（`*bool`，nil=关，`config.go:252-255`）；`rag.hyde_skip_simple`（`*bool`，**nil 视为 true=跳过**，`config.go:257-260`）；模板 `rag.hyde_template_path` |
| Prompt | `defaultHyDETemplate`（`prompt.go:84-86`） |
| 模型/温度 | 全局模型，temp **0.3**（`routing.go:75`，全链路唯一非低温的辅助调用） |
| 执行 | `hydeSearch`（`routing.go:63-131`）：生成假设文档 → Embed → `SearchByVector`（HyDE 路）→ `Search`（原查询路，`SkipRerank: true`）→ `FuseMultiQuery(...,60,TopK)` → 整体 Rerank 一次（`:129`） |
| 合并去重 | `FuseMultiQuery` RRF（`rrf.go:65`），k=60 |
| 额外延迟 | **+1 次 LLM + 1 次 Embedding + 1 次额外向量检索 + 1 次 Rerank**；两路检索**串行**（`:97` → `:107`），非并发 |
| 失败降级 | 4 处降级为原查询单路 `Search`：模板渲染失败（`:70-72`）、生成失败（`:78-80`）、Embed 失败或空向量（`:91-94`）、向量检索失败（`:98-101`）；原查询路失败则只用 HyDE 路结果（`:113-116`） |

`defaultHyDETemplate`（`prompt.go:84-86`）原文：

```
请根据以下问题，写一段详细的假设性文档作为检索查询。即使不确定，也要写得像真实文档一样具体、详细，包含关键术语与可能的相关信息。这段假设文档将用于向量检索真实文档。
问题：{{.Question}}
假设性文档：
```

### 2.6 Routing（路由）

| 维度 | 内容 |
|---|---|
| 触发条件 | `eff.Routing == "auto"`（`engine.go:523` / `:670`） |
| 开关配置项 | `rag.routing_enabled`（`*bool`，nil=关，`config.go:247-250`）；回退策略 `rag.routing_fallback`（默认 `multi_query`，`config.go:498-500`）；模板 `rag.routing_template_path` |
| Prompt | `defaultRoutingTemplate`（`prompt.go:66-81`） |
| 模型/温度 | 全局模型，temp **0.0**（`routing.go:31`） |
| 输出 | `routeResult{Complexity, Strategy, DataSource, Reasoning}`（`routing.go:15-20`），单次 JSON 解析 |
| 分流 | `direct`→强制 single（`engine.go:549-551`）；`multi_query`→`tryMultiQuery`；`decomposition`→`tryDecompose` |
| 数据源 | `resolveDataSource`：空/`auto`/未知 → `vector_store`；不在 `allowed` → `allowed[0]`；不可用 → allowed 内首个可用源 → 兜底 vector_store（`engine.go:470-498`） |
| 额外延迟 | **+1 次 LLM**（每次请求，无缓存） |
| 失败降级 | `err != nil \|\| !ok` → `strategy = ragCfgFor(o).RoutingFallback`（`engine.go:526-528`），并**不 Record 思考步骤**（`:542`, `:689` 注释） |
| 互斥约束 | `ValidateStrategy` 禁止 `routing=auto` 与 `decomposition≠off`、`step_back=on` 组合（`strategy.go:119-127`），非法时 `effective` 降级 `DefaultEffectiveStrategy()`（`engine.go:252-256`） |

`defaultRoutingTemplate`（`prompt.go:66-81`）原文：

```
分析以下查询的复杂度、选择合适的检索策略，并选择本次查询使用的数据源。
复杂度：
- simple：简单事实查询、单一明确问题、定义类问题
- medium：需要多角度召回、同义改写有帮助的问题
- complex：复合型问题（多信息需求）、需要对比分析、多步骤解答、复杂抽象问题
策略：
- direct：直接检索（simple 用）
- multi_query：多查询多路召回（medium 用）
- decomposition：问题分解后逐子问题检索综合（complex 用）
数据源：
- vector_store：向量知识库（企业内部文档）
- web_search：web 搜索（外部互联网）
可选数据源（本次查询只能从以下范围内选择，不得超出）：
{{.AllowedText}}
只输出 JSON：{"complexity": "simple|medium|complex", "strategy": "direct|multi_query|decomposition", "data_source": "vector_store|web_search", "reasoning": "简短理由"}，不要其他内容。
用户问题：{{.Question}}
```

`allowedText` 渲染（`engine.go:501-506`）：allowed 为空 → `"vector_store（默认）"`，否则 `strings.Join(allowed, " / ")`。

### 2.7 三级策略合并与默认值

`ResolveStrategy(global, kb, req)`（`strategy.go:37-76`）字段级 pick：`req` 非空覆盖 `kb`，`kb` 非空覆盖 `global`，全空取 def。默认值：`Query=multi`、`Fusion=rrf`、`Decomposition=off`、`StepBack=off`、`HyDE=off`、`Routing=off`、`Thinking=off`（`strategy.go:55-61`）；`DataSources` 全空 → `nil`（`:63-70`）。

**旧开关兜底分支**（`engine.go:214-244`）：仅当 `global` 的 `Query/Fusion/Decomposition/StepBack/HyDE/Routing` **全为空**时才从 `*On()` 旧开关推导。生产路径下 `applyDefaults` 已把它们填满非空（`config.go:502-519`），因此该分支在生产配置下**不可达**，只在测试直接构造 `RAGConfig` 时生效（如 `TestEffectivePreservesDataSourcesWithLegacyFallback`，`engine_datasource_test.go:253-265`）。

---

## 3. 上下文组装与 Token 预算

### 3.1 Token 计数：**估算函数，无 tokenizer**

`estimateTokens`（`internal/rag/context.go:146-183`）：

```go
// estimateTokens 轻量 token 估算：中文字符 2 token、英文按词 1、标点 1（与 chunker 思路一致）
func estimateTokens(text string) int {
	if text == "" { return 0 }
	count := 0
	wordLen := 0
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			if wordLen > 0 { count++; wordLen = 0 }
			count += 2
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			if wordLen > 0 { count++; wordLen = 0 }
			count++
		case unicode.IsSpace(r):
			if wordLen > 0 { count++; wordLen = 0 }
		default:
			wordLen++
		}
	}
	if wordLen > 0 { count++ }
	return count
}
```

**结论**：纯规则估算（汉字 2 / 英文词 1 / 标点符号 1），**未使用任何 tokenizer（tiktoken / BPE / HuggingFace）**。测试断言：`TestEstimateTokens`（`context_test.go:89-101`）验证 `"中文"→4`、`"hello world"→2`。估算基准单位是 `formatContextItem(item)`（`context.go:95-113`），即 `[n]（来源：文件 / 标题）\n内容\n\n` 的默认渲染格式——**注意：如果配置了自定义 `context_template_path`，实际渲染体积与估算基准可能不一致**（`renderContext` 用 `e.templates.context`，`engine.go:1000`）。

### 3.2 预算默认值与语义

| 配置项 | YAML 名 | 默认值 | 代码位置 |
|---|---|---|---|
| RAG 检索 TopK | `rag.top_k` | **5** | `config.go:464-466` |
| 上下文 token 预算 | `rag.max_context_tokens` | **2048** | `config.go:467-469` |
| 上下文章节数上限 | `rag.max_chunks` | **5** | `config.go:470-472` |
| 历史注入条数 | `rag.history_limit` | **10** | `config.go:480-482` |
| 历史容量 | `rag.history_capacity` | **50** | `config.go:477-479` |
| LLM 单次生成上限 | `llm.max_tokens` | **2048** | `config.go:447-449` |
| LLM 上下文窗口上限 | — | **代码中未找到** | — |
| 留给回答的 token 预算 | — | **代码中未找到**（`llm.max_tokens` 存在但与 `max_context_tokens` 完全独立，无「上下文 + 输出 ≤ 窗口」校验） | — |
| 每条 chunk 的 token/字符上限 | — | **代码中未找到**（检索阶段无单条截断）。最接近的是入库侧 `chunker.chunk_size` 默认 **512**（`config.go:399-401`）与联网结果单条 800 字符截断（`websearchtool.go:41`、`:78`） | — |

`buildContext` 自身兜底：`maxTokens<=0 → 2048`、`maxChunks<=0 → 5`（`context.go:36-41`）。

### 3.3 超预算裁剪顺序

`buildContext`（`context.go:35-81`）：

```go
for i, chunk := range chunks {
	if i >= maxChunks { break }
	item := ContextItem{Index: i + 1, Filename: ..., Heading: ..., Content: chunk.Content}
	if used+estimateTokens(formatContextItem(item)) > maxTokens { break }
	items = append(items, item)
	used += estimateTokens(formatContextItem(item))
	sources = append(sources, Source{...})
}
```

- **顺序**：完全按 `chunks` 传入顺序（`retriever` 返回顺序 = rerank 后的分数降序，或融合后降序）。
- **裁剪方式**：**整条 break，不截断单条内容**。
- **无按来源优先级**、**无按分数重排**（依赖上游已排序）、**无「跳过超预算项继续尝试后续小项」**。
- 关键副作用：若**首条**就超预算 → `items`/`sources` 均为空 → 触发 `engine.go:591` 的空检索兜底，返回「未找到相关资料。」而**完全不调用 LLM**，即使库里确实有内容。
- 空检索判定依据是 `len(sources) == 0`，而非 `len(chunks) == 0`。

### 3.4 去重与相邻 chunk 合并

- **跨路 ID 去重**：仅存在于 `FuseMultiQuery`（以 ID 为 map key 累加 RRF 分数，`rrf.go:73-96`）与 `FuseRRF`（`rrf.go:13`）。
- **分解路径**：`append` 拼接子问题结果，**无 ID 去重**（`decompose.go:214-217`）。
- **Step-Back 路径**：`append(backChunks, origChunks...)`，**无 ID 去重**（`decompose.go:305`）。
- 上述两处若 Reranker 关闭或失败（`rerankIfEnabled` 原样返回，`retriever.go:164-167`），重复 chunk 会直接进入上下文并**各自占用一个引用编号**。
- **相邻 chunk 合并（adjacent merge）**：**代码中未找到**。无同文档相邻序号的合并逻辑，也没有 overlap 消除。

### 3.5 引用编号 [1][2] 与来源映射

- `ContextItem.Index = i + 1`（`context.go:53`）——**按截断后位置编号**。
- `Source` 结构体（`context.go:12-23`）**没有 index 字段**，映射关系是**纯数组位置对齐**：`sources[k] ↔ 上下文 [k+1]`。`Source` 返回字段：
  - `id` 片段 ID、`filename` 来源文件名（`metadata["filename"]`）、`heading`（`metadata["heading_context"]`）、`score` 检索分数
  - `source_type`（`metadata["source_type"]`，omitempty）
  - `start_ms`/`end_ms`（音视频定位，omitempty）
  - `page_number`（PDF 页码，omitempty，从 metadata 转 int）
  - `anchor`（Markdown 锚点，omitempty）
  - `content`（**仅 `include_contexts=true` 时填充**，omitempty，`fillSourceContents` `context.go:86-92`）
- `[编号]` 由 **LLM 自行标注**（prompt 要求），服务端**不校验**回答里的 `[n]` 是否越界或确实被引用。
- 思考链路 `ChunksData` 与 `sources` 同集合、同顺序（`chunksDataFrom` `thinking.go:206-218`），测试断言两者 ID 集合一致（`engine_test.go:1315-1327`）。
- **编号冲突风险**：联网搜索结果文本也自带 `[1] [2]` 编号（`websearchtool.go:73`），与知识库上下文的 `[n]` 编号体系并存于同一 messages 中但语义不同。

### 3.6 上下文与历史的关系

`max_context_tokens` **只约束检索上下文文本**，不含 system prompt、不含历史消息、不含用户问题、不含输出。历史以 `history...` 原样展开插入消息序列（`engine.go:1015`），不参与预算计算。因此实际请求 token 上限无法由 `max_context_tokens` 保证。

---

## 4. Prompt 模板

所有模板用 `text/template` 渲染（`prompt.go:134-144`），默认内置，可由配置路径覆盖（`loadPromptTemplates` `prompt.go:102-114`、`loadOrDefault` `:116-125`；读文件失败**静默降级默认**）。

### 4.1 System Prompt（`internal/rag/prompt.go:14-16`，逐字）

```
你是一个基于企业知识库的问答助手。请严格基于以下检索到的资料回答用户问题。
基于检索到的资料尽力回答；资料未覆盖的部分，明确指出「资料未覆盖该方面」，不要编造。
回答时按 [编号] 标注引用来源，如 [1][2]。
```

- 配置项：`rag.system_prompt_path`（`config.go:222`），空则用内置。
- 加载点：`loadPromptTemplates(cfg)`（`prompt.go:104`）→ `e.templates.system`（`engine.go:267`）→ 注入为 messages[0]（`engine.go:1014`、`decompose.go:247`、`decompose.go:333`、`routing.go:181`）。

**增强模式专用 system prompt（`internal/rag/engine.go:292-295`，逐字）**：

```
你是一个基于企业知识库的问答助手，同时具备联网搜索能力。
你可以使用 web_search 工具搜索互联网，获取知识库未覆盖或需要实时更新的信息。
当检索到的资料不足以回答用户问题（包括资料未覆盖、问题涉及最新/实时信息、需要外部事实核实时），必须调用 web_search 工具获取相关信息，再基于检索结果回答；禁止直接回答「资料未覆盖」。
回答时按 [编号] 标注引用来源，如 [1][2]。
```

**增强模式下会覆盖掉用户配置的 system prompt**：`runToolLoop` 直接改写第一条 system 消息内容（`engine.go:358-369`）：

```go
systemReplaced := false
for i := range messages {
	if messages[i].Role == llm.RoleSystem {
		messages[i].Content = enhancedSystemPrompt
		systemReplaced = true
		break
	}
}
if !systemReplaced {
	messages = append([]llm.Message{{Role: llm.RoleSystem, Content: enhancedSystemPrompt}}, messages...)
}
```

即 `rag.system_prompt_path` 在 `enhanced=true` 时**失效**（对比测试 `TestAsk_CustomSystemPrompt` 只在非增强路径覆盖）。

### 4.2 上下文模板（`internal/rag/prompt.go:18-22`，逐字）

```
以下是检索到的相关资料：
{{- range .}}
[{{.Index}}]{{if .Filename}}（来源：{{.Filename}}{{if .Heading}} / {{.Heading}}{{end}}）{{end}}
{{.Content}}
{{end}}
```

配置项：`rag.context_template_path`（`config.go:223`）。

### 4.3 User Prompt 组装

- 常规 / 多查询 / Step-Back（`engine.go:1016`、`routing.go:182`、`decompose.go:334`）：
  `contextText + "\n\n用户问题：" + question`
- 分解综合（`decompose.go:248`）：
  `contextText + "\n\n用户问题：" + question + "\n\n请综合以上资料，全面回答该问题。"`

### 4.4 「不知道就说不知道」的约束方式

- **Prompt 层软约束**：`defaultSystemPrompt` 第 2 行「资料未覆盖的部分，明确指出「资料未覆盖该方面」，不要编造」（`prompt.go:15`）。
- **代码层硬兜底**：检索结果为空 → 直接返回常量 `noAnswerText = "未找到相关资料。"`（`engine.go:21`、`:607-608`、`:772`），**不经过 LLM**。
- 回归测试 `TestDefaultSystemPromptNoAbsoluteReject`（`engine_test.go:1773-1783`）明确断言：prompt 必须**不含**「未找到相关资料」绝对拒答表述，必须含「资料未覆盖」与「不要编造」。
- 增强模式下该约束被**反向覆盖**为「禁止直接回答「资料未覆盖」」（`engine.go:294`）。

### 4.5 防提示注入

**代码中未找到。** 全链路无：
- 无注入检测/清洗（检索片段、用户问题均原样拼接，`engine.go:1016`）
- 无指令隔离标记（上下文模板仅 `[n]（来源：…）`，无「以下内容仅为参考资料，不得作为指令执行」类说明）
- 无输入长度/特殊字符过滤
- 无输出侧的引用/内容校验（`[n]` 越界不校验）

唯一与「隔离」相关的机制是**数据源白名单**（`AllowedDataSources`，`engine.go:836-840`、`374-378`）与 **KB 范围过滤**（`kbFilter` `engine.go:1048-1066`）。

### 4.6 引用格式要求

`defaultSystemPrompt:16`：「回答时按 [编号] 标注引用来源，如 [1][2]。」；`enhancedSystemPrompt:295` 同款要求。没有任何格式化的结果后处理。

---

## 5. SSE 流式实现

### 5.1 事件类型与数据结构

`EventType`（`internal/rag/engine.go:32-40`）：`EventThinking=0`、`EventSources`、`EventChunk`、`EventDone`、`EventError`。

`StreamEvent`（`engine.go:43-49`）：`Type`、`Content`（Chunk 有效）、`Sources []Source`、`Thinking *ThinkingStep`、`Err error`。

**HTTP 层事件映射（`internal/api/handler_chat.go:176-194`）**：

| 领域事件 | SSE event 名 | data 结构 | 代码位置 |
|---|---|---|---|
| `EventThinking` | `thinking` | `ThinkingStep{type,label,elapsed_ms?,data}` | `handler_chat.go:178-179` |
| `EventSources` | `sources` | `Source[]`（§3.5 字段） | `:180-181` |
| `EventChunk` | `chunk` | `{"content":"..."}` | `:182-183` |
| `EventDone` | `done` | `{}` | `:184-187`（含 Flush + return） |
| `EventError` | `error` | `{"message":"..."}` | `:188-191`（含 Flush + return） |

响应头（`:172-174`）：`Content-Type: text/event-stream`、`Cache-Control: no-cache`、`Connection: keep-alive`。

> 注意：`ThinkingStep.ElapsedMS`（`thinking.go:30`）**全仓库无任何赋值点**（`grep ElapsedMS` 仅命中定义），因此 `elapsed_ms` 恒被 omitempty 省略——思考链路的「耗时」字段实际是空的。

### 5.2 Flush 机制

`c.Writer.Flush()` 在每个事件处理末尾调用（`handler_chat.go:193`），`Done`/`Error` 分支各自显式 Flush 后 return（`:186`、`:190`）。**每个 chunk 一帧、不聚合**，无攒批。

**缺少**：
- 无 `X-Accel-Buffering: no` 头（nginx 反代可能缓冲导致流式失效）
- 无 `Transfer-Encoding`/`Content-Length` 处理
- 未使用 `gin.Context.Stream`

### 5.3 心跳

**代码中未找到。** 无 ticker、无 `: ping` 注释帧、无 idle 超时。`grep -ri "heartbeat|keepalive|time.Ticker"` 在 `internal/api` 仅命中 `handler_chat.go:174` 的 `Connection: keep-alive` 响应头（那是连接语义，不是应用层心跳）。

**首字节延迟风险**：`sources` 事件要等 `prepare` 全部完成（历史读取 → 改写 LLM → 检索 → Rerank → 组装）才发。在 `routing=auto` + 分解/多查询开启时，首个事件前需串行经历最多 3 次 LLM 调用（路由 → 判定 → 列表）+ N 路检索 + Rerank。唯一缓解手段是 **thinking 事件**（`engine.go:658-660`）：开启时每个环节完成即发一个事件，等价于进度心跳。

### 5.4 客户端断连检测与 context 取消

- `StreamAsk(ctx, ...)` 的 ctx 来自 `c.Request.Context()`（`handler_chat.go:166`）；Go net/http 在客户端断开时取消该 ctx。
- 发送侧：`sendEvent`（`engine.go:1112-1119`）用 `select { case out <- ev: ...; case <-ctx.Done(): return false }`，失败即由调用方 return。
- 检查点：
  - `engine.go:795-797` chunk 转发失败即 return
  - `engine.go:801-803` `ctx.Err() != nil` → **丢弃截断结果、不落历史、不发 Done**
- LLM 侧：`sendChunk`（`stream.go:191-198`）同样 select ctx；`sendErr`（`stream.go:201-205`）在 ctx 已取消时**静默丢弃**错误。
- 测试：`TestStreamAsk_ContextCancel`（`engine_test.go:521-572`）断言取消后**无 EventDone** 且**历史为空**。

### 5.5 首字节延迟优化：引用先发还是正文先发

**引用先发**：`sendEvent(EventSources)` 在 `StreamGenerate` 之前（`engine.go:768` vs `:779`）。测试 `TestStreamAsk_EventSequence`（`engine_test.go:368`）硬断言 `types[0] == EventSources && types[3] == EventDone`（4 个事件：sources/chunk/chunk/done），前后端因此可在正文到达前渲染引用卡片。

### 5.6 流式过程中的错误下发

分三层：
1. **LLM 传输层**（`stream.go:20-61`）：收到首个 delta **之前**失败 → 限流 + 指数退避重试（`1<<(attempt-1)` 秒，最多 `MaxRetries` 默认 3，`config.go:435-437`）；收到首个 delta **之后**失败 → **不重试**，`sendErr` 透传（`stream.go:44-48`），避免重复输出。退避期间 select ctx（`:31-35`）。
2. **Engine 层**：`StreamGenerate` 返回 err → `EventError`（`engine.go:780-782`）；chunk.Err → `EventError` 并 return（`:787-790`）。
3. **Handler 层**：`EventError` → `c.SSEvent("error", {"message": ...})` + Flush + return，**不发 done**。测试 `TestChatSSEError`（`api_test.go:910-933`）断言有 `event:error`、有错误文案、**无 done 且无 chunk**。
- HTTP 状态码恒为 200（SSE 已开始），错误只能靠 event 表达；`StreamAsk` 自身唯一返回 error 的路径是 `prepare` 之前的同步错误（但 `StreamAsk` 实际只在 goroutine 内返回，函数体外层永返回 `nil, nil`，见 `engine.go:810`）——**因此 handler 的 `if err != nil` 分支（`handler_chat.go:167-170`）不可达**。

### 5.7 goroutine 生命周期与泄漏风险

- `StreamAsk` 起 1 个 goroutine，`defer close(out)`（`engine.go:648-649`）保证通道必关。
- `StreamGenerate` 起 1 个 goroutine（`stream.go:23`），`defer close(out)`（`:24`）。
- **背压**：`out` 与 `StreamGenerate` 的 `out` 都是**无缓冲**通道，发送方与接收方逐帧同步。若 handler 停止消费且 ctx 未取消，两个 goroutine 都会阻塞在 send —— 正常情况下 handler 会消费到 close，故不会。
- **潜在风险点**：
  1. `engine.go:795-797` 转发失败时 `return`，**不排空** `ch` —— 但该分支只在 `sendEvent` 因 ctx.Done 返回 false 时触发，此时 `sendChunk` 的 select 也会命中 ctx.Done，生产者随即退出，`close(out)` 正常执行。**当前无实际泄漏**，但这是「依赖 ctx 一定已取消」的隐式契约，属于脆弱点。
  2. `StreamAsk` 的 goroutine 在 client 断连后仍可能继续跑完当前阻塞的 LLM 调用（`llm.Generate` 在 `enhancedAnswer`/`runToolLoop` 中同步阻塞），直到下一次 `sendEvent` 才发现取消。**增强模式的 tool loop 最长可能多跑 4 轮 LLM + 多次 web 搜索才退出**（`engine.go:393-461`），这是明确的资源放大面。
  3. 无 `sync.WaitGroup`/goroutine 计数，无并发上限，无 `pprof` 埋点。

---

## 6. 多轮历史

### 6.1 存取实现

| 层 | 类型 | 位置 |
|---|---|---|
| 接口 | `rag.HistoryStore`（无 ctx）：`Append/Get/Clear` | `internal/rag/history.go:10-14` |
| 生产实现 | `store.PostgresHistoryStore`（带 ctx） | `internal/store/history.go:18-68` |
| 适配器 | `ragHistoryAdapter`（用 `context.Background()` 抹平 ctx） | `internal/app/app.go:334-349` |
| 内存实现 | `memoryHistoryStore`（map + RWMutex） | `internal/rag/history.go:16-73` |
| 内存实现使用范围 | **仅评测装配**（`app.go:381`）与测试 | — |

**PG 表与字段**（`internal/store/schema.go:50-57`）：

```sql
CREATE TABLE IF NOT EXISTS chat_history (
    id         BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    role       TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chat_history_session ON chat_history(session_id, created_at);
```

`sources` 列由幂等迁移补上（`schema.go:76-78`）：`ALTER TABLE chat_history ADD COLUMN IF NOT EXISTS sources TEXT NOT NULL DEFAULT '';`

**写入**（`store/history.go:23-29`）：单条 `INSERT INTO chat_history (session_id, role, content, sources)`，无事务、无批量。
**读取**（`:32-62`）：`limit>0` 时子查询按 `created_at DESC LIMIT` 再外层 `ORDER BY created_at ASC`（时间正序取最近 N 条）；`limit<=0` 返回全部。
**清空**（`:65-67`）：`DELETE ... WHERE session_id = $1` —— 但**无任何 HTTP 路由或 engine 调用它**（`grep .Clear(` 在 `internal/api`、`internal/rag` 非测试代码中零命中；`router.go` 只有 `GET /chat/history`），即 `Clear` 是死代码。

### 6.2 窗口大小与超长处理

- **注入条数**：`history.Get(sessionID, ragCfg.HistoryLimit)`（`engine.go:848`），`rag.history_limit` 默认 **10**（`config.go:480-482`）。
- **内存实现容量**：`rag.history_capacity` 默认 **50**（`config.go:477-479`），超出丢弃最旧（`history.go:42-45`）。PG 实现**无视 capacity**，只按 `limit` 取，表无限增长。
- **超长历史的截断/摘要**：**无摘要、无 token 级裁剪**。只有「按条数取最近 N 条」（`history.go:57-59` / `store/history.go:36-39`）。历史消息的 token 占用**不计入** `max_context_tokens`（§3.6），因此长历史会直接挤压模型上下文窗口。

### 6.3 session_id 语义

- 由**客户端提供**，`binding:"required"`（`handler_chat.go:16`），原样作为历史分区键（`engine.go:848`、`:632-633`）。
- **无归属校验**：`GetHistory`（`handler_history.go:18-30`）只按 `session_id` 查库，不校验该会话是否属于当前调用者；PG 表也**没有 user_id 列**。任何持有效 API Key / JWT 的调用者只要猜到 `session_id` 即可读取或续写他人会话历史（`kb_id` 有越权校验，`session_id` 没有）。这是链路中隔离语义最薄弱的一环。
- 空检索路径也会写历史（`engine.go:592` 写 user，`:607` 写 assistant）。

### 6.4 并发同一 session

- **无任何 per-session 互斥**（`RAGEngine` 无锁字段，`engine.go:168-178`）。
- 时序为「先 Get 历史 → 生成 → 再 Append 两条」（`engine.go:848` → `:632-633`）。两个并发请求会：各自读到**不含对方**的历史；然后两条 `INSERT` 独立提交，可能出现 `user1, user2, assistant1, assistant2` 的交错顺序，破坏 user/assistant 配对。
- 内存实现有 `sync.RWMutex`（`history.go:36`、`:53`、`:68`）保证单次操作安全，但 **Get + Append 非原子**，不解决上述交错。
- 测试 `TestAsk_Concurrent`（`engine_test.go:492-518`，8 goroutine × 10 次 × 不同 session）与 `TestHistoryStore_Concurrent`（`history_test.go:73-89`）只验证**不同 session** 无数据竞争；**同一 session 并发无测试**。
- 落历史失败只告警不阻断（`appendHistory` `engine.go:814-818`）。

### 6.5 历史消息的下发形态

`llm.Message` 含 `Sources string`（历史持久化用，`llm.go:39`）；`marshalSources`（`engine.go:1123-1137`）序列化时**显式剥离 `Content` 字段**（评测正文不入历史）。但 `toOpenAIMessages`（`llm.go:361-388`）只输出 `role`/`content`/`reasoning_content`/`tool_calls`/`tool_call_id`，**`Sources` 不下发到模型**（测试 `TestToOpenAIMessagesToolCallsNested` `llm_test.go:501-504` 断言该字段不出现在请求体中）。因此历史里的引用来源仅用于前端回放（`GET /chat/history`），不污染生成上下文。

---

## 7. 工具调用与联网搜索

### 7.1 触发条件

联网能力有**三条互不相同的路径**：

| 路径 | 触发条件 | 位置 | 是否进 Sources |
|---|---|---|---|
| **A. 增强模式 tool loop（模型自主）** | 请求体 `enhanced=true`（`handler_chat.go:20`）→ `WithEnhanced`（`:99`/`:158`）→ `o.Enhanced`；且 `e.tools` 中有未被 `AllowedDataSources` 过滤掉的工具 | `engine.go:613-623`、`:357-462` | ❌ 不进 |
| **B. 增强模式系统强制搜索** | `o.Enhanced` 且存在 `web_search` 工具：① 空检索兜底（`engine.go:594`）② 非空检索也**无条件**执行一次（`engine.go:616`） | `engine.go:314-351` | ❌ 不进 |
| **C. 数据源路由** | `routing=auto` + `data_sources` 含 `web_search` + LLM 判定 `data_source: web_search` | `engine.go:534` → `472-497` → `prepare` `:960-979` | ✅ 进（走 `buildContext`） |

**注册条件**：`NewEngine` 里仅当 `e.searchProvider != nil && e.searchProvider.Available()` 才注册 `web_search` 工具（`engine.go:284-286`）；生产装配在 `app/rebuild.go:47-49`（`search.New(cfg.WebSearch)`）。`bochaProvider.Available()` = `apiKey != ""`（`bocha.go:56`）；未配置时 `search.New` 走 `unavailableProvider`（`search.go:48-50`），`Available()==false`（`search.go:59`）。

**工具白名单**：`runToolLoop` 按 `o.AllowedDataSources` 过滤（`engine.go:372-378`），`len(allowed)>0 && !slices.Contains(allowed, t.Name()) → 跳过`；allowed 为空视为全部授权（注释 `:356`）。

**内置 `web_search` 数据源是占位不可用**（`datasource/websource.go:21` `Available()=false`，`Search` 直接返回 `"web_search 数据源未实现"`）。所以路由路径 **C 在生产默认配置下必然降级回 `vector_store`**（`engine.go:478-497`）。

### 7.2 搜索结果与私有库结果的融合排序

**代码中未找到任何融合。** 三条路径互斥：
- 路径 C 走 `resolveDataSource` 只选**一个**数据源（`engine.go:470-498`），不并行检索多源，因此不存在 web 与 vector 结果的 RRF/加权/交叉重排。
- 路径 A/B 的搜索结果以 **tool 消息**形式 `append` 到 messages 尾部（`engine.go:345-349`），不经 `buildContext`、无 score、无 RRF、不参与引用编号对齐。
- 融合机制（`FuseMultiQuery` / `FuseRRF`）只用于**同构的向量/BM25/多查询结果**之间。

唯一「排序」是 `web_search` 返回顺序原样格式化：`fmt.Fprintf(&sb, "[%d] %s\n链接: %s\n", i+1, r.Title, r.URL)`（`websearchtool.go:73`）。

### 7.3 是否进入引用来源

- **路径 C**：进入。`src.Search` 返回的 `[]retriever.RetrieveResult` 交给 `buildContext` 生成 `Source`，`source_type` 由数据源实现自己写入 metadata（`vectorstore.go:33-39` 写 `"vector_store"`；测试 `engine_datasource_test.go:204-209` 验证自定义 web 源可产出 `SourceType=web_search` 且 `Filename` 为域名）。
- **路径 A/B**：**不进入**。`forceWebSearchFallback` 只把结果塞进 messages（`engine.go:345-349`），`Ask` 在该分支返回 `&RAGResult{Answer: answer}`，**Sources 为 nil**（`engine.go:604`）；非空检索的增强分支返回的是私有库 `sources`（`:635`）。联网结果只出现在**思考链路**的 `ToolStepData.Items`（标题/URL/摘要，`thinking.go:143-156`、`engine.go:341-343`）中。

### 7.4 超时与失败降级

| 环节 | 默认值/行为 | 位置 |
|---|---|---|
| 搜索 HTTP 超时 | `web_search.timeout` 默认 **30s** | `config.go:457-459`、`bocha.go:38-41`、`:50` |
| 搜索限流 | `web_search.qps` 默认 **1**，`rate.NewLimiter`（`qps<=0` 兜底 1） | `config.go:460-462`、`bocha.go:42-45`、`:51`、`:88-90` |
| 单次返回条数 | `web_search.count` 默认 **5**；工具 `count` 参数可覆盖，0 → provider 默认 | `config.go:454-456`、`bocha.go:92-95` |
| 默认 base URL | `https://api.bochaai.com/v1/web-search` | `bocha.go:17`、`:31-33` |
| 单条正文截断 | 800 字符 | `websearchtool.go:41`、`:78` |
| 未配置 API Key | `Available()==false` → 工具不注册；`Search` 直接返回 error | `bocha.go:56`、`:84-86` |
| HTTP 非 200 | `fmt.Errorf("博查 API 错误 HTTP %d: %s")` | `bocha.go:125-127` |
| 业务错误码 | `code != 0 && != 200` → error | `bocha.go:133-135` |
| **是否重试** | **无重试**（`bochaProvider.Search` 无重试循环，仅一次 `client.Do`） | `bocha.go:83-153` |
| 工具执行失败 | 转成**文本** `"错误: " + err.Error()` 回传给模型（`role=tool`），**不终止请求** | `engine.go:337-339`、`:427-428`、`:437-439` |
| 搜索结果为空 | 返回 `"未搜索到相关结果。"` + 空 items | `websearchtool.go:66-68` |
| 未知工具名 | 记思考步骤（`Error: "未知工具"`）+ 回一条错误 tool 消息，继续循环 | `engine.go:411-418` |

**Tool loop 防死循环**：`maxToolRounds = 3`（`tool.go:9`），循环 `for round := 0; round < maxToolRounds; round++`（`engine.go:393`）；超限后再调 1 次 `GenerateTool` 取正文兜底，正文为空则用 `noAnswerText`（`engine.go:451-461`）。**总计最多 4 次生成调用**。

**增强模式无检索结果且无工具** → 回退 `noAnswerText`（`engine.go:607-608`；测试 `TestAsk_EmptyRetrievalEnhancedNoToolFallsBack` `engine_enhanced_test.go:317-337`）。

---

## 8. 测试覆盖证据

### 8.1 `internal/rag`（编排核心）

| 测试函数 | 断言行为 | 位置 |
|---|---|---|
| `TestAsk_FullChain` | 改写结果用于检索（`queries[0]=="rewritten-query"`）、生成用原问题、Sources 与检索结果一一对应、历史落 2 条 | `engine_test.go:187-253` |
| `TestAsk_RewriteReceivesHistory` | 改写 prompt 中携带历史消息内容 | `:256-288` |
| `TestAsk_EmptyRetrieval` | 空检索返回 `noAnswerText`、Sources 为空、**仅 1 次 LLM 调用**（只有改写，无生成） | `:291-322` |
| `TestStreamAsk_EventSequence` | 事件序列恰 4 个：`Sources,Chunk,Chunk,Done`；拼合内容 `"答案内容"`；历史落库 | `:325-383` |
| `TestAsk_CustomSystemPrompt` | `rag.system_prompt_path` 替换生效，messages[0] 为自定义内容 | `:386-419` |
| `TestAsk_RewriteFailureFallback` | 改写失败不报错、降级用原问题检索 | `:422-454` |
| `TestAsk_RewriteDisabled` | 禁用改写后 LLM 仅调用 1 次 | `:457-489` |
| `TestAsk_Concurrent` | 8 goroutine × 10 次 Ask 无 data race | `:492-518` |
| `TestStreamAsk_ContextCancel` | ctx 取消后**不发 Done**、**不落历史** | `:521-572` |
| `TestAsk_MultiQueryEnabled` | 原问题 + 2 变体 = 3 路检索，`queries[0]` 为原问题 | `:575-617` |
| `TestAsk_MultiQueryFallback` | 变体生成与改写均失败 → 降级为 1 路检索且不报错 | `:620-659` |
| `TestAsk_MultiQueryDisabled` | 关闭时单路检索 | `:662-688` |
| `TestAsk_DecompositionParallel` | 3 子问题 → 3 次检索、有引用来源 | `:699-740` |
| `TestAsk_DecompositionSequential` | sequential 模式 2 子问题 → 2 次检索 | `:743-781` |
| `TestAsk_DecompositionSkip` | 判定不分解 → 单路检索 | `:784-811` |
| `TestAsk_StepBack` | 回退 + 原问题 = 2 次检索 | `:814-852` |
| `TestAsk_StrategiesMutualExclusion` | 两者同开时仅 Decomposition 生效，LLM 调用 ≤3 次 | `:855-893` |
| `TestAsk_DecompositionFallback` | 判定返回非 JSON → 降级常规且不报错 | `:896-927` |
| `TestAsk_RoutingDirect` | direct → 单路检索 | `:939-976` |
| `TestAsk_RoutingMultiQuery` | multi_query → SearchMulti 被调 | `:979-1017` |
| `TestAsk_RoutingDecomposition` | decomposition → 走分解 | `:1020-1060` |
| `TestAsk_RoutingFallback` | 路由失败 → `RoutingFallback` | `:1063-1101` |
| `TestAsk_HyDE` / `_StrategyGate` / `_SkipSimple` / `_EmbedFail` | HyDE 双路 + 融合；策略门控；skip_simple；Embedding 失败降级原查询 | `:1104-1253` |
| `TestAsk_ThinkingEnabled` | 步骤序列 `[StepRewrite, StepRetrieval, StepChunks]`，载荷字段逐一断言，chunks 与 sources 同 ID 集合 | `:1256-1328` |
| `TestAsk_ThinkingDisabled` | 策略 thinking=off 时即使 `WithThinking(true)` 也无思考链 | `:1331-1354` |
| `TestStreamAsk_ThinkingEventsBeforeSources` | 所有 thinking 事件严格早于第一个 sources 事件，且含 rewrite/retrieval/chunks | `:1357-1425` |
| `TestAsk_ThinkingRouting` / `_MultiQueryVariants` / `_Decompose` | 各环节载荷完整性 | `:1439-1651` |
| `TestThinking_StreamMatchesAsk` | 流式与非流式思考链一致 | `:1536-1595` |
| `TestAsk_RoutingDirectForcesSingle` | direct 时仅 1 次检索、thinking **无** StepMultiQuery、**有** StepRetrieval | `:1656-1713` |
| `TestAsk_RoutingMultiQueryUnchanged` | multi_query 时 3 路检索、thinking 含 StepMultiQuery | `:1716-1768` |
| `TestDefaultSystemPromptNoAbsoluteReject` | 默认 prompt 不含「未找到相关资料」，含「资料未覆盖」与「不要编造」 | `:1773-1783` |
| `TestAsk_MultiQuerySingleRerank` | 多路场景整体重排**恰 1 次**，thinking 含恰 1 个 StepRerank 且带前后对比 | `:1788-1846` |
| `TestAsk_IncludeContexts` / `_DefaultOff` | `include_contexts=true` 时 Source 带正文；默认不带；**历史 sources 不含正文** | `:1849-1910` |

### 8.2 其余模块

| 文件 | 测试函数 | 断言行为 |
|---|---|---|
| `context_test.go` | `TestBuildContext_MaxChunks` | maxChunks=2 时 items/sources 均 2，Index=1,2（`:23-37`） |
| | `TestBuildContext_TokenBudget` | budget=60 时只保留第一条（注释：首条约 54 token，加第二条超 74）(`:40-55`) |
| | `TestBuildContext_SourceExtraction` / `_MissingMetadata` | ID/Filename/Heading/Score 提取；元数据缺失输出空串不 panic（`:58-86`） |
| | `TestEstimateTokens` | `""→0`、`"中文"→4`、`"hello world"→2`（`:89-101`） |
| | `TestBuildContext_SourceType` / `_Timestamp` / `_PageAnchor` | `source_type`、`start_ms/end_ms`（兼容 float64）、`page_number`/`anchor` 贯通（`:104-164`） |
| `prompt_test.go` | `TestRenderContext_DefaultTemplate` | 含 `[1]（来源：a.md / 标题A）`；无 Heading 时只显示 filename（`:14-34`） |
| | `TestRenderRewrite_DefaultTemplate` | 历史与问题均出现（`:37-53`） |
| | `TestRenderContext_CustomTemplate` | 自定义模板逐字生效（`:56-70`） |
| | `TestLoadPromptTemplates_FromFile` / `_FileMissing` | 文件优先；缺失降级默认（`:73-99`） |
| | `TestRenderRoutingIncludesDataSource` | 渲染含 `data_source`/`vector_store`/`web_search`/allowedText/问题（`:101-114`） |
| `strategy_test.go` | `TestResolveStrategyPriority` | req 覆盖 kb、kb 覆盖 global、global 兜底（`:9-34`） |
| | `TestResolveStrategyThinkingReqOverride` / `_Defaults` / `_DataSources` | thinking 请求级覆盖；全空默认 `multi/rrf/off...`；DataSources 三级覆盖 + 全空 nil（`:36-134`） |
| | `TestValidateStrategyIllegal` / `_Valid` | `single+rrf`、`routing+decomposition`、`routing+step_back`、未知枚举被拒（`:63-92`） |
| `thinking_test.go` | `TestSliceSink_AppendOrder` / `TestRecordStep_NilSink` | 顺序追加；nil sink 零开销（`:10-30`） |
| | `TestRetrievalDataFrom` / `_RerankDataFrom` / `_ChunksDataFrom` / `_TruncateRunes` | 载荷翻译、rune 截断（5 字 + 省略号 = 6 rune）(`:32-112`) |
| | `TestTraceSinkForRequest` | 检索 trace→StepRetrieval；纯 rerank→仅 StepRerank；混合→两个；nil sink→nil 回调（`:115-149`） |
| `tool_test.go` | `TestWebSearchToolExecuteStructured` | 文本含 `[1] 标题`/URL/摘要；items 完整 2 条（`:24-52`） |
| | `TestWebSearchToolMissingQuery` / `_NoResults` | 缺 query 报错；无结果返回「未搜索到」且 items 空（`:55-76`） |
| `history_test.go` | `TestHistoryStore_CapacityEvictsOldest` | 容量 3 写入 4 条 → 余 `"2","3","4"`（`:11-31`） |
| | `TestHistoryStore_GetLimit` / `_SessionIsolation` / `_Concurrent` | limit 生效；session 隔离 + Clear；并发无竞争（`:34-89`） |
| `engine_enhanced_test.go` | `TestAsk_EnhancedToolLoop` | 工具循环 + StepTool 思考步骤（`:30-107`） |
| | `TestAsk_EnhancedNoToolsFallback` / `_UnknownTool` | 无工具回退普通生成；未知工具不中断（`:108-192`） |
| | `TestStreamAsk_EnhancedNoToolsFallsBackToStream` | 无工具时增强回退**普通流式**，`"流式回答内容"`（`:194-243`） |
| | `TestAsk_EmptyRetrievalEnhancedForcesSearch` | 强制搜索注入 `role=tool` 消息、思考链含 StepTool(web_search)（`:246-314`） |
| | `TestAsk_EmptyRetrievalEnhancedNoToolFallsBack` | 无工具 → `noAnswerText`（`:317-337`） |
| `engine_datasource_test.go` | `TestAsk_DataSourceWebSearchFallsBackToVectorStore` / `_LimitedToVectorStore` / `_CustomInjected` | 占位源降级；白名单约束；自定义源可产出 `SourceType=web_search`（`:45-210`） |
| | `TestResolveDataSourceUnknown` | 6 组裁决分支全覆盖（`:213-250`） |
| | `TestEffectivePreservesDataSourcesWithLegacyFallback` | 旧开关兜底不丢 DataSources、routing=off（`:253-265`） |
| `kbfilter_test.go` | `TestKBFilterMultiScope` | 0/1/N 个 kb_id → nil/单值/数组（`:9-33`） |
| `internal/llm/llm_test.go` | `TestGenerate_Success/MessagesPassed/OptionsOverride/AuthHeader` | 请求体、温度覆盖、鉴权头（`:65-320`） |
| | `TestGenerate_RetryOn500` / `_RetryOn429` / `_NoRetryOn400` | 可重试错误重试；400 不重试（`:197-287`） |
| | `TestStreamGenerate_ChunksAssemble` | 分片拼接正确（`:149`） |
| | `TestStreamGenerate_ErrorAfterFirstDelta` | Hijack 手写 SSE：首 delta 后发非法 JSON → 错误透传、**count==1 不重试**（`:323-378`） |
| | `TestGenerateTool_ToolCallsParsed` / `_PlainContent` | tools 字段下发；tool_calls 解析；无调用时返回正文（`:381-432`） |
| | `TestStreamGenerate_ToolCallsAggregated` | `delta.tool_calls` 分片按 index 聚合，Done 携带完整 `{"query":"RAG"}`（`:435-466`） |
| | `TestToOpenAIMessagesToolCallsNested` | 嵌套格式 + `tool_call_id`，**Sources 内部字段不下发**（`:469-504`） |
| `internal/search/search_test.go` | `TestBochaSearch` / `_NoAPIKey` / `_HTTPError` / `TestNewFactory` / `_OptionsOverride` | 结果解析（summary 优先于 snippet）、未配置报错、HTTP 错误、工厂分发、count/freshness 覆盖（`:14-121`） |
| `internal/api/api_test.go` | `TestChatSSE` | body 含 `event:sources`/`event:chunk`/`event:done` 与内容（`:884-907`） |
| | `TestChatSSEError` | 有 `event:error` + 错误文案；**无 done 且无 chunk**（`:910-933`） |
| | `TestChatSSEEmptyStream` | 空流 `sources → done`，**无 chunk**（`:936-956`） |
| | `TestChatSSEThinkingAndOrder`（约 `:1330-1360`） | 有 `event:thinking` 且 thinking **在 sources 之前**（`strings.Index` 比较） |
| | `TestChatEnhancedPassThrough` / `_DefaultOff` / `TestChatEnhancements` | `enhanced` 透传到 AskOptions；默认不启用；能力列表 `available`（`:1362-1406`） |
| | `TestGetHistory` | 按 session_id 返回 2 条（`:993-1009`） |
| | `TestChatSSEUserNoKBSpecExpands` | SSE 路径同样执行 KB 范围展开（`:857-881`） |

**未覆盖（代码中找不到测试）**：同一 session 并发问答、`chat_history` PG 实现的跨租户隔离、SSE 客户端中途断连的 goroutine 收敛、`resolveDataSource` 在 `routing` 失败路径下的组合、历史超长时的 token 压力、Web 搜索真实网络路径（`bocha.go` 用 httptest 打桩）。

---

## 9. 面试官最可能深挖的 14 个点

### Q1. 一次问答最少/最多要调几次 LLM？为什么这么设计？

**依据**：`internal/rag/engine.go:509-636`（Ask 主链）、`internal/rag/routing.go:135-192`（tryMultiQuery）

```go
// engine.go:523-528  routing=auto 无条件多一次判定
if eff.Routing == "auto" {
	route, ok, err := e.routeQuery(ctx, question, allowedText(eff.DataSources))
	strategy := ""
	if err != nil || !ok {
		strategy = e.ragCfgFor(o).RoutingFallback
```
```go
// engine.go:855-863  多查询再叠一次
if eff.Query == "multi" {
	multiStart := time.Now()
	queries, err := e.multiQuery(ctx, history, question)
	if err != nil {
		slog.Warn("多查询生成失败，降级单查询", "err", err)
```

**答**：最少 2 次（改写 + 生成，`Query=single` 且改写开启；若关闭改写则 1 次）。默认配置（`query=multi`, `routing=auto`, `thinking=on`）下：路由 1 + 多查询 1 + 生成 1 = 3 次。极限路径 `routing→decomposition` = 路由 1 + 判定 1 + 列表 1 + 综合 1 = 4 次；叠加增强模式 tool loop 最多再 +4 次（`maxToolRounds=3` + 兜底 1，`engine.go:393`、`:452`）。设计取向是「用辅助 LLM 调用换检索精度」，代价是 TTFT 线性增长。

### Q2. 三级策略合并怎么做？非法组合怎么处理？

**依据**：`internal/rag/strategy.go:37-76`、`:119-127`、`engine.go:252-256`

```go
pick := func(globalV, kbV, reqV, def string) string {
	v := globalV
	if kbV != "" { v = kbV }
	if reqV != "" { v = reqV }
	if v == "" { v = def }
	return v
}
eff.Query = pick(global.Query, kb.Query, req.Query, "multi")
// ...
// routing=auto 已含分流，与 decomposition/step_back 冲突
if s.Routing == "auto" {
	if s.Decomposition != "off" { return fmt.Errorf("非法策略组合：routing=auto 时 decomposition 应为 off（路由已含分流）") }
```

**答**：字段级 pick，请求 > 知识库 > 全局，空串继承下层，全空取硬默认。`ValidateStrategy` 拒绝 `single+rrf`（无多路可融合）与 `routing=auto` + `decomposition/step_back`（路由已含分流）。校验失败时 `effective` 打 warn 并整体降级 `DefaultEffectiveStrategy()`（`multi/rrf/其余 off`），**不阻断请求**。

### Q3. 改写为什么用温度 0.1？改写结果为什么只用于检索、不用于生成？

**依据**：`engine.go:1029-1042`、`engine.go:1016`、`engine_test.go:239-246`

```go
rewritten, err := e.llm.Generate(ctx,
	[]llm.Message{{Role: llm.RoleUser, Content: prompt}},
	llm.WithTemperature(0.1),
)
// ...
rewritten = strings.TrimSpace(rewritten)
if rewritten == "" { return "", fmt.Errorf("改写结果为空") }
return rewritten, nil
```
```go
userContent := contextText + "\n\n用户问题：" + question   // engine.go:1016
```

**答**：改写是归一化任务，低温保证可复现与不发散。改写只喂给 `retriever.Search`（`engine.go:983-988`），生成时用**原始问题**——这样改写器的错误不会污染最终回答的语义（测试 `TestAsk_FullChain:244` 专门断言生成内容不含改写串）。空串视为失败，防「改写变成空查询」。

### Q4. 多查询变体生成失败/返回非 JSON 怎么办？

**依据**：`engine.go:1088-1108`、`engine.go:859-883`

```go
var variants []string
if err := json.Unmarshal([]byte(out), &variants); err != nil {
	return nil, fmt.Errorf("解析多查询变体失败（非 JSON 数组）: %w", err)
}
seen := make(map[string]bool, len(variants)+1)
queries := make([]string, 0, len(variants)+1)
queries = append(queries, question)   // 原问题恒为第一路
seen[question] = true
for _, v := range variants {
	v = strings.TrimSpace(v)
	if v == "" || seen[v] { continue }
	seen[v] = true
	queries = append(queries, v)
}
if len(queries) <= 1 { return nil, fmt.Errorf("多查询变体为空") }
```

**答**：严格 JSON 数组解析（无 markdown 兜底剥离）。失败 → warn → **回落单查询改写**（`engine.go:862`），再失败用原问题，请求不中断（测试 `TestAsk_MultiQueryFallback`）。去重以原问题为种子，原问题恒为第一路，保证「至少一路能召回」。

### Q5. 多路结果怎么融合？怎么保证只 rerank 一次？

**依据**：`internal/retriever/retriever.go:351`、`:365`、`rrf.go:65-106`、`engine_test.go:1821`

```go
fused := FuseMultiQuery(results, 60, topK)          // retriever.go:351
// ...
fused = r.rerankIfEnabled(ctx, req.Query, fused, topK, req.Trace)  // retriever.go:365
```
```go
for _, results := range listOfResults {
	for rank, r := range results {
		scores[r.ID] += 1.0 / float64(k+rank+1)     // rrf.go:77
	}
}
```

**答**：路内检索传 `SkipRerank: true`（`retriever.go:313`、`decompose.go:120`、`routing.go:111`），跨路用 RRF（k=60）按 `1/(k+rank+1)` 累加、ID 去重、降序取 TopK，然后**只在融合后重排一次**。测试 `TestAsk_MultiQuerySingleRerank:1821` 断言 `rerankCalls == 1`，thinking 中恰 1 个 StepRerank。目的是避免「先按各路重排再融合」把路内偏置固化。

### Q6. 路由判定失败会怎样？为什么 routing 与 decomposition 互斥？

**依据**：`engine.go:524-552`、`strategy.go:119-127`

```go
route, ok, err := e.routeQuery(ctx, question, allowedText(eff.DataSources))
strategy := ""
if err != nil || !ok {
	strategy = e.ragCfgFor(o).RoutingFallback
	slog.Warn("路由判定失败，回退默认策略", "fallback", strategy, "err", err)
} else {
	strategy = route.Strategy
	o.RouteComplexity = route.Complexity
	o.AllowedDataSources = eff.DataSources
	dsName, _ := e.resolveDataSource(eff.DataSources, route.DataSource)
	o.DataSource = dsName
```

**答**：失败时用 `routing_fallback`（默认 `multi_query`），**且不记 Routing 思考步骤**（`engine.go:542` 注释「判定失败不 Record」），然后按 fallback 策略分流或落常规。互斥是因为 `routing=auto` 本身已经做了「选策略」这件事，再叠 decomposition/step_back 会出现两层决策互相覆盖；`ValidateStrategy` 直接拒绝该组合并让 `effective` 降级全局默认。

### Q7. HyDE 的降级点有几个？为什么 skip_simple 默认开？

**依据**：`routing.go:51-59`、`:63-131`、`config.go:257-260`

```go
func (e *RAGEngine) shouldHyde(effHyDE string, route routeResult) bool {
	if effHyDE != "on" || e.embedder == nil { return false }
	if e.cfg.HyDESkipSimpleOn() && route.Complexity == "simple" { return false }
	return true
}
```

**答**：4 个降级点全部回落到 `retriever.Search(原查询)`——模板渲染失败、假设文档生成失败、Embedding 失败/空向量、HyDE 向量检索失败；原查询路失败则退化为只用 HyDE 路结果。`skip_simple` 默认 true（`*bool` nil 视为 true），因为 simple 查询（`routing` 已判为「简单事实查询」）用 HyDE 只增加一次 LLM + 一次 Embedding 而没有召回增益；启用 HyDE 还会要求 embedder 非 nil，评测装配路径同样注入 embedder（`app.go:381`）。

### Q8. 上下文 token 预算怎么算？超预算怎么裁？为什么是「整条丢弃」？

**依据**：`context.go:35-81`、`:146-183`、`engine.go:591-609`

```go
if used+estimateTokens(formatContextItem(item)) > maxTokens { break }
items = append(items, item)
used += estimateTokens(formatContextItem(item))
```
```go
if len(sources) == 0 {
	e.appendHistory(sessionID, llm.RoleUser, question, "")
	...
	e.appendHistory(sessionID, llm.RoleAssistant, noAnswerText, "")
	return withThinking(&o, &RAGResult{Answer: noAnswerText}), nil
}
```

**答**：规则估算（汉字 2、英文词 1、标点 1），以默认渲染格式为基准；预算 2048、条数上限 5。裁剪是「按序累加、超预算即 break」。整条丢弃是为了不切断语义单元（半句话进入 LLM 往往会诱发编造）。**风险**：首条超预算会让 `sources` 也为空，触发 `engine.go:591` 的空检索兜底，返回「未找到相关资料。」且**不调 LLM**——把「预算不够」误判成「没有资料」。

### Q9. 引用编号 [1][2] 和 sources 怎么对齐？会不会错位？

**依据**：`context.go:26-31`、`:53`、`:67-77`、`engine.go:1013-1017`

```go
type ContextItem struct {
	Index    int    // 从 1 开始的编号
	Filename string
	Heading  string
	Content  string
}
// ...
item := ContextItem{Index: i + 1, ...}
// sources 同步 append，顺序与 items 严格一致
```

**答**：**纯位置对齐**——`ContextItem.Index = i+1`，`sources` 在同一循环内同步 append，`Source` 结构体**没有 index 字段**，前端必须靠数组下标推断 `sources[k] ↔ [k+1]`。这依赖「items 与 sources 永远同长同序」这一不变量（`buildContext` 唯一产出点保证；`fillSourceContents` 也按同下标填充，`context.go:86-92`）。`ChunksData` 与 sources 同集合（测试 `engine_test.go:1315-1327`）。**风险**：分解/Step-Back 路径若 rerank 降级产生重复 ID，同一 chunk 会占两个编号；且服务端不校验模型输出的 `[n]` 是否越界。

### Q10. 为什么 sources 先发、正文后发？首字节延迟由什么决定？

**依据**：`engine.go:768` vs `:779`、`engine_test.go:368`

```go
sendEvent(ctx, out, StreamEvent{Type: EventSources, Sources: sources})   // :768
...
if len(sources) == 0 {  // :771
	...
}
ch, err := e.llm.StreamGenerate(ctx, messages)  // :779
```
```go
if len(types) != 4 || types[0] != EventSources || types[3] != EventDone {  // engine_test.go:368
	t.Fatalf("事件序列错误: %v", types)
}
```

**答**：引用在 `prepare` 结束时已确定，先发可让前端立刻渲染来源卡片，把「等待」变成「已有信息 + 流式正文」。TTFT 完全由 `prepare` 决定：历史查询 + 改写/多查询 LLM + 检索（含 Rerank）+ 组装。默认 `routing=auto` 还要再加一次路由 LLM。开启 thinking 时，每个环节完成即发一个 `thinking` 事件（`engine.go:658-660`），本质上充当了应用层进度心跳。

### Q11. 客户端断连怎么感知？goroutine 会不会泄漏？

**依据**：`engine.go:1112-1119`、`:795-803`、`stream.go:191-205`、`engine_test.go:521-572`

```go
func sendEvent(ctx context.Context, out chan<- StreamEvent, ev StreamEvent) bool {
	select {
	case out <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}
```
```go
// ctx 取消：丢弃截断结果，不落历史、不发 Done
if ctx.Err() != nil { return }   // engine.go:801-803
```

**答**：ctx 来自 `c.Request.Context()`，net/http 在断连时取消；生产/消费两端都用 `select` + `ctx.Done()` 退出；取消后**不落历史、不发 Done**（有测试）。通道都用 `defer close`，`StreamGenerate` 的 `sendErr` 在已取消时静默丢弃（`stream.go:201-205`），因此当前实现无实际泄漏。**脆弱点**：`engine.go:795-797` 转发失败直接 return、不排空 `ch`，正确性依赖「此时 ctx 必然已取消」；且增强模式 tool loop 同步阻塞在 `GenerateTool` 时无法被中途打断，最长要多跑 4 轮 LLM + 多次 web 搜索才在下一个 `sendEvent` 处发现取消。

### Q12. thinking 事件怎么保证一定在 sources 之前？

**依据**：`engine.go:648-661`、`:726-768`、`thinking.go:221-248`

```go
var sink TraceSink
if o.Sink != nil {
	sink = o.Sink
} else if o.Thinking && e.effective(o).Thinking == "on" {
	sink = TraceSinkFunc(func(step ThinkingStep) {
		sendEvent(ctx, out, StreamEvent{Type: EventThinking, Thinking: &step})
	})
}
o.Sink = sink
```

**答**：**同一通道 + 同一 goroutine**。所有 `recordStep` 都在 `prepare` 及其子流程的**同步完成点**调用（`thinking.go:53-58`，主 goroutine 顺序 Record，注释 N7 无竞争），而 `sources` 事件在 `prepare` 返回之后才发（`engine.go:768`），所以「thinking 全在 sources 前」是结构性保证而非时序巧合（测试用 `strings.Index` 对比断言，`api_test.go:1352-1358`）。sink 为 nil 时 `recordStep` 直接返回，零开销（`thinking.go:54-57`）。开关是**双门控 AND**：`eff.Thinking=="on"` **且** `o.Thinking`（`engine.go:150`）。

### Q13. 增强模式为什么要「系统强制搜索」？tool loop 怎么防死循环？

**依据**：`engine.go:309-351`、`:393-461`、`tool.go:9`

```go
// 保证增强模式必定联网（解决小模型 function calling 不稳定问题）
func (e *RAGEngine) forceWebSearchFallback(ctx context.Context, messages []llm.Message, query string, sink TraceSink) ([]llm.Message, bool, error) {
	tool, ok := e.tools.Get(datasource.SourceWebSearch)
	if !ok { return messages, false, nil }
	q := query
	...
	messages = append(messages,
		llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
			{ID: "auto_web_1", Name: datasource.SourceWebSearch, Arguments: string(argsJSON)}}},
		llm.Message{Role: llm.RoleTool, ToolCallID: "auto_web_1", Content: result},
	)
```
```go
const maxToolRounds = 3   // tool.go:9
```

**答**：注释写明动机——小模型 function calling 不稳定，靠模型自觉调用可能一次都不调，所以系统先替它执行一次并把结果作为 `role=tool` 消息注入（伪造一条 assistant tool_calls + 一条 tool 结果，ID 固定 `auto_web_1`）。防死循环靠 `maxToolRounds=3` 的硬上限，超限后再取一次正文，正文为空则 `noAnswerText`。**副作用**：`Ask` 在非空检索路径也**无条件**执行这次搜索（`engine.go:616`），即使知识库已足够，等于每次都付一次外部搜索的延迟与查询外泄。

### Q14. 检索为空时为什么不调 LLM？增强模式有什么不同？

**依据**：`engine.go:591-609`、`engine_test.go:291-322`、`engine_enhanced_test.go:246-337`

```go
if len(sources) == 0 {
	e.appendHistory(sessionID, llm.RoleUser, question, "")
	if o.Enhanced {
		injectedMsgs, injected, err := e.forceWebSearchFallback(ctx, messages, question, sink)
		if err != nil { return nil, err }
		if injected {
			answer, err := e.enhancedAnswer(ctx, injectedMsgs, o, sink)
			if err != nil { return nil, err }
			e.appendHistory(sessionID, llm.RoleAssistant, answer, marshalSources(sources))
			return withThinking(&o, &RAGResult{Answer: answer}), nil
		}
	}
	e.appendHistory(sessionID, llm.RoleAssistant, noAnswerText, "")
	return withThinking(&o, &RAGResult{Answer: noAnswerText}), nil
}
```

**答**：普通模式直接返回常量 `noAnswerText`，因为「没有资料」这件事代码已经确定，再让 LLM 基于空上下文生成只会增加幻觉面与成本（测试断言此时 LLM 只被调用 1 次即改写调用）。增强模式则先尝试强制联网补资料，成功才走 tool loop 生成；若连工具都没有（未配 `web_search.api_key`），仍回落 `noAnswerText`。**注意**：这个分支的 `Sources` 恒为 nil——联网答案不带引用。

---

## 10. 可被挑刺的缺陷（诚实清单）

### A. 上下文与预算

1. **Token 估算非 tokenizer**（`context.go:146-183`）：汉字固定 2、英文按空格分词、标点 1。对中英混排、代码块、表格、日韩文、emoji 误差可达 30%+。预算 2048 只是「估算 token」，与模型真实 BPE 计数无对应关系。
2. **预算只覆盖检索上下文**：system prompt、历史消息、用户问题、输出全部不计入（`engine.go:1013-1017`）。没有「模型上下文窗口」配置项，也没有「上下文 + max_tokens ≤ 窗口」的校验——**代码中未找到**。
3. **无每条 chunk 上限**：超预算即整条 break（`context.go:60-62`），单条超大 chunk 会独占预算并**把后续所有 chunk 挤掉**；若首条就超预算，`sources` 为空 → 误判为空检索 → 不调 LLM 直接答「未找到相关资料。」（`engine.go:591`）。这是最容易被面试官抓住的正确性缺陷。
4. **自定义 context 模板破坏估算基准**：估算用 `formatContextItem` 的固定格式，渲染用 `e.templates.context`（`context.go:95` vs `engine.go:1000`），配置自定义模板后预算与实际注入量脱钩。
5. **无相邻 chunk 合并、无 overlap 去重**：代码中未找到。
6. **分解/Step-Back 路径无 ID 去重**（`decompose.go:214-217`、`:305`）：rerank 关闭或失败时重复 chunk 直接进上下文并各占一个引用编号。

### B. 检索质量

7. **无重排缓存、无 Embedding 缓存、无检索结果缓存**：同一问题重复问必走全链路（改写 LLM + 检索 + rerank）。
8. **路由判定无缓存/无规则前置**：`routing=auto` 时每个请求固定 +1 次 LLM（`engine.go:524`），短问题（如「你好」）也照付。
9. **HyDE 两路检索串行**（`routing.go:97` → `:107`），未并发，白白多一个检索 RT。
10. **`FuseMultiQuery` 排序不确定**：`scores` 是 map，遍历顺序随机，`sort.Slice` 非稳定排序（`rrf.go:82-100`）——同分结果的相对顺序在多次请求间会抖动，导致引用顺序不稳定。
11. **`FuseMultiQuery` 信息取用策略粗糙**：从「第一个命中的路」取文档信息（`rrf.go:85-89`），而非取 score 最高的那路。

### C. 并发与限流

12. **无请求级并发限流**：`RAGEngine` 无信号量、无 per-session/per-user 配额（`engine.go:168-178`）。唯一的限流是 LLM QPS（`llm.go:128`）与 bocha QPS（`bocha.go:51`）。一次 `routing+decomposition+enhanced` 请求会连续占用 4+ 次 LLM 配额与 1 次外网搜索。
13. **同一 session 并发无串行化**：`Get → Generate → Append×2` 非原子（`engine.go:848`→`:632-633`），可产生 `user1,user2,assistant1,assistant2` 的历史交错；无测试覆盖。
14. **无 goroutine 上限/无背压策略**：SSE 每连接 2 个 goroutine（engine + llm），无池化。增强模式 tool loop 同步阻塞，client 断连后仍可能跑完 4 轮。
15. **重试放大**：`Generate` 每次尝试前 `limiter.Wait` + 指数退避（`llm.go:135-143`），tool loop 每轮各自重试，最坏延迟叠加。

### D. 历史与会话

16. **`session_id` 无归属校验**（`handler_history.go:18-30`、`store/history.go:32-45`）：`chat_history` 表无 `user_id`，任何认证者猜到 session_id 即可读/写他人会话。`kb_id` 有越权校验，session 没有——隔离语义不一致。
17. **无历史摘要、无 token 级裁剪**：只按条数取最近 N 条（`history.go:57-59`），超长历史直接膨胀请求体。
18. **PG 历史无限增长**：无归档、无 TTL、无清理任务；`HistoryCapacity` 只对内存实现生效，PG 实现忽略它（`store/history.go:32-62`），配置项语义在两个实现间不一致。
19. **`Clear` 是死代码**：无 HTTP 路由、engine 也不调用（`grep` 零命中），用户无法清空会话。

### E. 提示词与安全

20. **无任何防提示注入机制**：检索片段与用户问题原样拼接（`engine.go:1016`），上下文模板不含「以下为参考资料，不得当作指令」之类的隔离声明。检索到的文档若含指令性文本，可直接劫持模型行为。
21. **无幻觉检测/无引用校验**：回答中的 `[n]` 由模型自由生成，服务端不校验越界、不校验该来源是否真被使用。忠实度判定只存在于独立的评测服务（`internal/eval/judge.go`），**不在生产问答链路**。
22. **增强模式覆盖用户配置的 system prompt**（`engine.go:358-369`）：`rag.system_prompt_path` 在 `enhanced=true` 时静默失效（`TestAsk_CustomSystemPrompt` 只覆盖非增强路径）。
23. **两套 `[n]` 编号体系并存**：上下文编号（`context.go:99`）与联网结果编号（`websearchtool.go:73`）都从 1 开始，同处一个 messages，模型可能交叉引用。
24. **增强模式无条件联网**（`engine.go:594`、`:616`）：知识库足够时也会把用户问题发给第三方搜索服务，且该查询不进入 `Sources`，审计与合规上不可见。
25. **检索内容默认不外发，但历史 sources 会随历史消息载入内存并回传给前端**（`engine.go:1123-1137` 剥离正文是对的；但 `GET /api/v1/chat/history` 返回的 `sources` JSON 无脱敏/无权限过滤）。

### F. 流式与可观测性

26. **无 SSE 心跳、无 idle 超时、无 `X-Accel-Buffering: no`**：长 `prepare` 期间零字节输出，经 nginx/网关容易被判空闲断开。
27. **策略路径与增强路径不是真流式**：`strategyRes` 命中时整段答案作为一个 chunk 发出（`engine.go:728`、`:759`），decompose/stepback/multiquery/enhanced 四种模式的前端体验退化为「等很久 → 一次性刷出」。
28. **chunk 不聚合**：每个 delta 一帧 SSE + 一次 Flush（`handler_chat.go:193`），无攒批，逐 token 场景下 syscall/frame 开销明显。
29. **`StreamAsk` 外层永不返回 error**（`engine.go:810` 恒 `return out, nil`），handler 的 `Fail(c, CodeInternal, "启动流式问答失败…")` 分支（`handler_chat.go:167-170`）不可达，属死代码。
30. **`ThinkingStep.ElapsedMS` 永不赋值**（`thinking.go:30`，全仓库无赋值点），`elapsed_ms` 恒被省略——前端拿不到各环节耗时。
31. **无 metrics/tracing**：全链路只有 `slog` 文本日志（如 `engine.go:630`、`:904`、`:993`），无 Prometheus 指标、无 OpenTelemetry span、无请求级 trace_id 贯穿。

### G. 配置一致性

32. **配置快照只覆盖一半路径**：`prepare` 用快照 `ragCfg`（`engine.go:843-846`），但以下位置全部直读 `e.cfg`（引擎构建时配置），热重载期间同一请求可能混用新旧参数：
   - `multiQuery` 读 `e.cfg.MultiQueryCount`（`engine.go:1071`）
   - `listSubQuestions` 读 `e.cfg.DecompositionMaxSub`（`decompose.go:52`）
   - `searchSubQuery` 读 `e.cfg.TopK`、`e.cfg.MultiQueryOn()`（`decompose.go:118`、`:122`）
   - `tryDecompose`/`tryStepBack`/`tryMultiQuery` 读 `e.cfg.DecompositionMode`、`MultiQueryConcurrency`、`MaxChunks`、`MaxContextTokens`、`TopK`（`decompose.go:161`、`:180`、`:223`、`:229`、`:310`、`:315`；`routing.go:153`、`:165`）
   - `shouldHyde` 读 `e.cfg.HyDESkipSimpleOn()`（`routing.go:55`），`hydeSearch` 读 `e.cfg.TopK`（`routing.go:71`、`:79`、`:93`、`:97`、`:109`）
33. **`effective` 的旧开关兜底分支在生产配置下不可达**（`engine.go:214-244` vs `config.go:502-519`）：`applyDefaults` 已把 `strategy.*` 填满，因此这段「向后兼容映射」只在测试直接构造 `RAGConfig` 时生效，形成「被测路径 ≠ 生产路径」的落差。
34. **`enhancedAnswer` 回退分支用错变量**（`engine.go:306`）：`runToolLoop` 返回 `msgs` 后，回退却调用 `e.llm.Generate(ctx, messages)`（循环前的切片）。当前依赖「system prompt 是原地替换」才偶然等价；若 `runToolLoop` 走了 prepend 分支（`engine.go:368`，无 system 消息时），增强 system prompt 会被丢弃。经 handler 路径不可达（恒有 system），属潜在缺陷。
35. **`web_search` 数据源与 `web_search` 工具有同名但语义不同**：数据源占位不可用（`websource.go:21`），工具走 `search.Provider`。路由把 `web_search` 当数据源选择，选到后必然降级回 vector_store（`engine.go:478-497`）；而 `routing.go` 的 prompt 里却把 `web_search` 作为可选数据源告知 LLM（`prompt.go:77`）——配置 `data_sources: [web_search]` 时 LLM 会被引导选一个永远不可用的源。

---

## 附：关键行号速查

| 主题 | 位置 |
|---|---|
| Ask 主链 | `internal/rag/engine.go:509-636` |
| StreamAsk 主链 | `internal/rag/engine.go:639-811` |
| prepare（历史→改写→检索→组装） | `internal/rag/engine.go:821-1020` |
| effective 三级合并 | `internal/rag/engine.go:207-258` |
| rewriteQuery / multiQuery | `internal/rag/engine.go:1023-1042` / `1070-1109` |
| runToolLoop / forceWebSearchFallback | `internal/rag/engine.go:357-462` / `314-351` |
| resolveDataSource / allowedText | `internal/rag/engine.go:470-498` / `501-506` |
| kbFilter / marshalSources | `internal/rag/engine.go:1048-1066` / `1123-1137` |
| sendEvent | `internal/rag/engine.go:1112-1119` |
| buildContext / estimateTokens | `internal/rag/context.go:35-81` / `146-183` |
| 全部 prompt 默认模板 | `internal/rag/prompt.go:14-86` |
| tryDecompose / tryStepBack | `internal/rag/decompose.go:131-258` / `261-344` |
| routeQuery / hydeSearch / tryMultiQuery | `internal/rag/routing.go:24-46` / `63-131` / `135-192` |
| ResolveStrategy / ValidateStrategy | `internal/rag/strategy.go:37-76` / `79-129` |
| 思考链路埋点与载荷 | `internal/rag/thinking.go:27-249` |
| HistoryStore 接口/内存实现 | `internal/rag/history.go:10-73` |
| PG 历史实现 / 表结构 | `internal/store/history.go:23-67` / `internal/store/schema.go:50-57` |
| Chat / ChatStream handler | `internal/api/handler_chat.go:75-195` |
| LLM 客户端（生成/工具/重试） | `internal/llm/llm.go:133-297` |
| SSE 解析与聚合 | `internal/llm/stream.go:20-206` |
| 博查搜索 | `internal/search/bocha.go:29-153` |
| 配置结构体与默认值 | `internal/config/config.go:172-260` / `463-519` |
