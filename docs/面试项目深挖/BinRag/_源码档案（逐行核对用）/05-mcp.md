# BinRag MCP Server 深度技术档案（面试准备版）

> 所有结论均标注 `文件:行号`。代码库内找不到依据的地方明确写「代码中未找到」。
> 分析范围：`internal/mcp/*`、`internal/api/handler_mcp_my.go`、`internal/api/handler_key.go`、`internal/store/{apikey,audit,kb,schema,store}.go`、`internal/app/app.go`、`internal/config/*`、`docs/26-mcp-server/*`、`docs/27-web-mcp-manage/*`、`go.mod`。
> 未修改任何文件。

---

## 0. 结论速览

| 维度 | 结论 |
|---|---|
| SDK | `github.com/mark3labs/mcp-go v0.57.0`（**社区库，非官方 `modelcontextprotocol/go-sdk`**） |
| 传输 | 仅 Streamable HTTP（单端点，POST/GET/DELETE），**未使用 SSE 旧传输**、无 stdio |
| 协议版本 | 库支持 2025-11-25 / 2025-06-18 / 2025-03-26；本项目测试按 2025-03-26 握手 |
| 认证 | 自写 `net/http` 层，Bearer → SHA-256 → `api_keys` 查库 → 失败 HTTP 401（非 JSON-RPC） |
| 授权 | 自写 gateway 层解析 JSON-RPC body → 失败返回 **HTTP 200 + JSON-RPC error -32001** |
| 权限模型 | Tool 白名单 + KB 三态范围（`""` / `all` / `allowlist`）+ owner 交集（用户级凭据） |
| 审计 | 异步：buffered channel（硬编码 1024）+ 后台 worker，参数按 rune 截断 2000 |
| 能力范围 | 只注册 `tools`（6 个只读 Tool）；**无 resources / prompts / OAuth** |
| 最严重缺陷 | owner 范围解析失败时 **fail-open**（`server.go:66-70` 只 warn 不拒绝） |

---

## 1. 协议接入方式

### 1.1 库与版本

`go.mod:13`：
```
github.com/mark3labs/mcp-go v0.57.0
```
`go.sum:121-122` 有对应 `h1:` 与 `/go.mod` 哈希。全仓未找到官方 `modelcontextprotocol/go-sdk` 依赖（`go.mod` 中无该条目）。

`internal/mcp/server.go:15-16` 的导入别名：
```go
mcpgo "github.com/mark3labs/mcp-go/mcp"
mcpserver "github.com/mark3labs/mcp-go/server"
```

### 1.2 Streamable HTTP 的具体实现

`internal/mcp/server.go:31-45`：
```go
func NewHandler(deps Dependencies) http.Handler {
	t := &tools{ st: deps.Store, engine: deps.Engine, rt: deps.RT, cfg: deps.CfgMgr, audit: deps.Audit }
	ms := mcpserver.NewMCPServer("BinRag MCP", "1.0.0")   // server.go:39
	t.register(ms)                                          // server.go:40
	return &gateway{
		st:   deps.Store,
		next: mcpserver.NewStreamableHTTPServer(ms),        // server.go:43
	}
}
```
- 用的是 `mcpserver.NewStreamableHTTPServer`，**未传任何 `StreamableHTTPOption`** → 端点路径用库默认 `/mcp`（库内部 `endpointPath: "/mcp"`），session 管理器用库默认 `StatelessGeneratingSessionIdManager`（只校验格式、不校验存在性）。
- 项目里 **没有** `WithEndpointPath` / `WithHTTPContextFunc` / `WithStateful` / `WithStreamableHTTPCORS` 调用；keyCtx 是靠**自己动手改 `r.Context()`** 注入的（`server.go:84`）。

### 1.3 是否支持 SSE 旧传输

- 库本身**提供** SSE 旧传输实现：`mcp-go@v0.57.0/server/sse.go`（存在 `NewSSEServer` 等）。
- 项目**未使用**：全仓 grep 无 `NewSSEServer`；`server.go` 只构造了 streamable HTTP。
- 但 Streamable HTTP 端点本身支持 `GET` 建立 SSE 流：库 `streamable_http.go:426-431` 分派 `POST → handlePost`、`GET → handleGet`（SSE 通知流）、`DELETE → handleDelete`，其它方法 `http.NotFound`。
- 项目挂载用 `router.Any(...)`（`app.go:205`），因此 GET/DELETE 也走同一 handler，并**同样经过认证**（`gateway.ServeHTTP` 对任何方法都先 `authenticate`）。
- 非 POST 请求**跳过授权检查**：`server.go:127-129` 中 `if r.Method != http.MethodPost { return nil }`。
- stdio：代码中未找到。

### 1.4 协议版本日期

库侧（`mcp-go@v0.57.0/mcp/types.go:163-170`）：
```go
const LATEST_PROTOCOL_VERSION = "2025-11-25"
var ValidProtocolVersions = []string{
	LATEST_PROTOCOL_VERSION, "2025-06-18", "2025-03-26",
}
```
协商逻辑（库 `server/server.go:1196-1210`）：客户端未给版本 → 按 `"2025-03-26"`；给了且在 `ValidProtocolVersions` 内 → 原样回；否则回 `LATEST_PROTOCOL_VERSION`。

项目侧测试固定用 2025-03-26 握手（`internal/mcp/server_test.go:260`：`"protocolVersion": "2025-03-26"`）。项目代码中**没有**对协议版本做任何显式约束或校验。

### 1.5 端点路径与挂载方式

默认路径（`internal/config/config.go:541-543`）：
```go
if c.Server.MCP.Path == "" {
	c.Server.MCP.Path = "/mcp"
}
```
路径合法性约束（`internal/config/manager.go:134-137`）：必须以 `/` 开头，否则配置更新报错。

挂载（`internal/app/app.go:181-207`）：
```go
var auditSink *mcp.AuditSink
if cfg.Server.MCP.Enabled {                                     // app.go:183 启动期一次性判定
	auditSink = mcp.NewAuditSink(st, 1024, cfg.Server.MCP.AuditParamLimit)  // app.go:184
	mcpHandler := mcp.NewHandler(mcp.Dependencies{ ... })       // app.go:185-204
	router.Any(cfg.Server.MCP.Path, gin.WrapH(mcpHandler))      // app.go:205
	slog.Info("MCP Server 已挂载", "path", cfg.Server.MCP.Path)
}
```
- 挂载点在 `api.NewRouter` **之后**，而全局中间件在 `NewRouter` 内部用 `r.Use(Logger(), CORS(), RateLimit(deps.Config.RateLimitQPS))` 注册（`internal/api/router.go:67`），所以 `/mcp` 会经过 Logger / CORS / RateLimit。
- gin → net/http 的桥是 `gin.WrapH`。
- 库的 `StreamableHTTPServer.ServeHTTP` **不校验 `r.URL.Path`**（`streamable_http.go:385+`），所以自定义 path（如 `/custom-mcp`）能正常工作。
- `Enabled=false` 时不注册路由 → `GET /mcp` 返回 gin 默认 404（`docs/26-mcp-server/checklist.md:7` 的验收项，测试环境依赖真实启动验证）。

### 1.6 initialize / tools/list / tools/call 的注册与处理位置

| 方法 | 由谁处理 | 位置 |
|---|---|---|
| `initialize` | 库 | `mcp-go/server/server.go:1084 handleInitialize`；能力协商在 1089-1180，版本协商 `protocolVersion` 1196 |
| `tools/list` | 库 | `mcp-go/server/server.go:1862 handleListTools` → `filteredTools` 1767 → `listByPagination` 1334 |
| `tools/call` | 库分发 + 项目 handler | 库 `server.go:1893 handleToolCall`；项目侧 handler 注册在 `internal/mcp/tools.go:43-69`，实现 `tools.go:224/250/270/307/355/389` |
| 授权拦截（自有） | 项目 gateway | `internal/mcp/server.go:126-174 authorize` |

项目注册 6 个 Tool 的完整代码（`internal/mcp/tools.go:43-69`）：
```go
func (t *tools) register(s *mcpserver.MCPServer) {
	s.AddTool(mcpgo.NewTool(ToolListKBs, mcpgo.WithDescription("列出当前凭据可访问的知识库（全部或白名单）")), t.handleListKBs)
	s.AddTool(mcpgo.NewTool(ToolGetKB, ...), t.handleGetKB)
	s.AddTool(mcpgo.NewTool(ToolRetrieve, ...), t.handleRetrieve)
	s.AddTool(mcpgo.NewTool(ToolAsk, ...), t.handleAsk)
	s.AddTool(mcpgo.NewTool(ToolListDocs, ...), t.handleListDocs)
	s.AddTool(mcpgo.NewTool(ToolGetTask, ...), t.handleGetTask)
}
```
Tool 名常量与全量清单（`tools.go:21-31`）：
```go
const (
	ToolListKBs  = "list_knowledge_bases"
	ToolGetKB    = "get_knowledge_base"
	ToolRetrieve = "retrieve"
	ToolAsk      = "ask"
	ToolListDocs = "list_documents"
	ToolGetTask  = "get_task"
)
var AllTools = []string{ToolListKBs, ToolGetKB, ToolRetrieve, ToolAsk, ToolListDocs, ToolGetTask}
```

能力声明：`AddTool` 内部会隐式注册 tools 能力（库 `server.go:910-915 implicitlyRegisterToolCapabilities`，`listChanged: true`）。resources / prompts 能力对象为 `nil` → 库在 `initialize` 响应中不声明（库 `server.go:1091-1118`）。所以 `initialize` 只声明 `tools`。

---

## 2. 六个 Tool 逐个说明

### 2.0 输入 schema 的生成规则（重要前提）

项目全部使用 `mcpgo.NewTool(name, opts...)` + `WithString/WithNumber` + `Required()` + `Description()`（`tools.go:44-68`）。

- `NewTool` 生成的骨架（库 `mcp/tools.go:846-853`）：
```go
tool := Tool{
	Name: name,
	InputSchema: ToolInputSchema{ Type: "object", Properties: make(map[string]any), Required: nil },
	Annotations: ToolAnnotation{
		Title: "", ReadOnlyHint: ToBoolPtr(false),
		DestructiveHint: ToBoolPtr(true), IdempotentHint: ToBoolPtr(false), OpenWorldHint: ToBoolPtr(true),
	},
}
```
- `WithString` → `{"type":"string"}`，`WithNumber` → `{"type":"number"}`（库 `mcp/tools.go:1236-1270`）。
- `Required()` 把 `"required":true` **从属性节点删除**并追加到 `InputSchema.Required` 数组（库 `mcp/tools.go:1250-1255`）。
- **本项目未使用 `DefaultString` / `DefaultNumber` / `Enum` / 任何数值范围约束** → schema 中**没有任何 `default` 关键字**；「缺省值」全部在 Go 代码里处理。
- `Annotations` 字段的 JSON tag 是 `json:"annotations"`（非 omitempty），因此 `tools/list` 会**原样输出库默认注解**（`readOnlyHint:false, destructiveHint:true, idempotentHint:false, openWorldHint:true`）。项目未调用 `WithReadOnlyHintAnnotation` 等（`internal/mcp` 全包 grep 无 `Annotation`）。

### 2.1 `list_knowledge_bases`

| 项 | 内容 |
|---|---|
| 描述原文 | `列出当前凭据可访问的知识库（全部或白名单）`（`tools.go:44`） |
| 输入 schema | `{"type":"object","properties":{}}`，无 `required`（`Required` 为 nil → 序列化时省略） |
| 输出结构 | `[]kbView` 数组（`tools.go:240-245`），`kbView` 字段见 `tools.go:73-80` |
| 内部调用 | `kc.Scope.All` → `store.ListAllKBs(ctx)`（`tools.go:233`，实现 `internal/store/kb.go:35-52`）；否则 `store.ListKBsByIDs(ctx, kc.Scope.IDs)`（`tools.go:235`，实现 `kb.go:75-95`） |
| 分页 | **无分页**。`ListAllKBs` 是 `SELECT ... ORDER BY created_at DESC`（`kb.go:36-37`），无 LIMIT |
| 返回条数上限 | 无上限。系统级 Key `scope=all` 时返回**全库** |
| 截断 | 无输出截断（截断只作用于审计参数，`audit.go:52-69`） |
| 暴露 thinking | 不涉及 |

### 2.2 `get_knowledge_base`

| 项 | 内容 |
|---|---|
| 描述原文 | `按 ID 返回单个知识库详情；无权限按不存在处理`（`tools.go:46`） |
| 输入 schema | `kb_id`：`{"type":"string","description":"知识库 ID"}`，**required**（`tools.go:47`） |
| 输出结构 | 单个 `kbView` 对象（`tools.go:264-265`） |
| 内部调用 | 网关层先 `kc.Scope.CanAccess(kb_id)` 拦截（`server.go:159-161`）；handler 内 `t.st.GetKB(ctx, kbID)`（`tools.go:254`，实现 `internal/store/kb.go:98-101`），`pgx.ErrNoRows` → `NewKBForbidden()`（`tools.go:255-260`），再兜底 `CanAccess`（`tools.go:261-263`） |
| 分页 | 单对象，无 |
| 上限 | 无 |
| 截断 | 无 |
| thinking | 不涉及 |

### 2.3 `retrieve`

| 项 | 内容 |
|---|---|
| 描述原文 | `纯检索：按查询文本召回知识库 chunk 及来源信息`（`tools.go:50`） |
| 输入 schema | `query`：string，**required**，`"查询文本"`；`kb_id`：string，可选，`"知识库 ID，缺省用凭据可访问范围"`；`top_k`：**number**，可选，`"召回数量，缺省用全局配置"`（`tools.go:51-53`） |
| 输出结构 | `[]chunkView`（`tools.go:292-301`）；`chunkView` = `{id, content, score, metadata:{filename, kb_id}}`（`tools.go:103-108`），metadata 经 `safeMetadata` 白名单裁剪（`tools.go:420-429`） |
| 内部调用 | `kc.Scope.Resolve(kb_id)`（`tools.go:275` → `permission.go:42-56`）；`rt := t.rt()` 从 provider 取检索器（`tools.go:279`）；`rt.Search(ctx, retriever.RetrieveRequest{Query, TopK, Filter})`（`tools.go:283-287`）；`kbFilter` 构造过滤条件（`tools.go:210-219`：0 个 → nil 不过滤；1 个 → `{"kb_id": id}`；多个 → `{"kb_id": []string}`） |
| 分页 | **无分页**（只有 top_k 截断） |
| 返回条数上限 | **代码中未找到上限校验**。`top_k` 直接透传，只在下游做 `topK <= 0 → 全局 config.Retriever.TopK`（`internal/retriever/retriever.go:54-56` 与 `347-349`）。客户端可传 `top_k=1000000` |
| 截断 | 无输出截断；`Content` 为完整 chunk 正文 |
| 暴露 thinking | 不涉及（`RetrieveRequest.Trace` 未设置 → 零开销，`internal/retriever/types.go:8` 注释确认 `nil=关闭`） |

### 2.4 `ask`

| 项 | 内容 |
|---|---|
| 描述原文 | `RAG 问答：基于知识库回答并返回引用来源（不暴露内部推理）`（`tools.go:56`） |
| 输入 schema | `question`：string，**required**，`"问题"`；`kb_id`：string 可选；`session_id`：string 可选，`"会话 ID（复用现有会话历史），缺省为新会话"`（`tools.go:57-59`） |
| 输出结构 | 显式构造的 map：`{"answer": ..., "sources": ...}`（`tools.go:350`）。`answer` 为 `string`，`sources` 为 `[]rag.Source`（`internal/rag/context.go:12-23`：id/filename/heading/score/source_type/start_ms/end_ms/page_number/anchor/content） |
| 内部调用 | `question == ""` → `NewShowError("question 不能为空")`（`tools.go:312-314`）；`kc.Scope.Resolve`（`tools.go:315`）；`t.engine()` 取引擎（`tools.go:319-322`）；`session_id` 缺省 `uuid.New().String()`（`tools.go:323-326`）；**`sessionID = kc.KeyID + ":" + sessionID`**（`tools.go:329`）；配置快照 `t.cfg.Get()`（`tools.go:330-333`）；`rag.WithConfigSnapshot` + `rag.WithThinking(false)`（`tools.go:334-337`）；KB 注入 `rag.WithKBID/WithKBIDs`（`tools.go:338-344`）；`eng.Ask(ctx, sessionID, question, askOpts...)`（`tools.go:345`） |
| 分页 | 无 |
| 上限 | 无（答案长度由 LLM 侧 `MaxTokens` 决定） |
| 截断 | 无输出截断 |
| **暴露 thinking** | **明确不暴露**。双重保证：① `rag.WithThinking(false)`（`tools.go:336`）；② 返回值只取 `result.Answer` 与 `result.Sources`，**丢弃 `RAGResult.Thinking`**（`tools.go:350` 对比 `internal/rag/engine.go:24-28` 的 `Thinking []ThinkingStep json:"thinking,omitempty"`）。测试断言响应不含 `thinking` 字段（`integration_test.go:203-206`） |

补充：`ask` 不支持流式（`StreamAsk` 未使用，spec 明确列为「不做的事」，`docs/26-mcp-server/spec.md:49`）。

### 2.5 `list_documents`

| 项 | 内容 |
|---|---|
| 描述原文 | `列出知识库（或当前凭据可访问范围）内的文档`（`tools.go:62`） |
| 输入 schema | `kb_id`：string，**可选**，`"知识库 ID，缺省列出可访问全部知识库的文档"`（`tools.go:63`） |
| 输出结构 | `[]docView`（`tools.go:363/383`）；`docView` = `{id, kb_id, filename, format, size, status, created_at}`（`tools.go:82-90`） |
| 内部调用 | 指定 kb_id：`CanAccess` 校验（`tools.go:360`）+ `ListDocuments(ctx, kbID)`（`tools.go:363` → `internal/store/document.go:30-45`）；未指定：先按 scope 取 KB 列表（`ListAllKBs`/`ListKBsByIDs`，`tools.go:367-371`），再**逐个 KB 循环** `ListDocuments`（`tools.go:376-382`）→ **N+1 查询** |
| 分页 | **无分页** |
| 返回条数上限 | **无上限**（`SELECT ... FROM documents WHERE kb_id = $1 ORDER BY created_at DESC`，`document.go:31-32`，无 LIMIT） |
| 截断 | 无输出截断 |
| thinking | 不涉及 |
| 注意 | 未指定 kb_id 且可访问 KB 为空时，`var docs []docView`（`tools.go:375`）保持 nil → `json.Marshal(nil)` 得 `null`，与其它 handler 返回 `[]` 不一致 |

### 2.6 `get_task`

| 项 | 内容 |
|---|---|
| 描述原文 | `按任务 ID 查询入库任务状态与错误信息`（`tools.go:66`） |
| 输入 schema | `task_id`：string，**required**，`"入库任务 ID"`（`tools.go:67`） |
| 输出结构 | 单个 `taskView`（`tools.go:399-401`）；`taskView` = `{id, kb_id, document_id, status, retry_count, error_message, created_at, updated_at}`（`tools.go:92-101`） |
| 内部调用 | 网关层 `g.st.GetTask(ctx, task_id)` + `kc.Scope.CanAccess(task.KBID)`（`server.go:167-171`）；handler 内**再查一次** `t.st.GetTask`（`tools.go:392` → `internal/store/task.go:23-32`）+ `CanAccess`（`tools.go:396`）→ 同一次调用 2 次 DB 查询 |
| 分页 | 单对象，无 |
| 上限 | 无 |
| 截断 | 无（`error_message` 完整返回给调用方） |
| thinking | 不涉及 |

### 2.7 输出封装统一逻辑

`tools.go:114-150` 的 `run()`：所有 handler 都经它包装。
```go
func (t *tools) run(ctx context.Context, toolName string, args map[string]any, fn func() (any, error)) (*mcpgo.CallToolResult, error) {
	start := time.Now()
	argsJSON, _ := json.Marshal(args)
	result, err := fn()
	if kc := KeyCtxFrom(ctx); kc != nil && t.audit != nil { ... t.audit.Submit(...) }   // tools.go:119-132
	if err != nil { ... return &mcpgo.CallToolResult{Content: []mcpgo.Content{mcpgo.NewTextContent(msg)}, IsError: true}, nil }
	return okResult(result)
}
```
成功结果（`tools.go:153-165`）：`Content` = 序列化后的 JSON 文本，`StructuredContent` = 原始 Go 值。**注意 `StructuredContent` 可以是数组**（`[]kbView`/`[]chunkView`/`[]docView`），MCP 规范里 `structuredContent` 期望是对象。

---

## 3. 认证（Authentication）

### 3.1 HTTP 层如何拦截

`internal/mcp/server.go:57-62`：
```go
func (g *gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	kc, ok := authenticate(w, r, g.st)
	if !ok {
		return // HTTP 401 已写
	}
```
认证是 handler 链的**第一环**，在读取 body、授权检查、转发给 mcp-go 之前完成。任何 HTTP 方法（POST/GET/DELETE）都先认证。

### 3.2 Bearer 解析

`internal/mcp/auth.go:48-56`：
```go
header := r.Header.Get("Authorization")
var token string
if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
	token = header[7:]
}
if token == "" {
	writeUnauthorized(w)
	return nil, false
}
```
- `strings.EqualFold` → scheme 大小写不敏感（`bearer xxx` 也能过）。
- **只认 `Bearer ` 前缀**；`Token xyz` 会被判为无 token → 401（`auth_test.go:53` 有该用例）。
- 注意：`header[:7]` 与 `len(header) > 7` 组合意味着 `"Bearer "` 恰好 7 字符时（即空 token）会走 `token == ""` 分支 → 401。

### 3.3 SHA-256 校验流程

`internal/mcp/auth.go:58-71`：
```go
sum := sha256.Sum256([]byte(token))
hash := hex.EncodeToString(sum[:])
key, err := st.GetAPIKeyByHash(r.Context(), hash)
if err != nil {
	slog.Warn("MCP API Key 校验出错", "err", err)
	writeUnauthorized(w)
	return nil, false
}
if key == nil || !key.Enabled {
	writeUnauthorized(w)
	return nil, false
}
```
- 明文 Key 只在内存里存在，落库为 `key_hash`（`internal/store/apikey.go:14` 列清单含 `key_hash`；`internal/store/store.go:80` 注释 `KeyHash string // SHA-256 hex`）。
- 查库实现：`internal/store/apikey.go:70-80`，`WHERE key_hash = $1`，`pgx.ErrNoRows → (nil, nil)`。
- 认证路径**无外部网络调用**（spec N5，`docs/26-mcp-server/spec.md:41`）。

**与 REST 认证的差异（重要）**：REST 中间件在认证成功后会刷新 `last_used_at`（`internal/api/middleware.go:78`：`_ = s.TouchAPIKey(ctx, key.ID)`），而 MCP 认证**没有这一步** → MCP 调用不会更新 Key 的 `last_used_at`。

### 3.4 401 的具体返回体

`internal/mcp/auth.go:82-87`：
```go
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="mcp"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"认证失败"}`))
}
```
**是纯 HTTP 错误体，不是 JSON-RPC**。缺少/无效/停用 Key 三者**返回完全相同的响应**，不区分（`auth.go:67-71` 注释「不泄露区分」）。测试断言状态码 401 且带 `WWW-Authenticate`（`auth_test.go:70-77`、`integration_test.go:36-42`）。

注意：`WWW-Authenticate` 只给了 `Bearer realm="mcp"`，**没有 `resource_metadata` 指向 RFC 9728 保护资源元数据**，也没有实现 OAuth 授权服务器（spec 未要求）。

### 3.5 与 REST 认证是否复用代码

**不复用**，是有意的独立实现（plan D3，`docs/26-mcp-server/plan.md:250`：「现有 `api.Auth` 中间件含 JWT 分支且绑定 `gin.Context`，MCP 用独立实现更干净」）。

对比：

| | MCP（`internal/mcp/auth.go`） | REST（`internal/api/middleware.go:28-85`） |
|---|---|---|
| 接口签名 | `func authenticate(w http.ResponseWriter, r *http.Request, st keyLookup) (*keyCtx, bool)` | `func Auth(s store.Store, authMgr *auth.Manager, enabled bool, bootstrapKey string) gin.HandlerFunc` |
| 依赖抽象 | 最小接口 `keyLookup{ GetAPIKeyByHash }`（`auth.go:40-42`） | `store.Store` + `auth.Manager` |
| JWT 分支 | **无**（MCP 只接受 API Key） | 有（`jwtShapeRe` 三段式判别，`middleware.go:21/47-58`） |
| bootstrap 识别 | **不识别**（`auth.go:18` 注释、plan D6） | 识别（`middleware.go:79-82`，写 `is_bootstrap`） |
| `enabled=false` 开发放行 | **无**（MCP 恒校验） | 有（`middleware.go:30-33`） |
| 失败响应 | 裸 `http.ResponseWriter` 写 401 | `Fail(c, CodeUnauthorized, ...)`（业务响应包装） |
| `last_used_at` | 不更新 | 更新（`middleware.go:78`） |
| SHA-256 + 查库逻辑 | `auth.go:58-61` | `middleware.go:61-65`（**同一套算法，代码复制**） |

**结论**：算法与数据模型一致（同一张 `api_keys` 表、同一 SHA-256），但代码是两份，未抽公共函数。

---

## 4. 授权（Authorization）

### 4.1 权限模型字段结构

数据模型（`internal/store/store.go:77-97`）：
```go
type APIKey struct {
	ID         string
	Name       string
	KeyHash    string // SHA-256 hex
	Enabled    bool
	LastUsedAt *time.Time
	CreatedAt  time.Time
	OwnerID string      // "" = 系统级 Key；非空 = 用户 MCP 凭据（users.id，每用户至多一个）
	MCPTools   []string // 允许调用的 MCP Tool 白名单；空 = 无任何 MCP 权限
	MCPKBScope string   // ""（无 MCP 知识库权限）| "all"（全部）| "allowlist"（仅 MCPKBIDs）
	MCPKBIDs   []string // MCPKBScope=="allowlist" 时的知识库白名单
}
type APIKeyPermissions struct {
	MCPTools   []string `json:"mcp_tools"`
	MCPKBScope string   `json:"mcp_kb_scope"`
	MCPKBIDs   []string `json:"mcp_kb_ids"`
}
```
**注意**：spec 里的「任务范围」不是一个独立字段。task 的权限由任务所属 KB 派生（`task.KBID`），见 `tools.go:396` 与 `server.go:169`。

运行时身份（`internal/mcp/auth.go:19-24`）：
```go
type keyCtx struct {
	KeyID   string
	Tools   []string // MCP Tool 白名单（空 = 无任何 MCP Tool 权限）
	Scope   KBPermission
	OwnerID string // "" = 系统级 Key；非空 = 用户 MCP 凭据
}
```
KB 范围三态（`internal/mcp/permission.go:3-22`）：
```go
type KBPermission struct {
	All bool
	IDs []string
}
func ParseScope(scope string, ids []string) KBPermission {
	switch scope {
	case "all":       return KBPermission{All: true}
	case "allowlist": return KBPermission{IDs: ids}
	default:          return KBPermission{}   // 未知 scope 视为无权限
	}
}
```
DDL（`internal/store/schema.go:85-96`）：
```sql
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS mcp_tools TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS mcp_kb_scope TEXT NOT NULL DEFAULT '';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS mcp_kb_ids TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS owner_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_owner ON api_keys(owner_id) WHERE owner_id IS NOT NULL;
```
默认空 → 历史 Key 迁移后**无任何 MCP 权限**（spec F6，`docs/26-mcp-server/spec.md:31`）。

### 4.2 越权返回 -32001 的实现在哪

**在网关层手写，不在 tool handler 里**。`internal/mcp/server.go:126-174`：
```go
func (g *gateway) authorize(r *http.Request, kc *keyCtx, body []byte) *mcpgo.JSONRPCError {
	if r.Method != http.MethodPost { return nil }                      // 127-129
	... json.Unmarshal(body, &msg) ...
	if msg.Method != string(mcpgo.MethodToolsCall) { return nil }        // 139-141
	... 解析 params.Name / params.Arguments ...
	idAny := rawID(msg.ID)

	// Tool 白名单（spec F4）
	if !ToolAllowed(kc.Tools, params.Name) {                             // 153-155
		return permissionErr(idAny, NewToolForbidden())
	}
	switch params.Name {                                                // 158-172
	case ToolGetKB:
		if !kc.Scope.CanAccess(argString(params.Arguments, "kb_id")) { return permissionErr(idAny, NewKBForbidden()) }
	case ToolRetrieve, ToolAsk, ToolListDocs:
		if _, err := kc.Scope.Resolve(argString(params.Arguments, "kb_id")); err != nil { return permissionErr(idAny, err) }
	case ToolGetTask:
		task, err := g.st.GetTask(r.Context(), argString(params.Arguments, "task_id"))
		if err != nil || task == nil || !kc.Scope.CanAccess(task.KBID) { return permissionErr(idAny, NewTaskForbidden()) }
	}
	return nil
}
```
响应构造（`internal/mcp/errors.go:62-72` + `server.go:201-204`）：
```go
const ErrCodePermissionDenied = -32001                                  // errors.go:20

func (e *PermissionError) ToJSONRPCError(id any) mcpgo.JSONRPCError {
	return mcpgo.JSONRPCError{
		JSONRPC: mcpgo.JSONRPC_VERSION,                    // "2.0"
		ID:      mcpgo.NewRequestId(id),
		Error:   mcpgo.NewJSONRPCErrorDetails(ErrCodePermissionDenied, e.message, nil),
	}
}

func writeJSONRPCError(w http.ResponseWriter, e mcpgo.JSONRPCError) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(e)      // 不调用 WriteHeader → HTTP 200
}
```
实际响应体形如：`{"jsonrpc":"2.0","id":1,"error":{"code":-32001,"message":"知识库不存在或无权限"}}`，HTTP 状态 200。测试断言 HTTP 200 + code -32001（`integration_test.go:60-66`）。

**为什么必须自己写这一层**（`internal/mcp/errors.go:8-10` 注释 + `server.go:49-51`）：
> 探针结论（task T1）：mcp-go v0.57.0 的 tools/call handler 返回 error 固定映射 -32603，无法经 handler/middleware 返回自定义错误码。

代码层面可验证这一结论——库 `server/server.go:2008-2013`：
```go
result, err := finalHandler(ctx, request)
if err != nil {
	return nil, &requestError{
		id:   id,
		code: mcp.INTERNAL_ERROR,       // = -32603
		err:  err,
	}
}
```

### 4.3 为什么越权与不存在返回同一消息

消息常量（`internal/mcp/errors.go:22-27`）：
```go
const (
	msgToolForbidden = "无权限执行该操作"
	msgKBForbidden   = "知识库不存在或无权限"
	msgTaskForbidden = "任务不存在"
)
```
设计依据是 spec N2「不泄露存在性」（`docs/26-mcp-server/spec.md:38`）。实现上：
- `get_knowledge_base`：`CanAccess` 失败与 `pgx.ErrNoRows` **都返回 `NewKBForbidden()`**（`server.go:160-161` 与 `tools.go:256-257`）。
- `get_task`：查不到（`err != nil`）与查得到但 `CanAccess(task.KBID)` 为假 **都返回 `NewTaskForbidden()`**（`server.go:169-170`、`tools.go:393-398`）。
- 集成测试专门断言「越权任务」与「不存在的任务」响应**逐字节一致**（`integration_test.go:133-151`）。

### 4.4 双层开关的判定顺序

两层开关语义不同（spec F10，`docs/27-web-mcp-manage/spec.md:43`）：

| 层 | 存储 | 判定位置 | 效果 |
|---|---|---|---|
| 全局 `server.mcp.enabled` | 配置文件 | 启动期 `app.go:183` | `false` → `/mcp` 路由**根本不注册** → 404 |
| 用户级凭据 `api_keys.enabled` | DB | 每次请求 `auth.go:67-71` | `false` → HTTP 401 |

判定顺序：**全局先于用户级**。全局 false 时路由不存在，请求到不了认证层（404）；全局 true 时逐请求检查用户凭据 `Enabled`。前端通过 `GET /api/v1/mcp/my/status` 的 `global_enabled` 字段（`handler_mcp_my.go:64-67`，读取 `h.globalMCPEnabled()` → `cfg.Server.MCP.Enabled`，`handler_mcp_my.go:290-296`）提示「服务未开启」。

**关键限制**：全局开关是**启动期一次性判定**，热改配置不会重挂载路由。`handler_config.go:258-259` 注释明确：「MCP Server（重启生效：enabled/path 影响路由挂载…）」。

### 4.5 系统级 Key 与用户级 Key 的权限差异

差异由 `keyCtx.OwnerID` 驱动。`internal/mcp/server.go:66-107`：
```go
if kc.OwnerID != "" {
	if err := resolveOwnerScope(r.Context(), g.st, kc); err != nil {
		slog.Warn("MCP owner 知识库范围解析失败", "key", kc.KeyID, "err", err)
	}
}
...
func resolveOwnerScope(ctx context.Context, st store.Store, kc *keyCtx) error {
	ownerKBs, err := st.ListKBsByOwner(ctx, kc.OwnerID)
	if err != nil { return err }
	ownerIDs := make([]string, 0, len(ownerKBs))
	for _, kb := range ownerKBs { ownerIDs = append(ownerIDs, kb.ID) }
	switch {
	case kc.Scope.All:
		kc.Scope = KBPermission{IDs: ownerIDs}          // 用户 all = 自己的知识库
	case len(kc.Scope.IDs) > 0:
		kc.Scope = KBPermission{IDs: intersect(kc.Scope.IDs, ownerIDs)}   // 白名单 ∩ 自己的
	default:
		kc.Scope = KBPermission{}
	}
	return nil
}
```

| 维度 | 系统级 Key（`OwnerID == ""`） | 用户级 Key（`OwnerID != ""`） |
|---|---|---|
| 生成方式 | `POST /api/v1/api-keys`（bootstrap，`handler_key.go:65-89`） | `POST /api/v1/mcp/my/key`（登录会话，`handler_mcp_my.go:97-139`），每用户至多 1 个（部分唯一索引 `schema.go:95`） |
| `scope=all` 语义 | **全部知识库，含系统级与他人库**（`ListAllKBs`，`kb.go:34-37`） | **仅自己的知识库**（owner 交集，`server.go:99-100`） |
| `scope=allowlist` | 白名单即访问凭证，**不过滤 owner**（`kb.go:74` 注释：「不过滤 owner——显式授权即访问凭证」） | 白名单 ∩ 自己的 KB（`server.go:101-102`） |
| 权限授予者 | `PUT /api/v1/api-keys/:id/permissions`，需 bootstrap（`handler_key.go:157-165`，`handler_key.go:162` 检查 `is_bootstrap`） | 用户自助 `PUT /api/v1/mcp/my/key/permissions`（`handler_mcp_my.go:237-287`），仅会话身份（`requireUser`，`handler_mcp_my.go:39-46`），且 `kb_ids` 必须 ∈ 自己的 KB（`handler_mcp_my.go:263-280`），否则 400 |
| bootstrap Key 本身 | **不作 MCP 凭证、不绕过 MCP 权限**（plan D6，`docs/26-mcp-server/plan.md:253`）；MCP 认证层不识别 bootstrap，一律按权限字段授权（`auth.go:18` 注释）→ bootstrap Key 若无 MCP 权限列，调任何 Tool → -32001 | — |

### 4.6 MCP 权限校验与 REST 权限校验是否同一套代码

**不是同一套**。

| | MCP | REST |
|---|---|---|
| 授权入口 | `gateway.authorize`（`server.go:126-174`）+ `mcp/permission.go` | `internal/api` 层各 handler 内的 owner 过滤 / `auth.IdentityOf` 判定 |
| KB owner 隔离 | `resolveOwnerScope` + `ListKBsByOwner`（`server.go:89-107`） | 例如 `handler_kb.go` 走 `auth.IdentityOf(c)` 决定 `OwnerID` 后由 store 过滤（`kb.go:55-72`） |
| Tool 级授权（MCP 特有） | `ToolAllowed`（`permission.go:60-67`） | REST 无对应概念 |
| 越权表达 | JSON-RPC -32001 | HTTP 403（`Fail(c, CodeForbidden, ...)`） |
| 共享部分 | **共享 `store` 查询方法**（`ListKBsByOwner` / `GetKB` / `GetTask` / `ListDocuments`）与同一 `api_keys` 表 | 同上 |

---

## 5. 审计

### 5.1 记录字段

`internal/store/store.go:99-110`：
```go
type AuditLog struct {
	ID           int64
	APIKeyID     string // 仅 Key ID 引用，绝不存 Secret
	ToolName     string
	Params       string // 截断后参数 JSON（默认 ≤2000 字符）
	ParamsLen    int    // 截断前原始参数长度
	Status       string // success / error
	ErrorMessage string
	DurationMS   int64
	CreatedAt    time.Time
}
```
填充处（`internal/mcp/tools.go:119-132`）：
```go
if kc := KeyCtxFrom(ctx); kc != nil && t.audit != nil {
	status, errMsg := "success", ""
	if err != nil {
		status, errMsg = "error", err.Error()   // 审计保留详情（内部可查）
	}
	t.audit.Submit(store.AuditLog{
		APIKeyID:     kc.KeyID,
		ToolName:     toolName,
		Params:       string(argsJSON),
		Status:       status,
		ErrorMessage: errMsg,
		DurationMS:   time.Since(start).Milliseconds(),
	})
}
```
对应 checklist 要求：调用者（`api_key_id`）、tool（`tool_name`）、参数摘要（`params` + `params_len`）、耗时（`duration_ms`）、结果状态（`status`）、错误（`error_message`）。

> 精确度说明：`DurationMS` 只覆盖 `fn()` 业务执行耗时（`start` 在 `fn()` 之前，`time.Since` 在 `fn()` 之后，`tools.go:115/130`），**不含**网关授权、body 解析、审计投递本身的开销。
> 无 `created_at` 由代码传入——store 内 `time.Now()`（`internal/store/audit.go:15`）。

### 5.2 同步还是异步

**异步，有 channel + worker**（plan D8）。`internal/mcp/audit.go:23-48`：
```go
type AuditSink struct {
	st         auditStore
	ch         chan store.AuditLog
	paramLimit int
	done       chan struct{}
	closed     atomic.Bool // Shutdown 后禁止 Submit（防御 send-on-closed）
	closeOnce  sync.Once
}

func NewAuditSink(st auditStore, bufSize, paramLimit int) *AuditSink {
	if bufSize <= 0 { bufSize = 1024 }
	if paramLimit <= 0 { paramLimit = 2000 }
	s := &AuditSink{ st: st, ch: make(chan store.AuditLog, bufSize), paramLimit: paramLimit, done: make(chan struct{}) }
	go s.run()
	return s
}
```
投递非阻塞（`audit.go:52-69`）：
```go
func (s *AuditSink) Submit(log store.AuditLog) {
	if s.closed.Load() { return }
	log.ParamsLen = len(log.Params)                 // 截断前原始长度（字节）
	if s.paramLimit > 0 {
		if runes := []rune(log.Params); len(runes) > s.paramLimit {
			log.Params = string(runes[:s.paramLimit])
		}
	}
	select {
	case s.ch <- log:
	default:
		slog.Warn("MCP 审计队列已满，丢弃该审计事件", "api_key_id", log.APIKeyID, "tool", log.ToolName)
	}
}
```
worker（`audit.go:86-94`）+ shutdown flush（`audit.go:73-83`）：
```go
func (s *AuditSink) run() {
	defer close(s.done)
	for log := range s.ch {
		if err := s.st.AppendAuditLog(context.Background(), log); err != nil {
			slog.Warn("MCP 审计写入失败", "api_key_id", log.APIKeyID, "tool", log.ToolName, "err", err)
		}
	}
}
```
生命周期由 App 管理（`internal/app/app.go:184` 创建、`app.go:281-291` `Close()` 里 `Shutdown(ctx)` 带 5s 超时 flush）。

### 5.3 参数截断长度默认值

| 参数 | 默认 | 来源 |
|---|---|---|
| `paramLimit`（提交前截断） | **2000**（按 **rune** 截断） | `audit.go:37-39` 兜底；生产值来自配置 `server.mcp.audit_param_limit`（`config.go:544-546`），传入点 `app.go:184` |
| `bufSize`（channel 容量） | **1024**，**硬编码不可配** | `app.go:184`（`mcp.NewAuditSink(st, 1024, ...)`），`audit.go:34-36` 兜底 |
| `ParamsLen` 单位 | **字节**（`len(log.Params)`） | `audit.go:57`；测试断言 `ParamsLen == len(long)`（`audit_test.go:93`) |

配置项定义（`internal/config/config.go:281-288`）：
```go
type MCPConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Path            string `yaml:"path"`
	AuditParamLimit int    `yaml:"audit_param_limit"`
}
```

### 5.4 审计失败是否影响主流程

**不影响**，三重保证：
1. `Submit` 用 `select { case ch <- log: default: warn }`，队列满只丢弃 + 日志（`audit.go:63-68`）；测试 `TestAuditSinkQueueFullNonBlocking` 用 buffer=2 投 1000 条，3 秒内必须返回，否则 `t.Fatal("Submit 在队列满时阻塞了主流程")`（`audit_test.go:53-73`）。
2. worker 写入失败只 `slog.Warn`（`audit.go:90-92`）。
3. `Shutdown` 超时也只 warn（`audit.go:81`：「MCP 审计 Shutdown 超时，可能丢失部分审计」）。
4. `Submit` 在 `Shutdown` 后被调用直接返回，不 panic（`audit.go:53-55` 的 `closed` 原子标记 + `audit_test.go:99-104` 断言）。

### 5.5 表结构

`internal/store/schema.go:103-118`：
```sql
-- mcpAuditLogsDDL MCP 调用审计表（仅记录 api_key_id 引用与截断参数，绝不存 Secret，spec F7）
CREATE TABLE IF NOT EXISTS mcp_audit_logs (
    id            BIGSERIAL PRIMARY KEY,
    api_key_id    TEXT NOT NULL,
    tool_name     TEXT NOT NULL,
    params        TEXT NOT NULL DEFAULT '',
    params_len    INT NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    duration_ms   BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_mcp_audit_logs_created_at ON mcp_audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_mcp_audit_logs_api_key ON mcp_audit_logs(api_key_id, created_at);
```
写入（`internal/store/audit.go:10-17`）：
```go
_, err := s.pool.Exec(ctx,
	`INSERT INTO mcp_audit_logs (api_key_id, tool_name, params, params_len, status, error_message, duration_ms, created_at)
	 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	log.APIKeyID, log.ToolName, log.Params, log.ParamsLen,
	log.Status, log.ErrorMessage, log.DurationMS, time.Now(),
)
```
设计上**没有 Secret 列**（`schema.go:103` 注释、`docs/26-mcp-server/checklist.md:35`）。无外键约束到 `api_keys`。

### 5.6 审计的覆盖边界（重要）

只有**通过授权、进入 tool handler** 的调用才被审计（`tools.go:119` 在 `run()` 里）。测试明确断言：
- 越权调用（网关层 -32001）**不产生**审计；
- 认证失败（401）**不产生**审计。
`integration_test.go:234-255`：
```go
// 越权调用（网关层拦截，未进入 handler）不产生审计；认证失败（401）不产生审计
...
if got := st.auditCount(); got != 0 {
	t.Errorf("越权/未认证调用不应产生审计，实际 %d 条", got)
}
```

---

## 6. 错误处理

### 6.1 JSON-RPC 错误码表

| 错误码 | 名称/含义 | 谁产生 | 本项目位置 |
|---|---|---|---|
| **-32001** | 自定义：`ErrCodePermissionDenied`（授权失败） | 项目 gateway | `internal/mcp/errors.go:20`；构造 `errors.go:62-72`；写出 `server.go:201-204` |
| -32700 | `PARSE_ERROR`（JSON-RPC 标准，库常量 `mcp/types.go:449`） | 库 | 项目未显式产生（`server.go:136-138` 解析失败时**放行**交给库） |
| -32600 | `INVALID_REQUEST`（库常量 `mcp/types.go:452`） | 库 | 未显式产生 |
| -32601 | `METHOD_NOT_FOUND`（库常量 `mcp/types.go:455`） | 库 | 未显式产生 |
| -32602 | `INVALID_PARAMS`（库常量 `mcp/types.go:458`） | 库 | Tool 不存在时库返回（库 `server/server.go:1943-1948`）；**参数 schema 校验默认关闭**，`required` 缺失不会得到 -32602 |
| -32603 | `INTERNAL_ERROR`（库常量 `mcp/types.go:461`） | 库 | 仅当 handler **返回 Go error** 时才由库映射；本项目 handler 永远返回 `(result, nil)`，所以实际上不会触发（`tools.go:144-147`、`168-172` 都返回 `nil` error） |

> 注意 `docs/26-mcp-server/plan.md:239-240` 的表格写「参数不合法 → -32602（库内置）」「内部错误 → -32603（库内置）」，但**代码未启用 `WithInputSchemaValidation`**（`server.go:39` 未传任何 ServerOption，库选项在 `mcp-go/server/server.go:434`），且 handler 从不返回 error → 这两行在实现上不会以文档描述的方式生效。

### 6.2 错误码到 message 的映射

统一消息常量（`internal/mcp/errors.go:22-27`）：
```go
const (
	msgToolForbidden = "无权限执行该操作"
	msgKBForbidden   = "知识库不存在或无权限"
	msgTaskForbidden = "任务不存在"
)
```
三类工厂（`errors.go:41-48`）：
```go
func NewToolForbidden() *PermissionError { return &PermissionError{message: msgToolForbidden} }
func NewKBForbidden() *PermissionError   { return &PermissionError{message: msgKBForbidden} }
func NewTaskForbidden() *PermissionError { return &PermissionError{message: msgTaskForbidden} }
```
Tool handler 内的用户可见 message 映射（`tools.go:133-148`）：
```go
if err != nil {
	// 展示消息：业务/校验错误原样；内部错误（引擎/存储）统一通用消息，防细节泄漏（安全审查 MEDIUM）
	msg := "工具执行失败，请稍后重试"
	switch e := err.(type) {
	case *PermissionError:
		msg = e.Message()
	case *ShowError:
		msg = e.message
	default:
		slog.Error("MCP tool 执行失败", "tool", toolName, "err", err)
	}
	return &mcpgo.CallToolResult{
		Content: []mcpgo.Content{mcpgo.NewTextContent(msg)},
		IsError: true,
	}, nil
}
```
`ShowError` 定义（`errors.go:52-59`），唯一使用点是 `ask` 的 `question 不能为空`（`tools.go:313`）。

**注意**：handler 返回的是 `IsError: true` 的 `CallToolResult`（工具执行错误，MCP 规范推荐做法），**不是** JSON-RPC error。只有 gateway 的 -32001 是 JSON-RPC error。

### 6.3 panic 恢复

- **项目层：代码中未找到**。`internal/mcp` 全包 grep 无 `recover(`。
- 库层：`mcpserver.NewMCPServer("BinRag MCP", "1.0.0")`（`server.go:39`）**未传 `WithRecovery()`**（库选项在 `mcp-go@v0.57.0/server/server.go:401-415`）。库默认没有 panic 兜底中间件。
- 库在别处有 recover：notification forwarder（`streamable_http.go:708`）、SSE 通知 writer（`streamable_http.go:962`）、异步 task tool（`server/server.go:2124-2129`）——都**不覆盖同步 tool handler**。
- 结论：tool handler 内若发生 panic（例如 `kc` 为 nil 解引用、下游库 panic），panic 会冒泡到 net/http 的 per-connection recover，客户端得到连接中断而非结构化错误。**当前 6 个 handler 中 `KeyCtxFrom(ctx)` 返回值在 `handleRetrieve`/`handleAsk`/`handleListDocs`/`handleGetTask` 里被直接解引用**（`tools.go:272-275`、`309-315`、`357-360`、`391-392`），只有 `handleListKBs`（`tools.go:226-229`）和 `handleGetKB`（`tools.go:261`）做了 nil 检查。授权后的路径上 `kc` 一定非 nil（`server.go:84` 注入），所以实践中不可达，但这是脆弱的耦合。

---

## 7. 测试覆盖证据

### 7.1 `internal/mcp` 测试函数清单

| 测试函数 | 文件:行 | 断言行为 |
|---|---|---|
| `TestSmokeInitializeToolsListAndCall` | `server_test.go:273` | 握手冒烟：`initialize` 拿到 `Mcp-Session-Id` → `tools/list` 断言 **恰好 6 个** Tool（`:289`）→ `tools/call list_knowledge_bases` 断言 `structuredContent` 含 1 个 KB（`:308-311`） |
| `TestAuthenticate` | `auth_test.go:32` | 表驱动 5 例：缺失 / 非 Bearer 前缀 / 无效 / 停用 → `ok=false` + status 401 + `WWW-Authenticate` 非空；有效 → `ok=true` 且 `KeyID=="key-1"` 且 `Scope.All==true` |
| `TestAuthenticateNoScope` | `auth_test.go:92` | 历史 Key（无 MCP 权限字段）→ `kc.Scope` 为空、`len(kc.Tools)==0` |
| `TestParseScope` | `permission_test.go:6` | 四态：`""`/`all`/`allowlist`/未知 scope（未知 → 无权限） |
| `TestKBPermissionCanAccess` | `permission_test.go:30` | `All` 可访问任意；白名单内外；空权限全否 |
| `TestKBPermissionResolve` | `permission_test.go:51` | 四分支：指定+范围内 → 单值；指定+越权 → 消息 == `msgKBForbidden`；未指定+All → `nil`；未指定+allowlist → 白名单；未指定+无权限 → 报错 |
| `TestToolAllowed` | `permission_test.go:78` | nil 与空切片都拒绝任何 Tool；白名单内通过、外拒绝 |
| `TestAuditSinkSubmitAndFlush` | `audit_test.go:33` | 投 50 条 → `Shutdown` 后 fake store 恰好 50 条（flush 生效） |
| `TestAuditSinkQueueFullNonBlocking` | `audit_test.go:53` | buffer=2、投 1000 条，3s 内必须完成（不阻塞主流程） |
| `TestAuditSinkTruncation` | `audit_test.go:76` | limit=10、投 100 rune → 落库 `len([]rune(Params))==10`，`ParamsLen==len(long)`（字节） |
| `TestAuditSinkSubmitAfterShutdown` | `audit_test.go:99` | Shutdown 后 Submit 不 panic |
| `TestAuthFailures` | `integration_test.go:16` | 端到端 401 三分支（缺失/无效/停用），均断言 401 + `WWW-Authenticate` |
| `TestToolPermissionDenied` | `integration_test.go:49` | 历史 Key 调任何 tool → **HTTP 200** 且 body 的 `error.code == -32001`；白名单外 tool（只有 retrieve 却调 ask）→ -32001 |
| `TestKBPermissionDenied` | `integration_test.go:80` | allowlist 外 `kb_id` → -32001 且 `message == msgKBForbidden`（`:100-102`）；`get_knowledge_base` 越权 → -32001；`scope=''` 且未指定 kb_id → -32001 |
| `TestTaskPermissionDenied` | `integration_test.go:123` | 越权任务 → -32001 + `msgTaskForbidden`；**真实不存在的任务 → 完全相同的 code 与 message**（`:143-151`，验证不泄露存在性） |
| `TestToolsSuccessPaths` | `integration_test.go:157` | 6 个 Tool 逐个调用：HTTP 200、无 `error`、`isError != true`；再复查 `ask` → 有 `structuredContent`、**无 `thinking` 字段**、有 `answer` 与 `sources`；`Shutdown` 后断言**恰好 7 条**审计（`:218`），且每条 `api_key_id` 为调用方、`status=="success"` |
| `TestAuditOnlyForAuthorizedCalls` | `integration_test.go:234` | 越权调用与认证失败后 → 审计条数 **0** |
| `TestAuditSuccessAndErrorRecords` | `integration_test.go:259` | fake engine 返回错误 → `isError==true`；审计 1 条，`status=="error"`、`ToolName==ask`、`ErrorMessage != ""`、`APIKeyID=="ok"`、`ParamsLen != 0` |
| `TestMCPOwnerIsolation` | `owner_test.go:12` | 用户凭据 `scope=all` → `list_knowledge_bases` **只返回 1 个自己的 KB**；`get_knowledge_base` 他人库与**系统级库**均 -32001；系统级 Key `scope=all` → 返回 3 个 KB |
| `TestMCPOwnerAllowlistFiltered` | `owner_test.go:68` | 用户 allowlist 含他人 id → 被 owner 过滤为交集（只 1 个），调用被过滤的 KB → -32001 |

### 7.2 其他包的 MCP 相关测试

| 测试函数 | 文件:行 | 断言 |
|---|---|---|
| `TestMyMCPLifecycle` | `internal/api/handler_mcp_my_test.go:11` | 8 步闭环：初始 status `key=null` → 生成返回明文 → 重复生成 **409** → status 有凭据 → 配置权限 200 → 越权 kb_id **400** → 停用后 `enabled=false` → 吊销后 `key=null` → **API Key 访问 `/mcp/my/status` → 403** |
| `TestAPIKeyPermissions` | `internal/api/key_permissions_test.go:10` | 历史 Key 列表 `mcp_tools` 为空、`mcp_kb_scope` 为空；bootstrap 可更新；更新后可见（+ 非 bootstrap/会话 JWT → 403） |
| `TestConfigMCPUpdate` | `internal/api/config_mcp_test.go:9` | `PUT /config` 改 `server.mcp` |
| `TestUpdateAPIKeyPermissions` / `TestCreateAPIKeyNoMCPPermissions` | `internal/store/apikey_test.go:14/48` | pgxmock 断言 SQL 与参数；创建时权限列默认空 |
| `TestAppendAuditLog` | `internal/store/audit_test.go:48` | pgxmock 精确匹配 `INSERT INTO mcp_audit_logs (...)` 与 8 个参数（`(:54-55)`），确认无 Secret 列 |
| `TestMCPConfigDefaults` / `TestMCPConfigExplicit` | `internal/config/config_test.go:78/94` | 默认 `Enabled=false`、`Path=/mcp`、`AuditParamLimit=2000`；显式配置保留 |
| `TestValidateMCPPath` | `internal/config/manager_test.go:126` | 空=默认；非 `/` 开头报错 |

### 7.3 集成测试如何模拟客户端握手（重点）

**不用真实 mcp-go 客户端**，而是直接对 `http.Handler` 发原始 HTTP 请求，手工拼 JSON-RPC body。三个基础设施都在 `server_test.go`：

**（1）`fakeStore`**（`server_test.go:28-180`）：内存版 `store.Store`。
- `addKey`（`:49-53`）对明文 token 做 SHA-256 后写入 `byHash` 映射，与真实认证路径一致：
```go
func (f *fakeStore) addKey(k store.APIKey) {
	sum := sha256.Sum256([]byte(k.KeyHash))
	f.byHash[hex.EncodeToString(sum[:])] = k.ID
	f.keys[k.ID] = k
}
```
- `GetKB`/`GetTask` 未命中时返回 `pgxErrNoRows()`（`:23`、`:90`、`:110`），复刻真实 store 的 error 语义，从而能测「不存在」与「越权」响应一致。

**（2）`mcpPost`**（`server_test.go:233-250`）：一次 MCP 请求，手工设三个关键头：
```go
req := httptest.NewRequest("POST", "/mcp", strings.NewReader(string(b)))
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Accept", "application/json, text/event-stream")   // 协议要求的 Accept
if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
if sessionID != "" { req.Header.Set("Mcp-Session-Id", sessionID) } // 握手得到的 session
rec := httptest.NewRecorder()
h.ServeHTTP(rec, req)
var resp map[string]any
_ = json.Unmarshal(rec.Body.Bytes(), &resp)   // 单一 JSON 响应（非 SSE）时可直接解析
```
注意它按**普通 JSON 响应**解析 body——这能成立是因为 streamable HTTP 在非流式请求下直接回 application/json。

**（3）`doInitialize`**（`server_test.go:255-269`）：完整模拟 `initialize` 握手并**从响应头提取 session**：
```go
rec, _ := mcpPost(t, h, token, "", map[string]any{
	"jsonrpc": "2.0", "id": rpcID(1), "method": "initialize",
	"params": map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0.1"},
	},
})
if rec.Code != 200 { t.Fatalf("initialize 失败: %d %s", rec.Code, rec.Body.String()) }
return rec.Header().Get("Mcp-Session-Id")
```
后续所有 `tools/call` 都把返回的 session id 放进 `Mcp-Session-Id` 头（如 `integration_test.go:69-72`）。库侧默认 `StatelessGeneratingSessionIdManager` 会生成 `mcp-session-<uuid>` 并在后续请求里只校验格式。
> 注意：**没有** `notifications/initialized` 通知的发送（真实客户端握手要发），也**没有**真实 MCP 客户端 SDK 参与——这是纯服务端黑盒测试。

**（4）fakeEngine / fakeRetriever**（`server_test.go:183-215`）：`fakeEngine.Ask` 返回固定 `RAGResult{Answer, Sources}`，`err` 非 nil 时返回错误（用于测 isError + 审计 error）；`fakeRetriever.Search` 返回固定 1 条 `RetrieveResult`，metadata 含 `filename`/`kb_id`，用于验证 `safeMetadata` 白名单。

**（5）审计断言方式**：`newTestServer`（`server_test.go:220-230`）用 `NewAuditSink(st, 64, 2000)` 且 `t.Cleanup` 里 `Shutdown`；断言前显式 `sink.Shutdown(ctx)` 再数 `st.logs`（`integration_test.go:215-230`），避免异步竞态。

---

## 8. 面试官最可能深挖的 10 个点

### Q1. 为什么授权失败要在网关层「手写 JSON-RPC」，而不是在 tool handler 里返回 error？

**依据**：`internal/mcp/server.go:47-51` + `internal/mcp/errors.go:8-10`
```go
// 探针结论（task T1）：mcp-go v0.57.0 的 tools/call handler 返回 error 固定映射 -32603，
// 无法经 handler/middleware 返回自定义错误码。因此授权失败（Tool/KB/Task 越权）在此层
// 解析 JSON-RPC body 后直接构造 -32001 响应（spec F4/F5/F8）。
type gateway struct {
	st   store.Store
	next http.Handler
}
```
库侧证据（`mcp-go@v0.57.0/server/server.go:2008-2013`）：
```go
result, err := finalHandler(ctx, request)
if err != nil {
	return nil, &requestError{
		id:   id,
		code: mcp.INTERNAL_ERROR,   // 固定 -32603，无自定义入口
		err:  err,
	}
}
```
**答题要点**：先做了探针（`docs/26-mcp-server/task.md:30-40` T1）验证库能力边界，发现 handler error 只能落 -32603，于是把授权提前到 `http.Handler` 层，在 body 转发前用 `json.Unmarshal` 嗅探 `method=="tools/call"` 并构造自定义错误码；代价是 **body 被读两遍**（`server.go:72-77` 读一次 + `io.NopCloser` 还原，库再读一次）。

### Q2. 认证失败和授权失败为什么分成 HTTP 401 与 JSON-RPC -32001 两层？

**依据**：`internal/mcp/server.go:199-204` + `docs/26-mcp-server/plan.md:242`
```go
// writeJSONRPCError 写授权失败响应：HTTP 200 + JSON-RPC error -32001（plan D4：
// 认证失败 HTTP 401，授权失败在 JSON-RPC error 层表达，MCP 协议层无 403 概念）
func writeJSONRPCError(w http.ResponseWriter, e mcpgo.JSONRPCError) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(e)
}
```
对比 REST 的 `Fail(c, CodeForbidden, ...)`（`internal/api/handler_key.go:43`）。
**答题要点**：401 是传输/身份层问题（请求根本没进入 MCP 会话），用 HTTP 语义 + `WWW-Authenticate` 让客户端知道要重新带凭据；授权是**会话内**的调用级问题，必须让 LLM/客户端看到结构化错误（HTTP 200 + JSON-RPC error），否则 MCP 客户端会把 403 当成协议/传输故障。MCP 规范里没有 403 的对应物。

### Q3. 越权与「不存在」为什么必须返回同一条消息？

**依据**：`server.go:157-172` + `tools.go:255-260`
```go
// 知识库 / 任务资源权限（spec F5/F8）；越权与不存在统一消息（不泄露存在性，spec N2）
switch params.Name {
case ToolGetKB:
	if !kc.Scope.CanAccess(argString(params.Arguments, "kb_id")) {
		return permissionErr(idAny, NewKBForbidden())
	}
...
case ToolGetTask:
	task, err := g.st.GetTask(r.Context(), argString(params.Arguments, "task_id"))
	if err != nil || task == nil || !kc.Scope.CanAccess(task.KBID) {
		return permissionErr(idAny, NewTaskForbidden())
	}
}
```
**答题要点**：如果越权回 -32001「无权限」而存在但无权也回「不存在」，攻击者可以用响应差异做**资源枚举**（oracle）。这里两者严格同码同消息——`err != nil` 与 `!CanAccess` 合并进同一个分支；集成测试逐字段比对两种响应（`integration_test.go:139-151`）。

### Q4. 用户级凭据 `scope=all` 为什么不等于「全部知识库」？

**依据**：`internal/mcp/server.go:89-107`
```go
// resolveOwnerScope 把用户凭据的配置范围解析为实际可访问范围（与用户归属知识库取交集）。
func resolveOwnerScope(ctx context.Context, st store.Store, kc *keyCtx) error {
	ownerKBs, err := st.ListKBsByOwner(ctx, kc.OwnerID)
	if err != nil { return err }
	ownerIDs := make([]string, 0, len(ownerKBs))
	for _, kb := range ownerKBs { ownerIDs = append(ownerIDs, kb.ID) }
	switch {
	case kc.Scope.All:
		kc.Scope = KBPermission{IDs: ownerIDs}          // 用户 all = 自己的知识库
	case len(kc.Scope.IDs) > 0:
		kc.Scope = KBPermission{IDs: intersect(kc.Scope.IDs, ownerIDs)}
	default:
		kc.Scope = KBPermission{}
	}
	return nil
}
```
**答题要点**：「all」的语义是「相对于该凭据可拥有的全部」，而不是「数据库里的全部」。用户凭据的 `allowed universe` 被 owner 收敛为 `ListKBsByOwner` 的结果（`internal/store/kb.go:55-57`，`WHERE owner_id = $1`）。这样即使 DB 里的配置是越权写进去的（例如 allowlist 混入他人 kb_id），运行时也会被 `intersect` 过滤掉——**双层防御**：写入侧 Web 接口校验（`handler_mcp_my.go:263-280`）+ 读取侧交集（`server.go:101-102`）。

### Q5. owner 范围解析失败时会放行还是拒绝？（**本题是陷阱题**）

**依据**：`internal/mcp/server.go:66-70`
```go
	// 用户 MCP 凭据：按 owner 解析实际知识库范围（spec F9）——
	// scope=all → 仅自己的知识库；allowlist → 白名单 ∩ 自己的知识库；无 → 空
	if kc.OwnerID != "" {
		if err := resolveOwnerScope(r.Context(), g.st, kc); err != nil {
			slog.Warn("MCP owner 知识库范围解析失败", "key", kc.KeyID, "err", err)
		}
	}
```
**正确回答**：**当前实现是 fail-open，这是一个真实的安全缺陷**。`ListKBsByOwner` 出错（DB 抖动、连接池耗尽、查询超时）时只打 warn 就继续，此时 `kc.Scope` **保持 config 里的原值**：
- 若该用户 Key 配的是 `mcp_kb_scope=all` → `kc.Scope.All` 仍为 `true` → `handleListKBs` 走 `ListAllKBs` 列出全库（`tools.go:232-233`），`Resolve("")` 返回 `nil` 不过滤（`permission.go:49-51`）→ `retrieve`/`ask` 在**全部知识库（含系统级与他人库）**上检索。
- 若配的是 `allowlist` → `kc.Scope.IDs` 仍为配置值（可能含他人 id）→ 同样越权。
**应改成**：解析失败返回 HTTP 503 或直接把 `kc.Scope` 置空（fail-close）。这是「安全默认」原则在错误分支上的典型漏检——happy path 有测试（`owner_test.go:12/68`），**失败分支无任何测试**。

### Q6. 空的 Tool 白名单为什么表示「无权限」而不是「不限制」？

**依据**：`internal/mcp/permission.go:58-67`
```go
// ToolAllowed 校验 Key 是否被授予指定 Tool：
// 空白名单 = 无任何 MCP Tool 权限（历史 Key 默认无权限，spec F6）；否则 name 须在名单内。
func ToolAllowed(tools []string, name string) bool {
	for _, t := range tools {
		if t == name { return true }
	}
	return false
}
```
**答题要点**：这是**向后兼容 + 安全默认**的取舍。`api_keys` 表加了 `mcp_tools TEXT[] NOT NULL DEFAULT '{}'`（`schema.go:87`），迁移后所有历史 Key 都是空数组。如果空 = 不限制，那么**一次 schema 迁移就会把全仓所有历史 Key 变成全量 MCP 访问凭证**。所以选空 = 无权限，必须通过管理接口显式授予。测试覆盖 `nil` 与 `[]string{}` 两种空值（`permission_test.go:79-84`）。

### Q7. 审计为什么必须异步？队列满了会怎样？

**依据**：`internal/mcp/audit.go:50-69`
```go
// Submit 非阻塞投递审计事件：截断参数（默认 ≤2000 字符）并记录截断前原始长度（spec N4）。
// 队列满 → 丢弃并 warn，绝不影响 MCP 主请求耗时。
func (s *AuditSink) Submit(log store.AuditLog) {
	if s.closed.Load() { return }
	log.ParamsLen = len(log.Params)
	if s.paramLimit > 0 {
		if runes := []rune(log.Params); len(runes) > s.paramLimit {
			log.Params = string(runes[:s.paramLimit])
		}
	}
	select {
	case s.ch <- log:
	default:
		slog.Warn("MCP 审计队列已满，丢弃该审计事件", ...)
	}
}
```
**答题要点**：审计是**可观测性**需求，不能因为 DB 慢而拖慢 RAG 问答（`ask` 可能是秒级 LLM 调用，审计是毫秒级 INSERT，但 DB 抖动不可控）。取舍是「**审计可丢，请求不可阻塞**」（spec N3，`docs/26-mcp-server/spec.md:39`）。队列容量 1024（`app.go:184` 硬编码），满了丢弃并打 warn。注意这里用 `select+default` 而不是 `select+timeout`，是「零等待」策略。测试用极小 buffer（2）+ 1000 次投递验证不 hang（`audit_test.go:53-73`）。

### Q8. 认证失败 / 授权失败为什么不写审计？这样合理吗？

**依据**：`internal/mcp/tools.go:119` 是唯一的审计投递点，位于 tool handler 内；gateway 的 -32001 路径（`server.go:79-82`）直接 `writeJSONRPCError` 返回，**不经过 `run()`**。测试把这一行为固定为契约（`integration_test.go:234-255`）。
**答题要点（诚实版）**：从「调用审计」的定义上说得通（没有调用就没有调用日志），但从**安全审计**角度是缺口——认证失败（可能是 Key 爆破）、授权越权（可能是权限探测）恰恰是最需要留痕的事件，现在只有 `slog.Warn`（`auth.go:63`），没有落库、没有 `api_key_id`/来源 IP 维度、没有告警。改进方向：把 `mcp_audit_logs` 的 `api_key_id` 允许为空/NULL，或在 gateway 层加一条 `status=denied` 的记录（当前表结构 `api_key_id TEXT NOT NULL`，`schema.go:107`，需要迁移）。

### Q9. `ask` 为什么不暴露 thinking？`session_id` 为什么要加 KeyID 前缀？

**依据**：`internal/mcp/tools.go:327-350`
```go
		// 统一绑定 KeyID 前缀：所有 MCP ask 会话与调用方 Key 隔离，
		// 防跨 Key 复用同名 session_id 串读会话历史（安全审查 MEDIUM）
		sessionID = kc.KeyID + ":" + sessionID
		...
		askOpts := []rag.AskOption{
			rag.WithConfigSnapshot(snap),
			rag.WithThinking(false), // 不请求思考链路，不暴露内部推理
		}
		...
		result, err := eng.Ask(ctx, sessionID, question, askOpts...)
		if err != nil { ... }
		return map[string]any{"answer": result.Answer, "sources": result.Sources}, nil
```
**答题要点**：
- thinking：`rag.RAGResult.Thinking`（`internal/rag/engine.go:27`）会包含检索方式、rerank 前后对比、子问题等**内部推理链路与实现细节**。对外的产品语义是「RAG 问答服务」而不是「暴露推理过程」；同时 `WithThinking(false)` 让检索链路的 `Trace` 回调为 nil，按 `internal/retriever/types.go:8` 的注释是「nil=关闭（N2 零开销）」，等于**顺带省掉一层可观测性开销**。双保险：即使引擎返回了 thinking，返回值也只取 `Answer`/`Sources`（`tools.go:350` 显式构造 map，不是直接返回 `result`）。
- session 前缀：MCP 的 `session_id` 是客户端可控字符串，而底层 `engine.Ask` 的会话历史是按 sessionID 存取的（`store.HistoryStore`）。如果不加命名空间，A Key 用 `session_id="default"`、B Key 也用 `"default"`，就会互相读到对方的对话历史。用 `KeyID + ":"` 做**租户命名空间**把隔离下沉到存储键，比在读取时过滤更不易漏。测试断言在 `docs/26-mcp-server/checklist.md:50`，但**代码中未找到对应的自动化测试**（mcp 包内无「跨 Key 同名 session 隔离」用例）——这是一个可以主动承认的测试缺口。

### Q10. 内部错误为什么统一成「工具执行失败，请稍后重试」？

**依据**：`internal/mcp/tools.go:133-143`
```go
	if err != nil {
		// 展示消息：业务/校验错误原样；内部错误（引擎/存储）统一通用消息，防细节泄漏（安全审查 MEDIUM）
		msg := "工具执行失败，请稍后重试"
		switch e := err.(type) {
		case *PermissionError:
			msg = e.Message()
		case *ShowError:
			msg = e.message
		default:
			slog.Error("MCP tool 执行失败", "tool", toolName, "err", err)
		}
		return &mcpgo.CallToolResult{
			Content: []mcpgo.Content{mcpgo.NewTextContent(msg)},
			IsError: true,
		}, nil
	}
```
**答题要点**：这是**错误信息的双通道设计**。外部（LLM/客户端）只看到白名单化的消息：`*PermissionError`（授权语义，必须明确）与 `*ShowError`（参数校验，客户端可自纠）原样返回，其余（SQL 报错、embedding 服务不可用、DSN 片段、内部路径）统一成泛化消息，防止**通过错误信息做内部结构探测**。内部完整错误通过两条通道保留：`slog.Error`（运维日志）与审计的 `error_message`（`tools.go:122`，注释「审计保留详情（内部可查）」）。审慎点：`error_message` 会**原样入库**（`store/audit.go:14` 直接绑定），若底层错误包含敏感信息（如带 DSN 的连接错误），审计表就成了泄露面——当前没有对 `error_message` 做脱敏。

---

## 9. 可被挑刺的缺陷（诚实清单）

按严重度排序。

### 🔴 高：owner 范围解析失败 fail-open（越权读全库）
`internal/mcp/server.go:66-70`：
```go
	if kc.OwnerID != "" {
		if err := resolveOwnerScope(r.Context(), g.st, kc); err != nil {
			slog.Warn("MCP owner 知识库范围解析失败", "key", kc.KeyID, "err", err)
		}
	}
```
DB 错误时只 warn，`kc.Scope` 保持配置原值 → 用户凭据 `scope=all` 会退化为「全库（含系统级与他人库）」可访问。无失败分支测试。应 fail-close。

### 🟠 中高：`tools/list` 把只读 Tool 标注为「破坏性」
`server.go:39-40` 未传任何 `ServerOption`，库 `NewTool` 默认注解（库 `mcp/tools.go:856-861`）为 `ReadOnlyHint:false, DestructiveHint:true, IdempotentHint:false, OpenWorldHint:true`，且 `Annotations` 的 JSON tag 非 omitempty（库 `mcp/tools.go:647`）→ 客户端 UI 会把 6 个只读 Tool 显示成可能破坏环境的工具、也不会允许「免确认自动执行」。项目全包无 `WithReadOnlyHintAnnotation` 等调用（库提供于 `mcp/tools.go:1015-1046`）。

### 🟠 中高：tool handler 无 panic 恢复
`server.go:39` 未传 `WithRecovery()`（库 `server/server.go:401-415`）。库的默认 recover 只在 notification/SSE writer/异步 task 路径生效，**不含同步 tool handler**。`handleRetrieve`/`handleAsk`/`handleListDocs`/`handleGetTask` 直接解引用 `kc`（`tools.go:272-275/309-315/357-360/391-392`），一旦上游注入路径变化就是 nil panic → 连接中断。

### 🟠 中高：未启用 input schema 校验
未传 `WithInputSchemaValidation()`（库选项 `server/server.go:434`）→ 声明的 `required`（`kb_id`/`query`/`question`/`task_id`）**服务端不强制**。例如 `get_knowledge_base` 不传 `kb_id`：`argString` 返回 `""`（`tools.go:185-191`）→ 若 `Scope.All` 则网关 `CanAccess("")` 通过 → handler `GetKB("")` → `ErrNoRows` → 返回 `isError:true`「知识库不存在或无权限」，而**不是** -32602 InvalidParams。客户端拿到的语义错位。

### 🟡 中：`structuredContent` 顶层是数组
`tools.go:161-164` 的 `okResult` 把整个返回值的指针塞进 `StructuredContent`，而 `list_knowledge_bases`/`retrieve`/`list_documents` 返回的是切片（`tools.go:245/301/383`）。MCP 规范中 `structuredContent` 是对象（`{[key:string]: unknown}`）。同时未启用 `WithOutputSchemaValidation`（库 `server/server.go:485`），所以既没有声明 outputSchema 也没有校验。

### 🟡 中：`list_documents` 空结果返回 `null` 而非 `[]`
`tools.go:375` 用 `var docs []docView`（nil），只有循环里才 append；当用户没有任何可访问 KB 时 `json.Marshal(nil)` → `"null"`，且 `StructuredContent` 因 `omitempty` 被整个省略。其它 handler 都用 `make([]T, 0, n)`（`tools.go:240/292/411`）→ 返回 `[]`。客户端解析不一致。

### 🟡 中：`/mcp` 请求体无大小上限
`internal/mcp/server.go:72-77`：
```go
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "读取请求体失败", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
```
认证通过后**无条件**把整个 body 读进内存，无 `http.MaxBytesReader`、无长度检查。MCP 端点是公网可访问的，恶意大 body 直接吃内存。且这段在授权检查之前。

### 🟡 中：认证/授权失败无审计留痕
见 Q8。`auth.go:63` 只有 `slog.Warn`；`server.go:79-82` 直接返回。表结构 `api_key_id TEXT NOT NULL`（`schema.go:107`）也不支持「未知调用者」的失败记录。

### 🟡 中：`mcp.enabled` 热改不生效，且双层开关语义易误导
`app.go:183` 在启动时一次性判定是否挂载路由；`internal/api/handler_config.go:258-259` 注释承认「重启生效」。但 `GET /api/v1/mcp/my/status` 的 `global_enabled` 读的是**实时配置快照**（`handler_mcp_my.go:290-296`），所以会出现「面板说已开启，实际 `/mcp` 404（或反之）」的不一致窗口。

### 🟡 中：无 MCP 专属限流，与 REST 共享单 bucket
`internal/api/router.go:67` 全局 `RateLimit(deps.Config.RateLimitQPS)`；`internal/api/middleware.go:120` 是**单个 `rate.NewLimiter`**，即全站（含 `/mcp`）共用一个令牌桶，burst = qps。`ask` 这类昂贵调用会挤占 REST 的配额；spec N6 的「MCP 端点受现有 RateLimit 保护」字面成立，但没有任何按 Key / 按 Tool 的配额。

### 🟡 中：MCP 调用不更新 `last_used_at`
`auth.go:47-79` 无 `TouchAPIKey`，对比 REST `middleware.go:78`。运维侧无法从 `api_keys.last_used_at` 判断一个 Key 是否只在 MCP 场景被使用，「清理僵尸 Key」的判据失效。

### 🟢 低：审计 worker 无超时，DB 卡死时静默丢单
`audit.go:89` 用 `context.Background()` 且无超时；一条 INSERT 卡住 → worker 阻塞 → channel（1024）填满 → 后续审计全部静默丢弃（只有 warn）。且丢弃没有计数器/指标暴露。`Shutdown` 有 5s 超时（`app.go:284`）但那只是退出路径。

### 🟢 低：审计表无保留策略
`mcp_audit_logs` 只有 `created_at` 与 `(api_key_id, created_at)` 两个索引（`schema.go:116-117`），无 TTL / 分区 / 归档 / 清理任务（代码中未找到）。高频 MCP 调用下会无限增长。

### 🟢 低：审计参数只截断不脱敏
`Params` 是 `json.Marshal(args)` 原文（`tools.go:116`），只做长度截断（`audit.go:58-62`）。`ask` 的 `question`、`retrieve` 的 `query` 会以**用户原始文本**入库，可能是 PII 或业务机密。spec N4 只要求防超大参数，未要求脱敏——但审计表一旦被查就是泄露面。

### 🟢 低：`get_task` 双重 DB 查询
网关 `server.go:168` 与 handler `tools.go:392` 各调一次 `GetTask`；`list_documents` 未指定 kb_id 时对每个 KB 单独 `ListDocuments`（`tools.go:376-382`）→ N+1。

### 🟢 低：session 无状态、`Mcp-Session-Id` 不校验存在性
未传 `WithStateful` 等选项 → 库默认 `StatelessGeneratingSessionIdManager`（库 `streamable_http.go:1845-1890`）：`Generate` 返回 `mcp-session-<uuid>`，`Validate` **只校验前缀与 UUID 格式，不校验是否签发过**。后果：① 可以直接发 `tools/call` 而不先 `initialize`（认证通过即可）；② `DELETE` 终止会话是 no-op（`Terminate` 直接返回 nil）→ 无法真正 revoke 一个 MCP session。好处是水平扩展友好（无本地会话状态）。

### 🟢 低：浏览器型 MCP 客户端可能不可用
项目的 CORS 是 gin 全局中间件（`internal/api/middleware.go:102-113`），只设 `Access-Control-Allow-Origin/Methods/Headers`，**没有 `Access-Control-Expose-Headers`**；而 streamable HTTP 会话依赖客户端读取响应头 `Mcp-Session-Id`（测试正是这么做的：`server_test.go:268`）。同时 mcp-go 自带 CORS 未启用（未传 `WithStreamableHTTPCORS`，库默认 `corsConfig` 为 nil/空 → `enabled()` 为 false，库 `http_cors.go:105-107`）。浏览器里的 MCP 客户端会拿不到 session id。

### 🟢 低：未实现的能力与协议面
- **无 resources / prompts**：只 `AddTool`（`tools.go:43-69`），`initialize` 只声明 `tools`（库 `server/server.go:1091-1118`）。spec `docs/26-mcp-server/spec.md:47` 明确本版不做。
- **无 OAuth 授权服务器 / RFC 9728 保护资源元数据**：401 只给 `WWW-Authenticate: Bearer realm="mcp"`（`auth.go:84`），无 `resource_metadata` 参数。库支持该能力（`server/protected_resource.go` 存在）但项目未接。
- **无幂等性保障**：`ask` 每次调用若不带 `session_id` 都新建 uuid（`tools.go:323-326`）；`retrieve`/`ask` 无请求级缓存或幂等键，重试即重复计费（LLM 调用）。
- **无 Tool 级速率/配额、无并发上限**。
- **`tools/list` 不做按 Key 过滤**：始终返回 6 个（`docs/26-mcp-server/task.md:166` 明确「不按 Key 过滤列表」，因为按 spec 列表是能力发现而非授权结果），但客户端可能误以为都能调。
- **用户凭据不能轮换**：已有凭据 → 409（`handler_mcp_my.go:109-112`），只能「吊销 → 重建」，重建期间旧 Key 立即失效（无过渡期、无多 Key 并存）。
- **`name` 截断逻辑可疑**：`handler_mcp_my.go:121-124`
```go
	name := "mcp-" + userID
	if len(name) > 12 {
		name = name[:12]
	}
```
按**字节**截断（非 rune），且 12 字节的展示名意义有限（uuid 型 userID 几乎全被截掉），注释与 plan 里说的「用户前 8 位」也不完全一致（`docs/27-web-mcp-manage/plan.md:97`）。
- **`resolveOwnerScope` 的 user-all 分支丢掉了 `All` 语义**：转成 `KBPermission{IDs: ownerIDs}` 后，`Resolve("")` 在用户**没有任何 KB** 时返回 `NewKBForbidden()`（`permission.go:52-54`）——这是对的；但若与「用户拥有很多 KB」组合，`retrieve` 过滤条件会变成 `kb_id IN (...)`（`kbFilter` 多值分支，`tools.go:216-218`），大列表会拼进向量库 filter，存在性能/精度隐患（未见上限保护）。

---

## 附：一页速记（面试开场用）

1. **一句话架构**：同进程嵌入 `internal/mcp` 包，`gin.WrapH` 挂 `/mcp`，请求链路 = 自写 Bearer 认证（HTTP 401）→ 自写 gateway 授权（解析 JSON-RPC body，越权回 -32001）→ mark3labs/mcp-go v0.57.0 Streamable HTTP → 6 个只读 Tool → 异步审计（channel + worker）。
2. **最漂亮的设计**：「授权必须返回自定义错误码，而 SDK 只给 -32603」这个约束，是通过**先做探针（T1）→ 把授权提前到 http.Handler 层**解决的，而不是妥协成 isError（`errors.go:8-10`、`server.go:47-51`、`task.md:30-40`）。
3. **最硬的证据**：`integration_test.go` 用 `httptest` + 手拼 JSON-RPC + 从响应头取 `Mcp-Session-Id` 完成真实握手模拟（`server_test.go:255-269`），并断言「越权任务」与「不存在任务」响应完全一致（不泄露存在性）。
4. **最该主动承认的问题**：`server.go:66-70` owner 范围解析失败 fail-open，是真实的越权读全库路径，且无失败分支测试。
