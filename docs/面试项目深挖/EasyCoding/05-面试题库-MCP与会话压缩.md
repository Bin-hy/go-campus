# 05 面试题库：MCP + 会话持久化 + 上下文压缩

> 每题四段：**面试官想听什么** → **口述答案（可直接背）** → **备注讲解（代码依据）** → **可能的追问**

---

# 第一部分：MCP 协议接入

---

## Q5.1 ⭐ MCP 是什么？你为什么要接它？

**面试官想听什么**：你是不是真的理解 MCP 解决什么问题，还是只是「简历上写个好听的」。

**口述答案**：

> 「MCP 是 Anthropic 提出的 **Model Context Protocol**，本质是**给 LLM 应用和外部工具之间定一层标准协议**。
>
> 它解决的问题是 **N × M 的组合爆炸**：假设有 N 个 Agent 应用（Claude Desktop、Cursor、我这个工具）和 M 个工具提供方（GitHub、Slack、数据库、内部 API），如果每个应用都自己对接每个工具，就是 N × M 份集成代码。有了协议之后，工具提供方实现一次 MCP server，所有 MCP 客户端都能用——变成 N + M。
>
> **对我这个项目，接 MCP 的价值是「我不需要为每个新能力写一个内置工具」**。比如用户想让我操作 GitHub，我不需要写一个 `github_tool.go`——他配一行 YAML 指向官方的 GitHub MCP server，启动时我就把它的工具全拉过来注册好了。**这就是『零配置接入』的含义：加能力不用改代码。**
>
> 实现上我做的是 **MCP 客户端**那一侧：连接 server、发现工具、把远端工具适配成我自己的 `Tool` 接口。」

**备注讲解**：

**架构定位**：

```
我的 Agent 循环
    ↓ 只认 tool.Tool 接口
统一注册中心 Registry
    ├── 6 个内置工具（read_file / bash / ...）
    ├── 2 个 Skill 工具
    ├── 4 个 Task 工具
    ├── 1 个 Agent 工具
    └── N 个 MCP 工具  ← mcpTool 适配器实现同一个接口
            ↓ MCP 协议（stdio / Streamable HTTP）
        MCP Servers（GitHub / Slack / 内部服务 ...）
```

**关键设计**：`mcpTool` 实现了和内置工具**完全相同的接口**，所以注册之后它们在系统里没有任何区别。

**可能的追问**：
- **「MCP 和普通的 function calling 有什么区别？」** → 「层次不同。**function calling 是「模型怎么表达『我要调一个函数』」的协议**，这是模型 API 的一部分。**MCP 是「应用怎么发现和调用外部工具」的协议**，它管的是工具从哪来、怎么连接、怎么发现、参数 schema 怎么传。两者是互补的：MCP server 提供的工具，最终还是通过 function calling 让模型调用。」

---

## Q5.2 ⭐⭐ stdio 和 HTTP 两种传输有什么区别？怎么选？

**面试官想听什么**：对进程管理和网络编程的实际理解。

**口述答案**：

> 「**stdio**：MCP server 是**本地子进程**，通过标准输入输出交换 JSON-RPC 消息。适合本地工具——比如访问本地文件系统、本地数据库、或者用 `npx` 起的一个 Node 服务。
>
> **Streamable HTTP**：MCP server 是**远端服务**，通过 HTTP 通信。适合团队共享的服务——比如一个部署在内网的代码搜索服务。
>
> **选型依据主要是「工具在哪」和「凭据在哪」**：
> - 工具需要访问本地资源 → stdio。因为 HTTP 的 server 在远端，碰不到你本机的文件。
> - 工具需要访问内网/共享服务 → HTTP。因为不需要每个人都在本地装一遍。
> - 凭据敏感、不想经过网络 → stdio。
>
> **实现上有个重要差异**：stdio 是**我启动这个子进程**，所以生命周期归我管——退出时我要负责把它关掉，不然就是孤儿进程。HTTP 是**我连一个已知端点**，没有进程要管。
>
> 另外一个细节：stdio 传输我用 `exec.CommandContext` 起进程，把它的 **stderr 直接接到我的 stderr**——这样 server 的报错我能看到，便于排查。stdout 不行，因为那是协议通道，混入日志会破坏 JSON-RPC。」

**备注讲解**：

```go
			switch srv.Type {
			case "stdio":
				cmd := exec.CommandContext(ctx2, srv.Command, srv.Args...)
				cmd.Env = mergeOSEnv(srv.Env)
				cmd.Stderr = os.Stderr                 // ← stderr 透传，stdout 留给协议
				transport = &sdkmcp.CommandTransport{Command: cmd}
			case "http":
				hc := &http.Client{
					Transport: &headerRoundTripper{base: http.DefaultTransport, headers: srv.Headers},
				}
				transport = &sdkmcp.StreamableClientTransport{
					Endpoint:             srv.URL,
					HTTPClient:           hc,
					DisableStandaloneSSE: true,
				}
			}
```

**`DisableStandaloneSSE: true`**：Streamable HTTP 传输默认会额外开一个独立的 SSE 长连接用于服务端主动推送。置 true 表示只走请求-响应。**理由**：项目里没有用到服务端推送能力，多一个长连接是纯粹的资源浪费，而且长连接需要处理重连、心跳等复杂情况。

**环境变量的处理差异**（这个对比很有意思）：

| | 环境变量处理 |
|---|---|
| `bash` 工具 | **清空**，只留 `HOME` / `PATH` / `USER` / `TERM` |
| MCP stdio server | **继承全部宿主环境变量** + 配置里的覆盖 |

**为什么不一样**：`bash` 工具执行的命令**是模型生成的**（不可信），所以不能让它看到 API key 之类的环境变量；而 MCP server 是**用户自己配置的**（可信），它需要完整环境——比如 `npx` 需要 `PATH`、Python server 需要 `VIRTUAL_ENV`、Node server 可能需要 `NODE_OPTIONS`。

**可能的追问**：
- **「stdio 的子进程如果卡住了怎么办？」** → 「有两层保护：① 连接阶段有 30 秒的 `ctx2` 超时；② `Close()` 时有 5 秒的兜底超时（`closeDeadline`）——因为 `cs.Close()` 可能永久阻塞，所以用一个 goroutine + `time.After` 保证进程一定能退出。**代价是可能留下孤儿进程**，这是个已知的取舍。」

---

## Q5.3 ⭐⭐ 命名空间为什么用双下划线？

**面试官想听什么**：他可能只是好奇，但答好了能显得你考虑过「命名冲突」这个真问题。

**口述答案**：

> 「因为要**防歧义**。
>
> MCP 工具的名字格式是 `mcp__<server名>__<工具名>`。用**双**下划线而不是单下划线，是因为 server 名和工具名本身都**可能含下划线**。
>
> 举个反例：假设 server 叫 `my_server`、工具叫 `get_user`。如果用单下划线拼接，得到 `my_my_serverget_user`——**你没法反解出「server 名在哪结束、工具名从哪开始」**。双下划线把「分隔符」和「名字内部的字符」区分开了。
>
> 另外我还加了**字符白名单校验**：
> ```go
> var validToolName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
> ```
> 只允许字母、数字、下划线、连字符。**这个限制不是随便定的——很多模型 API 对函数名有硬性约束**，不允许空格、点号、特殊符号。所以不符合的工具会被**跳过并告警**，而不是带着非法名字注册进去（那样会在发请求时被 API 拒绝，错误信息还很难懂）。」

**备注讲解**：

```go
func adaptTool(serverName string, t *sdkmcp.Tool, cs callerSession) (*mcpTool, bool) {
	fullName := "mcp__" + serverName + "__" + t.Name

	// 禁用字符校验
	if !validToolName.MatchString(fullName) {
		fmt.Fprintf(os.Stderr, "[mcp] warn: skip tool %s: name contains illegal characters\n", fullName)
		return nil, false
	}
	...
}
```

**注意校验的是 `fullName`（含前缀）**——所以如果 server 名里含非法字符（比如 `my server` 带空格），**它下面所有工具都会被跳过**。

**⚠️ 这个设计有一个副作用值得指出**：`validToolName` 允许 `__`（因为 `_` 在白名单里），所以理论上 server 名叫 `a__b`、工具名叫 `c` 时，`mcp__a__b__c` 会有歧义。**但实际影响很小**。

**README 与简历的差异**：简历写的是 `mcp<server><tool>`，README 写的是「`mcp__<server>__<tool>` 前缀」——**代码是后者**。所以被问到「你的命名空间用什么分隔」时，答双下划线。

---

## Q5.4 ⭐⭐ 单服务失败隔离是怎么做的？为什么不能阻塞启动？

**面试官想听什么**：可用性设计的意识。

**口述答案**：

> 「做法是**每个 server 一个 goroutine + `WaitGroup`，各自独立超时 30 秒，失败就 warn 一下跳过**。总启动时间是所有 server 里最慢的那个，而不是它们的和。
>
> **为什么必须隔离？** 因为 MCP server 是**外部依赖**，它的可用性不在我的控制范围内。想象一下：用户配了 5 个 server，其中一个网络不通。如果没有隔离：
> - **串行连接**：启动要等 5 × 30 = 150 秒（每个都超时）——用户会以为程序挂了。
> - **不隔离失败**：任何一个连不上就整个启动失败——用户为了用一个本地工具，被迫删掉一个配错的远端 server 配置。
>
> 这两种体验都不可接受。**MCP 是可选能力，它坏掉不该影响核心功能。**
>
> 实现上就三行关键代码：每个 goroutine 里 `defer wg.Done()`，连不上就 `return`（只跳过自己），最后 `wg.Wait()` 汇总。**但要注意一个细节：`ListTools` 失败时也要 `cs.Close()`**——因为 `Connect` 已经成功了，不关就是连接泄漏。
> ```go
> 			lst, err := cs.ListTools(ctx2, nil)
> 			if err != nil {
> 				fmt.Fprintf(os.Stderr, "[mcp] warn: list tools for server %s failed: %v\n", name, err)
> 				_ = cs.Close() // ← 释放连接
> 				return
> 			}
> ```」

**备注讲解**：

```go
func NewManager(ctx context.Context, cfg Config, version string) *Manager {
	mgr := &Manager{}
	var wg sync.WaitGroup

	for name, srv := range cfg.Servers {
		wg.Add(1)
		go func(name string, srv ServerConfig) {
			defer wg.Done()
			ctx2, cancel := context.WithTimeout(ctx, connectTimeout)   // 30s
			defer cancel()

			// ...建 transport、Connect、ListTools、adaptTool...

			mgr.mu.Lock()
			mgr.sessions = append(mgr.sessions, &session{name: name, cs: cs})
			mgr.tools = append(mgr.tools, adapted...)
			mgr.mu.Unlock()

			fmt.Fprintf(os.Stderr, "[mcp] info: server %s connected, %d tools registered\n", name, len(adapted))
		}(name, srv)
	}
	wg.Wait()

	// 稳定排序（先 server 名再 tool 名，fullName 已含 server 前缀）
	sort.Slice(mgr.tools, func(i, j int) bool { return mgr.tools[i].Name() < mgr.tools[j].Name() })
	return mgr
}
```

**三个细节**：

1. **`connectTimeout` 是包级 `var` 不是 `const`**：
   ```go
   // 连接超时与关闭兜底。包级 var 便于单测改为短值。
   var (
       connectTimeout = 30 * time.Second
       closeDeadline  = 5 * time.Second
   )
   ```
   **这是可测试性的刻意设计**——测试里改成 50ms 就能快速验证超时路径。

2. **共享状态用 `mgr.mu` 保护**：因为多个 goroutine 会并发 append `sessions` 和 `tools`。**但 `waitGroup` 之外还有一层保障**——`Tools()` 返回的是拷贝，防止外部修改。

3. **最后统一排序**：`sort.Slice` 按名字排。**为什么要排序？** 因为并发 append 的顺序是不确定的（goroutine 调度决定）。**如果不排序，工具列表的顺序每次启动都可能不一样**——而工具列表会进入发给模型的请求，**顺序变了就破坏 Prompt Cache**（不同模型 API 对工具定义的处理方式不同，但保持一致总是更安全的）。

**这个「并发写入 + 最终排序」的模式很值得说**——它同时拿到了并发性能和确定性输出。

**可能的追问**：
- **「如果 server 连上了但一个工具都没有呢？」** → 「会打出 `server X connected, 0 tools registered`。这是正常情况（有些 server 可能只提供 resource 或 prompt，没有 tool）。不报错。」

---

## Q5.5 ⭐⭐ `readOnlyHint` 为什么不能信？

**面试官想听什么**：这是个安全判断问题——**为什么你要对一个「官方声明的」字段保持怀疑**。

**口述答案**：

> 「因为 **`readOnlyHint` 是 server 自己声明的，我无法验证**。而且它声明错了后果很严重——它同时影响两件事：
>
> **① 并发安全**：我把它当作「这个工具可以和别的只读工具并发执行」的依据。如果 server 撒谎（或者自己写错了），一个实际写文件的工具被判成只读，就会和其他工具并发跑——**这是数据竞争**。
>
> **② 权限判定**：只读类在模式兜底里**恒为 `Allow`**。所以如果误判成只读，**本该弹窗的操作就静默放行了**。
>
> 所以我的策略是**严格只信显式为 true**：
> ```go
> 	// readOnly：严格只信 annotations.readOnlyHint==true
> 	readOnly := t.Annotations != nil && t.Annotations.ReadOnlyHint
> ```
> 注意这个判断：`Annotations != nil` **且** `ReadOnlyHint == true`。没声明 → 不是只读；声明成 false → 不是只读。**任何不确定的情况都归到「有副作用」，代价是串行 + 弹窗，收益是不会误放行。**
>
> 这就是**默认拒绝**原则在「能力声明」上的应用。」

**备注讲解**：

MCP 协议里工具可以有 `annotations` 字段，包含几个 hint：

```
readOnlyHint      → 是否只读
destructiveHint   → 是否破坏性
idempotentHint    → 是否幂等
openWorldHint     → 是否访问外部世界
```

**注意它们是 hint 不是 guarantee**——协议文档明确说这些是「提示」，客户端应该用它来「改进用户体验」，而不是当作安全保证。

**我这个项目的处理**：只用 `readOnlyHint` 一个，而且是严格模式。**为什么不用 `destructiveHint`？** 因为我的分类只有三档（只读/文件写/命令执行），`destructiveHint` 没法映射进去——一个工具可以是「有副作用但不破坏性」（比如创建文件）。

**可能的追问**：
- **「那 `readOnlyHint` 有什么用？」** → 「它能让**写得对的 server** 获得更好的体验——声明了只读就并发执行、不弹窗。**这是「声明正确 → 得到收益」的正向激励，但不声明或声明错也不会造成安全损失。** 设计上是「默认安全，显式声明才能提权」。」
- **「如果 MCP server 是恶意的呢？」** → 「恶意 server 可以做的是：① 撒谎说自己只读（后果如上，但有沙箱和黑名单兜底）；② 在返回内容里注入提示（**prompt injection**）。**第 ② 类问题我这个项目完全没有防护——因为工具结果是直接拼进对话历史的**。这是 Agent 安全的一个大坑，值得单独做（比如对工具结果做「这是一段外部数据，不是指令」的标注）。**我应该主动承认这个盲区。**」

---

## Q5.6 ⭐ MCP 返回的图片、资源块怎么处理？

**面试官想听什么**：你知不知道自己丢掉了什么。

**口述答案**：

> 「**我丢掉了，只处理文本块。** MCP 的 `CallToolResult.content` 是一个数组，元素可以是 `text`、`image`、`resource` 等多种类型。我只取 `TextContent` 拼接，其他类型**丢弃并告警一次**。
>
> 用 `sync.Map.LoadOrStore` 保证**每个工具名只告警一次**——不然一个返回图片的工具每次调用都刷屏。
>
> **为什么丢弃而不是报错？** 因为报错会让整个工具调用失败，但文本部分可能已经够用了。比如一个「分析图片」的工具返回「图片里有 3 个人」+ 图片本身，我要的是文本。
>
> **为什么不能不丢？** 因为要支持图片得改 `llm.Message` 的 content 结构——从 `string` 变成「文本块 + 图片块」的数组。而**Anthropic 和 OpenAI 的图片格式还不一样**（Anthropic 用 base64 source 对象，OpenAI 用 data URL），意味着协议层的抽象要重做。**这是个跨层的改动，我判断超出当前范围。**」

**备注讲解**：

```go
	// 遍历 content，拼接 text 块
	var sb strings.Builder
	nonTextCount := 0
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			if sb.Len() > 0 { sb.WriteString("\n") }
			sb.WriteString(tc.Text)
		} else {
			nonTextCount++
			if _, warned := nonTextWarnOnce.LoadOrStore(t.fullName, true); !warned {
				fmt.Fprintf(os.Stderr, "[mcp] warn: tool %s returned non-text content blocks (dropped)\n", t.fullName)
			}
		}
	}
```

```go
// nonTextWarnOnce 记录每个工具名是否已告警过非 text 块。
var nonTextWarnOnce sync.Map
```

**`sync.Map` 的适用场景**：`sync.Map` 适合「一次写入、多次读取」或者「key 集合基本不重叠」的场景——这里正是（每个工具名只写一次，之后全是读）。**用普通的 `map + mutex` 也可以，但 `sync.Map` 在这个模式下的读路径是无锁的。**

**`nonTextCount` 变量其实没被使用**（只自增，没读）——这是个小的死代码，可以主动指出。

**可能的追问**：
- **「如果非要支持多模态，你会怎么改？」** → 「我会把 `llm.Message.Content` 从 `string` 改成 `[]ContentBlock`，每个 block 有类型。然后在两个适配器里把统一的 block 转成各家格式。**关键是抽象层要表达「一个消息可以同时有文本和图片」，这是两家协议都支持的语义。** 改动量大概在 300 行左右，主要在协议层。」

---

## Q5.7 ⭐⭐ MCP 工具和内置工具是怎么做到「上层无感」的？

**面试官想听什么**：接口抽象的落地效果。

**口述答案**：

> 「靠**同一个接口**——`mcpTool` 实现了和内置工具完全一样的 `tool.Tool`：
> ```go
> type Tool interface {
> 	Name() string
> 	Description() string
> 	Parameters() map[string]any
> 	ReadOnly() bool
> 	Execute(ctx context.Context, args json.RawMessage) Result
> }
> ```
> 然后在主程序里，MCP 工具和内置工具**注册进同一个 `Registry`**：
> ```go
> 	reg := tool.NewDefaultRegistry()              // 6 个内置
> 	mgr := mcp.NewManager(context.Background(), mcpCfg, version)
> 	for _, t := range mgr.Tools() {
> 		reg.Register(t)                            // MCP 工具进同一个注册中心
> 	}
> ```
>
> 注册之后，它们就会**自动共享四条路径**：
> ① `Registry.Definitions()` 一起导出给模型；
> ② `permission.Engine.Check()` 一起走权限判定（包括黑名单、规则、模式）；
> ③ `executeBatched()` 一起参与只读并发/写串行的分批；
> ④ `Registry.Execute()` 一起执行、结果一起回灌。
>
> **上层（agent 循环、权限引擎、压缩）里没有一行代码知道 MCP 的存在。** 这也是我设计 `Tool` 接口时的目标——**让扩展能力和内置能力在系统里没有等级差别**。」

**备注讲解**：

**`mcpTool` 的接口实现**：

```go
func (t *mcpTool) Name() string               { return t.fullName }
func (t *mcpTool) Description() string        { return t.descr }
func (t *mcpTool) Parameters() map[string]any { return t.schema }
func (t *mcpTool) ReadOnly() bool             { return t.readOnly }
func (t *mcpTool) Execute(ctx context.Context, args json.RawMessage) tool.Result { ... }
```

**唯一「有等级差别」的地方**：`tool.SystemTool` 可选接口。实现它的工具（比如 `LoadSkill`）在工具白名单过滤时**永远豁免**：

```go
// Registry.DefinitionsFiltered
	for _, name := range r.order {
		t := r.tools[name]
		// 系统工具豁免
		if IsSystemTool(t) {
			defs = append(defs, llm.ToolDefinition{...})
			continue
		}
		if allowedSet[t.Name()] { defs = append(defs, ...) }
	}
```

**MCP 工具没有实现 `IsSystem()`**，所以它受白名单约束——这是对的（子 Agent 不应该自动获得所有 MCP 工具，除非显式允许）。

**可能的追问**：
- **「MCP 工具的 schema 是怎么转的？」** → 「直接透传。MCP 的工具定义本身就带 JSON Schema，我做一个 `json.Marshal` → `json.Unmarshal` 到 `map[string]any` 的往返来去掉 SDK 的类型：
> ```go
> 	var schema map[string]any
> 	if t.InputSchema != nil {
> 		b, _ := json.Marshal(t.InputSchema)
> 		_ = json.Unmarshal(b, &schema)
> 	}
> 	if len(schema) == 0 { schema = map[string]any{"type": "object"} }
> ```
> **为什么不直接类型断言/转换？** 因为 MCP SDK 的 schema 类型可能变，而 JSON 往返是**类型无关的稳定桥接**。代价是多一次序列化——但这只发生在启动时，一次性开销。**兜底 `{"type": "object"}` 是必要的**——如果工具没有 schema，发一个 `nil` 给某些 API 会报错。」

---

## Q5.8 ⭐ 30 秒超时是怎么定的？子进程会不会泄漏？

**面试官想听什么**：超时值的依据 + 资源泄漏意识。

**口述答案**：

> 「**30 秒有两层**：连接阶段 30 秒（`connectTimeout`），工具调用阶段 30 秒（`mcpTool.Execute` 里的 `context.WithTimeout`）。
>
> 30 秒的依据是：**MCP 工具通常是「一次操作」，不是长任务**。搜索、查数据库、调 API，30 秒足够。如果某个工具真的要跑几分钟（比如全仓库索引），那它**应该做成异步任务**，而不是让 Agent 阻塞等。
>
> **顺带说一个我发现的不一致**：Agent 层给每个工具的默认超时也是 30 秒（`tool.DefaultTimeout`），所以 MCP 工具实际上**受到双重 30 秒约束**——外层是 agent 给的，内层是 `mcpTool` 自己加的。**两者相同，所以内层那个其实是冗余的**（外层先到期）。这个冗余不算 bug，但说明两层各自独立地定了同一个值，**将来改一处就会不一致**。
>
> **子进程泄漏怎么处理？** `Manager.Close()` 里做了并发关闭 + 5 秒兜底超时：
> ```go
> 	var wg sync.WaitGroup
> 	for _, s := range sessions { wg.Add(1); go func(cs){ defer wg.Done(); _ = cs.Close() }(s.cs) }
> 	done := make(chan struct{})
> 	go func() { wg.Wait(); close(done) }()
> 	select {
> 	case <-done:
> 	case <-time.After(closeDeadline):   // 5s 兜底，不等了
> 	}
> ```
> **为什么要兜底？** 因为 stdio server 是子进程，如果它卡死不响应关闭请求，`cs.Close()` 会永久阻塞 → 主程序退不出来。**兜底保证进程一定能退出，代价是可能留下孤儿进程。**
>
> 另外 `main.go` 里用 `defer mgr.Close()` 保证正常退出时会调用。」

**备注讲解**：

**超时的层次**：

```
Agent.executeBatched: context.WithTimeout(ctx, tool.DefaultTimeout)  ← 30s（外层）
    ↓
mcpTool.Execute:      context.WithTimeout(ctx, 30*time.Second)       ← 30s（内层，冗余）
    ↓
SDK CallTool
```

**`context.WithTimeout` 的嵌套语义**：内层的 deadline 是「父 deadline 和内层超时的**最小**」。因为两者相同，谁先创建的谁先到期——外层先创建，所以外层总是先生效。

**一个更优的实现**是让 `mcpTool` 不自己加超时，而是信任调用方传入的 ctx。**这样超时策略只有一处可配**。

**可能的追问**：
- **「孤儿进程真的会留下吗？」** → 「会。超时之后 `Close()` 返回了，但那个子进程可能还活着。**要彻底解决需要进程组**：用 `Setpgid` 把子进程放进独立进程组，退出时 `kill(-pgid, SIGKILL)` 干掉整组。这是标准的做法，我没做。**在 stdio 场景下这是应该补的**——因为 MCP server 经常是 `npx` 起的，`npx` 又会 fork 出真正的 server 子进程，**杀父进程不会杀掉孙进程**。」

---

## Q5.9 ⭐ 配置里的 `${VAR}` 是干什么的？

**面试官想听什么**：安全意识。

**口述答案**：

> 「**防止凭据落盘。**
>
> 项目的 MCP 配置是设计成**可以提交到 git** 的（README 里明说「项目根，可提交 git」）。但如果 server 的凭据要写在配置里，直接写明文就泄露了。
>
> 所以我支持 `${VAR}` 环境变量展开：
> ```yaml
> mcp_servers:
>   github:
>     type: stdio
>     command: npx
>     args: ["-y", "@modelcontextprotocol/server-github"]
>     env:
>       GITHUB_TOKEN: "${GITHUB_TOKEN}"     # 配置里只有变量名
> ```
> 配置进仓库，真实值从运行环境来。
>
> **实现上有个细节**：只匹配 `\$\{[A-Za-z_][A-Za-z0-9_]*\}` 这个精确形式，**不用 `os.ExpandEnv`**。因为 `os.ExpandEnv` 会把**单个 `$VAR`** 也当变量展开——而 shell 命令里 `$` 太常见了（比如 `$HOME`、脚本里的 `$1`），会误伤。
>
> **未定义的变量**：展开成空串 + stderr 告警（去重后一次性打出），**不阻断**——因为一个 server 的变量没设不该让整个配置加载失败。
>
> **另外有个不一致**：`bash` 工具**清空了环境变量**只留 4 个，而 MCP server **继承全部宿主环境变量**。区别在于**可信度**：bash 执行的命令是模型生成的（不可信，不能让 `env` 命令把 API key 打出来）；MCP server 是用户配置的可信进程，需要完整环境才能跑。」

**备注讲解**：

```go
// 变量展开正则：仅匹配 \$\{VAR_NAME\}，VAR_NAME 由字母、数字、下划线组成。
var varPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func expandVars(s string) (string, []string) {
	var undefined []string
	out := varPattern.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1]
		val, ok := os.LookupEnv(varName)
		if !ok { undefined = append(undefined, varName); return "" }
		return val
	})
	return out, undefined
}
```

**`collectUndefined` 去重**，避免同一个变量被多个 server 引用时打多次告警。

**作用范围**：只对 `env` 和 `headers` 的**值**做展开——不对 `command` / `args` / `url` 展开。**这个边界是刻意的吗？** 从代码看是：

```go
func applyExpansion(name string, srv *rawServer) {
	expandedEnv := make(map[string]string, len(srv.Env))
	for k, v := range srv.Env { out, undef := expandVars(v); expandedEnv[k] = out; ... }
	srv.Env = expandedEnv

	expandedHeaders := make(map[string]string, len(srv.Headers))
	for k, v := range srv.Headers { out, undef := expandVars(v); expandedHeaders[k] = out; ... }
	srv.Headers = expandedHeaders
	...
}
```

**为什么不展开 `url`？** 实际上 URL 里带 token 也很常见（`https://api.example.com?token=${TOKEN}`）。**这是一个功能缺失**，被问到可以说：「`url` 和 `args` 也应该支持展开——比如有些 MCP server 的认证 token 是放在 URL 查询参数里的。现在的实现只覆盖了 `env` 和 `headers`，是个遗漏。」

---

## Q5.10 ⭐ 两层配置为什么是「完整覆盖」而不是「字段合并」？

**面试官想听什么**：配置系统的语义设计。

**口述答案**：

> 「因为**「完整覆盖」的语义更好预测**。
>
> 假设用户级配了 `env: {TOKEN: x, DEBUG: 1}`，项目级只配了 `command`。如果是字段合并，结果是「command 来自项目级、env 来自用户级」——**用户看到项目级配置里没有 `TOKEN`，会以为不需要，实际上它从用户级悄悄继承过来了**。这种「隐形继承」在排查问题时非常痛苦。
>
> 完整覆盖的语义是：**项目级只要定义了这个 server，就完全取代用户级的定义**。虽然多写几行，但**所见即所得**。
>
> 这也是很多配置系统的通行做法——比如 nginx 的 `server` 块、Docker 的 service 定义，都是覆盖而非深合并。**深合并看起来「聪明」，但它把「优先级」这个心智负担转嫁给了用户。**」

**备注讲解**：

```go
// mergeServers 两层合并：项目级同名 server 完整覆盖用户级。
func mergeServers(user, project map[string]rawServer) map[string]rawServer {
	merged := make(map[string]rawServer, len(user)+len(project))
	for k, v := range user { merged[k] = v }
	for k, v := range project { merged[k] = v } // 完整覆盖
	return merged
}
```

**注意项目级的路径不一样**：

| 层级 | MCP 配置路径 |
|---|---|
| 用户级 | `~/.mewcode/config.yaml` |
| 项目级 | `<root>/.mewcode.yaml`（**不是** `.mewcode/config.yaml`！） |

**这是个容易搞混的点**：项目级的**LLM 配置**在 `.mewcode/config.yaml`，而**MCP 配置**在 `.mewcode.yaml`（项目根下的隐藏文件）。

**为什么不一样？** 大概是历史原因（MCP 是后加的功能，选了不同的路径）。**这是个真实的可用性问题**——用户在 `.mewcode/config.yaml` 里写 MCP 配置不会生效，而且不会有任何提示。**被问到要承认：「这个路径不一致是设计缺陷，应该统一到 `.mewcode/` 目录下。」**

**可能的追问**：
- **「合并的时机是什么？」** → 「启动时一次性完成：读两层 → 各自做 `${VAR}` 展开 → 合并 → 逐条校验 → 组装成 `Config`。**展开是在合并之前做的**，所以「项目级覆盖用户级」时覆盖的是**已展开的值**——这个顺序是对的，否则被覆盖的那层的未定义变量告警会误报。」

---

# 第二部分：会话持久化

---

## Q5.11 ⭐⭐ 为什么用 JSONL 不用 SQLite？

**面试官想听什么**：技术选型能力。这是经典的「你为什么不选 X」题。

**口述答案**：

> 「三个理由，我按重要性排：
>
> **① 崩溃安全，而且是「天然的」**。追加写模型最坏情况只丢最后一行——因为已经写进去的字节不会被动过。如果用单文件覆盖写（比如一个 JSON 数组），每次保存都要重写整个文件，**重写过程中崩溃就全丢了**。SQLite 也能做到崩溃安全，但需要正确配置 WAL 模式。
>
> **② 零依赖 + 可观测**。JSONL 用标准库就够了，不用引入 SQLite 的 CGO 或者一个几 MB 的纯 Go 实现。而且**调试的时候 `tail -f` 就能看实时对话流**——我在开发这个项目时，这个体验的价值超出预期，它让我能直接看到模型每一轮收到了什么。
>
> **③ 查询需求几乎为零**。我需要的操作只有两个：列出会话（按时间倒序）、读某一条会话。前者扫目录、后者读文件就够了。**用 SQL 是在为一个不存在的需求付出复杂度成本。**
>
> **什么情况下我会换 SQLite？** 当需要**跨会话检索**的时候——比如「找出所有讨论过 auth 模块的会话」。那时候全扫 JSONL 是 O(会话数 × 文件大小)，必须上索引。
>
> **代价我也要讲**：JSONL 有一个真实问题——**压缩之后旧内容不删**，文件会持续增长（老历史和新历史都在里面）。一个长时间用的会话可能到几 MB。我的缓解方式是 30 天清理会整体删掉。**如果要做「单文件永久保留」，就得做 compaction（重写文件去掉 compact 标记之前的内容）。**」

**备注讲解**：

**目录结构**：

```
<root>/.mewcode/sessions/<YYYYMMDD-HHMMSS-xxxx>/
├── conversation.jsonl                    ← 消息流（追加写）
└── tool-results/                         ← 第一层压缩落盘的工具结果
    └── <tool_use_id>
```

**Entry 结构**：

```go
type Entry struct {
	Type        string           `json:"type,omitempty"`   // "compact" 或空
	Role        string           `json:"role,omitempty"`
	Content     string           `json:"content,omitempty"`
	ToolCalls   []llm.ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []llm.ToolResult `json:"tool_results,omitempty"`
	Timestamp   int64            `json:"ts"`
	Model       string           `json:"model,omitempty"`  // 仅首条消息
}
```

**`Model` 只写首条**——因为会话的模型是固定的，每条都写纯属浪费。

**对照表（面试时可以直接画）**：

| 维度 | JSONL | SQLite |
|---|---|---|
| 依赖 | 标准库 | CGO 或纯 Go 实现（+几 MB） |
| 崩溃安全 | 追加写，最坏丢一行 | WAL 下更强 |
| 查询 | 全扫 | 索引 |
| 可观测 | `tail -f` / `cat` | 需要工具 |
| 文件增长 | 需要手动 compaction | 不会（原地更新） |
| 并发写 | 需要自己加锁 | 内建事务 |

**「并发写需要自己加锁」这点我处理了**：

```go
type Writer struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
	path string
}
```

**可能的追问**：
- **「`Writer` 的锁和 `Conversation` 的锁会不会死锁？」** → 「**不会，而且是刻意设计的**。看这个顺序：`Conversation.AddXxx` 是**先解锁再调回调**：
> ```go
> 	c.mu.Lock()
> 	c.messages = append(c.messages, llm.Message{...})
> 	msg := c.messages[len(c.messages)-1]
> 	c.mu.Unlock()                        // ← 先解锁
> 	if c.onAppend != nil { c.onAppend(msg) }   // ← 回调里拿 Writer 的锁
> ```
> 所以锁的获取顺序是「先 Conversation 后 Writer」，**永远不会反向**，构不成循环等待。**如果持锁做回调，那 Writer 的磁盘 IO 会阻塞所有读 `Messages()` 的 goroutine——这是个必须避免的设计。**」

---

## Q5.12 ⭐⭐ 每次写都 fsync，性能不会崩吗？

**面试官想听什么**：你知不知道自己付了什么代价。

**口述答案**：

> 「会有代价，但在这个场景下可以接受。
>
> **代价的量化**：`file.Sync()` 就是 `fsync(2)`，每次调用是一个系统调用 + 一次磁盘刷盘，典型耗时 1–10ms（机械盘更慢、SSD 更快）。一轮对话假设产生 10 条消息，就是 **10–100ms 的额外开销**。
>
> **收益**：进程被 `kill -9`、终端崩溃、系统断电，最多丢最后一行。
>
> **为什么我认为值得**？因为**终端 Agent 是长时间交互的工具，会话历史就是用户的工作成果**。一轮对话本身要几秒到几十秒，多 100ms 完全无感；而丢对话是不可接受的。
>
> **如果要优化，正确做法是「批量 fsync」**：攒够 N 条或者每 100ms fsync 一次，把 10 次系统调用降成 1 次。代价是崩溃时可能丢最近 100ms 的消息。**但我不打算在这种情况下做这个优化——正确性优先于 100ms。**
>
> **另外一个我要承认的问题**：写失败被忽略了：
> ```go
> func (w *Writer) OnAppend(model string) func(llm.Message) {
> 	isFirst := true
> 	return func(msg llm.Message) {
> 		_ = w.Append(msg, model, isFirst)     // ← 错误被吞掉
> 		isFirst = false
> 	}
> }
> ```
> 现在磁盘满了、权限不对，用户**完全无感**——对话照常进行，但什么都没落盘。**这应该至少变成一条可见的告警**（比如通过事件流发一个 Notice），或者在连续失败 N 次后提示用户。**「静默的数据丢失」是最坏的一类失败。**」

**备注讲解**：

```go
func (w *Writer) Append(msg llm.Message, model string, isFirst bool) error {
	// ...构造 entry...
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.enc.Encode(entry); err != nil {
		return fmt.Errorf("JSONL 编码失败: %w", err)
	}
	return w.file.Sync()          // ← fsync
}
```

**`json.Encoder.Encode` 会自己加换行**——所以 JSONL 的行分隔是 Encoder 保证的，不需要手动写 `\n`。这也是为什么用 `Encoder` 而不是 `Marshal + Write`。

**`WriteCompactMarker` 也 fsync**——保证标记行落盘，否则恢复时可能读不到标记（导致读了不该读的旧内容）。

**可能的追问**：
- **「fsync 和 fdatasync 有什么区别，你为什么用 Sync？」** → 「`fsync` 刷数据 + 元数据（文件大小、mtime 等），`fdatasync` 只刷数据。对追加写场景，**文件大小变化是必须刷的元数据**——否则崩溃后文件长度可能不对。所以 `fsync` 是正确选择。Go 标准库的 `File.Sync()` 在 Linux 上调用的是 `fsync`。」

---

## Q5.13 ⭐⭐ 恢复会话时怎么处理坏行和孤立工具调用？

**面试官能听什么**：你有没有真实处理过「崩溃后的数据」。

**口述答案**：

> 「两个都是崩溃场景的处理。
>
> **① 坏行跳过。** 崩溃时最后一行大概率是**写到一半**的（fsync 之前被 kill）。`LoadSession` 里对 `dec.Decode` 的错误直接 `continue`——**跳过这一行继续读**。
> 为什么不报错？因为**报错意味着用户永远恢复不了这个会话**，代价太大。跳过一行的代价是少一条消息，模型完全能继续工作。
>
> **② 孤立工具调用截断。** 如果进程在「assistant 请求了工具」和「工具结果落盘」之间崩溃，JSONL 里就只剩前一半——历史末尾是一条**带 `tool_calls` 但没有对应结果的 assistant 消息**。带着这个历史发请求，**Anthropic 会直接 400**（要求每个 `tool_use` 必须有配对的 `tool_result`）。
>
> 我的处理是**丢掉最后那条 assistant**：
> ```go
> func TruncateOrphanedToolCalls(msgs []llm.Message) []llm.Message {
> 	if len(msgs) == 0 { return msgs }
> 	last := msgs[len(msgs)-1]
> 	if last.Role == llm.RoleAssistant && len(last.ToolCalls) > 0 {
> 		return msgs[:len(msgs)-1]
> 	}
> 	return msgs
> }
> ```
>
> **为什么是「丢掉」而不是「补一个假结果」？** 因为补假结果会**污染上下文**——模型会看到一个「[结果已丢失]」的工具结果，可能基于它做推理。而丢一条 assistant 的代价很小：**模型下一轮会重新决定调什么工具**，反正它本来就要重新规划。」

**备注讲解**：

```go
// LoadSession 从 conversation.jsonl 恢复消息列表。
// 从最后一个 compact 标记之后加载，跳过坏行，截断孤立工具调用。
func LoadSession(sessionDir string) ([]llm.Message, error) {
	...
	var entries []Entry
	dec := json.NewDecoder(f)
	for dec.More() {
		var entry Entry
		if err := dec.Decode(&entry); err != nil {
			continue                        // ← 跳过坏行
		}
		if entry.Type == "compact" {
			entries = nil                   // ← 清空，从 compact 之后重新开始
			continue
		}
		entries = append(entries, entry)
	}
	msgs := entriesToMessages(entries)
	msgs = TruncateOrphanedToolCalls(msgs)
	return msgs, nil
}
```

**「从最后一个 compact 标记之后加载」的实现很巧妙**：`entries = nil` 直接清空切片，之后收集的就是最终历史。**这是「流式扫描 + 重置累加器」的模式**——不需要先扫一遍找到最后一个标记的位置，一次遍历就够了。

**可能的追问**：
- **「如果坏行在文件中间呢？」** → 「`json.Decoder` 的行为是：一个 `Decode` 失败后，**下一次 `Decode` 会尝试从失败点之后重新找一个合法的 JSON 值**。所以中间坏一行不会导致后面全丢。**但如果坏行破坏了很多字节（比如整个 block 的写入顺序乱了），后面可能连续失败**——那样就会丢掉较多内容。对追加写模型来说这种情况几乎不会发生，因为写入是按行原子的（`Encoder.Encode` 一次性写一个完整的 JSON + `\n`，小于一个 page 时是原子的）。」
- **「孤立工具调用的判断只看最后一条，够吗？」** → **这是个真实的局限**：「只检查了最后一条。如果崩溃导致中间某条也孤立的（理论上不会，因为写入是顺序的），检测不到。**更严格的做法是遍历整个历史做配对校验**——每条 `ToolCalls` 都能找到对应的 `ToolResults`。现在的简化版够用，因为**追加写保证了前缀总是完整的**。」

---

## Q5.14 ⭐⭐ `/resume` 恢复的完整流程是什么？

**面试官想听什么**：你对自己项目的核心链路熟不熟。**这题答得流畅，可信度直接拉满。**

**口述答案**：

> 「用户敲 `/resume` 之后是**异步加载 + 7 步恢复**。
>
> **第一步是列表**：`OpenResumeMenu` 切到 `stateResuming`，然后发一个 `tea.Cmd` 异步调 `session.ListSessions`——**不阻塞 UI**，加载期间显示「正在加载会话列表...」。列表用 `bubbles/list` 组件，支持上下键导航、输入搜索过滤、Enter 选择、Esc 取消。
>
> **列表的标题怎么来的？** 读 JSONL 的**第一条 `role=user` 的消息，截断到 50 个字符**。而且**拿到标题和模型名就立刻 break**——因为一个长会话的 JSONL 可能几 MB，列表里几十个会话全读一遍就是几百 MB 的 IO。**列表是高频操作，必须只读开头。**
>
> **⚠️ 这里有个细节**：截断用的是 `[]rune` 而不是 `[]byte`——**中文一个字符 3 字节，按字节截断会把汉字切碎变成乱码。**
>
> **Enter 之后是 7 步**：
> ① `LoadSession` 读消息（跳坏行、从最后一个 compact 标记后、截孤立调用）
> ② **token 估算 + 超阈值压缩**——估算超过 `ContextWindow - 8000` 就立刻调 `RunForceCompact` 压一次
> ③ **时间跨度提醒**——如果距离会话最后修改超过 6 小时，追加一条系统提示告诉模型「部分上下文可能已过时」
> ④ `OpenSessionContext` 重建会话上下文（含 `SpillDir`）
> ⑤ **`OpenWriter` 以追加模式重开 JSONL**
> ⑥ `NewFromMessages` 构造带回调的 Conversation
> ⑦ 返回 `resumeDoneMsg`，TUI 替换 `conv` / `writer` / `runtime.Session`，把历史回放到 scrollback，切回 idle
>
> **第 ⑤ 步有一个顺序坑**：必须**在构造 Conversation 之前**打开 writer。因为 `Conversation` 的回调是**构造时注入**的——如果先构造再换 writer，回调还指向旧的 writer，**恢复后新增的消息会写回旧会话文件**。代码注释专门强调了这一点。
>
> **第 ③ 步是个我觉得挺重要的设计**：Agent 的历史里有大量「文件内容的快照」。6 小时前的会话恢复后，那些文件可能已经被改过了。如果不提醒，模型会**基于过期内容推理**——**这比没有上下文更危险**。所以我的原则是「**不要假装上下文还有效**」。」

**备注讲解**：

```go
		// 检查时间跨度（超过 6 小时追加提醒）
		if elapsed := time.Since(info.ModifiedAt); elapsed > 6*time.Hour {
			// 追加 llm.Message{Role: RoleUser,
			//   Content: "[系统提示] 本会话已暂停 %s。部分上下文可能已过时，如需最新信息请重新读取相关文件。"}
		}
```

```go
		// token 估算 + 超阈值压缩
		est := estimateTokens(msgs)
		if est > int64(runtime.ContextWindow - 8000) {
			// 用临时 Conversation 调 m.ag.RunForceCompact(...)，成功则用压缩后的消息
		}
```

**注意阈值是 `ContextWindow - 8000`**，和运行中自动压缩的 `ContextWindow - 20000 - 13000` 不同。**为什么？** 因为场景不同：

| 场景 | 余量 | 理由 |
|---|---|---|
| 运行中自动压缩 | 33000 | 要给**摘要请求本身**留 20000（它要把整段历史发出去），再留 13000 吃估算误差 |
| 恢复时压缩 | 8000 | 一次性的、**静止的**历史，只检查「压完之后能不能继续工作」 |

**「历史回放」的实现**：

```
user      → renderUserBlock
assistant → glamour 渲染后 renderAssistantBlock
tool      → 截断 200 字符的 renderNoticeBlock
```

**tool 结果只回放 200 字符**——因为回放是为了让用户「想起来这是哪个会话」，不是真的要读所有工具输出。**全量回放会把终端刷爆。**

**可能的追问**：
- **「恢复后 `runtime` 的哪些状态要重置？」** → 「`ResetForNewSession` 清空 compact 的子状态（Replacement / Recovery / AutoTracking）、锚点、轮次计数、活跃 Skill、pending reminders，并重置 Hook 引擎的 `only_once` 集合。**但 `ContextWindow` 保留不变。** 因为这些状态都是「会话级的」，换会话就必须换掉。」
- **「为什么需要重置 Hook 的 once 集合？」** → 「因为 `only_once: true` 的 Hook 是「每个会话触发一次」——比如 SessionStart 时打一条欢迎语。恢复新会话后应该能再触发一次，所以集合要清空。」

---

## Q5.15 ⭐ 30 天清理为什么放 goroutine？

**面试官想听什么**：启动性能意识。

**口述答案**：

> 「因为它要**遍历目录 + 递归删目录**，是纯 IO 操作，不该拖慢启动。而且它失败也无所谓（打条日志就行），所以 fire-and-forget 是合适的。
>
> **它不是「定时任务」而是「每次启动跑一次」**——如果一个用户启动一次之后挂 40 天不关，那 40 天里不会清理。**但对终端工具这个场景够用**——它的生命周期就是一次使用。
>
> **实现上有个细节：清理只处理「新格式」的会话目录 ID**（能解析出时间戳的那种）：
> ```go
> 		t, err := compact.ParseSessionTime(id)
> 		if err != nil { continue }                    // 旧格式跳过
> 		if now.Sub(t) > maxAge { os.RemoveAll(sessionsDir + "/" + id) }
> ```
> **为什么把时间编进目录名？** 因为这样清理和排序都**不需要 stat 每个目录**——直接从目录名解析。而且「旧格式跳过」是刻意的向后兼容：老版本的会话 ID 格式解析不出来，**不会被误删**。」

**备注讲解**：

```go
	// --- ch09: 后台清理过期会话 ---
	sessionsDir := filepath.Join(root, ".mewcode", "sessions")
	go func() {
		if err := session.CleanExpired(sessionsDir, 30*24*time.Hour); err != nil {
			fmt.Fprintf(os.Stderr, "[session] 过期会话清理失败: %v\n", err)
		}
	}()
```

**目录名解析**：

```go
// ParseSessionTime 从新格式 session ID 中解析出时间戳。
// 新格式前 15 位为 "YYYYMMDD-HHMMSS"。
// 旧格式（如 "<unix_ts>-<hex>"）无法解析，返回 error。
func ParseSessionTime(sessionID string) (time.Time, error) {
	if len(sessionID) < 15 { return time.Time{}, fmt.Errorf("session ID 太短，非新格式: %s", sessionID) }
	timeStr := sessionID[:15] // "YYYYMMDD-HHMMSS"
	t, err := time.ParseInLocation("20060102-150405", timeStr, time.Local)
	...
}
```

**三个小问题可以主动提**：

1. **`sessionsDir + "/" + id` 用了字符串拼接而不是 `filepath.Join`**——在 Windows 上也能工作（因为 Windows 接受 `/`），但不规范。
2. **清理和 `ListSessions` 都用 `ParseSessionTime` 判断「是否新格式」，但它们对同一个目录可能结论不同**——`ListSessions` 还额外要求 `conversation.jsonl` 存在。所以一个「有合法 ID 名但没有 jsonl」的目录会被 `CleanExpired` 删掉，`ListSessions` 却看不到它。**这个不一致不算 bug（都是无用目录），但语义上应该统一。**
3. **没有并发保护**：如果用户正在 `ListSessions`（读操作），清理 goroutine 可能同时 `RemoveAll`。**理论上存在竞态**（读到一半目录被删）。实际上概率极低（清理在启动早期、列表要用户主动触发），而且失败也只是「列表少一项」。**但严格说应该加个锁或者把清理推迟到列表之后。**

---

# 第三部分：上下文压缩

---

## Q5.16 ⭐⭐⭐ 为什么要做两层压缩？一层不够吗？

**面试官想听什么**：⚠️ **这是压缩部分最核心的问题**。他要看你是不是理解「成本 / 信息损失 / 触发时机」这三者的权衡。

**口述答案**（背熟）：

> 「因为**两层的成本和收益完全不同，应该分开触发**。
>
> **第一层是「纯本地的、无损的」**：把超大的工具结果**落盘**，历史里只留一段头部预览和路径指引。它**不调用 LLM**，所以零 token 成本、零延迟；而且信息没丢——模型需要的时候可以按路径读回来。
>
> **第二层是「LLM 摘要」，有成本和有损**：要把整段历史发给模型做摘要，**摘要请求自己也要花 token、要几秒钟**；而且摘要必然有信息损失。
>
> 所以顺序是：**先用便宜的手段**。每轮我都是先跑第一层、**重新估算 token**，如果这样就已经降到阈值以下，就**根本不需要触发第二层**。
>
> **举个实际例子**：一个会话里如果有一轮跑了个大测试，工具结果 300KB。光是第一层把它落盘，就能从 300KB 降到 2KB——省掉的可能就是几十轮对话的空间。**这种情况下让 LLM 去摘要整段历史，完全是浪费。**
>
> **反过来，如果只有一层（只有 LLM 摘要）**，那么每一轮都要把「跑测试的输出」也塞进摘要的输入里——**摘要请求会变得又慢又贵，而且那个测试输出在摘要里大概率会被压缩掉**（模型不觉得它重要）。但**它可能非常重要**（比如错误信息）。
>
> **所以两层是「先做无损的、零成本的，再做有损的、有成本的」——这是个性价比的排序问题。**」

**备注讲解**：

```go
func manageAuto(ctx context.Context, in ManageInput, out ManageOutput) (ManageOutput, error) {
	// a. 执行 layer1（必须先做，因为 layer1 节省的 token 需要反映在阈值判断里）
	layer1Out, _ := OffloadAndSnip(in.Conv.Messages(), in.Replacement, in.Session)
	in.Conv.ReplaceMessages(layer1Out)

	// b. 用 layer1 之后的 updatedMsgs 重算估算 token
	estTokens := EstimateTokens(in.UsageAnchor, layer1Out, in.AnchorMsgLen)

	// c. sanity check：context_window 过小时跳过自动摘要
	if in.ContextWindow <= SummaryReserve+AutoSafetyMargin { ... }

	threshold := in.ContextWindow - SummaryReserve - AutoSafetyMargin

	// d. 未达阈值或熔断中：仅 layer1 生效
	if estTokens < int64(threshold) || in.AutoTracking.Tripped() {
		out.AfterTokens = estTokens
		return out, nil
	}

	// e. 触发自动摘要
	newMsgs, _, afterTok, err := AutoCompact(ctx, in)
	...
}
```

**关键的三行注释**：`// a. 执行 layer1（必须先做，因为 layer1 节省的 token 需要反映在阈值判断里）`。

**这个顺序本身就是最重要的优化**——如果先判断阈值再压 layer1，那「其实 layer1 就够了」的场景会白跑一次 LLM 摘要。

**两层的对照表**：

| 维度 | 第一层（落盘） | 第二层（摘要） |
|---|---|---|
| 是否调 LLM | ❌ | ✅ |
| 信息损失 | 无损（可读回） | 有损 |
| 触发条件 | 单条 > 50KB 或单条 tool 消息聚合 > 200KB | 估算 > 窗口 − 33000 |
| 触发频率 | 每轮都跑 | 超阈值才跑 |
| 恢复手段 | 按路径读原文 | 恢复三段 + 「不要猜」提示 |
| 幂等性 | 有账本（决策不可翻转） | 重写历史 |

---

## Q5.17 ⭐⭐ 第一层为什么不用 LLM，而用「落盘 + 预览」？

**面试官想听什么**：你对「信息损失」的敏感度。

**口述答案**：

> 「因为对一个**工具结果**来说，丢弃或者摘要它是愚蠢的——**它本来就在磁盘上（或者能重新拿到），只是它的完整内容太长**。
>
> 我有三个方案可选，前两个都不好：
> **① 直接丢弃**：模型彻底失去这个信息。如果是搜索结果、命令输出这类**不可重现**的内容（比如 `go test` 的输出、某个动态查询的结果），重新拿到的可能不一样。**而且模型不知道自己丢了东西。**
> **② 直接截断**：模型会**以为**截断后的就是全部，基于残缺信息推理。**这是最危险的一种——它不知道自己在猜。**
> **③ 落盘 + 预览 + 明确指引**（我选的）：信息没丢（可以按路径读回），同时**明确告诉模型「你看的是预览，要看全文请读这个路径，不要凭预览猜」**。
>
> 所以替换后的内容是：
> ```
> [content offloaded] original size: 123456 bytes
> [saved to] /path/to/.mwecode/sessions/<id>/tool-results/<tool_use_id>
> [head preview]
> <前 20 行 / 前 2048 字节>
>
> 完整内容已保存到上述路径，如需查看请用文件读取工具读取该路径，不要凭头部预览猜测全文
> ```
>
> **最后那句话是刻意写的**——它是**提示工程的一部分**。因为模型的默认行为是「看到内容就当作全部」，不告诉它，它就会基于那 20 行做判断。**这个设计的本质是「无损降级」——用一次额外的工具调用换取信息的完整性。**」

**备注讲解**：

```go
func buildPreview(originalBytes int, head, spillPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[content offloaded] original size: %d bytes\n", originalBytes)
	fmt.Fprintf(&b, "[saved to] %s\n", spillPath)
	b.WriteString("[head preview]\n")
	b.WriteString(head)
	b.WriteString("\n\n完整内容已保存到上述路径，如需查看请用文件读取工具读取该路径，不要凭头部预览猜测全文")
	return b.String()
}
```

```go
// headPreview 取 content 的前 20 行或前 2048 字节中的较短者。
func headPreview(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > previewHeadLines { lines = lines[:previewHeadLines] }    // 20 行
	head := strings.Join(lines, "\n")
	if len(head) > previewHeadBytes { head = head[:previewHeadBytes] }      // 2048 字节
	return head
}
```

**「取两者中较短者」**——两个上限都生效。

**落盘是幂等的**：

```go
// spillSingle 把单条 tool_result 内容写入 SpillDir/<tool_use_id>。
// 幂等：文件已存在则不重写、不报错。
func spillSingle(session *SessionContext, toolUseID, content string) error {
	path := filepath.Join(session.SpillDir, toolUseID)
	if _, err := os.Stat(path); err == nil { return nil }      // 已存在 → 跳过
	return os.WriteFile(path, []byte(content), 0o644)
}
```

**用 `tool_use_id` 做文件名**——因为它天然唯一（provider 生成的），而且能跟历史里的 `ToolCall.ID` 对应起来。

**可能的追问**：
- **「如果模型不去读那个路径呢？」** → 「那就跟丢了一样。这是我这个设计的**赌注**——我赌「明确告诉模型路径」比「直接丢弃」好。**实际上这是可验证的：可以统计「替换后模型读回落盘文件的比率」。如果比率很低，说明这个设计没达到目的，那还不如换个策略（比如保留摘要而不是预览）。**
>
- **「为什么落盘到会话目录而不是系统临时目录？」** → 「因为**生命周期对齐**。会话目录被清理（30 天）时，`tool-results/` 作为子目录会被一起删掉——**不需要单独管理清理逻辑**。如果放到 `/tmp`，就要处理「临时目录被系统清理了但历史里还引用着它」的情况。」

---

## Q5.18 ⭐⭐⭐ 「替换决策账本」解决什么问题？

**面试官想听什么**：⚠️ **这是整个压缩设计里最精妙的一处**。答出来会让人印象深刻。

**口述答案**：

> 「它解决的是「**同一份历史在不同轮次里内容不一致**」的问题。
>
> **先说问题是什么**：`ManageContext` 是**每一轮都调用**的。如果每次都用「当前字节数」重新决策要不要落盘，会出现这种情况：
>
> - 第 5 轮：结果 60000 字节 > 50000 阈值 → 落盘替换，历史里变成 2KB 预览
> - 第 6 轮：历史里这条现在是 2KB → 不大于阈值 → 决策为「保留」→ **但它已经不是原文了**
>
> 结果就是：**这条消息的内容在轮次之间来回跳变**。危害有两个：
>
> **① 破坏 Prompt Cache。** 前缀变了，后面全部失效——本来想省钱，结果更贵。
>
> **② 更要紧的是「模型的视野在轮与轮之间悄悄变化了」**。它上一轮看到的完整内容，这一轮变成了预览——**模型会困惑，甚至基于预览做出错误判断。**
>
> **我的解法是账本：决策一次，永久生效。** 第一次决策时把结论（包括替换后的预览字符串）记下来，后面每轮直接取账本结果。
> ```go
> type ContentReplacementState struct {
> 	mu           sync.Mutex
> 	seenIds      map[string]struct{}
> 	replacements map[string]string
> }
> ```
> `seenIds` 记录已决策的 id，`replacements` 存替换后的内容。**同一个 id 一旦进入 `seenIds` 就不可翻转**（注释原话）。
>
> **`DecideOnce` 还是个原子操作**：
> ```go
> // DecideOnce 在持锁状态下完成"查账本 → 决策 → 写账本"原子操作。
> func (s *ContentReplacementState) DecideOnce(id, original string, decide func() (decision, preview string)) string
> ```
> **它的回调是在持锁时执行的**——这不常见（通常我们会避免在持锁时跑用户回调），但这里**必须这样**才能保证「查/决策/写」三步不可分割。而回调里的操作（写文件 + 拼字符串）都是本地的，没有重入风险。
>
> **还有一个「skip」状态**：落盘失败时返回 `skip`，**不写账本**——这样下一轮可以重试。如果写失败也记成「已决策」，内容会永远保持原样且再也不会尝试压缩，**最终爆窗口**。」

**备注讲解**：

```go
// DecideOnce 在持锁状态下完成"查账本 → 决策 → 写账本"原子操作。
// decide 回调在持锁时调用，返回 (decision, preview)：
//   - "kept": 写 seenIds，不写 replacements，返回原 content
//   - "replaced": 写 seenIds + replacements，返回 preview
//   - "skip": 不写账本，返回原 content（下一轮可重试）
//
// 若 id 已 Seen：直接返回账本存量结果。
func (s *ContentReplacementState) DecideOnce(id, original string, decide func() (decision, preview string)) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, seen := s.seenIds[id]; seen {
		if replaced, ok := s.replacements[id]; ok { return replaced }
		return original
	}

	decision, preview := decide()
	switch decision {
	case "replaced":
		s.seenIds[id] = struct{}{}
		s.replacements[id] = preview
		return preview
	case "kept":
		s.seenIds[id] = struct{}{}
		return original
	default: // "skip"
		return original
	}
}
```

**调用处的写法有点绕**（可以主动指出这是个可以简化的地方）：

```go
			if needSpill {
				state.DecideOnce(it.id, it.content, func() (string, string) {
					if err := spillSingle(session, it.id, it.content); err != nil {
						return "skip", ""
					}
					spillPath := filepath.Join(session.SpillDir, it.id)
					preview := buildPreview(it.size, headPreview(it.content), spillPath)
					return "replaced", preview
				})
				// 账本已写入，Content 需要在 DecideOnce 之后额外更新
				// DecideOnce 返回 preview 或原 content
				// 因为已经写入账本，再次调用 DecideOnce 会短路返回 preview：
				tr.Content = state.DecideOnce(it.id, it.content, func() (string, string) {
					return "kept", ""
				})
```

**这里 `DecideOnce` 被调了两次**——第一次做决策并写账本，第二次靠账本短路来取回结果。**这是因为 `DecideOnce` 的返回值被丢弃了**。

**更干净的写法**是直接用返回值：

```go
	tr.Content = state.DecideOnce(it.id, it.content, func() (string, string) { ... })
```

**「调用两次」虽然结果正确**（第二次必然短路返回账本值），但**可读性差、而且有一次不必要的锁获取**。**这是个可以直接指出来的代码改进点。**

---

## Q5.19 ⭐⭐ 摘要 prompt 为什么分「分析 + 正式」两阶段？

**面试官想听什么**：提示工程的实战理解。

**口述答案**：

> 「因为 **Chain-of-Thought 在压缩任务上同样有效**。
>
> Prompt 是这样的：
> ```
> ## Phase 1: Analysis (will be discarded)
> Write your analysis draft inside <analysis> tags. Think through:
> - What was the user trying to accomplish?
> - What key decisions were made?
> - What files were modified or read?
> - What errors were encountered and how were they fixed?
> - What is the current state of work?
>
> ## Phase 2: Formal Summary (will be kept)
> Write the formal summary inside <summary> tags. Follow these 9 sections exactly:
> ...
> ```
> **Phase 1 的输出我们不保留**——它只是给模型一个「先自由梳理」的机会。让我自己实测对比过：**先分析再产出的摘要，质量明显比直接要结构化的高。**
>
> **「9 个固定小节」也是刻意的，不是「让模型自由总结」**。因为自由总结会**漏掉关键维度**——典型的漏项是「用户的原始措辞」和「待办任务」。固定小节是**用结构强制覆盖那些容易被忽略的信息**。
>
> 九个是：
> 1 主要请求和意图、2 关键技术概念、3 文件和代码段、4 错误和修复、5 问题解决过程、**6 所有用户消息原文**、7 待办任务、**8 当前工作（最详细）**、9 可能的下一步。
>
> **第 6 节特别重要**：用户的原始措辞携带了摘要无法替代的信息——比如「我不要那个方案，我要另一个」里的「那个」指代什么。所以我要求**原文保留**。
>
> **第 8 节标了「最详细」**：因为恢复后会话要继续，模型最需要知道的就是「现在进行到哪一步了」。**这一节的质量直接决定恢复后的续接质量。**」

**备注讲解**：

```go
const summaryInstruction = `You are summarizing a coding agent conversation. Output in two phases.
...
IMPORTANT:
- Do NOT call any tools. Output plain text only.
- In section 6, preserve every user message in its original language, in chronological order.
- Section 8 should be the most detailed — describe exactly what is being worked on right now and at which step the work stopped.
- Write in the same language as the user's messages.`
```

**三条 IMPORTANT 各有用意**：
- **不许调工具**——因为摘要请求我传的是 `Tools: nil`，但如果不管，模型可能尝试幻觉一个工具调用。
- **第 6 节保留原文和顺序**——防止模型「归纳」用户消息。
- **用什么语言写**——因为**小节标题是中文的**，如果不指定，用户全程说英文时会得到中英混杂的摘要。

**摘要请求本身不传工具**：

```go
func summarizeOnce(ctx context.Context, in ManageInput, msgs []llm.Message) (string, error) {
	req := llm.Request{
		Messages: BuildSummaryPrompt(msgs),
		Tools:    nil, // 摘要不传工具
	}
	...
}
```

**为什么**：① 摘要不需要工具；② 传了工具会**多花 token**（工具定义也是输入）；③ 减少模型跑偏的可能。

**提取摘要的降级策略**：

```go
func ExtractSummary(raw string) string {
	start := strings.Index(raw, "<summary>")
	if start == -1 { return raw }              // ← 找不到标签，降级用原文
	start += len("<summary>")
	end := strings.Index(raw[start:], "</summary>")
	if end == -1 { return raw }
	return strings.TrimSpace(raw[start : start+end])
}
```

**降级很重要**：如果模型没按格式输出（比如拒绝用标签、或者被截断了），**直接把原文当摘要用**，而不是报错放弃压缩。**压缩失败会导致爆窗口，比摘要质量差严重得多。**

**可能的追问**：
- **「如果摘要被截断了呢？（比如撞到 max_tokens）」** → 「会拿到一个不完整的 `<summary>` 块——`IndexOf("</summary>")` 找不到 → 降级返回**整个原始输出**（包括 `<analysis>` 那段）。这种情况下摘要里会混着分析草稿。**这是个可以改进的地方：至少可以只取 `<summary>` 之后的内容，即使没有闭合标签。**」

---

## Q5.20 ⭐⭐ 压缩后的「恢复三段」各解决什么问题？

**面试官想听什么**：你对「摘要必然失真」这件事的理解深度。

**口述答案**：

> 「先讲前提：**摘要是模型对另一个模型输出的压缩，必然有信息损失**。所以我不能只给摘要，还要补上三段恢复信息：
>
> **第一段：最近读过的文件快照。** 取最近 5 个读过的文件，每个最多 5000 token。因为**摘要里说的「修改了 handler.go」不如文件的实际内容有用**。而且这些快照是 agent 主循环在每次 `read_file` 成功后记录的**纯净字节**——不是带行号的工具输出。
>
> **第二段：当前可用工具列表。** 因为摘要产出的时候是过去，模型可能忘了自己现在有什么工具可用。**而且工具集是随模式变的**（Plan 模式只有只读工具），所以这段必须用**当前这轮的真实工具集**渲染。
>
> **第三段：边界提示。** 这是最重要的一段：
> ```
> ## 重要提示
> 请注意：以上摘要仅用于提供上下文脉络。如果你需要获取文件的完整内容、确切的错误信息、或用户的原始措辞，请使用文件读取工具重新读取对应路径。不要依据摘要中的描述做代码推断或猜测。
> ```
> **它在明确告诉模型「摘要不可信，要读原文」**。这跟第一层替换的预览里那句「不要凭头部预览猜测全文」是同一个思路——**明确标注信息的可信度，而不是让模型自己猜**。」

**备注讲解**：

```go
func BuildRecoveryAttachment(snapshot []FileReadRecord, toolDefs []llm.ToolDefinition) string {
	var b strings.Builder
	// 第一段：最近读过的文件快照
	b.WriteString("## 最近读过的文件\n")
	// 第二段：当前可用工具列表
	b.WriteString("\n## 当前可用工具\n")
	// 第三段：边界提示
	b.WriteString("\n")
	b.WriteString(boundaryNotice)
	return b.String()
}
```

**注释里有一句关键约束**：

```go
// toolDefs 必须与下一次 Stream 请求的 Request.Tools 来自同一引用。
```

**为什么必须同一引用？** 如果恢复段说「你可以用 write_file」，但实际这一轮的工具集里没有（因为切到 Plan 模式了），模型会请求一个不存在的工具。**所以恢复段的工具列表必须和实际发出去的工具列表一模一样。**

**这就是为什么压缩必须在「取工具集之后」执行**（前面 Q3.20 讲过）。

**文件快照的来源**（`recordFileReads`）：

```go
// recordFileReads 在工具结果回灌前记录 ReadFile 调用的纯净字节。
func (a *Agent) recordFileReads(calls []llm.ToolCall, results []llm.ToolResult) {
	if a.runtime == nil || a.runtime.Recovery == nil { return }
	for i := range calls {
		if calls[i].Name != "read_file" { continue }
		if i >= len(results) || results[i].IsError { continue }
		var args map[string]any
		if err := json.Unmarshal(calls[i].Input, &args); err != nil { continue }
		path, ok := args["path"].(string)
		if !ok || path == "" { continue }
		absPath, err := filepath.Abs(path)
		if err != nil { continue }
		b, err := os.ReadFile(absPath)          // ← 重新读文件，拿纯净字节
		if err != nil { continue }
		a.runtime.Recovery.RecordFile(absPath, string(b))
	}
}
```

**注意 `os.ReadFile(absPath)` 这行**——**它是重新读一遍文件，而不是从工具结果里取**。为什么？因为 `read_file` 工具的返回**带行号前缀**（`fmt.Sprintf("%6d\t%s", i+1, lines[i])`），带行号的内容塞进恢复段会让模型困惑（它会以为行号是文件内容的一部分）。**重读一遍拿到纯净字节**，代价是多一次文件读取。

**⚠️ 这里有个真实的局限**：如果文件在「Agent 读它」和「压缩触发」之间被改过了，恢复段里的是**新内容**，而历史里说的是**旧内容**。这可能造成不一致。**更严谨的做法是记录工具返回时的文件内容**（去掉行号前缀），而不是重读。

**快照必须「拍一次」**（代码注释）：

```go
	// 入口拍快照，整个 runSummary 生命周期只用这一份
	recoverySnapshot := in.Recovery.Snapshot()
```

**为什么**：摘要是耗时操作（几秒到几十秒）。这期间主循环别的地方可能还在调 `RecordFile`。不拍快照的话，恢复段的内容会在渲染过程中变化，**变成一份「东拼西凑」的混合状态**。

**`Snapshot()` 按时间戳倒序**：

```go
	sort.Slice(list, func(i, j int) bool { return list[i].Timestamp.After(j.Timestamp) })
```

所以「最近 5 个」是**真正最近**的。

---

## Q5.21 ⭐⭐ 摘要请求自己也会超长，怎么办？

**面试官想听什么**：你会不会想到「兜底方案本身也需要兜底」。

**口述答案**：

> 「这是个很真实的死锁场景：**历史长到连摘要请求本身都超窗口**。这时候摘要永远发不出去，压缩永远失败，用户彻底卡死。
>
> 我的解法是**分组丢弃 + 分段重试**：
> ```
> 前 3 次：每次丢最旧的一组用户回合
> 之后：按 20% 的比例丢（至少丢 1 组）
> 直到成功或全部丢光
> ```
> ```go
> func ptlRetry(ctx context.Context, in ManageInput, msgs []llm.Message) (string, error) {
> 	groups := groupByUserTurn(msgs)
>
> 	for retry := 0; retry < ptlRetryLimit && len(groups) > 1; retry++ {
> 		groups = groups[1:]                      // 丢最旧 1 组
> 		if summary, err := summarizeOnce(ctx, in, flattenGroups(groups)); err == nil { return summary, nil }
> 	}
>
> 	for len(groups) > 1 {
> 		drop := int(math.Ceil(float64(len(groups)) * ptlDropPercentage))   // 20%
> 		...
> 	}
> 	return "", context.DeadlineExceeded          // 全部丢光
> }
> ```
>
> **分段策略的智慧在「先小步试探、再大步跨越」**：前 3 次每次只丢一组是**保守的**（尽量多保留信息）；如果还不行，说明差得远，就按 20% 大步丢，快速收敛。**如果一上来就按比例丢，可能丢掉大量本来不需要丢的内容。**
>
> **分组依据是「用户回合」而不是「单条消息」**——因为一条 assistant 的 `tool_use` 和后续的 `tool_result` 必须成对处理，拆开会产生非法历史。**这个约束在压缩里到处都要注意。**」

**备注讲解**：

```go
// groupByUserTurn 按"用户消息 → 后续 assistant/tool 往返"分组。
func groupByUserTurn(msgs []llm.Message) [][]llm.Message {
	var groups [][]llm.Message
	var cur []llm.Message
	for _, m := range msgs {
		if m.Role == llm.RoleUser && len(cur) > 0 {
			groups = append(groups, cur)
			cur = nil
		}
		cur = append(cur, m)
	}
	if len(cur) > 0 { groups = append(groups, cur) }
	return groups
}
```

**这就是「按用户回合切分」**——每次遇到 user 消息就开一组。

**注意一个细节**：`RoleTool` 消息在 Anthropic 侧对应的是 `user` 角色的消息（带 `tool_result`），但在**我的协议无关模型里它们是 `RoleTool`**。所以 `groupByUserTurn` 只按 `RoleUser` 切——**不会把工具结果误判成新的用户回合**。这个细节是对的。

**返回的 sentinel 错误**：

```go
	return "", context.DeadlineExceeded // sentinel: 全部丢光
```

**用 `context.DeadlineExceeded` 当 sentinel 其实有点不严谨**——它是个语义无关的错误，用在这里只是「表示失败」。**更好的做法是定义一个包级 `ErrCompactImpossible`**。可以主动指出。

---

## Q5.22 ⭐⭐ 熔断器为什么需要？为什么手动路径不计入？

**面试官想听什么**：对「失败重试可能变成慢性病」的理解。

**口述答案**：

> 「**熔断解决的是「反复失败拖慢一切」的问题。**
>
> 场景：假设模型服务挂了，或者 API key 余额不足。如果不熔断，**每一轮循环都会尝试一次摘要**——多花一次失败的请求、多几秒延迟，而且永远卡在「压缩不成功 → 下一轮又试」的状态。用户看到的现象是「每一轮都莫名卡十秒」，而根本原因是压缩在反复失败。
>
> 我的熔断是**连续 3 次失败就跳闸**，之后自动路径彻底不再尝试压缩。用户会看到「上下文满了报错」，而不是「每轮都卡」。
>
> **为什么是 3 次不是立即熔断？** 因为失败可能是**瞬时的**——网络抖动、限流、临时 5xx。立即熔断意味着一次抖动就永久失去自动压缩能力。3 次大概覆盖 3 轮循环、30 秒左右的时间窗——**如果还在失败，说明不是抖动而是真故障。**
>
> **为什么手动和紧急路径不计入熔断？** 这是关键设计：
> ```go
> // ForceCompact 手动/紧急摘要：不走熔断器，失败不计入熔断计数。
> ```
> 两个理由：
> - **手动 `/compact`**：如果用户手动压失败了，他应该能**再试一次**——不能被之前的失败计数挡住。而且手动是一个明确的用户意图。
> - **紧急压缩**：这是收到 `ErrPromptTooLong` 之后的**最后一道防线**。如果因为熔断跳闸而不执行，**用户就彻底没救了**（请求反复被拒，压缩又不敢跑，死锁）。
>
> **所以熔断只约束「自动的、机会主义的」压缩**——它防的是「无谓的重复尝试」，不是「必要的兜底」。**这是个「熔断的适用范围」问题，不是「熔断的值」问题。**」

**备注讲解**：

```go
// 自动摘要熔断常量（包内私有）
const maxConsecutiveAutoCompactFailures = 3
```

```go
// AutoCompact 自动摘要：成功后清零失败计数；整轮失败累加失败计数。
func AutoCompact(ctx context.Context, in ManageInput) ([]llm.Message, int64, int64, error) {
	beforeTok := in.EstimatedToken
	newMsgs, err := runSummary(ctx, in)
	if err != nil {
		in.AutoTracking.RecordFailure()       // ← 累加
		return nil, beforeTok, 0, err
	}
	in.AutoTracking.RecordSuccess()           // ← 清零
	afterTok := EstimateTokens(0, newMsgs, 0)
	return newMsgs, beforeTok, afterTok, nil
}

// ForceCompact 手动/紧急摘要：不走熔断器，失败不计入熔断计数。
func ForceCompact(ctx context.Context, in ManageInput) ([]llm.Message, int64, int64, error) {
	beforeTok := in.EstimatedToken
	newMsgs, err := runSummary(ctx, in)
	if err != nil {
		return nil, beforeTok, 0, err             // ← 不记录失败
	}
	afterTok := EstimateTokens(0, newMsgs, 0)
	return newMsgs, beforeTok, afterTok, nil
}
```

```go
// Tripped 熔断器是否跳闸。
func (a *AutoCompactTrackingState) Tripped() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ConsecutiveFailures >= maxConsecutiveAutoCompactFailures
}
```

**跳闸后的行为**（`manageAuto`）：

```go
	// d. 未达阈值或熔断中：仅 layer1 生效
	if estTokens < int64(threshold) || in.AutoTracking.Tripped() {
		out.AfterTokens = estTokens
		return out, nil
	}
```

**注意熔断后 layer1 仍然继续跑**——因为它不花钱、不失败。

**⚠️ 一个值得指出的局限**：**熔断跳闸后没有「恢复」机制**。一旦连续 3 次失败，**整个会话内自动压缩就永久失效了**（`ResetForNewSession` 会重建 `AutoTracking`，但那是换会话）。如果失败的原因是临时的（比如限流恢复了），用户只能手动 `/compact`。

**更好的设计是「半开状态」**：跳闸后隔 N 轮再试一次，成功就恢复。**这是标准的 circuit breaker 三态模型（closed / open / half-open）**，我实现了前两态。**被问到可以直接说这个。**

---

## Q5.23 ⭐⭐ Token 估算为什么用「锚点 + 增量」？

**面试官想听什么**：你对「精度」和「成本」的取舍。

**口述答案**：

> 「因为**误差不能累积**。
>
> **反面做法**：每轮都在上一轮的估算值上叠加新增的字数——那样误差会**指数放大**，10 轮之后完全不准，要么过早压缩（浪费钱），要么过晚压缩（请求被拒）。
>
> **我的做法是「锚点 + 增量」**：
> - **锚点**用**上一次 provider 返回的真实 usage**——这是精确值，不是估算。
> - **增量**只有锚点之后新增的消息，按「字符数 / 3.5」估算。
>
> 所以**每一轮都被真实值重新校准**，估算误差只覆盖「本轮新增的那几条消息」——这个误差量级很小。
>
> **一个容易漏的细节：锚点要把 4 个 token 字段全加起来。**
> ```go
> func UsageAnchor(u *llm.Usage) int64 {
> 	return u.InputTokens + u.OutputTokens + u.CacheWrite + u.CacheRead
> }
> ```
> **`CacheRead` 特别重要**——它代表被缓存的部分，**但这部分 token 依然占上下文窗口**。如果只算 `InputTokens`，命中缓存时会严重低估，导致压缩触发太晚。
>
> **另一个细节**：摘要请求的 usage **不更新锚点**。因为摘要是另一个对话（只有一条 user 消息），它的 usage 跟主对话的上下文长度无关。
>
> **关于系数 3.5**：这是经验值。中文场景会低估（中文一个字符可能对应更少的 token 但字节数更多）。**我不打算做精确的分词器**——那要给每种模型捆绑一个 tokenizer，模型换代就得跟着升级。**我的策略是「用安全余量吃掉误差」**：自动触发留了 13000 token 的余量，就是为了这个。」

**备注讲解**：

```go
// EstimateTokens 锚定最近一次 provider usage + 之后新增消息的字符增量。
// 返回 anchor + ceil(sum(chars(allMsgs[anchorMsgLen:])) / estimateCharsPerToken)
func EstimateTokens(anchor int64, allMsgs []llm.Message, anchorMsgLen int) int64 {
	if anchorMsgLen < 0 { anchorMsgLen = 0 }
	var tail []llm.Message
	if anchorMsgLen < len(allMsgs) { tail = allMsgs[anchorMsgLen:] }
	chars := messageChars(tail)
	return anchor + int64(math.Ceil(float64(chars)/estimateCharsPerToken))
}
```

```go
const estimateCharsPerToken = 3.5
```

**`messageChars` 统计三类内容**：

```go
func messageChars(msgs []llm.Message) int {
	total := 0
	for i := range msgs {
		m := &msgs[i]
		total += len(m.Content)
		for _, tc := range m.ToolCalls { total += len(tc.Input) }
		for _, tr := range m.ToolResults { total += len(tr.Content) }
	}
	return total
}
```

**三类都要算**——`ToolCalls[].Input`（JSON 参数）和 `ToolResults[].Content`（工具输出）经常是最大的部分。**漏掉任何一个都会严重低估。**

**锚点更新的时机**：

```go
			// 主对话路径完成后更新锚点
			if usage != nil {
				a.runtime.UpdateAnchor(compact.UsageAnchor(usage), conv.Len())
			}
```

**注释强调「主对话路径」**——因为摘要请求也会返回 usage，但那个不能用来更新锚点。

**`ResetAnchor()` 在紧急压缩后必须调用**：

```go
				a.runtime.ResetAnchor()
				est2 := compact.EstimateTokens(0, conv.Messages(), 0)
```

因为**历史被重写了**，之前的锚点对应的消息长度完全对不上，继续用会算出天文数字。

**⚠️ 三个已知问题**：

1. **`resume.go` 里另有一个 `estimateTokens`，系数是 `chars * 0.25`（即 4 字符/token）**，和 `compact` 的 3.5 不一致。方向上是偏保守（估得多、更容易触发压缩），不会漏压，但**两处应该收敛成一个共享常量**。
2. **字符数是字节数（`len(string)`），不是字符数**。中文一个字 3 字节，所以中文的「字符数」被高估了 3 倍——**这实际上让估算偏保守**（更容易触发压缩），方向是安全的。但这也意味着**中文场景下 3.5 这个系数会严重低估 token 数**（因为按字节算的 chars 更大，而真实 token 数并没那么多）……

   更精确地说：`len(s)` 是字节数。对英文，字节数 ≈ 字符数；对中文，字节数 = 3 × 字符数。而真实 token 数是「按语言学单位」算的，中文大约 0.6–1 token/字。所以对中文：`字节数 / 3.5` ≈ `3 × 字符数 / 3.5` ≈ `0.86 × 字符数`——而真实值约 `0.6 × 字符数`。**结果是高估**，方向安全。
3. **空的历史（`anchorMsgLen >= len(allMsgs)`）会返回 `anchor` 而不含新增**——如果历史被压缩后消息数减少了，`anchorMsgLen` 可能大于当前长度，`tail` 为空。**这时候估算值就是旧锚点，可能偏高。** 这就是为什么压缩后要 `ResetAnchor`。

---

## Q5.24 ⭐⭐ 压缩会破坏 Prompt Cache 吗？你怎么权衡？

**面试官想听什么**：⚠️ **这是个非常锋利的题**——它把「缓存」和「压缩」两个机制放在一起看，能答出来说明你系统性思考过。

**口述答案**：

> 「**会破，而且是严重破坏。**
>
> 因为 Prompt Cache 是**按前缀命中**的。压缩会**重写整个历史**——前缀彻底变了，**所有缓存断点之后的全部失效**。
>
> 具体代价：假设压缩前有 170K token 的历史，其中大部分是缓存命中的。压缩后变成 10K 摘要 + 恢复段。**那 170K 的缓存全废了**，下次请求要重新 prefill 全部内容——又慢又贵。
>
> **我的权衡是「尽量晚触发、尽量少触发」**：
> - 阈值设在 `ContextWindow − 33000`，也就是**用到 83.5% 才压**。留这么多余量，一方面给摘要请求本身的空间，另一方面就是**尽量减少压缩次数**。
> - **第一层压缩不破坏缓存**（因为它带账本、决策不可翻转），所以能扛的部分都由它扛。
> - 压缩之后，新的前缀（摘要 + 恢复段 + 近期原文）**是稳定的**，后续轮次又能重新累积缓存。所以这是一次性损失，不是持续的。
>
> **一个可以改进的方向**：现在的摘要完全重写了历史。**一个更缓和的方案是「分层摘要」——比如把最老的 50% 压成摘要，保留中间 30% 的原文，最近 20% 也保留**。这样前缀的一部分还能命中。**代价是压缩效果变差**（保留的原文还是占空间）。
>
> **但有个反直觉的点**：`pickRecentTail` 已经保留了近期原文（≥10000 token 且 ≥5 条消息），所以实际上我的压缩**不是全量重写**——是「前缀换成摘要 + 保留尾部原文」。**而尾部原文和压缩前是同一份内容，理论上那部分还能命中缓存**——只要摘要段的长度稳定。
>
> **但摘要段的长度每轮都可能变**（摘要文本长度不同），所以实际命中率还是不高。**要真正优化，需要让摘要段的内容稳定下来**——比如压缩后就不再重新摘要，直到下一次硬阈值。」

**备注讲解**：

**缓存失效的机制**：

```
压缩前：[Stable(断点)] [Env] [msg1..msg200]              ← 断点后全都缓存了
压缩后：[Stable(断点)] [Env] [摘要+恢复段] [msg195..msg200]  ← 断点后完全不同
```

**只有 `[Stable]` 这一段继续命中**——因为它在断点之前，字节没变。**这就是为什么「系统提示拆两段」这么重要**：即使历史全变，那 2–3K 的稳定段还一直命中。

**`pickRecentTail` 的保留策略**：

```go
// pickRecentTail 从消息尾部累加，满足 token 和条数两个下界后停止，再做配对修正。
// 两个下界都满足后才停手（"择宽"语义）。
func pickRecentTail(msgs []llm.Message) []llm.Message {
	...
	if estTokens >= recentKeepTokens && count >= recentKeepMessages { break }   // 10000 token / 5 条
```

**注意「择宽」这个说法**——两个条件都满足才停，意味着**实际保留的量是两个下界里更大的那个**。

**可能的追问**：
- **「如果不压缩，一直扩大 context window 呢？」** → 「模型窗口有硬上限（200K）。而且**即使窗口无限大，成本也是线性的**——每一轮都要为整个历史付输入 token 的钱。**所以压缩不只是「避免报错」，也是成本控制手段。**」
- **「怎么衡量压缩的收益？」** → 「可以量化：压缩省下的 token 数 × 单价 vs 摘要请求的成本 + 缓存失效的损失。**我在代码里已经采集了数据**——`CompactEvent` 里带 `Before` 和 `After` 的 token 数，TUI 会显示「已压缩，token 从 X 降至 Y」。**但缓存失效的损失我没量化**，这是个可以补的度量。」

---

## Q5.25 ⭐ 压缩后为什么要「重置锚点」？

**面试官想听什么**：状态一致性的敏感度。

**口述答案**：

> 「因为**锚点和历史的对应关系被破坏了**。
>
> 锚点是「某个时刻的真实 token 数 + 当时的历史长度」。`EstimateTokens` 的算法是「锚点值 + 锚点之后新增消息的字符数」。如果历史被压缩重写了，那么：
> - 「锚点之后的消息」这个概念完全失效——原来的 `msg150` 现在是完全不同的内容；
> - 如果压缩后长度变短，`anchorMsgLen` 可能大于当前长度，`tail` 为空切片，估算值就等于旧锚点——**而旧锚点可能远高于真实值**。
>
> 结果是：**估算严重偏高 → 下一轮又触发压缩 → 摘要又一次失败或浪费**。
>
> 所以压缩后必须 `ResetAnchor()`，让锚点归零，下一轮用纯粹的字符数估算（`EstimateTokens(0, msgs, 0)`）。**虽然这时的估算精度差一些，但至少不会偏高。** 而等这一轮 stream 结束拿到真实 usage 后，锚点就会被重新校准。」

**备注讲解**：

```go
				a.runtime.ResetAnchor()
				est2 := compact.EstimateTokens(0, conv.Messages(), 0)
				if est2 >= int64(cw-compact.ManualSafetyMargin) {
					emit(ctx, ch, Event{Err: sErr})
					ensureAssistantStep... // 实际是 ensureAssistantTail(conv, noticeStreamErr)
					return
				}
```

```go
// ResetAnchor 重置锚点（紧急压缩后使用）。
func (r *SessionRuntime) ResetAnchor() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.UsageAnchor = 0
	r.AnchorMsgLen = 0
}
```

**注释写得很明确**：「紧急压缩后使用」。

**⚠️ 但这里有个细节**：`ResetAnchor()` 只在**紧急压缩路径**被调用了。**自动压缩（`manageAuto`）和手动压缩（`manageManual`）之后没有重置锚点！**

让我核对一下：

```go
// manageAuto 里
	newMsgs, _, afterTok, err := AutoCompact(ctx, in)
	if err != nil { ... }
	in.Conv.ReplaceMessages(newMsgs)
	out.AfterTokens = afterTok
	return out, nil
```

**没有 `ResetAnchor`。** 而 `ManageContext` 的签名是 `(ctx, in ManageInput)`，它**拿到了 `in.Conv` 但没有 `SessionRuntime`**——所以它**没有能力**重置锚点。

**这意味着**：自动压缩后，锚点还是压缩前的值，`anchorMsgLen` 还是压缩前的长度。**下一轮的估算会偏高**（因为锚点大 + 尾部逻辑失效）。

**这是一个真实的缺陷**。实际影响：可能**连续触发压缩**（估算一直偏高），或者压缩过度。

**不过有个缓解**：每轮 stream 结束后都会 `UpdateAnchor(usage, conv.Len())`——所以**只要下一轮能正常发出请求**，锚点就会被真实值覆盖。所以这个缺陷的窗口只有「压缩后的那一轮」。而那一轮里如果又触发压缩，最多是多花一次摘要请求。

**如果被问到，可以这样答**：

> 「这里有个我事后发现的不一致：`ResetAnchor()` 只在**紧急压缩**路径被调用了，**自动压缩和手动压缩之后没有重置**。原因是 `ManageContext` 拿不到 `SessionRuntime`——它只有 `Conv`，没有权限改锚点。
>
> 后果是：压缩后的**那一轮**，估算值仍然基于旧锚点，可能偏高、甚至再次触发压缩。缓解机制是「每轮 stream 结束后都会用真实 usage 更新锚点」，所以窗口只有一轮。
>
> **正确的修法是把 `ResetAnchor` 提到 `Agent.Run` 里**——在 `ManageContext` 返回、且 `out.AfterTokens` 明显小于 `out.BeforeTokens`（说明真的压缩了）时重置。**判断依据应该是 `AfterTokens < BeforeTokens`，而不是 `Trigger` 类型**——因为自动路径也可能没压缩（未达阈值时 `AfterTokens == estTokens`）。」

**这段回答展示了「状态一致性」这个层面的思考**——非常能打。

---

## Q5.26 ⭐ 如果上下文永远压缩不下来怎么办？

**面试官想听什么**：极端情况的处理 + 终止性。

**口述答案**：

> 「我设计了**三级逐步放弃**：
>
> **第一级：摘要请求自己重试时逐步丢弃历史。** 前 3 次每次丢最旧一组用户回合，之后按 20% 比例丢，**直到全部丢光**。如果丢光了还是失败，返回 sentinel 错误。
>
> **第二级：紧急压缩后再检查一次。** 紧急压缩完成后，我重新估算：如果 `est2 >= ContextWindow - 3000`（只留 3000 余量），说明**压了还是不够**，那就**直接报错返回**，不再重试：
> ```go
> 				if est2 >= int64(cw-compact.ManualSafetyMargin) {
> 					emit(ctx, ch, Event{Err: sErr})
> 					ensureAssistantTail(conv, noticeStreamErr)
> 					return
> 				}
> ```
>
> **第三级：`emergencyRetried` 标志保证只重试一次。** 收到 `ErrPromptTooLong` 后压缩重发**只做一次**——如果重发还是超长，不再试第三次：
> ```go
> 			if sErr != nil && errors.Is(sErr, llm.ErrPromptTooLong) && !emergencyRetried {
> ```
> 如果没有这个标志，可能陷入「压缩 → 重发 → 超长 → 压缩 → ...」的无限循环。
>
> **还有一个 sanity check**：如果 `ContextWindow` 本身太小（`<= SummaryReserve + AutoSafetyMargin`，即 ≤33000），直接跳过自动摘要：
> ```go
> 	if in.ContextWindow <= SummaryReserve+AutoSafetyMargin {
> 		log.Printf("[compact] ContextWindow=%d 过小（≤%d），跳过自动 layer2", ...)
> 		out.AfterTokens = estTokens
> 		return out, nil
> 	}
> ```
> **这是防止配置错误导致的无意义循环**——如果用户配了一个 16000 窗口的模型，摘要请求的 20000 预留空间比窗口还大，永远不可能成功。**直接跳过比反复失败好。**
>
> **最终用户看到的是「请求出错」**——而不是程序卡死或者无限循环。**这是我在所有兜底逻辑里坚持的原则：宁可明确失败，也不要静默卡住。**」

**备注讲解**：

**三级放弃的代码位置**：

| 级别 | 代码位置 | 触发条件 |
|---|---|---|
| 摘要请求内重试丢组 | `layer2.go:ptlRetry` | 摘要请求本身 PTL |
| 压后仍超则放弃 | `agent.go:304-310` | `est2 >= cw - 3000` |
| 只重试一次 | `agent.go:283` 的 `!emergencyRetried` | 第二次 PTL |
| 窗口过小跳过 | `compact.go:manageAuto` | `cw <= 33000` |

**注意 `ManualSafetyMargin = 3000` 在紧急路径的使用**：

```go
// ManualSafetyMargin 手动触发 / 紧急压缩的安全余量：仅检查摘要请求本身能否塞下
ManualSafetyMargin = 3000
```

**注释解释了为什么是 3000 而不是 13000**：因为紧急压缩之后，历史已经是「摘要 + 恢复段 + 近期原文」了，估算相对准确（都是新内容），**不需要留那么大的余量吃估算误差**。

**可能的追问**：
- **「如果用户配的模型窗口真的很小，怎么用？」** → 「会自动跳过 layer2，只靠 layer1（落盘替换）。**这实际上会让会话能支持的长度受限**——因为 layer1 只能处理「工具结果太大」，不能处理「消息条数太多」。用户会发现用一会儿就报错。**更好的做法是在启动时校验配置，如果窗口小于某个阈值就给一个明确的警告。**」

---

➡️ 下一篇：`06-压力面-陷阱题与简历修正.md`
