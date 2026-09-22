# MewCode 模块分析 01：LLM 协议层（llm / config / conversation）

> 阅读范围（全部逐行读完）：
> `mewcode/internal/llm/provider.go`(107 行)、`anthropic.go`(285 行)、`openai.go`(230 行)、
> `mewcode/internal/config/config.go`(86 行)、`protocol_defaults.go`(15 行)、
> `mewcode/internal/conversation/conversation.go`(160 行)。
> 交叉验证的调用方：`internal/agent/agent.go`（`streamOnce`/主循环）、`internal/compact/layer2.go`、`cmd/mewcode/main.go`。

## 模块职责

把 Anthropic / OpenAI 两套互不兼容的流式协议收敛成**一个协议无关的 `Provider` 接口 + 一条 `StreamEvent` 事件流**，让上层的 ReAct 循环、压缩器、TUI 完全不感知具体供应商。

要点：

1. **协议无关的数据模型**：`Message`/`ToolCall`/`ToolResult`/`Usage`/`Request` 全部用本项目自己的 struct 表达，SDK 类型只出现在 `anthropic.go`/`openai.go` 两个文件内部，`provider.go` 不 import 任何 SDK（`agent` 包注释也强调“不 import SDK，保持协议无关”）。
2. **调用形态统一为「一次性 `Request` 进 → `<-chan StreamEvent` 出」**：`Provider.Stream(ctx, req) <-chan StreamEvent`（`provider.go:94`），内部起 goroutine、无缓冲 channel、`defer close(ch)`，消费端用 `for range` 吃干。
3. **五态事件语义**：`Text`（文本增量）/`ToolCalls`（工具调用，Done 之前一次性给出）/`Usage`（本轮 token 用量）/`Done`/`Err`，`Done` 与 `Err` 互斥且为终结事件（`provider.go:59-74` 注释显式定义）。
4. **配置层只做「解析 + 校验 + 默认值」三件事**：YAML → `Config.Providers`，`validate()` 强制 5 个必填字段，`EffectiveContextWindow()` 在未配置时按协议回退默认窗口（200k / 128k）。
5. **会话层只做「并发安全的历史容器」**：`Conversation` 用 `sync.Mutex` 保护 `[]llm.Message`，所有读方法深拷贝子切片，完全不关心 token、压缩、截断——压缩职责被切给 `internal/compact`。

## 关键类型与接口

### 1. 协议无关的错误哨兵与角色常量

```go
// internal/llm/provider.go:12-20
var ErrPromptTooLong = errors.New("prompt too long for context window")

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool" // 携带工具执行结果的回合
)
```

`ErrPromptTooLong` 是整个系统唯一的「语义化错误」：它把「文本里含有 prompt is too long」这种脆弱特征，转成 `errors.Is` 可判定的哨兵，`agent.go:283` 用它触发紧急压缩并**重试一次**。`RoleTool` 是本项目自造的协议无关角色，映射到 Anthropic 时变成 `user + tool_result` 块，映射到 OpenAI 时变成 `role:"tool"`（命名上不与任何一家对齐，纯中间层概念）。

### 2. 工具调用 / 结果 / 定义

```go
// internal/llm/provider.go:23-41
type ToolCall struct {
	ID    string          // provider 侧调用 id；回灌结果时配对
	Name  string          // 工具名（注册中心按名查找）
	Input json.RawMessage // 拼接完成的 JSON 参数
}

type ToolResult struct {
	ToolCallID string // 对应 ToolCall.ID
	Content    string // 执行产出（成功内容或结构化错误文本）
	IsError    bool   // 是否为错误结果（F9）
}

type ToolDefinition struct {
	Name        string         // 工具名
	Description string         // 给模型的用途说明
	InputSchema map[string]any // 完整 JSON Schema 对象：type/properties/required
}
```

- `ToolCall.Input` 是 `json.RawMessage`（不是 `string`）：Anthropic 侧直接把它塞进 `NewToolUseBlock`，避免「解析成 map 再序列化」带来的 key 顺序/精度变化；OpenAI 侧则转成 string 交给 `Arguments`。
- `ToolResult.ToolCallID` 是配对键。多工具并行执行时结果顺序必须与 `ToolCalls` 顺序一致，`agent.executeBatched` 用 `results[k] = ...{ToolCallID: calls[k].ID}` 按下标写回保证这一点。
- `ToolResult.IsError` 让「工具执行失败」作为**正常内容**回灌给模型，而不是中断循环（Anthropic 映射为 `tool_result` 的 `is_error` 字段）。
- `ToolDefinition.InputSchema` 是**完整** schema，但 Anthropic 适配器只取其中两个字段（见下）。

### 3. 消息与请求

```go
// internal/llm/provider.go:44-49
type Message struct {
	Role        string       // RoleUser | RoleAssistant | RoleTool
	Content     string       // 文本内容
	ToolCalls   []ToolCall   // 仅 assistant：本回合请求的工具调用
	ToolResults []ToolResult // 仅 RoleTool：工具执行结果（一条消息可含多个）
}
```

```go
// internal/llm/provider.go:76-88
type System struct {
	Stable      string // 可缓存：装配好的稳定系统提示（不含时间/环境等变化成分）
	Environment string // 不缓存：环境信息段（每轮可能变化）
}

type Request struct {
	Messages []Message        // 持久对话历史（不含本轮 reminder）
	Tools    []ToolDefinition // 本轮工具集（普通=全量 / 规划=只读）
	System   System           // 稳定系统提示 + 环境段
	Reminder string           // 本轮 system-reminder 内容（已含标签；空=不注入）
}
```

关键点：**`System` 被拆成两段不是审美问题，而是缓存策略**。`Stable` 是缓存断点的载体（Anthropic 打 `cache_control`，OpenAI 靠前缀自动命中），`Environment` 每轮重建（含活跃 Skill、cwd、时间），放在 `Stable` 之后，从而不破坏前缀。`Reminder` 单独成字段（而不是塞进 `Messages`）是为了让适配器各自决定「并入末条 user」还是「新增尾部 user」——这是两家协议角色交替规则不同的直接产物。

### 4. 事件流与用量

```go
// internal/llm/provider.go:52-74
type Usage struct {
	InputTokens  int64 // 本轮请求输入（含完整历史）token 数
	OutputTokens int64 // 本轮响应输出 token 数
	CacheWrite   int64 // 缓存写入 token（Anthropic: cache_creation_input_tokens; OpenAI: 恒 0）
	CacheRead    int64 // 缓存读取 token（Anthropic: cache_read_input_tokens; OpenAI: cached_tokens）
}

type StreamEvent struct {
	Text      string     // 文本增量
	ToolCalls []ToolCall // 非空：本轮模型请求执行这些工具
	Usage     *Usage     // 非空：本轮 token 用量（Done 之前一次性发出）
	Done      bool       // 本轮正常结束
	Err       error      // 出错
}
```

`StreamEvent` 用**零值可判别**的字段而不是 `Type` 枚举 + `Payload any`：消费端 `switch { case ev.Err != nil: ... case ev.Text != "": ... }`（`agent.go:493-505`）天然是按优先级匹配，省掉一次 type assertion，也避免 `any` 装箱。代价是「空文本增量」与「无增量」不可区分（当前实现里两者语义相同，无影响）。

### 5. Provider 接口与工厂

```go
// internal/llm/provider.go:91-107
type Provider interface {
	Name() string                                               // 状态栏左侧：供应商名称
	Model() string                                              // 状态栏右侧：模型名
	Stream(ctx context.Context, req Request) <-chan StreamEvent // 发起一轮流式对话
}

func New(cfg config.ProviderConfig) (Provider, error) {
	switch cfg.Protocol {
	case config.ProtocolAnthropic:
		return newAnthropicProvider(cfg)
	case config.ProtocolOpenAI:
		return newOpenAIProvider(cfg)
	default:
		return nil, fmt.Errorf("不支持的协议类型: %s", cfg.Protocol)
	}
}
```

接口只有 3 个方法：没有 `Close()`、没有 `CountTokens()`、没有 `Embed()`。`Stream` 把 `error` 挤进事件流而不作为第二个返回值，因此**接口签名里没有任何同步错误路径**——连「构造请求失败」也只能在 goroutine 里发 `StreamEvent{Err: ...}`。

### 6. Anthropic 适配器：工具定义转换（含 schema 裁剪）

```go
// internal/llm/anthropic.go:16-31
func toAnthropicTools(tools []ToolDefinition) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		schema := anthropic.ToolInputSchemaParam{
			Properties: t.InputSchema["properties"],
			Required:   toStrings(t.InputSchema["required"]),
		}
		tool := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: schema,
		}
		result = append(result, anthropic.ToolUnionParam{OfTool: &tool})
	}
	return result
}
```

只搬 `properties` 与 `required`，**丢弃** `type`、`additionalProperties`、`$schema` 等键（SDK 的 `ToolInputSchemaParam` 自己补 `type: object`）。`toStrings`(`anthropic.go:34-54`) 同时容忍 `[]string` 与 `[]interface{}` 两种形态——后者来自 MCP 工具 JSON Schema 反序列化。

### 7. Anthropic 适配器：system 分块 + 缓存断点

```go
// internal/llm/anthropic.go:120-137
// Stable 块打缓存断点（必须用 NewCacheControlEphemeralParam 构造器，空字面量会被 omitzero 丢弃）；
// Environment 块不打缓存断点。
func toAnthropicSystem(sys System) []anthropic.TextBlockParam {
	var blocks []anthropic.TextBlockParam
	if sys.Stable != "" {
		blocks = append(blocks, anthropic.TextBlockParam{
			Text:         sys.Stable,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		})
	}
	if sys.Environment != "" {
		blocks = append(blocks, anthropic.TextBlockParam{Text: sys.Environment})
	}
	return blocks
}
```

注释里的「空字面量会被 omitzero 丢弃」是一个真实的 SDK 陷阱：`anthropic.CacheControlEphemeralParam{}` 会被 `omitzero` 序列化规则吃掉，必须走构造器。这是「协议字段存在性」与「Go 零值语义」冲突的典型代价。

### 8. OpenAI 适配器：单条 system 拼接

```go
// internal/llm/openai.go:34-50（节选）
	// 首条 system 消息 = Stable + "\n\n" + Environment（单条拼接兼容端点对多条 system 支持不一）；
	// Stable 居前缀使端点前缀缓存自动命中稳定部分。
	var systemText string
	if req.System.Stable != "" { systemText = req.System.Stable }
	if req.System.Environment != "" {
		if systemText != "" { systemText += "\n\n" }
		systemText += req.System.Environment
	}
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages)+2)
	if systemText != "" { result = append(result, openai.SystemMessage(systemText)) }
```

注意两侧**并行但不相同的抽象**：Anthropic 用「两个 block + 断点」，OpenAI 用「一个字符串 + 靠顺序吃前缀缓存」，而 `Request.System` 的字段划分恰好同时服务两种实现。

### 9. 配置类型

```go
// internal/config/config.go:11-35
type ProviderConfig struct {
	Name          string `yaml:"name"`           // 状态栏左侧：供应商可读名称
	Protocol      string `yaml:"protocol"`       // "anthropic" | "openai"
	BaseURL       string `yaml:"base_url"`       // 自定义端点地址，空则用 SDK 默认
	APIKey        string `yaml:"api_key"`        // 认证密钥
	Model         string `yaml:"model"`          // 模型名，状态栏右侧显示
	Thinking      bool   `yaml:"thinking"`       // 是否启用扩展思考（仅 anthropic 生效）
	ContextWindow int    `yaml:"context_window"` // 上下文窗口 token 数，0 走协议默认
}

func (p ProviderConfig) EffectiveContextWindow() int {
	if p.ContextWindow > 0 { return p.ContextWindow }
	switch p.Protocol {
	case ProtocolAnthropic: return DefaultAnthropicContextWindow
	case ProtocolOpenAI:    return DefaultOpenAIContextWindow
	default:                return DefaultAnthropicContextWindow
	}
}
```

字段里**没有** `max_tokens`、`temperature`、`timeout`、`max_retries`、`proxy`、`headers`——采样与超时参数完全不受配置控制（见「边界与容错」）。

```go
// internal/config/protocol_defaults.go:3-15
const (
	ProtocolAnthropic = "anthropic"
	ProtocolOpenAI    = "openai"
)
const (
	DefaultAnthropicContextWindow = 200000
	DefaultOpenAIContextWindow    = 128000
)
```

### 10. 会话容器

```go
// internal/conversation/conversation.go:11-16
type Conversation struct {
	mu        sync.Mutex
	messages  []llm.Message
	onAppend  func(llm.Message)   // 可选：消息追加回调
	onReplace func([]llm.Message) // 可选：消息替换回调
}
```

```go
// internal/conversation/conversation.go:48-57
func (c *Conversation) AddUser(text string) {
	c.mu.Lock()
	c.messages = append(c.messages, llm.Message{Role: "user", Content: text})
	msg := c.messages[len(c.messages)-1] // 拷贝一份给回调（已解锁后使用）
	c.mu.Unlock()

	if c.onAppend != nil {
		c.onAppend(msg)
	}
}
```

- 回调**在锁外**调用，避免 `onAppend`（实际是 session JSONL writer）阻塞或反向调用 conversation 造成死锁。
- `AddUser` 用的是字面量 `"user"`，而 `AddAssistantWithToolCalls`/`AddToolResults` 用 `llm.RoleAssistant`/`llm.RoleTool`（`conversation.go:75,91`）——常量使用不一致，值恰好相同。
- `Messages()`(`103-120`) 与 `NewFromMessages`(`30-45`)、`ReplaceMessages`(`140-160`) 都对 `ToolCalls`/`ToolResults` 做深拷贝；`ReplaceMessages` 的回调收到的是**调用方原始切片**而非深拷贝（`conversation.go:157-159`），这是唯一一处「不暴露外部引用」原则的例外。
- 没有任何 `TokenCount()`/`Trim()`/`Truncate()`：会话只管存，压缩由 `internal/compact` 通过 `ReplaceMessages` 回写。

## 关键流程

### A. Anthropic 一次流式请求（从 `Stream` 到事件产出）

1. **建管道**：`Stream` 先 `ch := make(chan StreamEvent)`（**无缓冲**，`anthropic.go:157`），立刻返回 channel；所有工作在同 goroutine 内，`defer close(ch)`（`159-160`）。
2. **消息转换**：`toAnthropicMessages(req.Messages)`（`163`）逐条映射——`RoleUser`→`NewUserMessage(text)`；`RoleAssistant` 有 `ToolCalls` 时拼成 `[text?, tool_use...]`（`Content != ""` 才加文本块，`66-74`）；`RoleTool`→**一条 user 消息，内部放 N 个 `NewToolResultBlock`**（`78-84`），因为 Anthropic 要求 `tool_result` 只能出现在 user 角色里。
3. **reminder 织入**：`req.Reminder != ""` 时 `appendReminderAnthropic`（`166-168`）：末条是 user → 把 reminder 作为**新文本块追加**到该 user 消息；末条非 user → 新起一条 user 消息（`141-154`）。注释点明动机是「确保角色交替合法（N3）」。
4. **组装参数**：`anthropic.MessageNewParams{Model, MaxTokens: 4096, System: toAnthropicSystem(...), Messages}`（`170-175`）；`len(req.Tools) > 0` 时填充 `params.Tools`（`178-180`）。
5. **扩展思考开关**：`p.cfg.Thinking && !hasToolHistory(req.Messages)` 才 `params.Thinking = ThinkingConfigParamOfEnabled(16000)`（`183-185`）。`hasToolHistory` 只要历史里出现过 `RoleTool` 或任一 `ToolCalls` 就返回 true（`91-98`）。
6. **发起流**：`stream := p.client.Messages.NewStreaming(ctx, params)`（`188`）。**SSE 解析完全在 SDK 内部**（本项目没有手写 `bufio.Scanner`/`data:` 解析），上层只拿到已反序列化的事件对象。对应协议事件序列是 `message_start → content_block_start → content_block_delta* → content_block_stop → ... → message_delta(stop_reason, usage) → message_stop`，但源码只按 Go 类型分支处理。
7. **事件累积**：`acc := anthropic.Message{}`，循环 `for stream.Next()`，每轮 `acc.Accumulate(event)`（`191-203`）；`Accumulate` 负责把分散的 `input_json_delta` 片段拼成完整 `tool_use.input`、累积 `thinking` 块、并保存最终 `Usage`/`StopReason`。**拼接失败的路径**：发 `StreamEvent{Err: wrapAnthropicPTL(err)}` 后 `return`（`196-203`）。
8. **文本增量上抛**：只处理 `*anthropic.ContentBlockDeltaEvent` 且 `evt.Delta` 是 `anthropic.TextDelta` 的情况，发 `StreamEvent{Text: delta.Text}`（`206-214`）；**`ThinkingDelta` 与 `InputJSONDelta` 被显式丢弃**（`215` 注释），即思考内容不进 UI、工具参数增量不逐片段上抛。
9. **阻塞背压**：每次发送都是 `select { case ch <- ...: case <-ctx.Done(): return }`（`209-213`）。若 TUI 卡住不读，生产端停在 send 上（无缓冲，天然背压）；若 ctx 取消，goroutine 立即退出，避免泄漏。
10. **错误收口**：循环结束后 `stream.Err() != nil` → 发 `wrapAnthropicPTL(err)` 事件并 `return`（`220-226`），**不发 `Done`**。
11. **用量上抛**：仅当 `acc.Usage.InputTokens > 0 || acc.Usage.OutputTokens > 0` 才发 `StreamEvent{Usage}`（`229-240`），并映射 `CacheCreationInputTokens→CacheWrite`、`CacheReadInputTokens→CacheRead`。`Usage` 一定在 `ToolCalls`/`Done` 之前发出。
12. **结束原因判定**：只有 `acc.StopReason == anthropic.StopReasonToolUse` 才从 `acc.Content` 里筛 `AsToolUse()` 且 `ID != ""` 的块，组成 `[]ToolCall` 并一次性发出（`243-262`）。**`max_tokens` 截断、`end_turn`、`refusal` 等其它 StopReason 一律不区分**。
13. **终结事件**：发 `StreamEvent{Done: true}`（`264-267`），goroutine 结束，`defer close(ch)` 关闭 channel，消费端 `for range` 退出。
14. **消费端聚合**（`agent.go:481-513` `streamOnce`）：`Text` 累积进 `strings.Builder` 并同时 `emit` 给 TUI；`Usage` 暂存；`ToolCalls` append；遇 `Err` **立即 return 丢弃已累积文本**；结束后若 `ctx.Err() != nil` 也算失败（`509-511`）。
15. **PTL 的二次消费**：`streamOnce` 返回的 `err` 若 `errors.Is(err, llm.ErrPromptTooLong)` 且未紧急重试过，则 `agent.go:283-313` 触发 `compact.ManageContext(Trigger: TriggerEmergency)`，成功后 `ResetAnchor` 并**重发同一次请求**（`emergencyRetried = true` 只允许一次）；若压缩后估算 token 仍 ≥ `cw - compact.ManualSafetyMargin`，直接放弃并报原始错误（`306-310`）。

### B. OpenAI 一次流式请求

1. **消息构造**：`toOpenAIMessages(req)`（`openai.go:126`）拼单条 system（Stable + `"\n\n"` + Environment）→ 逐条转换历史 → 最后追加 reminder 为**新的尾部 user 消息**（`91-94`，注释：「OpenAI 容忍连续 user」）。
2. **`RoleTool` 展开**：与 Anthropic 相反的粒度——一条 `RoleTool` 消息里的 N 个结果被拆成 **N 条独立的 `role:"tool"` 消息**（`84-88`）。
3. **assistant 带工具调用**：构造 `ChatCompletionAssistantMessageParam{ToolCalls: [...]}`，`Content` 仅在非空时用 `param.Opt[string]{Value: ...}` 显式设置（`70-80`）——绕过「省略 vs 空串」的语义差异。
4. **参数**：`Model`、`Messages`、`StreamOptions{IncludeUsage: true}`（`128-134`）。**没有设置 `MaxTokens`/`Temperature`**，即完全使用服务端默认；`IncludeUsage` 是能拿到 `Usage` 事件的前提（否则 OpenAI 流式默认不回吐 usage）。
5. **发起流**：`p.client.Chat.Completions.NewStreaming(ctx, params)`（`142`），SSE 解析同样由 SDK 承担；chunk 类型为 `chat.completion.chunk`。
6. **累积 + 文本上抛**：`acc := openai.ChatCompletionAccumulator{}`，每 chunk `acc.AddChunk(evt)`（`148`，返回值未检查），然后只读 `evt.Choices[0].Delta.Content` 非空时上抛 `Text`（`150-159`）。**只取 `Choices[0]`**，忽略 `n>1` 场景（本项目也从不设置 `n`）。
7. **工具调用拼接**：**不在 chunk 层处理**，全靠 `acc` 在流结束后给出完整 `acc.Choices[0].Message.ToolCalls`，其中 `Function.Arguments` 已是拼接完成的 JSON 字符串（`186-198`）；`args == ""` 时兜底为 `"{}"`（`190-192`）。
8. **错误/用量/结束**：`stream.Err()` → `wrapOpenAIPTL` 后发 `Err` 并 return（`163-169`）；`Usage` 映射 `PromptTokens→InputTokens`、`CompletionTokens→OutputTokens`、`CacheWrite: 0`、`PromptTokensDetails.CachedTokens→CacheRead`（`172-183`）；最后发 `Done`（`208-211`）。
9. **与 Anthropic 的语义差**：OpenAI 侧**不检查 `finish_reason`**（`length` 截断与 `tool_calls` 结束都不区分），也无 thinking 概念——两条链路的「结束原因」抽象度不同。

### C. 上下文过长（PTL）错误处理链

1. 适配器层：`wrapAnthropicPTL`（`anthropic.go:274-285`）匹配 `"prompt is too long"`、`"context_length"`、`"too many tokens"`；`wrapOpenAIPTL`（`openai.go:218-230`）匹配 `"context_length_exceeded"`、`"maximum context length"`、`"too long"`、`("token" && "exceed")`。
2. 包装方式统一为 `fmt.Errorf("%w: %v", ErrPromptTooLong, err)`，保留原始文本但支持 `errors.Is`。
3. 上层：`agent.go:283` 判定 → 紧急压缩 → 重试一次 → 仍失败则报原错。

### D. 会话历史的追加/替换与持久化钩子

1. 主循环每轮：`conv.AddAssistantWithToolCalls(text, calls)`（`agent.go:369`）或 `conv.AddAssistant(final)`（`agent.go:348`）、`conv.AddToolResults(results)`（`agent.go:385`）。
2. 每次追加在锁外触发 `onAppend`，`main.go:129` 注入的是 `writer.OnAppend(modelName)`（session JSONL writer），即**会话持久化是 conversation 的回调，而不是 llm 的职责**。
3. 压缩完成后由 `compact` 调 `ReplaceMessages`（`conversation.go:140`）整体替换并触发 `onReplace`，writer 借此把压缩后的历史落盘。

## 设计决策与权衡

1. **决策：`Provider` 接口把所有错误都塞进 `StreamEvent.Err`，不返回 `error`。**
   为什么：一次流式请求的错误可能发生在「连接建立后、事件中途」，与返回值时机不匹配；统一走 channel 后，消费端只需要一个 `for range` + 一个 `switch`。替代方案是「`(<-chan StreamEvent, error)`」或「事件流 + 独立 err channel」，前者只能表达建连前错误，后者让消费端要 `select` 两个 channel、并依赖约定谁先关闭——本项目选了最少约定的一种。

2. **决策：`Request.System` 拆成 `Stable` + `Environment` 两段。**
   为什么：Anthropic 的 prompt cache 断点必须打在**稳定前缀**上，任何每轮变化的内容（时间、cwd、活跃 Skill）落到断点之前都会使缓存全量失效；拆字段把「什么可变」变成类型级约束。替代方案是「一整段 system + 由适配器猜哪部分可变」——不可靠；或「完全不做缓存」——Claude Code 类工具的输入 token 成本会高一个量级。

3. **决策：`toAnthropicSystem` 只给 `Stable` 打 `cache_control: ephemeral`，`Environment` 不打断点、且排在后面。**
   为什么：Anthropic 最多 4 个断点，最省钱的位置是「所有固定前缀之后」；把断点放在 `Stable` 末尾即可让 system 稳定段（以及可能更靠后的工具定义，**源码未在工具上打断点**）命中。替代方案是给工具定义也打断点——可以多省一次写入，但会占用断点额度并增加复杂度；本项目选择了最小的单断点策略。

4. **决策：OpenAI 侧把 `Stable + "\n\n" + Environment` 拼成**一条** system 消息。**
   为什么（源码注释直给）：不同 OpenAI 兼容端点对多条 system 消息的支持不一致，单条最稳；同时把 `Stable` 放前缀以吃自动前缀缓存。替代方案是多条 system（语义更清晰但兼容性差）、或用 `developer` role（新模型专有，老端点不支持）。

5. **决策：工具调用参数**不逐片段上抛**，只在流结束后一次性发 `ToolCalls`。**
   为什么：`input_json_delta` 是**非累积**片段，任何中间时刻的 JSON 都是不可解析的，上抛给 UI 没有可用语义；Anthropic 侧靠 `acc.Accumulate` 拼接，OpenAI 侧靠 `ChatCompletionAccumulator.AddChunk`。代价是「工具参数打字机效果」做不了——替代方案是上抛原始片段并让 UI 自己做 buffer，但那样每个消费端都要重新实现一遍拼接逻辑。

6. **决策：扩展思考（thinking）在「历史含任何工具交互」时整体关闭。**
   为什么：Anthropic 的 extended thinking 在带 `tool_use` 的多轮里要求把上一轮的 thinking block 原样回传，而本项目的 `Message` 结构**根本不存 thinking**（`provider.go:44-49` 只有 `Content`/`ToolCalls`/`ToolResults`），回传时会 400，因此用 `hasToolHistory` 一刀切规避。代价很直接：**只要发生过一次工具调用，本次会话后续所有轮次都不再有 thinking**，即使后续轮与工具无关。替代方案是扩展 `Message` 增加 `ThinkingBlocks []json.RawMessage` 并原样回灌——正确但要动协议无关模型，源码未采纳。

7. **决策：`Usage` 在流**结束后**才发出（`acc.Usage` 完整时），而不是随 `message_delta` 实时转发。**
   为什么：两家协议的 usage 都可能在最后一个 chunk / `message_delta` 才齐全，早发会得到 0；放在 `ToolCalls` 与 `Done` 之前、一次性发，消费端无需合并多次 usage。替代方案是「每次带 usage 的 chunk 都发、消费端覆盖」——多一次事件且语义重复。

8. **决策：无缓冲 channel + 每个 send 都包 `select {... case <-ctx.Done()}`。**
   为什么：无缓冲让「生产速度」受消费端约束，形成天然背压，避免模型吐 10k token 而 UI 卡死时把事件堆在内存里；`ctx.Done()` 分支解决「消费端提前 return（如 `streamOnce` 遇错就 return，`agent.go:494-495`）导致生产者永久阻塞」的 goroutine 泄漏。替代方案是带缓冲 channel（如 1024）+ 单独退出信号，复杂度更高且仍需 ctx 兜底。

9. **决策：`Conversation` 的回调在 `Unlock()` 之后触发。**
   为什么：`onAppend` 实际是 `session.Writer.OnAppend`（磁盘 IO），持锁做 IO 会阻塞所有 `Messages()` 读；锁外回调也让「回调里再读 conversation」不会自死锁。代价是回调顺序不严格保证与 `messages` 顺序一致（并发 Append 时）。替代方案是回调入队到单独 goroutine —— 会丢失「落盘先于下一步」的因果，恢复会话时可能漏消息。

10. **决策：`Messages()`/`ReplaceMessages()`/`NewFromMessages()` 全部深拷贝 `ToolCalls`/`ToolResults` 子切片。**
    为什么：`streamOnce` 拿到历史后要在适配器里转成 SDK 结构，若共享底层数组，将来有人就地 append（`append` 可能复用容量）会污染会话历史；而 `Content` 是字符串（不可变）无需拷贝。替代方案是「返回内部切片 + 文档约束不可改」——在 Go 里等于埋雷。

11. **决策：PTL 不靠「错误码」而靠**错误文本子串**识别。**
    为什么：Anthropic/OpenAI 的 PTL 在不同版本、不同兼容端点下错误类型不稳定，SDK 也未统一暴露 `IsContextLengthExceeded()` 之类的判定（`anthropic.go:274-285` 也只拿到 `err.Error()`），子串匹配最鲁棒。代价是脆弱：措辞一变就失效（例如 `wrapOpenAIPTL` 里 `strings.Contains(errStr, "too long")` 相当宽泛，可能误判其它错误）。替代方案是解析 HTTP 状态码 + `error.type`，需要穿透 SDK 拿原始响应。

12. **决策：`New()` 用 `switch` 显式分支而非注册表。**
    为什么：协议只有两种，编译期可穷尽，注册表（`map[string]func` + `init()`）在这个规模下只是额外间接层。替代方案是插件式 `Register(protocol, factory)`，有利于第三方扩展——本项目是单仓工具，不需要；`default` 分支返回中文错误并列出不支持的类型，已经提供了可诊断性。

13. **决策：`ContextWindow` 只作为「估算基准」交给 `compact`，llm 层不做任何截断。**
    为什么：截断策略（保留最近 N 条、工具结果落盘、LLM 摘要）需要看到消息全貌和工具定义，属于上层职责；LLM 层只负责「发出去、拿回来」。`EffectiveContextWindow()` 只在 `main.go:104` 被读取一次并放进 `SessionRuntime.ContextWindow`（`agent.go:419` 里还有一处硬编码的 200000 兜底）。替代方案是在 Provider 里做滑动窗口裁剪——会把「协议适配」和「上下文管理」耦合，且无法做语义级压缩。

14. **决策：配置校验「宁可启动失败」——`api_key` 为空直接报错（`config.go:77-79`）。**
    为什么：AI 工具最差体验是启动正常、提问时才 401；把错误提前到加载期并带上 `providers[i].name` 便于定位。替代方案是允许空 key（支持本机无鉴权网关如 Ollama/vLLM，用 `base_url` 指向本地）——**本项目用 `BaseURL` 支持自定义端点，但没有为「无 key 端点」留口子**，这是与本地推理网关集成时的实际摩擦点。

15. **决策：`Thinking` 只对 Anthropic 生效，配置里不做协议分支校验。**
    为什么：`config.go:17` 注释明说「仅 anthropic 生效」——避免为一个软开关写死校验逻辑，OpenAI 侧静默忽略（`openai.go` 全文件不含 `Thinking`）。

## 边界与容错

### 硬编码常量清单（本模块内）

| 常量/值 | 位置 | 含义与风险 |
|---|---|---|
| `MaxTokens: 4096` | `anthropic.go:172` | Anthropic 输出上限**硬编码**，不可配置。对长代码生成（大文件重写、长 diff）容易截断，且截断时 `StopReason` 为 `max_tokens`，本层**不区分**、不报错，上层只看到文本突然结束。 |
| `ThinkingConfigParamOfEnabled(16000)` | `anthropic.go:184` | 扩展思考预算 16000 token。**潜在风险（源码未做交叉校验）**：Anthropic 要求 `budget_tokens < max_tokens`，而此处 `MaxTokens` 是 4096，启用 thinking 的首轮可能被端点拒绝为 400；本项目未做该一致性检查，实际是否报错取决于端点校验，源码未体现。 |
| `DefaultAnthropicContextWindow = 200000` | `protocol_defaults.go:12` | 未配置 `context_window` 时的估算基准，喂给 `compact` 的触发阈值。 |
| `DefaultOpenAIContextWindow = 128000` | `protocol_defaults.go:14` | 同上。**注意**：该值与具体模型无关，`gpt-4o-mini`(128k) 与 `o1`(200k) 共用同一个默认值，配置不写就按 128k 保守估算。 |
| `EffectiveContextWindow` 的 `default: Anthropic` | `config.go:32-33` | 协议未知时回退 200k。**与 `New()` 的行为冲突**：`New()` 对未知协议直接报错，所以这条 `default` 分支实际不可达（`validate()` 已限定协议只能取两个值）。 |
| `ch := make(chan StreamEvent)`（容量 0） | `anthropic.go:157`、`openai.go:120` | 无缓冲 = 强背压；任何未消费的事件都会阻塞生产 goroutine。 |
| `"\n\n"`（system 拼接分隔符） | `openai.go:42` | Stable 与 Environment 的分隔；也让 `Stable` 成为可缓存前缀。 |
| `args = "{}"`（空参数兜底） | `openai.go:190-192` | 模型返回空字符串参数时兜底成空对象，避免 `json.RawMessage("")` 造成下游 unmarshal 失败。 |
| `strings.Contains` 关键字表 | `anthropic.go:279-281`、`openai.go:223-226` | PTL 识别正则的替代品；`openai.go` 中 `... || strings.Contains(errStr, "token") && strings.Contains(errStr, "exceed")` 依赖 Go 运算符优先级（`&&` 高于 `||`），等价于 `A \|\| B \|\| C \|\| (D && E)`。 |
| 未设置的参数 | `openai.go:128-134` | **无 `MaxTokens`、无 `Temperature`、无 `TopP`、无 `Stop`、无 `Seed`**：完全吃服务端默认，结果不可复现。Anthropic 侧同样没有 temperature。 |
| 未设置的缓存 TTL | `anthropic.go:128` | `NewCacheControlEphemeralParam()` 未指定 `ttl`，需按 SDK 默认（源码未体现具体值）。 |

### 本模块**没有**的东西（面试高频对比点）

- **没有超时**：`llm` 包内不出现 `context.WithTimeout`；唯一的超时来源是调用方传入的 `ctx`（TUI 用户取消）。全仓超时分布在别处：`tool.DefaultTimeout = 30s`（`internal/tool/registry.go:13`）、MCP 连接 30s（`internal/mcp/manager.go:20`）、hook 默认 30s、环境采集 2s（`internal/prompt/environment.go:44`）。**含义：模型端「卡住不吐 token」时，agent 主循环会无限等待**（无 idle/首字节超时）。
- **没有重试/退避**：`llm` 包内无 429/5xx/网络抖动的重试逻辑（全仓 `Retry` 只出现在 `compact/layer2.go` 的 `ptlRetry`，那是**摘要请求自身的 PTL 重试**：`ptlRetryLimit = 3` 次每次丢最旧 1 组 turn，之后按 `ptlDropPercentage = 0.2` 比例丢）。底层 SDK 是否内置重试属于 SDK 行为，**本项目源码未体现**、也未显式配置 `WithMaxRetries`。
- **没有 token 计数校验**：发送前不做 `count_tokens` 预检，只有事后 PTL 兜底（`agent.go:283` 的单次紧急压缩重试）。
- **没有可观测性**：无 request id 采集、无耗时统计、无 OTel/日志埋点；`Usage` 是唯一的量化信号，且只被 `agent` 用于锚点更新（`agent.go:331-343`）。
- **没有多模态**：`Message.Content` 是 `string`，无法承载 image/audio/document 块；也没有 `cache_control` 之外的内容块控制（不使用 `TextBlockParam` 的 citations 等）。

### 异常路径清单

1. **不支持的协议**：`New()` 返回 `不支持的协议类型: %s`（`provider.go:105`）；但 `validate()`（`config.go:74-76`）已在加载期拦掉，运行期正常不可达。
2. **配置文件缺失/权限错误**：`Load` 包装为 `无法读取配置文件 %s: %w`（`config.go:45-47`）。
3. **YAML 语法错误**：`配置文件 YAML 格式错误: %w`（`config.go:50-52`）。
4. **校验失败**：`providers` 为空 / `name` 为空 / `protocol` 为空或非法 / `api_key` 为空 / `model` 为空，错误文案均带 `providers[i]` 下标或 `name`（`config.go:62-85`）。**未校验**：`name` 重名、`base_url` 合法性、`context_window` 上下界、`Providers[0]` 是否存在之外的 provider 选择逻辑。
5. **建连/中途断流**：`stream.Err()` → `wrapXxxPTL` → `StreamEvent{Err}` → **不发 `Done`**，channel 由 `defer close` 关闭；`streamOnce` 见 `Err` 立即 return，已累积的部分文本被丢弃（`agent.go:494-495`）——UI 上表现为「已打出的内容保留在屏幕，但历史里没有这条 assistant 消息」，随后由 `ensureAssistantTail(conv, noticeStreamErr)` 补一条错误提示消息（`agent.go:326`）。
6. **用户取消**：`ctx.Done()` 命中所有 `select` 分支 → goroutine 静默 return，`ch` 关闭；`streamOnce` 在 `509-511` 用 `ctx.Err() != nil` 把「channel 正常关闭」升级为失败；`agent.go:315-318` 走 `finishCancelled`。注意：**取消路径下 `Usage` 可能已发出但 `ToolCalls` 不完整**，上层不会执行工具。
7. **`Accumulate` 失败**：Anthropic 独有的中间态错误（`anthropic.go:196-203`），同样走 `Err` 事件后 return。
8. **`RoleTool` 但 `ToolResults` 为空**：Anthropic 侧会生成一条**零内容块的 user 消息**（`anthropic.go:80-84` 无长度检查），端点大概率 400；OpenAI 侧则什么都不发（`84-88` 循环零次）——同一畸形输入在两家行为不一致，源码未做防御。
9. **`RoleTool` 带 `Content` 但无 `ToolResults`**：`Content` 被两家适配器**完全忽略**，消息静默消失。
10. **工具参数非法 JSON**：Anthropic 侧 `json.RawMessage(tc.Input)` 原样塞进 SDK，序列化/服务端校验阶段才可能报错；OpenAI 侧作为字符串传递，服务端可能返回 400。本层**不校验** `ToolCall.Input` 是否为合法 JSON。
11. **未知 `Role`**：两个 `switch m.Role` 都没有 `default` 分支，未知角色被**静默丢弃**（`anthropic.go:61-85`、`openai.go:53-88`）——这是有意的「忽略而非报错」，但也意味着将来加 `RoleSystem` 会悄无声息地失效。
12. **空历史 + reminder**：`appendReminderAnthropic` 对 `len(msgs)==0` 特判，返回一条只含 reminder 的 user 消息（`141-144`）。
13. **`Usage` 全零**：两个适配器都只在 `InputTokens>0 || OutputTokens>0` 时发 usage 事件（`anthropic.go:229`、`openai.go:172`），因此上层 `usage` 可能为 `nil`——`agent.go:331` 和 `337` 都做了 `usage != nil` 保护。
14. **压缩回写并发**：`ReplaceMessages` 持锁替换，回调在锁外（`conversation.go:155-159`），期间若有其它 goroutine 读历史会拿到「替换前」的快照（`Messages()` 深拷贝），不会撕裂。

## 面试官可能追问（15 条）

**Q1：为什么把 provider 抽象放在 `Provider` 接口 + `StreamEvent` channel，而不是让上层直接调 SDK？**
A：核心诉求是让 `agent` 包零 SDK 依赖（`agent/agent.go` 包注释明确写了「只依赖 llm、tool、conversation、permission，不 import SDK，保持协议无关」），这样 ReAct 循环、压缩器（`compact/layer2.go` 里也用 `in.Provider.Stream`）、TUI 三处消费方共用一套事件语义。其次，两家的差异（Anthropic 的 `tool_result` 必须放 user、OpenAI 的 `role:"tool"` 一条一结果、reminder 注入位置不同）全部收敛在 `anthropic.go` / `openai.go` 的转换函数里。代价是 SDK 的能力（thinking block 回传、citations、多模态）被这一层「抹平」了，想用就得扩 `Request`/`Message`。

**Q2：`StreamEvent` 为什么不用 `Type` 枚举 + `Payload interface{}`？**
A：用零值可判别的字段（`Text`/`ToolCalls`/`Usage`/`Done`/`Err`）让消费端一个 `switch` 就够（`agent.go:493-505` 的 `streamOnce`），不需要 type assertion，也没有装箱分配。注释 `provider.go:59-74` 显式写明了五态语义和「`Text` 与 `ToolCalls` 不同时非空、`Done`/`Err` 互斥且终结」的不变量——这是把协议契约写成文档而不是靠类型系统强制。风险是新增事件类型时必须新增字段并更新这份注释。

**Q3：一次流式请求里，工具调用的参数是怎么拼起来的？**
A：分两条路，都不在本项目代码里逐片段拼。Anthropic：`acc := anthropic.Message{}`，每个事件 `acc.Accumulate(event)`（`anthropic.go:191-203`）内部把 `input_json_delta` 累积成完整 `tool_use.input`；流结束后判 `acc.StopReason == anthropic.StopReasonToolUse`，遍历 `acc.Content` 取 `AsToolUse()` 且 `ID != ""` 的块组成 `[]ToolCall`（`243-262`）。OpenAI：`openai.ChatCompletionAccumulator{}` + `acc.AddChunk(evt)`（`openai.go:145-148`），结束后从 `acc.Choices[0].Message.ToolCalls` 取 `Function.Arguments`（已是完整 JSON 字符串），空串兜底 `"{}"`（`190-192`）。`InputJSONDelta` 明确不上抛（`anthropic.go:215` 注释）。

**Q4：为什么 `Text` 增量实时上抛，`ToolCalls` 却要等到流结束？**
A：因为 `input_json_delta` 的片段是非累积的，任意中间时刻拼出来的都是非法 JSON，上抛没有可用语义；而文本 delta 天然可增量渲染。这是「可用性 vs 实时性」的取舍：代价是工具调用没有打字机效果。替代方案是上抛原始片段由 UI 缓冲——但 `agent` 与 `compact` 两个消费端都得重实现拼接，不划算。

**Q5：`System.Stable` / `System.Environment` 拆分的收益是什么？OpenAI 那边怎么处理？**
A：`Stable` 承载稳定系统提示并作为缓存前缀，`Environment` 每轮重建（时间、cwd、活跃 Skill，见 `agent.go:266-277`）。Anthropic 侧 `toAnthropicSystem`（`anthropic.go:123-137`）给 `Stable` 打 `anthropic.NewCacheControlEphemeralParam()`、`Environment` 不打断点且排后面；OpenAI 侧 `toOpenAIMessages` 把两者拼成**一条** system（`Stable + "\n\n" + Environment`，`openai.go:36-50`），注释说明动机是「兼容端点对多条 system 支持不一」且「Stable 居前缀使端点前缀缓存自动命中」。注意源码里**工具定义没有打 Anthropic 缓存断点**，只用了 system 这一个断点。

**Q6：`NewCacheControlEphemeralParam()` 那个注释说的坑是什么？**
A：`anthropic.go:121` 注释原文：「必须用 NewCacheControlEphemeralParam 构造器，空字面量会被 omitzero 丢弃」。因为 SDK 结构体字段带 `omitzero` 序列化标签，直接写 `CacheControl: anthropic.CacheControlEphemeralParam{}` 会在 JSON 里消失，服务端就看不到 `cache_control`，缓存静默失效。这是 Go 零值语义与「字段存在性有语义」的协议设计冲突，只有读 SDK 源码才能发现。

**Q7：扩展思考为什么一旦用过工具就永久关闭？**
A：`anthropic.go:183` 的条件是 `p.cfg.Thinking && !hasToolHistory(req.Messages)`，而 `hasToolHistory`（`91-98`）只看历史里有没有 `RoleTool` 或任何 `ToolCalls`。原因是 Anthropic 带 `tool_use` 的多轮要求把上一轮 thinking block 原样回传，而协议无关的 `Message`（`provider.go:44-49`）只有 `Content`/`ToolCalls`/`ToolResults`，**根本不存 thinking**，回传缺失会 400。这个规避是「正确性优先」，代价是长会话里 thinking 基本拿不到。正解是给 `Message` 加 thinking 块字段并原样回灌。

**Q8：thinking 预算 16000 和 `MaxTokens: 4096` 会不会冲突？**
A：会存在风险：Anthropic 要求 `budget_tokens` 小于 `max_tokens`，而 `anthropic.go:172` 把 `MaxTokens` 写死 4096、`184` 把 budget 写成 16000，本层**没有做交叉校验**，启用 thinking 的首轮可能直接 400；实际行为取决于端点校验，源码未体现。能确定的是：`MaxTokens` 硬编码 4096 本身就会截断长输出，而截断（`StopReason == max_tokens`）在 `243` 的判定里被当作「非 tool_use」→ 只发 `Done` 不报错，上层只会看到回答突然中断。

**Q9：PTL（上下文过长）是怎么被识别与恢复的？**
A：识别在适配器：`wrapAnthropicPTL`（`anthropic.go:274-285`）匹配 `"prompt is too long"`/`"context_length"`/`"too many tokens"`，`wrapOpenAIPTL`（`openai.go:218-230`）匹配 `"context_length_exceeded"`/`"maximum context length"`/`"too long"`/`("token" && "exceed")`，统一 `fmt.Errorf("%w: %v", ErrPromptTooLong, err)` 保留原文。恢复在 `agent.go:283-313`：`errors.Is(sErr, llm.ErrPromptTooLong) && !emergencyRetried` → `compact.ManageContext(TriggerEmergency)` → `ResetAnchor` → 重新 `streamOnce`（只重试一次）；若压缩后 `EstimateTokens` 仍 ≥ `cw - compact.ManualSafetyMargin(3000)` 就放弃并报原错。它靠**错误文本子串**而非错误码，脆弱但对多端点兼容性最好。

**Q10：`Conversation` 为什么要深拷贝？回调为什么在锁外？**
A：深拷贝（`Messages()` `conversation.go:103-120`、`NewFromMessages` `30-45`、`ReplaceMessages` `140-160`）是因为转 SDK 结构、压缩重排都可能就地改切片，共享底层数组会被 `append` 复用容量时污染历史；`Content` 是 string 不可变所以不必拷。回调放锁外（见 `AddUser` `48-57` 的注释「拷贝一份给回调（已解锁后使用）」）是因为 `onAppend` 是 session JSONL 落盘 IO，持锁做 IO 会阻塞所有读，还可能因回调反向调用 conversation 而死锁。唯一例外：`ReplaceMessages` 的 `onReplace(msgs)` 传的是**调用方原始切片**（`157-159`），没有深拷贝。

**Q11：`Conversation` 里为什么没有 token 计数和截断？**
A：职责切分：`Conversation` 只做并发安全的历史容器 + 变更通知，上下文管理全部交给 `internal/compact`（`ManageContext` 两层压缩：工具结果落盘替换阈值 `singleResultLimit = 50000` 字节 / `messageAggregateLimit = 200000` 字节，LLM 摘要预留 `SummaryReserve = 20000`，自动触发安全余量 `AutoSafetyMargin = 13000`，最近原文保留 `recentKeepTokens = 10000` / `recentKeepMessages = 5`，token 估算 `estimateCharsPerToken = 3.5`）。压缩完成由 `conv.ReplaceMessages(...)` 整体回写并触发 `onReplace` 落盘，读取侧由 `agent` 传入 `ContextWindow: cfg.Providers[0].EffectiveContextWindow()`（`main.go:104`）。

**Q12：如果 UI 停止读取 channel 会怎样？**
A：无缓冲 channel（`anthropic.go:157`、`openai.go:120`）意味着生产 goroutine 会**阻塞在 send** 上，形成背压——内存不会堆积，但模型侧的流会在客户端不读时被 TCP 窗口压住。真正的风险是泄漏：若消费端**提前 return 且不取消 ctx**，生产 goroutine 永久卡在 send。本项目的防御是每个 send 都写成 `select { case ch <- ev: case <-ctx.Done(): return }`（`anthropic.go:209-213` 等），消费端 `streamOnce` 在返回时也总伴随 ctx 取消或流已终止。所以真正保证正确性的是「ctx 一定会被取消」这一上层约定，而不是这个 channel 本身。

**Q13：`streamOnce` 遇到 `Err` 就 `return`，已经打出来的文本为什么不算数？**
A：`agent.go:494-495` 直接 `return textBuilder.String(), calls, nil, ev.Err`，调用方在 `319-328` 判定 `sErr != nil` 后 `emit(Event{Err})` 并 `ensureAssistantTail(conv, noticeStreamErr)` —— **不会**把部分文本 `AddAssistant`，所以历史里只有一条错误提示消息。好处是不会把「半截回答」当成模型的完整输出再喂回下一轮；坏处是 UI 上已流式渲染的文本与持久化历史不一致（重新加载会话时会消失）。要一致的话应该在 `ensureAssistantTail` 里带上 `textBuilder.String()`。

**Q14：`Reminder` 为什么单独成字段，两家分别怎么注入？**
A：因为角色交替规则不同：Anthropic 要求 user/assistant 严格交替且 `tool_result` 必须在 user 消息里，所以 `appendReminderAnthropic`（`anthropic.go:141-154`）把 reminder 作为**追加的文本块**塞进末条 user 消息（末条非 user 才新起一条 user）；OpenAI 容忍连续 user，`toOpenAIMessages` 在末尾直接 `append(openai.UserMessage(req.Reminder))`（`openai.go:91-94`）。内容侧由 `agent.buildReminder`（`agent.go:1162-1181`）按轮次生成：Plan Mode 下 `iter == 1 || (iter-1)%planReminderInterval == 0`（`planReminderInterval = 4`，`agent.go:152`）发完整提醒、否则发精简版，再拼上 `runtime.TakeReminders()`（**取出即清空**，保证 reminder 只注入一轮）。`prompt.SystemReminder` 负责包 `<system-reminder>` 标签。

**Q15：配置层做了哪些校验，还缺什么？**
A：`validate()`（`config.go:62-85`）校验 `providers` 非空、`name`/`protocol`/`api_key`/`model` 非空、`protocol` 只能是 `"anthropic"`/`"openai"`（`ProtocolAnthropic`/`ProtocolOpenAI` 常量，`protocol_defaults.go:5-7`），错误信息带下标和 name，便于定位。缺失的：`name` 重名检测、`base_url` URL 合法性、`context_window` 下界（写 1 会导致压缩疯狂触发）、`Providers` 多于 1 个时的选择语义（`main.go` 一律用 `cfg.Providers[0]`，但 TUI 侧支持切换——**多 provider 的默认选择逻辑源码未在 config 层体现**）。另外 `APIKey` 从 YAML 明文读取，**没有 `os.Getenv`/`os.ExpandEnv` 支持**（`internal/config` 全目录无环境变量读取），密钥只能落到 `.mewcode/config.yaml` 或 `~/.mewcode/config.yaml`（`main.go:157-173`），这是实打实的凭据管理缺口。

## 企业级对应方案

1. **Prompt 缓存与成本控制**：本项目只在 system 的 `Stable` 块打了一个 Anthropic `cache_control` 断点，工具定义不打；工业界（Claude Code 一类 CLI）会系统化地把「system + 工具定义 + 会话稳定的早期消息」分层打断点（Anthropic 上限 4 个），并在会话中**增量推进断点位置**以持续命中。本项目若上生产需要：给 `ToolDefinition` 列表增加断点、把「历史前 N 轮的静态部分」纳入断点规划、在 `Usage` 基础上算真实 cache hit rate 并做告警（现在 `CacheRead/CacheWrite` 只被 `agent.go:337-342` 转发给 UI，没有聚合指标）。

2. **可靠性：重试、超时、熔断、降级**：本项目 `llm` 包**零重试、零超时**，只靠 SDK（未显式配置）和上层单次 PTL 紧急压缩。工业界普遍在网关层（LiteLLM / Envoy AI Gateway / 自建 vLLM 前置网关）实现：按错误类型分级重试（429/503 指数退避 + jitter，400/401 不重试）、请求级 deadline、模型级 fallback 链、以及并发/预算限流。本项目若上生产需要：`ProviderConfig` 增加 `timeout`/`max_retries`/`fallback_provider`，在 `Stream` 外包一层可重试的装饰器（注意**流式中途失败要不要重试**是关键设计点：已产生 `Text` 的重试会导致重复输出），并在 UI 上区分「重试中」与「失败」。

3. **协议层：SSE 手写解析 vs SDK 抽象 vs 统一网关**：本项目把 SSE 解析完全交给 SDK（[anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go)、[openai-go](https://github.com/openai/openai-go)），换来的是零自研解析成本，但也被 SDK 的事件模型绑架（例如只取 `Choices[0]`、`AddChunk` 返回值未检查、无法拿到原始 HTTP 响应做精细化错误分类）。工业界两条路：一是像 [OpenAI Agents SDK](https://openai.github.io/openai-agents-python/)/Responses API 那样定义语义化事件（`response.output_text.delta`、`response.function_call_arguments.delta`）并要求客户端按事件类型驱动状态机；二是自建统一网关（LiteLLM/vLLM）把 provider 差异收到服务端，客户端只对接一种协议。本项目若上生产建议：至少补齐 `finish_reason`/`stop_reason` 的全量语义（`length`、`content_filter`、`refusal`），因为它们直接决定「要不要重试」和「要不要截断告警」。

4. **流式与并发模型**：本项目用「无缓冲 channel + ctx 兜底」的单生产者模型，简单且背压正确，但**没有 idle/首字节超时**，模型端静默挂起会让 agent 主循环无限等待；也没有「流式执行工具调用」（工具必须等整轮结束才执行，`agent.go:379` 的 `executeBatched` 在 `streamOnce` 返回后才跑），而 Claude Code 类产品会做 early tool execution 以降低感知延迟。工业界常用：per-request deadline + idle timeout、`context.Context` 贯穿、事件多路复用（如用 `errgroup`/actor 模型管理同一会话的多个并发请求）。本项目若上生产需要：在 `ProviderConfig` 加 idle timeout，并考虑在 `ToolCalls` 完整时立刻触发 `PreToolUse`（权限预检）以隐藏工具准备延迟。

5. **会话状态与持久化**：本项目 `Conversation` 是进程内 `[]llm.Message` + 两个回调，持久化交给 `session.Writer` 的 JSONL，恢复靠 `conversation.NewFromMessages`（`tui/resume.go:58,92`）——**没有 schema 版本、没有事件溯源、没有幂等写**。工业界（LangGraph 的 checkpointer、Temporal 式工作流、OpenAI Agents SDK 的 session 存储）会把「状态快照 + 增量事件」分离，支持从任意检查点重放、跨进程恢复、以及并行分支的合并。本项目若上生产需要：给 `Message` 加版本字段与迁移逻辑、把「LLM 请求/响应元数据（model、usage、request id、stop_reason）」一起持久化（便于事后审计与回归测试），并把 `Conversation` 的锁换成更细粒度的存储抽象。

6. **Token 预算与上下文管理**：本项目用 `estimateCharsPerToken = 3.5` 的字符估算 + 事后 PTL 兜底（`agent.go:283` 一次紧急压缩重试），而不是精确计数。工业界普遍使用模型侧 `count_tokens` 接口或本地 tokenizer（tiktoken）做发送前预检，并对「工具结果」单独设预算（本项目用字节阈值 `singleResultLimit = 50000` / `messageAggregateLimit = 200000` 落盘替换）。vLLM 这类推理网关还有 KV cache 感知的调度与 automatic prefix caching（前缀复用做到显存层）。本项目若上生产需要：接入精确 tokenizer、把 `EffectiveContextWindow` 从「协议默认」升级为「模型级元数据表」（现在 `DefaultOpenAIContextWindow = 128000` 对所有 OpenAI 模型一视同仁），并在 PTL 之前就用软阈值主动压缩。

7. **安全与凭据**：本项目 `APIKey` 明文存于 `.mewcode/config.yaml`，`Load` 不做环境变量展开，进程内也没有密钥脱敏逻辑（配置错误信息只带下标/name，暂不会回显 key，这点是好的）。工业界做法：从环境变量/系统 keychain（macOS Keychain、1Password CLI）/云 IAM 角色获取短期凭据，日志与错误上报统一脱敏，并按 provider 维度做用量配额与审计。本项目若上生产需要：支持 `${ENV_VAR}` 展开（一行 `os.ExpandEnv` 但要防止把占位符当字面 key 用）、优先读环境变量、并提供 `mewcode config set-key` 之类的安全写入路径。
