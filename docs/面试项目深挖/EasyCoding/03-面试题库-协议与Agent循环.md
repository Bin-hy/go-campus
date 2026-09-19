# 03 面试题库：协议抽象 + ReAct 循环

> 每题四段：**面试官想听什么** → **口述答案（可直接背）** → **备注讲解（代码依据）** → **可能的追问**
> 代码路径相对 `mewcode/`。⭐ 标记的是「高概率出现」。

---

# 第一部分：多协议 LLM 引擎

---

## Q3.1 ⭐ `Provider` 接口为什么只有三个方法？够用吗？

**面试官想听什么**：你是真的在「按需设计接口」，还是抄了别人的接口定义。

**口述答案**：

> 「三个方法是因为**上层真的只需要这三件事**：显示供应商名字、显示模型名、发一轮流式对话。
>
> 我见过很多抽象会加一堆方法——`CountTokens`、`ListModels`、`Embed`、`CreateCompletion`——结果每个适配器都要写一堆「不支持就报错」的空实现。**抽象的成本在于每个实现方都要付出代价**，所以接口应该贴着**唯一调用方的真实需求**长出来。
>
> 唯一的例外是 `CountTokens`——因为它**确实**对上下文管理有用（精确算 token 就不用留 13000 的安全余量了）。但我没加，因为：① OpenAI 没有 count_tokens 这种端点，加了就有实现会退化成「假装不支持」；② 我的 token 估算是锚点式的，用真实 usage 校准，精度够用。**如果将来真要做精确预算，我会单独定义一个可选接口 `TokenCounter`，用类型断言探测，而不是塞进 `Provider`。**」

**备注讲解**：

接口定义（`internal/llm/provider.go`）：

```go
type Provider interface {
	Name() string                                               // 状态栏左侧：供应商名称
	Model() string                                              // 状态栏右侧：模型名
	Stream(ctx context.Context, req Request) <-chan StreamEvent // 发起一轮流式对话
}
```

「可选接口 + 类型断言」这个模式项目里已经有先例——`tool.SystemTool`：

```go
type SystemTool interface { IsSystem() bool }
func IsSystemTool(t Tool) bool { st, ok := t.(SystemTool); return ok && st.IsSystem() }
```

所以你可以说「项目里已经有这个模式，扩展成本很低」。

**可能的追问**：
- **「那 `Name()` 和 `Model()` 不是可以合成一个 `Info()` 吗？」** → 「合成一个返回结构体的方法确实更省调用，但 `Name`/`Model` 是两个语义完全不同的东西（一个是配置里的可读名、一个是真实的模型 ID），拆开更清楚。而且这两次调用都在 UI 渲染路径上，不是热路径。」

---

## Q3.2 ⭐ 如果要接第三家协议（比如 Google Gemini），要改哪些地方？

**面试官想听什么**：你的抽象是「真抽象」还是「假抽象」——**如果加一家要改 10 个文件，那抽象就失败了**。

**口述答案**：

> 「只改两个地方，不改上层任何代码。
>
> 第一，加一个适配器文件 `gemini.go`，实现 `Provider` 三个方法，并在 `llm.New()` 的 `switch` 里加一个 `case`；
> 第二，在 `config` 里加协议常量和默认的上下文窗口。
>
> **上层的 Agent 循环、工具系统、权限引擎、压缩逻辑一行都不用动。** 因为它们只认 `llm.Request` / `llm.StreamEvent` / `llm.Message` 这些协议无关的类型。
>
> 这也正是我做这层抽象的目的——**在 `agent.go` 里 grep 一下，你找不到任何一个 SDK 的 import**。」

**备注讲解**：

`agent.go` 的包注释直接写了这条约束：

```go
// Package agent 承载 ReAct 循环编排：多轮调 LLM → 权限判定 → 执行工具 → 结果回灌，直到任务完成。
// 对外吐出一条 Event 流供 TUI 渲染。只依赖 llm、tool、conversation、permission，不 import SDK，保持协议无关。
```

**「不 import SDK」是一条可以被验证的设计约束**——面试官如果真的去 grep，会发现 `agent` 包确实干净。

`llm.New()` 的扩展点：

```go
func New(cfg config.ProviderConfig) (Provider, error) {
	switch cfg.Protocol {
	case config.ProtocolAnthropic: return newAnthropicProvider(cfg)
	case config.ProtocolOpenAI:    return newOpenAIProvider(cfg)
	default: return nil, fmt.Errorf("不支持的协议类型: %s", cfg.Protocol)
	}
}
```

**可能的追问**：
- **「Gemini 的工具调用格式跟这两家都不同，你的抽象撑得住吗？」** → 「撑得住，因为我抽象的是**语义**不是**格式**。`ToolCall{ID, Name, Input}` 这三个字段是任何 function calling 协议的公共语义。真正可能出问题的是**能力差异**——比如 Gemini 可能不支持并行工具调用，或者它的 thinking 需要单独配置。这类差异我倾向于**在适配器内降级**（比如把并行调用合并成串行），而不是往接口上加能力标志位。**降级实现的复杂度，比污染抽象的复杂度低。**」

---

## Q3.3 ⭐ 流式接口为什么用 channel 而不是 callback？

**面试官想听什么**：并发模型的理解深度，以及你是否知道 channel 的代价。

**口述答案**：

> 「三个理由：
> 第一，**取消语义统一**。channel 方案里消费者 `range` 或者 `select { case ev := <-ch; case <-ctx.Done() }`，取消天然可中断；callback 方案里要么让 callback 返回 `bool` 表示停止（很丑），要么靠 panic（更丑）。
> 第二，**背压天然存在**。我用的是无缓冲 channel——消费者不读，生产者就阻塞。这在 Agent 场景下是对的：UI 卡住时不该让 LLM 请求继续往里灌数据。
> 第三，**组合性好**。消费端就是一个 `for ev := range stream` 循环，没有回调嵌套。
>
> 代价有两个，我都付出并处理了：
> 一是**每个请求多一个 goroutine**，而且必须在生产端 `defer close(ch)`，否则消费端 `range` 永远不会退出；
> 二是**调用点要处理「消费者消失」**——我在 `agent` 里所有发送都走一个 `emit` 函数，它带 `ctx.Done()` 分支，保证取消时不会永久阻塞。
>
> 而 `emit` 有大概 30 个调用点，每个都要处理「发送失败」的分支。**这是 channel 方案的隐性成本——代码比 callback 方案啰嗦不少。**」

**备注讲解**：

```go
// emit 尝试向 ch 发送事件，返回 true。若 ctx 已取消则返回 false。
func emit(ctx context.Context, ch chan<- Event, e Event) bool {
	select {
	case ch <- e:       return true
	case <-ctx.Done():  return false
	}
}
```

**为什么这个必须做**：`Run` 里的 goroutine 如果卡在 `ch <- e` 上，而 TUI 已经退出了，这个 goroutine 就**永久泄漏**——它持有 `conv`、`registry`、`provider` 的引用，永远不会被 GC。对一个长时间运行的终端程序，反复触发就是内存泄漏。

**可能的追问**：
- **「为什么不用带缓冲的 channel 减少阻塞？」** → 「缓冲只是把问题延后——缓冲满了还是要阻塞。而且缓冲会让 `ctx.Done()` 的响应变慢（要先把缓冲塞满才能走到 select）。**无缓冲 + select 是最诚实的方案：语义上就是「我发你收」，同步点清晰。**」
- **「无缓冲会不会拖慢性能？」** → 「会——TUI 每渲染一帧就卡住 agent 一次。实测在流式渲染场景下确实有这个开销（`bench_test.go` 就是在量这个）。但这是**有意的背压**：我宁可 agent 等 UI，也不要 UI 积压一堆待渲染的数据。见后面的 O(n²) 那道题。」

---

## Q3.4 ⭐ `ToolCall.Input` 为什么用 `json.RawMessage` 而不是 `map[string]any`？

**面试官想听什么**：对 Go 类型系统和 JSON 细节的敏感度。

**口述答案**：

> 「三个理由。
> 第一，**避免双重编解码**。用 `map` 意味着协议层解一次 JSON、工具层再序列化一次传给下游。`RawMessage` 是原样透传，零成本。
> 第二，**数字精度**。`map[string]any` 会把所有 JSON 数字解成 `float64`，大整数会失真。比如工具参数里有个 `timeout: 9007199254740993`，用 float64 就变成别的数了。
> 第三，**容忍模型的坏 JSON**。模型偶尔会产出不合法的参数（少个引号、多个逗号）。用 `RawMessage` 我可以原样传给工具，让工具的 `json.Unmarshal` 统一报「参数解析失败」——**错误处理集中在一处**，而不是在协议层就因为解析失败崩掉。」

**备注讲解**：

```go
type ToolCall struct {
	ID    string          // provider 侧调用 id；回灌结果时配对
	Name  string          // 工具名（注册中心按名查找）
	Input json.RawMessage // 拼接完成的 JSON 参数
}
```

对应的错误处理（比如 `bash.go`）：

```go
	var a bashArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return errorResult("参数解析失败: %v", err)
	}
```

**所有工具都遵循这个模式**——协议层不解析参数，工具自己解析并转成结构化错误。**这样协议层就完全没有「工具参数」这个概念**，它只是一段不透明的字节。

---

## Q3.5 ⭐⭐ Prompt Cache 是怎么做的？为什么系统提示要拆两段？

**面试官想听什么**：这是 Agent 工程里**最能看出成本意识**的一题。

**口述答案**：

> 「Prompt Cache 的核心机制是**按前缀命中**：你在请求里打一个缓存断点，从消息开头到这个断点之间的内容整段被缓存；下次请求只要这段**字节完全一致**就命中，Anthropic 的缓存读取价格大约是正常输入 token 的十分之一。
>
> 所以设计的关键是：**把「每轮都变的东西」从缓存区里挪出去**。
>
> 我的做法是把系统提示拆成两段——`Stable`（固定的七个模块：身份、约束、工作方式、工具策略、选择优先级、风格、输出格式）和 `Environment`（工作目录、平台、日期、git 状态、版本、模型）。**只给 Stable 打断点，Environment 不打。**
>
> 结构上就变成了：`[system: Stable(断点)] → [system: Environment] → [消息历史]`。Stable 这段从进程启动到退出**逐字节不变**，所以每一次请求都命中缓存。
>
> 具体收益：Stable 段大概 2–3K token，命中缓存后这部分按 10% 计价。在 25 轮循环里，省下来的是 25 × 2.7K × 90% ≈ 60K token 的钱。**更重要的是延迟**——缓存命中的 token 不需要重新 prefill，首 token 时间明显更短。」

**备注讲解**：

代码（`anthropic.go:123-137`）：

```go
func toAnthropicSystem(sys System) []anthropic.TextBlockParam {
	var blocks []anthropic.TextBlockParam
	if sys.Stable != "" {
		blocks = append(blocks, anthropic.TextBlockParam{
			Text:         sys.Stable,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),   // ← 断点
		})
	}
	if sys.Environment != "" {
		blocks = append(blocks, anthropic.TextBlockParam{
			Text: sys.Environment,                                     // ← 无断点
		})
	}
	return blocks
}
```

**注释里有一个真实的坑**：

```go
// Stable 块打缓存断点（必须用 NewCacheControlEphemeralParam 构造器，空字面量会被 omitzero 丢弃）
```

**这是被 SDK 的 `omitzero` 坑过的痕迹**。Anthropic SDK 的 `TextBlockParam` 上 `CacheControl` 字段带 `omitzero` tag——如果你写 `CacheControl: anthropic.CacheControlEphemeralParam{}`（零值字面量），序列化时会被整个 omit 掉，**断点静默失效**。必须用 `NewCacheControlEphemeralParam()` 构造器，它设置了一个内部的 `Type` 字段，让结构体不再是零值。

**「逐字节稳定」怎么保证？** 靠 `AssembleSystem`：

```go
func AssembleSystem(mods []Module) string {
	// 防御性拷贝后排序，避免副作用
	sorted := make([]Module, len(mods))
	copy(sorted, mods)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Priority < sorted[j].Priority })

	var parts []string
	for _, m := range sorted {
		if m.Content != "" { parts = append(parts, m.Content) }     // ← 空模块跳过，不产生多余空行
	}
	return strings.Join(parts, "\n\n")
}
```

三重保证：① **拷贝后排序**——不改调用方的切片（否则多次调用会因排序状态不同产生不同结果）；② **`SliceStable`**——同优先级顺序稳定（如果用 `sort.Slice`，Go 的快排是不稳定的，同优先级的模块顺序可能变）；③ **空模块跳过**——如果某个可选模块为空，直接不加入 `parts`，而不是加一个空字符串（那会在 `\n\n` 连接时产生多余的空行，**多一个字节缓存就失效**）。

**环境段的 `Date` 只到「天」**：

```go
	Date = time.Now().Format("2006-01-02")   // 只有日期，没有时间
```

如果精确到秒，环境段每轮都变——虽然它不打缓存断点，但它**在断点之后**，所以不影响 Stable 段的命中。不过日期还是会变（跨天），所以放在断点之后是对的。

**OpenAI 侧不一样**：

```go
	// 首条 system 消息 = Stable + "\n\n" + Environment（单条拼接兼容端点对多条 system 支持不一）；
	// Stable 居前缀使端点前缀缓存自动命中稳定部分。
```

OpenAI 的缓存是**自动**的（不需要显式断点），所以只要 Stable 在前面就能吃到前缀缓存。而拼成单条消息是为了兼容第三方端点。

**可能的追问**：
- **「命中了怎么验证？」** → 「用 `Usage` 里的 `CacheRead` 字段。我把它透传到 TUI 的状态栏，如果一直是 0，说明没命中。**这个字段就是我调试缓存的主要手段**——Anthropic 返回 `cache_read_input_tokens`，OpenAI 返回 `prompt_tokens_details.cached_tokens`。」
- **「历史消息变长了，会不会破坏缓存？」** → 「**不会**，而且是反过来的——前缀缓存正是为了「前缀不变、后面追加」这个模式设计的。每轮我做的事就是「在末尾追加消息」，前缀（Stable + Environment + 已有历史）都没变，所以缓存一直有效。**真正会破坏缓存的是压缩**——它重写了历史，后面全失效。所以我压缩的阈值留了 33000 token 的余量，尽量晚触发。」

---

## Q3.6 ⭐⭐ Anthropic 和 OpenAI 的工具调用格式差异在哪？你踩过什么坑？

**面试官想听什么**：是否真的两家都跑通过，而不是只写了一个。

**口述答案**：

> 「最大的差异在**工具结果怎么表达**。
>
> Anthropic 的 Messages API 里没有 `tool` 这个角色。工具结果必须是一条 **user 消息里的 `tool_result` content block**，`tool_use_id` 跟前面 assistant 的 `tool_use` block 配对。而且**一条 user 消息里可以塞多个 `tool_result`**。
>
> OpenAI 完全相反——有独立的 `role: "tool"` 消息，**每个结果一条消息**，靠 `tool_call_id` 配对。
>
> 所以我的适配器里，同一个 `RoleTool` 的协议无关消息，在 Anthropic 侧变成一条 user 消息带 N 个 block，在 OpenAI 侧变成 N 条 tool 消息。
>
> 第二个差异是**工具调用的参数格式**：Anthropic 的 `input` 是一个 **JSON 对象**，OpenAI 的 `function.arguments` 是一个 **JSON 字符串**。字符串意味着理论上可能是坏 JSON，所以我加了兜底——空字符串时替换成 `{}`。
>
> 第三个差异是**并行工具调用**：Anthropic 靠 `stop_reason == "tool_use"` 判断本轮是工具调用；OpenAI 要看 `choices[0].message.tool_calls` 是否非空。」

**备注讲解**：

```go
// Anthropic：把所有 tool_result 塞进一条 user 消息
case RoleTool:
	var blocks []anthropic.ContentBlockParamUnion
	for _, tr := range m.ToolResults {
		blocks = append(blocks, anthropic.NewToolResultBlock(tr.ToolCallID, tr.Content, tr.IsError))
	}
	result = append(result, anthropic.NewUserMessage(blocks...))
```

```go
// OpenAI：每个结果一条独立 tool 消息
case RoleTool:
	for _, tr := range m.ToolResults {
		result = append(result, openai.ToolMessage(tr.Content, tr.ToolCallID))
	}
```

**OpenAI 的空参数兜底**：

```go
		for _, tc := range acc.Choices[0].Message.ToolCalls {
			args := tc.Function.Arguments
			if args == "" { args = "{}" }        // ← 兜底
			calls = append(calls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(args)})
		}
```

**为什么需要**：工具没有参数时，OpenAI 有时会返回空字符串而不是 `"{}"`。如果不兜底，工具的 `json.Unmarshal([]byte(""))` 会报「unexpected end of JSON input」。

**工具定义格式也不同**：

```go
// Anthropic 要显式拆出 properties 和 required
schema := anthropic.ToolInputSchemaParam{
	Properties: t.InputSchema["properties"],
	Required:   toStrings(t.InputSchema["required"]),
}

// OpenAI 直接透传整个 JSON Schema
Parameters: shared.FunctionParameters(t.InputSchema)
```

**Anthropic 为什么只取 `properties` 和 `required`？** 因为它的 `ToolInputSchemaParam` 结构体只有这两个字段——`type: "object"` 是隐含的。所以 `toStrings` 这个辅助函数要处理 `[]interface{}` 和 `[]string` 两种形态（JSON 反序列化出来的是前者，手写的可能是后者）。

**可能的追问**：
- **「`toStrings` 为什么要处理两种类型？」** → 「因为手写的 JSON Schema 里我写的是 `"required": []string{"command"}`（Go 字面量），而从 YAML/JSON 反序列化出来的是 `[]interface{}`。**类型断言的实现要覆盖两条路径**，否则某一边会静默丢字段——`required` 丢了模型就不知道哪些参数必填。」

---

## Q3.7 ⭐ 为什么「历史里有工具交互」就关闭扩展思考？

**面试官想听什么**：是否理解 Anthropic 的协议约束，而不是抄了个 `if`。

**口述答案**：

> 「因为 Anthropic 有个硬约束：**如果最后一条 assistant 消息带 `tool_use`，那么它后面必须紧跟一条带 `tool_result` 的消息**，而且这种请求不允许开 thinking。
>
> 我的判定函数 `hasToolHistory` 做了个**保守判断**——只要历史里**出现过**工具调用就返回 true，而不是只看最后一条。理由：宁可牺牲一些思考能力，也不能让请求 400。**thinking 是锦上添花，请求成功是底线。**」

**备注讲解**：

```go
// 启用扩展思考（历史含工具交互时关闭，避免 400）
if p.cfg.Thinking && !hasToolHistory(req.Messages) {
	params.Thinking = anthropic.ThinkingConfigParamOfEnabled(16000)
}
```

```go
// hasToolHistory 检查消息历史中是否包含工具交互。
func hasToolHistory(msgs []Message) bool {
	for _, m := range msgs {
		if m.Role == RoleTool || len(m.ToolCalls) > 0 { return true }
	}
	return false
}
```

**实际后果**：在一个多轮 Agent 会话里，从第二轮开始历史里就有工具调用了——所以**扩展思考实际上只在第一轮生效**。

**这是个真实的能力损失**，被追问时要承认：

> 「代价是——在一个多轮任务里，从第二轮开始实际上就没有扩展思考了，只有第一轮有。这是 Anthropic 协议约束下的妥协。
>
> 更精确的做法是**只在「最后一条 assistant 带 tool_use 且后面没跟 tool_result」时才关**，其他情况（历史中间有工具调用、但结尾是正常的文本回合）应该可以开着。我之所以做保守判定，是因为当时遇到了 400 但没完全定位清楚边界，就先用最严的条件保证跑通。**如果继续做，第一件要改的就是把这个判定收窄。**」

**这个回答的价值**：它承认了一个真实的能力损失，并且给出了精确化的方向——**这正是「懂细节」和「抄代码」的区别**。

---

## Q3.8 ⭐ 用字符串匹配识别「上下文超长」，这不脆弱吗？

**面试官想听什么**：你会不会为自己的代码辩护到底，还是能清醒地看到弱点。

**口述答案**：

> 「脆弱，我承认。但先讲清楚**为什么只能这么做**：两家的 SDK 都没有提供结构化的「超长」错误类型，只有泛化的 API 错误，所以我只能匹配错误文案。
>
> 然后讲清楚**失效的后果有多严重**——这是关键：**不会崩，只会降级**。识别失败意味着它变成一个普通错误，走 `emit(Event{Err})` 结束本轮，用户看到报错。**代价是失去「紧急压缩后自动重试」这个体验，不是正确性问题。**
>
> 最后讲**为什么我不太担心**：因为紧急压缩是**最后一道兜底**。正常路径下，我的本地 token 估算——锚点加增量——应该在 provider 报错之前就触发自动压缩了。我留了 13000 token 的安全余量就是干这个的。**紧急压缩是「估算失灵」时的保险，不是主路径。**
>
> 真要根治，正确方向是**把「事后识别」变成「事前不可能发生」**：用 Anthropic 的 count_tokens 端点做精确计数，把安全余量从 13000 降下来。但那是另一个取舍——多一次网络往返换精度。」

**备注讲解**：

```go
func wrapAnthropicPTL(err error) error {
	errStr := err.Error()
	if strings.Contains(errStr, "prompt is too long") ||
		strings.Contains(errStr, "context_length") ||
		strings.Contains(errStr, "too many tokens") {
		return fmt.Errorf("%w: %v", ErrPromptTooLong, err)
	}
	return err
}
```

```go
// OpenAI 那版更弱，最后一个条件是复合的：
	if strings.Contains(errStr, "context_length_exceeded") ||
		strings.Contains(errStr, "maximum context length") ||
		strings.Contains(errStr, "too long") ||
		strings.Contains(errStr, "token") && strings.Contains(errStr, "exceed") {
```

**OpenAI 那个复合条件其实是多余的**——`"too long"` 已经覆盖了大部分情况，而且 `token && exceed` 的组合有误伤风险（比如某个不相关的错误里同时出现这两个词）。

**注意 `errors.Is` 的用法**（这是 Go 错误包装的正确姿势）：

```go
	if sErr != nil && errors.Is(sErr, llm.ErrPromptTooLong) && !emergencyRetried {
```

配合 `fmt.Errorf("%w: %v", ErrPromptTooLong, err)` 使用——`%w` 建立包装链，`errors.Is` 沿链查找。这样上层能精确识别，同时保留原始错误信息。

**可能的追问**：
- **「第三方兼容端点返回中文错误怎么办？」** → 「会失效。这是这个方案的真实盲区。**一个改进方向是把判定逻辑从「匹配文案」改成「匹配 + 试探」**：收到任何非预期错误时，如果本地估算显示已经接近窗口上限，就主动压一次再重试。**用本地状态兜住外部协议的不确定性**——这比猜文案可靠。」

---

## Q3.9 ⭐ 多 Provider 切换是怎么实现的？有什么问题？

**面试官想听什么**：有没有意识到自己项目里的隐藏缺陷。

**口述回答要点**：

> 「配置里 `providers` 是个列表，启动时如果有多于一个，TUI 先弹一个选择界面，用户选中后构造对应的 Provider 实例，状态栏显示选中的名字和模型。
>
> 但**这里有个我已知的缺陷**：`ContextWindow` 是在 `main.go` 里构造 `SessionRuntime` 时算好的，代码写的是 `cfg.Providers[0].EffectiveContextWindow()`——**写死了第 0 个**。所以如果用户切到第 2 个 provider，压缩阈值还是按第 0 个算的。
>
> 后果是：如果第 0 个是 200K 窗口、第 2 个是 128K，那么用第 2 个的时候压缩阈值会设成 200K-33K=167K，**远超 128K 的实际窗口**——请求会被 provider 拒绝，然后走紧急压缩兜底。功能没坏（紧急压缩能救回来），但**体验上会先撞一次墙**。
>
> 修法很简单：把 `ContextWindow` 从构造期改成**切换到 provider 时同步更新**，或者在 `Agent.Run` 里从 `provider` 查而不是从 runtime 读。」

**备注讲解**：

```go
	runtime := &agent.SessionRuntime{
		...
		ContextWindow: cfg.Providers[0].EffectiveContextWindow(),   // ← 写死第 0 个
	}
```

**为什么这个回答有杀伤力**：它展示了两件事——① 你真的读过 `main.go`（不是只知道 TUI）；② 你能推理出**缺陷的实际后果**（不是「有个 bug 但不知道影响」）。

---

## Q3.10 流式返回的 `tool_use` 参数是一段一段来的，你怎么拼？

**面试官想听什么**：对 SSE 流式协议的理解。

**口述答案**：

> 「工具参数是**增量拼出来的**——Anthropic 会用 `input_json_delta` 事件一段一段吐 JSON 片段，OpenAI 用 `tool_calls[].function.arguments` 的 delta。
>
> 两家 SDK 都提供了解析器：Anthropic 是 `Message.Accumulate(event)`，OpenAI 是 `ChatCompletionAccumulator.AddChunk(evt)`。我在流循环里每收到一个事件就喂给解析器，**流结束后再从解析器里一次性取出完整的工具调用列表**。
>
> 关键点是：**文本增量要立刻上抛给 UI（要流式显示），但工具调用只能等流结束**——因为 `input` 在流中间是不合法的 JSON，提前抛出去也没用。
>
> 代码里有一条注释写了这个区分：`ThinkingDelta / InputJSONDelta 由 Accumulate 缓冲，不上抛`。所以我在流循环里只处理 `TextDelta`。」

**备注讲解**：

```go
		acc := anthropic.Message{}
		for stream.Next() {
			event := stream.Current()
			if err := acc.Accumulate(event); err != nil { ... }     // ← 全部喂给 Accumulator

			switch evt := event.AsAny().(type) {
			case *anthropic.ContentBlockDeltaEvent:
				if delta, ok := evt.Delta.AsAny().(anthropic.TextDelta); ok {
					select { case ch <- StreamEvent{Text: delta.Text}: case <-ctx.Done(): return }
				}
				// ThinkingDelta / InputJSONDelta 由 Accumulate 缓冲，不上抛
			}
		}
		// 流结束后才取工具调用
		if acc.StopReason == anthropic.StopReasonToolUse {
			for _, block := range acc.Content {
				toolBlock := block.AsToolUse()
				if toolBlock.ID != "" { calls = append(calls, ToolCall{ID: toolBlock.ID, Name: toolBlock.Name, Input: json.RawMessage(toolBlock.Input)}) }
			}
		}
```

**`Usage` 也是流结束后才发的**：

```go
		// 上抛本轮 token 用量（流结束后 acc.Usage 完整；含缓存字段 F4/N6）
```

因为 usage 在流的最后一个事件（`message_delta`）里才完整。

**这个顺序约束在 `StreamEvent` 的定义里写清楚了**：

```go
//	Text 非空 → 文本增量（正文或 preamble）
//	ToolCalls 非空 → 模型请求执行这些工具（Done 之前发出）
//	Usage 非空 → 本轮 token 用量（Done 之前一次性发出）
```

---

# 第二部分：ReAct Agent 循环

---

## Q3.11 ⭐⭐ 你的 ReAct 循环的停止条件有哪些？为什么要这么多？

**面试官想听什么**：**收敛性**。这是 Agent 工程最核心的问题——如果一个循环没有可靠的停止条件，它就是个烧钱的死循环。

**口述答案**：

> 「一共 5 条，我按「正常 → 异常」的顺序说：
>
> **① 自然完成**：模型这一轮没有请求任何工具，只回了文本。这是正常路径——它的语义是「我干完了」。
>
> **② 连续未知工具**：`allUnknown` 判定——**一整轮里模型请求的每一个工具名在注册中心都查不到**，计一次；连续 3 轮这样的全部落空就停。中途只要它试对了一次，计数器清零。
>
> **③ 迭代上限**：25 轮。
>
> **④ 用户取消**：`ctx.Done()`。
>
> **⑤ 请求出错**：provider 返回错误，且不是可恢复的「超长」（超长会先走紧急压缩重试一次）。
>
> 为什么这么多？因为它们**覆盖的是完全不同的失败模式**：①是正常收敛；②是模型能力问题（幻觉）；③是任务复杂度失控或者模型陷入循环；④是用户意图；⑤是外部依赖故障。**少任何一个都会出现「卡住不放」的场景。**
>
> 而且更关键的是——**所有 5 条路径都会保证历史末尾是一条 assistant 消息**。这是个容易漏的点，我下面可以说。」

**备注讲解**：

| 条件 | 代码 | 提示文案 |
|---|---|---|
| 自然完成 | `if len(calls) == 0` | 无 |
| 未知工具 | `if unknownRun >= maxUnknownRun`（3） | 「（连续多轮只请求到未注册的工具，自动停止。）」 |
| 迭代上限 | `for iter := 1; iter <= maxIterations; iter++`（25） | 「（已达最大迭代轮数 25，自动停止；可继续发消息推进。）」 |
| 取消 | `if ctx.Err() != nil` | 「（已取消。）」 |
| 出错 | `if sErr != nil` | 「（请求出错，本轮已中断。）」 |

**注意最后一个文案里的细节**：「可继续发消息推进」——因为**停止不等于会话结束**，用户可以直接发下一句话接着干。这比「任务失败」的提示友好得多。

**可能的追问**：
- **「如果模型一直调用同一个工具、参数也一样，会怎么办？」** → **这是个真缺陷，要承认**：「现在没有**重复调用检测**。如果模型陷入「读同一个文件 → 说还需要看 → 再读同一个文件」的循环，它只受 25 轮上限约束，会白烧 20 多轮 token。**应该加一个检测：同一个（工具名 + 参数）在最近 N 轮内重复出现超过 M 次就中断。** 我做了「未知工具」的检测但没做这个，是因为前者更容易实现（不需要比较参数），而且幻觉工具名在实际使用中更常见。但这确实是个应该补的洞。」

---

## Q3.12 ⭐⭐⭐ 「只读工具同轮并发、副作用工具串行保序」是怎么实现的？

**面试官想听什么**：这是简历原文里的技术点，**必问**。他想看你能不能把算法讲清楚，以及你懂不懂为什么要这么做。

**口述答案**（这段要背熟）：

> 「核心是一个 **`executeBatched` 函数，算法是「保序分批并发」**。
>
> 我用一个游标 `i` 扫过模型请求的工具列表：
> - 如果 `calls[i]` 是只读的，我就**向右吃入一段连续的只读区间 `[i, j)`**，把这一整段作为一个批次并发执行；
> - 如果 `calls[i]` 是有副作用的，就单独串行执行它，游标前进一格。
>
> 只读批次内部有 5 个阶段，顺序很重要：
> **① 逐个做 Hook 拦截 + 权限判定**——被拒的记进一个 `preDenied` map，**但它们不影响批次的并发性**，因为只读工具在模式兜底里恒为「允许」，不会阻塞；
> **② 先按原始顺序 emit 所有 Start 事件**——让 UI 能立刻显示「三个工具同时在跑」；
> **③ 被拒的项预先写好错误结果**，不纳入并发执行；
> **④ 未被拒的用 `sync.WaitGroup` 并发执行**；
> **⑤ 再按原始顺序 emit 所有 End 事件**，并派发 PostToolUse Hook。
>
> 结果集合是**预分配的 `make([]llm.ToolResult, len(calls))`**，每个 goroutine 只写自己那个索引 —— 因为写的是不同的内存位置，**不需要任何锁**，也不会串位。
>
> 下游拿到的 `results` 天然就是按模型请求的原始顺序排列的。」

**备注讲解**：

关键代码（`agent.go:518-660`）：

```go
	for i < len(calls) {
		if ctx.Err() != nil { /* 给剩余全部填取消结果 */ return results, false }

		if a.registry.IsReadOnly(calls[i].Name) {
			// 吃入连续只读区间 [i, j)
			j := i
			for j < len(calls) && a.registry.IsReadOnly(calls[j].Name) { j++ }

			// 阶段①：权限检查，被拒项记入 preDenied
			preDenied := make(map[int]string)
			for k := i; k < j; k++ {
				hr := a.dispatchHook(ctx, hook.EventPreToolUse, ...)
				if hr.Blocked { results[k] = hookBlockedResult(...); preDenied[k] = ...; continue }
				d, reason := a.eng.Check(mode, calls[k], true)      // ← readOnly=true
				if d == permission.Deny { preDenied[k] = reason }
			}

			// 阶段②：按序 emit 所有 Start
			for k := i; k < j; k++ { emit(ctx, ch, Event{Tool: &ToolEvent{Name: calls[k].Name, Args: argPreview(calls[k].Input), Phase: PhaseStart}}) }

			// 阶段③：被拒项预填结果
			for k := i; k < j; k++ {
				if reason, denied := preDenied[k]; denied {
					results[k] = llm.ToolResult{ToolCallID: calls[k].ID, Content: reason, IsError: true}
				}
			}

			// 阶段④：并发执行未被拒的
			var wg sync.WaitGroup
			for k := i; k < j; k++ {
				if _, denied := preDenied[k]; denied { continue }
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					tctx, cancel := context.WithTimeout(ctx, tool.DefaultTimeout)
					defer cancel()
					r := a.registry.Execute(tctx, calls[idx].Name, calls[idx].Input)
					results[idx] = llm.ToolResult{ToolCallID: calls[idx].ID, Content: r.Content, IsError: r.IsError}
				}(k)
			}
			wg.Wait()

			// 阶段⑤：按原始顺序 emit End + PostToolUse
			for k := i; k < j; k++ { emit(ctx, ch, Event{Tool: &ToolEvent{... Phase: PhaseEnd ...}}) ; a.dispatchHook(ctx, hook.EventPostToolUse, ...) }

			i = j
		} else {
			// 串行执行单个有副作用工具……
			i++
		}
	}
```

**「为什么写 `results[idx]` 不需要锁」** 是个可以展开讲的点：

Go 的 slice 元素写入 `results[idx] = x` 实际上是「计算偏移量 + 写内存」。不同的 `idx` 对应不同的内存地址，所以多个 goroutine 写不同索引**没有数据竞争**——这在 Go 内存模型里是安全的（不同的内存位置）。

**前提是 `idx` 是唯一的**——代码里每个未拒绝的 `k` 只启动一个 goroutine，所以成立。

**可能的追问**：
- **「为什么先 emit 所有 Start，而不是 Start/End 成对发？」** → 「为了让 UI 显示「三个工具同时跑」。如果成对发，UI 上看到的就是一个跑完再跑下一个，**用户会以为没有并发**——虽然实际上并发了，但视觉上没有体现。这是个纯 UI 驱动的小设计，但它让这个并发能力变得「可见」。」
- **「如果并发执行的某个工具 panic 了怎么办？」** → **这是真缺陷**：「现在**没有 recover**。任何一个工具在 goroutine 里 panic，整个进程就挂了。`wg.Wait()` 也不会被影响（panic 会直接终止进程）。`task.Manager` 那层的 goroutine 都有 `recover()`，但 `executeBatched` 这里没有。**应该加：`defer func(){ if r := recover(); r != nil { results[idx] = 错误结果 } }()`**。工具本身的设计是「绝不 panic」——`tool.Result` 的包注释写了「所有工具执行失败均以 `Result{IsError:true}` 返回，绝不 panic」——但那是**约定不是强制**，而 `mcpTool` 是外部代码路径，更不该信任。这是一个应该补的防御。」
- **「只读的判定依据是什么？」** → 「`Tool.ReadOnly()` 这个接口方法。内置工具里 `read_file` / `glob` / `grep` 返回 true，`write_file` / `edit_file` / `bash` 返回 false。MCP 工具**严格只信远端声明的 `annotations.readOnlyHint == true`**——没声明就当有副作用，串行 + 走完整权限。」

---

## Q3.13 ⭐ 为什么权限检查不破坏只读并发？

**面试官想听什么**：这是项目 spec 里一条明确的不变量（N3），问出来是看你知道不知道「加权限不该让性能倒退」。

**口述答案**：

> 「因为**只读工具永远不会触发 Ask**。
>
> 权限判定的模式兜底里有一条：`if cat == CategoryRead || mode == ModeBypass { return Allow, "" }`——只读类直接返回 `Allow`，**不会进入人在回路的阻塞等待**。
>
> 所以权限检查可以在并发之前**同步地、逐个地跑完**：每个只读工具要么被允许、要么被 deny 规则拦下。**没有一个是需要等待用户输入的**，所以不会把并发退化成串行。
>
> 这个约束我在 spec 里显式写成了不变量：『只读工具的并发执行不因权限检查退化为串行（只读永不触发 Ask）』，并且有对应的验收项验证——批量只读调用不弹人在回路。」

**备注讲解**：

**关键在 `modeFallback` 的第一行**：

```go
func modeFallback(mode Mode, cat Category) (Decision, string) {
	// 只读 / bypass 全 Allow
	if cat == CategoryRead || mode == ModeBypass {
		return Allow, ""
	}
	...
}
```

**但注意：只有「模式兜底」这一层保证 Allow**。如果用户写了 `deny: Read(.env)`，规则层会先拦下来。**这时候它会返回 `Deny` 而不是 `Ask`**——而 `Deny` 是同步返回的，不阻塞并发。所以不变量仍然成立：**只读工具永远不会让权限检查阻塞**。

**设计上的一致性**：`Deny` 和 `Allow` 都是**同步决策**，只有 `Ask` 是**异步阻塞**。而只读工具只可能产生前两者。

---

## Q3.14 ⭐ 工具被拒绝之后为什么不中断循环？

**面试官想听什么**：对 Agent 自我纠正能力的理解。

**口述答案**：

> 「因为**拒绝原因是给模型看的有效信息**。
>
> 我把拒绝原因当成一条 `IsError: true` 的工具结果回灌进历史，内容是比如「路径在项目目录之外：`/etc/passwd`」或者「命中危险命令黑名单：`rm -rf /`」。模型看到这个具体原因之后，**大概率会自己换个路径重试**——比如改成读项目内的文件。
>
> 如果直接中断整个循环，模型就没机会纠正，用户的体验是「我问了一句话，Agent 说不行，然后就结束了」。**而实际上大部分被拒的操作都是可以换个方式完成的。**
>
> 这其实也是 Anthropic 官方推荐的工具设计模式——**工具失败要返回结构化的错误信息而不是抛异常**，让模型有机会自己修。我的 `tool.Result` 里那个 `IsError` 布尔就是为这个设计的：它不是「Go 层面的错误」，而是「给模型看的结果类型标记」。」

**备注讲解**：

```go
// Result 工具执行结果——永远以值类型返回，从不返回 Go error。
type Result struct {
	Content string // 回灌给模型的文本（已截断/带行号等）
	IsError bool   // true 表示结构化错误，Content 即错误描述
}
```

**注意接口签名**：

```go
Execute(ctx context.Context, args json.RawMessage) Result     // ← 不返回 error
```

**整个 Tool 接口没有 `error` 返回值**。这是刻意的——**在接口层面就消除「工具可能失败」这个 Go 层的概念**，所有失败都是「结果的语义」。

**这条设计的好处**：`Registry.Execute` 不需要处理 `if err != nil`，不需要决定「错误要不要往上抛」，调用方永远拿到一个可用的 `Result`。

**可能的追问**：
- **「如果模型反复被拒、一直重试呢？」** → **承认缺陷**：「现在靠 25 轮上限兜底。理想应该加「同一工具被拒 N 次后就不再尝试」或者「在提示里注入一条『该操作已被拒绝，请换方案』」。**目前没有专门处理，这属于「未知工具检测」应该覆盖但没覆盖的场景。**」

---

## Q3.15 ⭐⭐ 什么是 `ensureAssistantTail`？不处理会怎样？

**面试官想听什么**：协议细节的实战经验——**这是「跑通过」和「没跑通过」的分水岭**。

**口述答案**：

> 「它是一个**历史一致性兜底**函数：如果对话历史的最后一条不是 assistant 角色，就补一条 assistant 消息。
>
> 为什么必须做？因为 **Anthropic 的 Messages API 要求角色严格交替，而且请求的最后一条消息在语义上应该是 assistant（或者说：user 和 assistant 必须成对）**。在我的循环里，正常情况下 `AddAssistantWithToolCalls` 之后必然跟着 `AddToolResults`，历史末尾是 tool 消息（在 Anthropic 侧表现为一条带 `tool_result` 的 user 消息）。
>
> **如果这时候异常退出**——用户按了 Esc、网络断了、撞上迭代上限——历史就停在一条 user/tool 消息上。用户再发一条新消息时，请求体变成 `... user(tool_result) → user(新消息)`，**Anthropic 直接返回 400**。
>
> 所以我在**所有 5 条退出路径**上都调了这个函数：取消、流错误、上下文管理错误、未知工具停止、迭代上限。」

**备注讲解**：

```go
// ensureAssistantTail 若历史末尾不是 assistant 角色，补一条兜底文本。
func ensureAssistantTail(conv *conversation.Conversation, fallback string) {
	if conv.LastRole() != llm.RoleAssistant {
		conv.AddAssistant(fallback)
	}
}
```

5 个调用点（`agent.go`）：

| 行 | 场景 | 兜底文案 |
|---|---|---|
| 201 | 取消（emit Iter 失败） | `noticeCancelled` |
| 255 | 上下文管理出错 | `noticeStreamErr` |
| 301 | 紧急压缩失败 | `noticeStreamErr` |
| 326 | 流请求出错 | `noticeStreamErr` |
| 389 | 执行中被取消 | `noticeCancelled` |
| 396 | 连续未知工具 | `noticeUnknownTools` |
| 405 | 迭代上限 | `noticeMaxIter` |

**兜底文案的分工很讲究**：
- 这些文本**既作为 `Event{Notice}` 推给 UI，也作为历史里的 assistant 内容**。所以它们必须**语义上是「一个合理的助手回复」**，不能是「[错误]」这种机器语言——因为模型下一轮会读到它。比如「（已达最大迭代轮数 25，自动停止；可继续发消息推进。）」就是在**告诉模型「你被截断了，用户可能接着说」**。

**可能的追问**：
- **「`ensureFinal` 又是干什么的？」** → 「类似的思路，但针对的是另一个场景：模型返回了**空文本且没有工具调用**。这时候如果直接把空字符串写进历史，会出现空的 assistant 消息——下一轮请求时某些端点会报错（或者模型会困惑）。所以 `ensureFinal` 会兜底写「（任务已完成）」，并且通过一个 `select` + `default` 非阻塞地把这个占位文本也推给 UI，保证 UI 和历史的显示一致。」

---

## Q3.16 ⭐ 「连续幻觉检测」为什么是「整轮全未知」而不是「单个未知」？

**面试官想听什么**：你的阈值设计有没有真实依据，还是拍脑袋定的 3。

**口述答案**：

> 「因为**要给模型自我纠正的空间**。
>
> 想象一下模型一次请求了 `[read_file, reaad_file]`——第二个名字拼错了。如果按「单个未知就计数」，这一轮就计一次。但实际上下一轮它大概率能自己发现并纠正。**我要求「一整轮里每一个工具名都查不到」才计数，就是为了区分「偶尔打错字」和「它根本不知道有什么工具可用」。**
>
> 而且中途只要有一轮正常，计数器就清零——这样它有一次成功就说明在正常工作。
>
> 至于为什么是 3 轮：一轮可能是偶然，两轮可能是上下文噪音，**连续三轮全部落空，基本可以确定它已经「不知道自己在干什么」了**。再让它试下去只是烧 token。」

**备注讲解**：

```go
// allUnknown 判断所有调用是否都请求了注册中心不存在的工具。
func allUnknown(registry *tool.Registry, calls []llm.ToolCall) bool {
	if len(calls) == 0 { return false }
	for _, c := range calls {
		if _, ok := registry.Get(c.Name); ok { return false }    // 有一个认识 → false
	}
	return true
}
```

```go
			// 统计未知工具
			if allUnknown(a.registry, calls) { unknownRun++ } else { unknownRun = 0 }
```

**为什么需要这个检测**：`Registry.Execute` 对未知工具已经有兜底——返回 `Result{Content: "未知工具: xxx", IsError: true}`。**但这只是「不崩」，不解决「它一直试」**。如果没有这个检测，模型可能陷在「请求不存在的工具 → 收到错误 → 再请求一次」的循环里，直到撞 25 轮上限——**白烧 22 轮的 token**。

**`maxUnknownRun = 3` 的注释写得很精确**：

```go
	maxUnknownRun        = 3  // 连续「整轮只产生未知工具调用」的迭代数上限（F2）
```

**子 Agent 用的是更严的 2**（`run_to_completion.go` 的 `maxUnknownRunSub = 2`）——因为子 Agent 的轮数预算本来就小（默认 25 但内置角色配的是 15–30），早点停更划算。

**可能的追问**：
- **「如果模型请求了 `[read_file, fake_tool]`，会检测到吗？」** → 「**不会**——因为有一个认识的，`allUnknown` 返回 false，计数器清零。这个工具名会被 `Registry.Execute` 兜底成「未知工具」错误回灌给模型，模型下一轮通常会放弃它。**这是刻意的宽松。**」

---

## Q3.17 ⭐⭐ 用户取消是怎么处理的？channel 会不会泄漏？

**面试官想听什么**：Go 并发的基本功 + 你是否想过 goroutine 泄漏。

**口述答案**：

> 「取消有两个层次。
>
> **第一个层次是「本轮取消」**：TUI 按 Esc 或 Ctrl+C 时，调用 `turnCancel()`，触发 `ctx.Done()`。Agent 侧所有 `emit` 都会立刻返回 false，所有循环开头的 `ctx.Err() != nil` 检查都会命中，然后走统一的收尾路径。
>
> **最关键的一点是：无论是否取消，工具结果都要回灌**。代码注释写了「无论是否取消都回灌工具结果」。因为历史里已经有了一条「assistant 请求了这 3 个工具」的记录，**如果不补工具结果，历史就是残缺的**——下次请求 Anthropic 会报「`tool_use` 没有对应的 `tool_result`」。所以取消路径会**给所有还没执行的工具填一个「（已取消。）」的错误结果**，然后 `ensureAssistantTail` 补一条 assistant，保证历史完整。
>
> **第二个层次是「程序退出」**：`m.cancel()` 取消根 context，同时 `tea.Quit`。
>
> 至于 channel 泄漏——我专门处理了这个。所有发送都走 `emit` 函数，它带 `ctx.Done()` 分支；所有接收侧（TUI 的 `waitForEvent`）在 channel 关闭时返回一个 `Done` 事件，不会永久阻塞。**唯一的泄漏风险是「agent 卡在 `ch <- e` 上而消费者永远不读」**——而 `emit` 的 `ctx.Done()` 分支就是解决这个的：只要 ctx 被取消，阻塞立刻解除。」

**备注讲解**：

**取消路径的完整收尾**（`agent.go:522-535`）：

```go
	for i < len(calls) {
		// 取消检查
		if ctx.Err() != nil {
			for k := i; k < len(calls); k++ {
				if results[k].ToolCallID == "" {
					results[k] = llm.ToolResult{
						ToolCallID: calls[k].ID,
						Content:    noticeCancelled,
						IsError:    true,
					}
				}
			}
			return results, false
		}
		...
```

**注意 `if results[k].ToolCallID == ""` 这个判断**——它保证**已经执行完的工具不会被覆盖**。如果前两个工具已经执行了、第三个还没执行，取消时只填第三个。

**返回的 `completed = false`** 让上层知道被取消了：

```go
			// 无论是否取消都回灌工具结果
			conv.AddToolResults(results)

			// 执行中被取消——最高优先级终止
			if !completed {
				ensureAssistantTail(conv, noticeCancelled)
				return
			}
```

**注释「最高优先级终止」很关键**——取消检查在「连续未知工具检测」之前，意味着**取消优先于其他所有终止原因**。

**TUI 侧的兜底**（`tui.go`）：

```go
		// Ctrl+C：streaming/approving 时取消本轮，否则退出程序
		if msg.String() == "ctrl+c" {
			if m.state == stateStreaming || m.state == stateApproving {
				if m.state == stateApproving && m.pending != nil {
					// 兜底解除 agent 阻塞
					select {
					case m.pending.Respond <- permission.OutcomeDenyOnce:
					default:
					}
				}
				if m.turnCancel != nil { m.turnCancel() }
				return m, waitForEvent(m.events)
			}
			if m.cancel != nil { m.cancel() }
			return m, tea.Quit
		}
```

**那个「兜底解除 agent 阻塞」非常关键**：如果 agent 正卡在审批弹窗的 `select` 上等回复，光 `turnCancel()` 也能让 `ctx.Done()` 触发、`select` 返回 false——**双保险**。但它的作用是让取消**更快生效**，因为 `Respond` 的 `select` 和 `ctx.Done()` 的 `select` 是同一个 `select` 的两个 case，随机选一个。

**注意那个 `default:` 分支**——非阻塞发送。因为 `Respond` 缓冲是 1，如果已经有值在里面（理论上不该有），非阻塞写会失败但不会卡住 UI。

**可能的追问**：
- **「`turnCancel` 和 `cancel` 为什么要分开？」** → 「语义完全不同。`cancel` 是**程序级**的根 context，取消它意味着退出；`turnCancel` 是**本轮**的 context，取消它只是中断这一轮对话，会话继续。如果不分开，按 Esc 取消一轮就会把整个程序干掉。**这也是为什么 `submitMessage` 里每轮都用 `context.WithCancel(context.Background())` 重新建一个 ctx。**」

---

## Q3.18 ⭐ `Run` 为什么返回 channel 而不是同步返回结果？

**面试官想听什么**：API 设计的判断力。

**口述答案**：

> 「因为 **TUI 需要中间状态**。
>
> 如果 `Run` 同步返回最终文本，用户在几秒到几十秒里只能看到一个转圈。而返回 channel 之后，我能实时吐出去这些东西：**进入第几轮迭代**、**哪个工具开始跑了、参数是什么**、**工具跑完了、结果是成功还是失败**、**每轮的 token 用量**、**是否需要用户在回路批准**、**上下文是否在压缩**。
>
> 有了这些中间事件，TUI 才能做出真正的流式体验——工具行实时出现、spinner 显示「第 3 轮」、状态栏实时涨 token 数、审批弹窗能插进来。
>
> 另外还有一个设计上的好处：**加新的事件类型不用改接口**。`Event` 是个结构体，所有字段可选，消费端按「非零字段」分派。要加一种新事件，就加一个字段 + 消费端加一个 `case`。**如果用的是 interface + 类型断言，加一种类型就要改所有实现。**」

**备注讲解**：

```go
type Event struct {
	Text     string           // 模型文本增量
	Tool     *ToolEvent       // 工具调用开始/结束
	Usage    *Usage           // 本轮 token 用量
	Iter     int              // >0：进入第 Iter 轮迭代
	Notice   string           // 系统提示，仅用于 UI 展示，不入对话历史
	Done     bool             // 本轮（整个 Loop）结束
	Err      error            // 出错（不中断会话）
	Approval *ApprovalRequest // 非空：请求人在回路批准
	Compact  *CompactEvent    // 压缩生命周期事件
}
```

**注意 `Notice` 的注释**：「仅用于 UI 展示，不入对话历史」。这是个重要的语义区分——`Notice` 是「给用户看的」，而 `Text` 是「给模型看的」（会进历史）。

**TUI 侧的分派**（`tui.go:421-499`，共 9 个 case）——**按非零字段 `switch`**：

```go
func (m *Model) handleAgentEvent(ev agentEvent) (tea.Model, tea.Cmd) {
	switch {
	case ev.Compact != nil: ...
	case ev.Err != nil: ...
	case ev.Approval != nil: ...
	case ev.Tool != nil && ev.Tool.Phase == agent.PhaseStart: ...
	case ev.Tool != nil && ev.Tool.Phase == agent.PhaseEnd: ...
	case ev.Usage != nil: ...
	case ev.Iter > 0: ...
	case ev.Notice != "": ...
	case ev.Text != "": ...
	case ev.Done: ...
	}
	return m, nil
}
```

**可能的追问**：
- **「为什么不用 `interface{}` + 类型断言？」** → 「因为 `Event` 里的字段经常**一次带多个语义**。比如一条最终答复的事件可能是 `{Text: "...", Done: true}`——既有文本又有结束标记。用 interface 的话必须拆成两个事件（`TextEvent` + `DoneEvent`），消费端要做状态机。**用结构体 + 字段天然支持「多语义组合」，而且没有类型断言的运行时开销。**」

---

## Q3.19 ⭐ Agent 一轮最多花多少 token？你怎么控制成本？

**面试官想听什么**：**成本意识**。字节做 Agent 产品，这个问题非常实际。

**口述答案**：

> 「先说理论上限：25 轮 × 每轮大约「输入 100K + 输出 4K」。因为输入是**累积**的（每轮都要发完整历史），最坏情况大概是 25 轮平均 50K 输入 = 1.25M 输入 token，加上 100K 输出。
>
> 但这个上限在实际使用中几乎到不了，因为我做了四件事控制成本：
>
> **① Prompt Cache**。系统提示的稳定段打了缓存断点，而且历史是「只追加不改写」的，所以每轮的输入里**绝大部分都是缓存命中的前缀**——价格是正常输入的 10%。这一项就砍掉了 60–70% 的输入成本。
>
> **② 输出有硬上限**。`MaxTokens = 4096`，所以输出侧是封顶的。
>
> **③ 上下文有压缩**。到 167K 就触发摘要，把历史压下来，避免「输入越滚越大」。如果没有压缩，到后面每轮的输入都是 200K，成本是线性爆炸的。
>
> **④ 有停止条件**。自然完成、连续幻觉、25 轮上限——三条路径保证不会无限跑。
>
> **但我要说一个真实的成本盲区**：**我没有任何「成本预算」的概念**。比如「这个任务最多花 10 万 token」——现在做不到。用户只能看着状态栏的累计 token 数自己判断，然后手动 Esc。**如果做成产品，这是必须补的：一个 token 预算上限，超了就停。**」

**备注讲解**：

**`MaxTokens` 写死在代码里**：

```go
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(p.cfg.Model),
			MaxTokens: 4096,      // ← 硬编码
			...
		}
```

**这是个应该可配的值**——4096 对生成大文件来说偏小。被问到可以承认。

**成本追踪的数据链**：

```
provider 返回 Usage → StreamEvent{Usage} → agent 更新锚点 + emit(Event{Usage})
  → TUI 累加 m.usageIn / m.usageOut → 状态栏显示
```

**`UsageAnchor` 把 4 个字段全加起来**：

```go
func UsageAnchor(u *llm.Usage) int64 {
	return u.InputTokens + u.OutputTokens + u.CacheWrite + u.CacheRead
}
```

**注意 `CacheRead` 也占上下文窗口**——虽然它便宜，但它仍然是输入的一部分。所以算「窗口占用」时必须带上它。

**可能的追问**：
- **「25 轮上限的成本上限是多少？」** → 可以现场估算：「按 Sonnet 的价格，输入 $3/M、输出 $15/M、缓存读 $0.3/M。最坏 1.25M 输入里假设 80% 是缓存命中（1M × $0.3 = $0.3）加上 250K 全新输入（$0.75），加 100K 输出（$1.5），一共约 $2.55。**单次任务最坏两三美元**——所以「成本预算」这个功能对重度用户是刚需。」

---

## Q3.20 ⭐ 上下文压缩放在循环的哪个位置？为什么在那里？

**面试官想听什么**：你对「什么时候该压」的理解。

**口述答案**：

> 「放在**每一轮循环的最开始，在发请求之前**。
>
> 为什么不放在「请求失败之后」？因为那时候已经浪费了一次请求（而且很可能还失败了）。**要在花钱之前做判断。**
>
> 为什么不放在「每轮结束之后」？因为循环的结尾有 5 条退出路径，**每一处都要记得调压缩**——漏一处就会爆窗口。放在循环开头只需要写一次，而且天然覆盖所有进入下一轮的情况。
>
> 位置还有一个细节：**压缩必须在「取工具集」之后、构造 reminder 之前**。因为压缩的恢复段里要写「当前可用工具列表」，而这个列表依赖于**当前模式下的工具集**——Plan 模式只导出只读工具，恢复段里就该只有只读工具。所以代码里是：先按 mode 算 `defs`，再 `ManageContext(ctx, ManageInput{ToolDefs: defs, ...})`。」

**备注讲解**：

```go
		for iter := 1; iter <= maxIterations; iter++ {
			// 1. 进度事件
			// 2. 按 mode 取工具集          ← defs 在这里算
			var defs []llm.ToolDefinition
			if mode == permission.ModePlan { defs = a.registry.ReadOnlyDefinitions() } else { defs = a.registry.Definitions() }

			// 3. 上下文管理                  ← defs 传进去
			anchor, anchorLen := a.runtime.GetAnchor()
			est := compact.EstimateTokens(anchor, conv.Messages(), anchorLen)
			willSummarize := est >= int64(cw - compact.SummaryReserve - compact.AutoSafetyMargin)
			if willSummarize {
				a.dispatchHook(ctx, hook.EventPreCompact, ...)      // ← 压缩前 Hook
				emit(ctx, ch, Event{Compact: &CompactEvent{Phase: CompactPhaseBeforeAuto}})
			}
			out, mcErr := compact.ManageContext(ctx, in)
			if willSummarize {
				a.dispatchHook(ctx, hook.EventPostCompact, ...)     // ← 压缩后 Hook
				emit(ctx, ch, Event{Compact: &CompactEvent{Phase: CompactPhaseAfterAuto, Before: out.BeforeTokens, After: out.AfterTokens}})
			}
```

**`ManageContext` 是「每轮必调的唯一入口」**（包注释原话），它内部按 `Trigger` 分三条路径：

```go
	switch in.Trigger {
	case TriggerManual:    return manageManual(ctx, in, out)     // 手动 /compact
	case TriggerEmergency: return manageEmergency(ctx, in, out)  // PTL 兜底
	default:               return manageAuto(ctx, in, out)       // 自动
	}
```

**这就是「单一入口」的价值**：三条触发路径共享同一份实现（`runSummary` / `OffloadAndSnip`），差别只在「跳过哪些检查」。

**压缩的 UI 反馈**：`willSummarize` 是**预先判断**的——因为要决定「要不要发 Before 事件」。如果等 `ManageContext` 返回再判断，就无法表达「压缩开始了」这个中间态。

**可能的追问**：
- **「为什么 PreCompact Hook 只在自动路径派发，手动 `/compact` 不派发？」** → **这是个已知的不一致**（我核过代码：`grep` 只有自动路径的两个派发点）。答法：「这是实现遗漏。手动压缩也应该派发 PreCompact/PostCompact——用户可能想用 Hook 在压缩前后做审计或者备份。修法很简单，在 `manageManual` 里补两个派发点。」

---

## Q3.21 ⭐ 25 轮这个数字是怎么定的？为什么不做成可配置？

**面试官想听什么**：你的阈值有没有依据 + 你对「可配置」的态度。

**口述答案**：

> 「**先说这个数字从哪来**：我实际跑过一些任务——比如「重构 auth 模块」这种：glob 找文件（1 轮）、读三个文件（1 轮）、grep 找引用（1 轮）、写一个文件（1 轮）、跑测试（1 轮）、修三个错误（3 轮）、再跑测试（1 轮）——大概 10 轮。中间只要有反复，15–20 轮很常见。**我一开始设的 10 轮，发现在任务快完成时被切断，非常挫败**，所以调到 25。
>
> **再说为什么不做成可配置**：我认为「迭代上限」是**安全兜底**，不是**功能参数**。它的作用是「防止异常情况无限烧钱」。如果做成可配置，用户很可能为了「让任务跑完」调成 100 或者 1000——**那就等于关掉了兜底**。而真正的需求「让我把任务跑完」，应该在**下一轮继续**（我的停止提示就是「可继续发消息推进」）——用户发一句话，新一轮 25 轮又开始。
>
> **所以我的设计是：上限固定，但任务可以跨轮次连续推进。** 这比「一次跑很久」更可控——每一轮结束用户都有机会看一眼、纠个方向。」

**备注讲解**：

```go
	noticeMaxIter      = "（已达最大迭代轮数 25，自动停止；可继续发消息推进。）"
```

**这句文案和「不可配」的设计是配套的**——文案本身就在告诉用户正确的用法。

**一个真实的细节**：这个常量在代码里是 `const maxIterations = 25`，注释写着 `// 迭代上限兜底（F2）`。而且**停止提示文案里也硬编码了 25**：

```go
	noticeMaxIter      = "（已达最大迭代轮数 25，自动停止；可继续发消息推进。）"
```

**这是两处独立的硬编码**——如果改成 30，要改两个地方。**这是个代码坏味道**（应该用 `fmt.Sprintf` 拼）。被问到可以承认：「这两处应该合成一处，用 `fmt.Sprintf` 从常量生成文案，避免不同步。」

---

## Q3.22 ⭐⭐ 请画出一次完整请求的时序图

**面试官想听什么**：整体把握能力。他可能在白板上让你画。

**口述 + 手画**：

```
用户敲回车
    │
    ├─ 是 "/" 开头？ ──→ 命令分发（13 条内置命令）
    │                        ├─ KindLocal：只打印
    │                        ├─ KindUI：改 Model 状态
    │                        └─ KindPrompt：注入预设 → 走下面
    │
    └─ 普通消息
         │
         ├─[1] Hook UserPromptSubmit（可拦截 → 拦了就结束，不进历史）
         │
         ├─[2] conv.AddUser(text)        ← 触发 onAppend → JSONL 追加 + fsync
         │
         ├─[3] turnCtx = context.WithCancel(bg)
         │
         ├─[4] events = ag.Run(turnCtx, conv, mode)     ← 返回 channel，agent 起 goroutine
         │
         └─[5] TUI: state = stateStreaming；返回 waitForEvent(events) 的 tea.Cmd
                    │
    ┌───────────────┘
    │  Agent 侧（goroutine）
    │
    ├── for iter = 1..25:
    │     │
    │     ├─ a) emit(Event{Iter: iter})            ──────→ TUI 更新轮次显示
    │     │
    │     ├─ b) defs = mode==Plan ? ReadOnlyDefs : Defs
    │     │
    │     ├─ c) est = EstimateTokens(anchor, msgs, anchorLen)
    │     │      if est ≥ cw - 20000 - 13000:
    │     │          emit(Compact Before) → ManageContext(自动) → emit(Compact After)
    │     │      else:
    │     │          layer1 落盘替换（无 LLM）
    │     │
    │     ├─ d) reminder = plan提醒(每4轮完整版) + hook注入的 reminders
    │     │
    │     ├─ e) envText = env.Render() + 活跃Skill块      ← 每轮重建
    │     │
    │     ├─ f) streamOnce(...)  ──→ provider.Stream(...)
    │     │      │                       │
    │     │      │  for ev := range stream:
    │     │      │     Text    → emit(Event{Text})  ──────→ TUI 累加到 curReply
    │     │      │     Usage   → 记下来
    │     │      │     ToolCalls → 记下来（流结束后才完整）
    │     │      │     Done    → 退出循环
    │     │      │
    │     │      └─ 若 Err == ErrPromptTooLong 且没重试过:
    │     │            ManageContext(紧急) → ResetAnchor → 重发一次
    │     │
    │     ├─ g) UpdateAnchor(usage, conv.Len())
    │     │
    │     ├─ h) len(calls) == 0 ?
    │     │      ├─ 是 → ensureFinal → conv.AddAssistant(final)
    │     │      │       每5轮或命中「记住」→ memMgr.UpdateAsync
    │     │      │       Hook Stop → emit(Done) → return
    │     │      │
    │     │      └─ 否 ↓
    │     │
    │     ├─ i) conv.AddAssistantWithToolCalls(text, calls)
    │     │
    │     ├─ j) allUnknown? → unknownRun++ / =0 ;  if ≥3 → 停止
    │     │
    │     ├─ k) executeBatched(ctx, calls, mode, ch)
    │     │      │
    │     │      for i < len(calls):
    │     │        ├─ IsReadOnly(calls[i]) ?
    │     │        │    ├─ 是：吃入 [i,j)
    │     │        │    │     ① hook+权限逐个检查（被拒记录）
    │     │        │    │     ② 按序 emit 所有 Start ──────→ TUI 显示 3 个工具在跑
    │     │        │    │     ③ 被拒项预填错误结果
    │     │        │    │     ④ WaitGroup 并发执行
    │     │        │    │     ⑤ 按序 emit 所有 End + PostToolUse Hook ──→ TUI 用 FIFO 配对
    │     │        │    │     i = j
    │     │        │    │
    │     │        │    └─ 否：单个串行执行
    │     │        │          ├─ hook（可拦截）
    │     │        │          ├─ eng.Check → Deny/Ask/Allow
    │     │        │          ├─ Deny → 回灌拒绝原因，i++
    │     │        │          ├─ Ask → emit(Event{Approval})  ═══════╗
    │     │        │          │        select{ <-respond / <-ctx.Done() }
    │     │        │          └─ Allow → 执行
    │     │        │
    │     ├─ l) recordFileReads（记录纯净文件字节，供压缩恢复段用）
    │     │
    │     ├─ m) conv.AddToolResults(results)   ← 无论是否取消都回灌
    │     │
    │     └─ n) !completed → ensureAssistantTail + return
    │
    └── 循环结束 → emit(Event{Notice: 达到上限}) → ensureAssistantTail → emit(Done)

    TUI 侧（Update 循环）
    │
    ├─ waitForEvent 返回 agentEvent → Update → handleAgentEvent 分派渲染
    └─ 每个分支处理完再返回一个新的 waitForEvent  ← 自循环
         │
         └─ Approval 分支例外：返回 nil（不续读），因为 agent 正阻塞

    ═══════╗ 审批回传
           └─ TUI: updateApproving → commitApproval(outcome)
                   → m.pending.Respond <- outcome   （缓冲=1，非阻塞）
                   → state = stateStreaming → 恢复 waitForEvent
```

**备注**：面试时画这个图，**只需要画三条竖线**（用户 / TUI / Agent）和箭头的来回，不用把所有的细节都写上去。**图的目的是让面试官能指着某一处问「这里为什么这样」**——你已经准备好了答案。

---

➡️ 下一篇：`04-面试题库-权限与安全.md`
