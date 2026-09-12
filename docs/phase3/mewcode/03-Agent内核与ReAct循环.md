# 03 · Agent 内核：ReAct 循环

> 源码：`mewcode/internal/agent/agent.go`（1190 行）、`runtime.go`、`run_to_completion.go`、`fork.go`
> 这是面试**最高频**、**最深挖**的一章。建议把 `agent.go` 的 `Run` 函数完整读三遍。

---

## 一、这一章要回答什么

| 问题 | 本节位置 |
|---|---|
| Agent 到底是什么？循环怎么转？ | §2.1 |
| 循环怎么停？会不会死循环？ | §2.5 |
| Agent 和界面怎么解耦？ | §2.2 |
| 一次回复里 10 个工具调用，怎么执行最快又不乱序？ | §2.4 |
| 用户按 ESC 取消，会不会把对话历史搞坏？ | §2.6 |
| 模型幻觉出不存在的工具怎么办？ | §2.5 |
| 子 Agent 怎么复用同一套循环？ | §2.8 |

---

## 二、项目在做什么

### 2.1 循环全景

MewCode 的内核是一个标准的 **ReAct（Reason + Act）循环**，但工程细节比教科书版本复杂得多。

```mermaid
flowchart TD
    U[用户消息] --> RUN["Agent.Run(ctx, conv, mode)"]
    RUN --> ENV["① 采集环境 + 装配系统提示（Run 起始一次）"]
    ENV --> LOOP{"for iter := 1..25"}

    LOOP --> TOOLSET["② 按权限模式取工具集<br/>Plan=只读 / 其他=全量"]
    TOOLSET --> CTX["③ compact.ManageContext<br/>两层上下文管理"]
    CTX --> STREAM["④ streamOnce<br/>流式请求 LLM"]

    STREAM -->|ErrPromptTooLong| EM["紧急压缩 → 重试一次"]
    EM --> STREAM
    STREAM -->|sErr| CANCEL[终止 + 兜底历史]
    STREAM -->|"calls == 0"| DONE["自然完成：文本即最终答复"]
    STREAM -->|"len(calls) > 0"| ADD["⑤ conv.AddAssistantWithToolCalls"]

    ADD --> EXEC["⑥ executeBatched<br/>保序分批并发 + 五层权限判定"]
    EXEC --> FEED["⑦ conv.AddToolResults<br/>结果回灌历史"]
    FEED --> LOOP

    DONE --> MEM["记忆更新触发（每 5 轮 / 关键词）"]
    MEM --> STOPHOOK["Hook: Stop"]
    STOPHOOK --> END([Event{Done:true}])
```

**一句话概括**：把「模型说要调什么工具」这件事，变成「确定性代码按权限判定后安全地执行，再把结果塞回历史」的循环。

### 2.2 代码骨架

```go
// agent.go:164
func (a *Agent) Run(ctx context.Context, conv *conversation.Conversation, mode permission.Mode) <-chan Event {
    ch := make(chan Event)

    go func() {
        defer close(ch)

        atomic.StoreInt32(&a.running, 1)
        defer atomic.StoreInt32(&a.running, 0)

        a.runMu.Lock()          // 保证 Run 与 /compact 不并发
        defer a.runMu.Unlock()

        env := prompt.GatherEnvironment(a.version, a.provider.Model())
        sys := prompt.BuildSystemPrompt(a.instructionText, a.memoryText, skillsCatalogText)

        for iter := 1; iter <= maxIterations; iter++ {   // maxIterations = 25
            // ① 进度事件
            // ② 按 mode 取工具集
            // ③ 上下文管理（自动压缩判断 + 执行）
            // ④ 构建 reminder（Plan Mode + Hook 注入）
            // ⑤ streamOnce
            // ⑥ 无工具调用 → 自然完成
            // ⑦ 有工具调用 → 执行 + 回灌
        }
    }()

    return ch
}
```

**注意三个设计细节，面试官很容易追问：**

1. **返回 `<-chan Event` 而不是阻塞调用**：`Run` 立即返回 channel，循环跑在独立 goroutine 里。调用方（TUI）只消费事件，完全不知道循环内部发生了什么。
2. **`atomic.StoreInt32(&a.running, 1)`**：让 TUI 能通过 `IsRunning()` 判断 Agent 是否忙碌（用于决定 ESC 是「取消本轮」还是「退出程序」）。
3. **`a.runMu`**：串行化 `Run` 与 `RunForceCompact`（`/compact` 命令），避免用户在 Agent 跑动时触发压缩导致状态竞争。

---

## 三、核心设计逐项拆解

### 3.1 事件流解耦（Agent ↔ UI）

```go
// agent.go:79
type Event struct {
    Text     string           // 模型文本增量（流式打字机）
    Tool     *ToolEvent       // 工具调用开始/结束
    Usage    *Usage           // 本轮 token 用量
    Iter     int              // >0：进入第 Iter 轮迭代
    Notice   string           // 系统提示（停止原因），仅 UI 展示，不入历史
    Done     bool             // 整个 Loop 结束
    Err      error            // 出错（不中断会话）
    Approval *ApprovalRequest // 人在回路请求
    Compact  *CompactEvent    // 压缩生命周期事件
}
```

设计要点：

- **单一结构体 + 非零字段分派**：不用 `interface{}` 多态事件（那样 TUI 要写 type switch，且新增事件类型要改所有实现）。用一个结构体，谁非零就渲染谁。代价是字段会随功能增长（现在 9 个字段）。
- **`Notice` 与 `Text` 分开**：`Notice` 是「已达最大迭代轮数」这类系统提示，**不写入对话历史**；`Text` 是模型输出，**会写入历史**。这个区分很关键——如果把系统提示写进历史，模型会以为自己在自言自语。
- **`Approval` 带 `Respond chan permission.Outcome`（缓冲=1）**：Agent 发出请求后阻塞在这个 channel 上等 TUI 回传决策，实现「人在回路」。

> **面试官追问点**：`Respond` 为什么是缓冲 1？
> **答**：为了让 Agent 在阻塞等待时，TUI 即使已经因为超时/取消不再接收，也能无阻塞地写入结果，避免 TUI 侧 goroutine 泄漏。且整个流程只可能有一次回传（Agent 收到就返回），缓冲 1 足够。

### 3.2 流式收集双路

```go
// agent.go:481
func streamOnce(ctx, provider, msgs, tools, sys, envText, reminder, ch) (text string, calls []llm.ToolCall, usage *llm.Usage, err error) {
    var textBuilder strings.Builder

    req := llm.Request{
        Messages: msgs,
        Tools:    tools,
        System:   llm.System{Stable: sys, Environment: envText},
        Reminder: reminder,
    }

    stream := provider.Stream(ctx, req)
    for ev := range stream {
        switch {
        case ev.Err != nil:
            return textBuilder.String(), calls, nil, ev.Err
        case ev.Usage != nil:
            usage = ev.Usage
        case len(ev.ToolCalls) > 0:
            calls = append(calls, ev.ToolCalls...)
        case ev.Text != "":
            textBuilder.WriteString(ev.Text)
            if !emit(ctx, ch, Event{Text: ev.Text}) {
                return textBuilder.String(), calls, nil, ctx.Err()
            }
        }
    }
    if ctx.Err() != nil {
        return textBuilder.String(), calls, nil, ctx.Err()
    }
    return textBuilder.String(), calls, usage, nil
}
```

**「双路」的含义**：

| 路 | 目的地 | 用途 |
|---|---|---|
| 实时路 | `emit(ctx, ch, Event{Text: ...})` | TUI 立刻渲染打字机效果 |
| 累积路 | `textBuilder` + `calls` | 循环判断下一步：有没有工具调用？要不要继续？ |

**为什么一定要双路**：Agent 需要**等整轮响应结束**才能知道模型是否调用了工具（tool_use 块可能在文本之后）。所以必须先攒完，再决策。但用户体验上必须边收边显示。两路并行是唯一解。

> **面试官追问**：工具调用的 JSON 参数是分片到达的，你怎么拼？
> **答**：拼接在**协议适配层**（`llm/anthropic.go` / `llm/openai.go`）完成。Anthropic 的 `input_json_delta`、OpenAI 的 `tool_calls[].function.arguments` 增量，都在适配器内部累积，只有拼成完整 JSON 后才通过 `StreamEvent.ToolCalls` 一次性抛出。这样 Agent 层拿到的一定是 `json.RawMessage` 完整对象，不用关心分片。

### 3.3 保序分批并发执行（本项目最有含金量的设计之一）

模型一次回复可能请求多个工具（比如「读这 3 个文件 + 搜这个符号」）。天真做法是全串行（慢）或全并发（危险：写文件顺序不确定）。

MewCode 的方案：**扫描调用序列，连续的只读调用合并成一个并发批，遇到有副作用的调用单独串行执行，整体保持模型给出的相对顺序。**

```go
// agent.go:518 摘要
func (a *Agent) executeBatched(ctx, calls, mode, ch) ([]llm.ToolResult, bool) {
    results := make([]llm.ToolResult, len(calls))
    i := 0
    for i < len(calls) {
        if ctx.Err() != nil { /* 取消：剩余全部补「已取消」结果 */ }

        if a.registry.IsReadOnly(calls[i].Name) {
            j := i
            for j < len(calls) && a.registry.IsReadOnly(calls[j].Name) { j++ }   // 吃入连续只读区间 [i,j)

            // ① 逐个权限 Check + PreToolUse hook，标记被拒项
            // ② 按序 emit 所有 PhaseStart 事件
            // ③ 被拒项预先写结果，不纳入并发
            // ④ wg 并发执行未被拒的只读工具
            // ⑤ 按原始顺序 emit PhaseEnd + PostToolUse hook
            i = j
        } else {
            // 串行执行单个有副作用工具（含权限判定 + 人在回路）
            i++
        }
    }
    return results, true
}
```

**为什么这么设计（面试必答）**：

| 维度 | 全串行 | 全并发 | **本项目：保序分批** |
|---|---|---|---|
| 延迟 | 3 个 read_file = 3×RTT | 1×RTT | 1×RTT（只读批） |
| 正确性 | ✅ | ❌ 写顺序不定 | ✅ 有副作用严格串行 |
| 顺序语义 | ✅ | ❌ | ✅ 结果按原始 index 写回 `results[k]` |
| 确定性 | ✅ | ❌ | ✅ |

关键实现细节：
- **结果按 `idx` 写回固定下标**（`results[idx] = ...`），而不是按完成顺序 append——这是保序的核心。
- **事件发射顺序也是保序的**：先按序 emit 所有 `PhaseStart`，并发跑完后再按序 emit 所有 `PhaseEnd`。所以 UI 上看到的是「3 个工具行一起出现 → 依次出结果」，不会交错。
- **被拒的工具也要 emit Start/End**：否则 UI 上看不到它，用户会疑惑「模型说要用这个工具，怎么没了」。

> **面试官追问 1**：如果模型给出 `read(A) → write(B) → read(A)`，你怎么处理？
> **答**：扫描结果是：`[read A]` 一个并发批 → `write B` 单独串行 → `[read A]` 又一个并发批。严格保持模型给出的顺序。代价是失去跨批重排优化（比如 read C 本可以和 write B 并发），但这是**确定性优先**的取舍——写操作后面的读必须看到写完的结果。

> **面试官追问 2**：并发批里某个工具 panic 了怎么办？
> **答**：这是当前实现的一个**已知缺口**——`registry.Execute` 内部没有 recover，工具 panic 会打穿整个 agent goroutine 导致进程崩溃。生产做法是在 `Execute` 外面包一层 `defer recover()`，把 panic 转成 `Result{IsError:true, Content:"工具内部错误: ..."}` 回灌给模型。这个点答出来会显得你对自己代码的边界很清楚。

> **面试官追问 3**：为什么不用 `errgroup`？
> **答**：`errgroup` 会在第一个 error 时取消其余任务，而这里**工具错误不是失败**——工具报错要作为观察结果回灌给模型让模型自己调整。所以用裸 `sync.WaitGroup` 更贴切。这是一个「语义决定选型」的例子。

### 3.4 工具错误即观察结果（Error as Observation）

```go
// agent.go:734 —— 权限拒绝也是「结果」，不是异常
results[i] = llm.ToolResult{
    ToolCallID: call.ID,
    Content:    reason,        // 例："匹配 项目 deny 规则：Bash(rm *)"
    IsError:    true,
}
```

**整个 Agent 里没有一处因为工具失败而中断循环**。所有错误都被包装成 `llm.ToolResult{IsError: true}` 回灌进对话历史，让模型看到「我刚才那步失败了，原因是 X」，自己决定下一步（换个路径、换个命令、向用户求助）。

这是 Agent 和普通程序最本质的差别之一：

```
普通程序：error → return err → 上层处理
Agent：   error → 变成观察 → 模型重新规划 → 继续
```

> **面试官追问**：`IsError: true` 最终怎么变成模型能看懂的东西？
> **答**：协议适配层负责转换。Anthropic 的 `tool_result` 块有 `is_error` 字段；OpenAI 则是在 `role: tool` 消息内容前加前缀标记。这层差异被 `llm` 包完全吸收，Agent 只认 `ToolResult.IsError`。

### 3.5 五类停止条件

```go
// agent.go:149
const (
    maxIterations        = 25 // 迭代上限兜底
    maxUnknownRun        = 3  // 连续「整轮只产生未知工具调用」的迭代数上限
    planReminderInterval = 4  // 规划模式下每隔 4 轮重复完整提醒
)
```

| # | 停止条件 | 触发点 | 历史收尾 |
|---|---|---|---|
| 1 | **自然完成** | `len(calls) == 0` | `conv.AddAssistant(final)` |
| 2 | **迭代上限** | `iter > 25` | `ensureAssistantTail(conv, noticeMaxIter)` |
| 3 | **连续未知工具** | `unknownRun >= 3` | `ensureAssistantTail(conv, noticeUnknownTools)` |
| 4 | **用户取消** | `ctx.Err() != nil` | `finishCancelled(conv)` |
| 5 | **流出错** | `sErr != nil` | `ensureAssistantTail(conv, noticeStreamErr)` |

**第 3 条最值得讲**（面试官会觉得你真的跑过 Agent）：

```go
// agent.go:1025
func allUnknown(registry *tool.Registry, calls []llm.ToolCall) bool {
    if len(calls) == 0 { return false }
    for _, c := range calls {
        if _, ok := registry.Get(c.Name); ok { return false }
    }
    return true
}
```

**为什么要专门检测这个**：模型（尤其是小模型 / 弱 function calling 能力）会幻觉出不存在的工具名，比如 `search_codebase`、`run_tests`。如果不检测，Agent 会陷入「请求幻觉工具 → 报错回灌 → 再请求同一个幻觉工具」的死循环，烧掉 25 轮 token 后放弃。

**为什么阈值是 3 而不是 1**：单轮幻觉可能是偶发（模型手滑），给它 1-2 次自我修正机会；连续 3 轮说明模型已经陷入固定的错误模式，再给机会也没用。

**注意 `allUnknown` 的语义是「整轮全是未知工具」**——只要有一个工具是合法的，计数就清零。这避免了「混合调用被误杀」。

### 3.6 历史一致性（最容易出 400 错误的地方）

LLM API 对消息序列有强约束（以 Anthropic 为例）：
- `user` / `assistant` 必须交替
- 每个 `tool_use` 块**必须**有配对的 `tool_result` 块
- 最后一条消息如果是 `user`，模型会当作新提问

一旦循环因为取消/出错提前退出，很容易破坏这些约束。MewCode 用三个函数专门兜这个底：

```go
// agent.go:1050
// ensureAssistantTail 若历史末尾不是 assistant 角色，补一条兜底文本
func ensureAssistantTail(conv *conversation.Conversation, fallback string) {
    if conv.LastRole() != llm.RoleAssistant {
        conv.AddAssistant(fallback)
    }
}

// agent.go:1057 取消路径统一收尾
func finishCancelled(conv *conversation.Conversation) {
    ensureAssistantTail(conv, noticeCancelled)
}
```

**还有一个更隐蔽的场景**：取消发生在工具执行**中途**。此时 `conv` 里已经写入了 `assistant(带 tool_calls)`，但 `tool_results` 还没写。如果不补，下一轮请求就是「悬空 tool_use」→ 400。

MewCode 的做法（`executeBatched` 的取消分支）：

```go
// agent.go:524 —— 取消时给所有未完成的调用补「已取消」结果
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
```

然后 `Run` 里**无论是否取消都回灌工具结果**：

```go
// agent.go:384 —— 注意注释，这是刻意的
// 无论是否取消都回灌工具结果
conv.AddToolResults(results)

// 执行中被取消——最高优先级终止
if !completed {
    ensureAssistantTail(conv, noticeCancelled)
    return
}
```

> **面试官追问**：为什么取消后还要把「已取消」结果写进历史？用户都取消了，为什么不直接丢弃这一轮？
> **答**：因为 `assistant(tool_calls)` 已经写进历史了，不能只写一半。要么整轮回滚（要把 conversation 做成事务性结构，改动大且影响持久化），要么补齐配对保持合法。MewCode 选了后者——**成本低、语义清晰**（模型下次会看到「上次那步被用户取消了」），而且用户取消后往往还会继续对话，历史必须是合法的。

### 3.7 取消传播

```go
// agent.go:1015 —— emit 里也监听 ctx，防止 TUI 不再消费时 goroutine 卡死
func emit(ctx context.Context, ch chan<- Event, e Event) bool {
    select {
    case ch <- e:
        return true
    case <-ctx.Done():
        return false
    }
}
```

`emit` 不是简单的 `ch <- e`，而是带 `ctx.Done()` 的 select。**这是防 goroutine 泄漏的关键**：如果 TUI 因为某种原因停止消费 channel，单纯的 `ch <- e` 会永久阻塞，Agent goroutine 泄漏。加上 `ctx.Done()` 后，取消时立即返回 false，调用方据此走收尾路径。

取消链路：

```
用户按 ESC
  → TUI 调 cancel()（context.CancelFunc）
  → ctx.Done() 关闭
  → emit 返回 false → 各层 return
  → streamOnce 的 provider.Stream 内部 http request 被 ctx 取消
  → executeBatched 的 context.WithTimeout(ctx, 30s) 继承取消
  → Run 走 finishCancelled → ensureAssistantTail → goroutine 退出 → close(ch)
  → TUI 收到 channel 关闭 → 回 idle 态
```

### 3.8 RunToCompletion：子 Agent 复用同一套循环

```go
// run_to_completion.go:29
func (a *Agent) RunToCompletion(ctx, conv, task string, events chan<- Event) (string, error) {
    if task != "" { conv.AddUser(task) }
    turns := a.maxTurns; if turns == 0 { turns = maxIterations }

    for iter := 1; iter <= turns; iter++ {
        // ① 取工具集（Plan / allowedTools 白名单 / 全量）
        // ② 上下文管理
        // ③ streamOnce（走内部 channel）
        // ④ 无工具调用 → 返回 final
        // ⑤ 有工具调用 → executeBatched → 回灌
    }
    return lastAssistantText(conv), ErrMaxTurnsReached
}
```

与主循环 `Run` 的差异（面试官喜欢问「你怎么复用的」）：

| 维度 | `Run` | `RunToCompletion` |
|---|---|---|
| 返回方式 | `<-chan Event`（异步） | `(string, error)`（同步阻塞） |
| 系统提示 | `BuildSystemPrompt(...)` | 若有 `a.systemPrompt` 则**完全覆盖** |
| 迭代上限 | 固定 25 | `a.maxTurns`（角色 frontmatter 可配） |
| 工具集 | 全量 / 只读 | 额外支持 `allowedTools` 白名单 |
| 记忆更新 | 每 5 轮触发 | **不触发**（子 Agent 上下文短，不值得） |
| 未知工具阈值 | 3 | 2（更严格，子 Agent 试错预算更小） |
| 事件转发 | 直接 emit 到 ch | 走 `internalCh` → `drainEvents` 转发 |

> **代码里的一个瑕疵（主动指出来会加分）**：`Run` 与 `RunToCompletion` 的循环体有大量重复代码（约 120 行），抽出公共 `loopIteration` 是更好的做法。作者在 spec 里写的是「同一段循环代码与主对话 Run 共用，不重复实现」——**实现与设计有偏差**。面试时主动说「这块我后来意识到可以抽成 `runLoop(ctx, conv, opts)`，把差异项（系统提示、工具集、轮数、事件出口）参数化」是很好的反思分。

### 3.9 Fork：克隆父对话的子 Agent

```go
// fork.go:41
func BuildForkedMessages(parentMsgs []llm.Message, task string) []llm.Message {
    cloned := cloneMessages(parentMsgs)     // ① 深拷贝（含 ToolCalls/ToolResults 切片）
    cloned = fixPendingToolCalls(cloned)    // ② 补悬空 tool_use 的 placeholder
    userMsg := llm.Message{
        Role:    llm.RoleUser,
        Content: ForkBoilerplate + task,     // ③ 拼 Boilerplate + 任务
    }
    return append(cloned, userMsg)
}
```

**Fork 的价值（面试可讲）**：克隆父对话历史，让子 Agent 一启动就拥有父 Agent 的全部上下文——省去「重新读文件、重新理解项目」的过程。**更重要的是能命中 Prompt Cache**：父对话的 prefix 逐字节相同，子 Agent 首次请求的 input token 大部分是 cache_read（便宜 10 倍）。

**`fixPendingToolCalls` 解决什么**：父对话末尾可能停在 `assistant(tool_calls)` 但结果还没写（比如用户刚好在这一刻 Fork）。如果不补 placeholder，子 Agent 第一次请求就会因为悬空 `tool_use` 报 400。

**Fork Boilerplate 的约束**：

```go
// fork.go:20
const ForkBoilerplate = `<fork_boilerplate>
你是一个 Fork 出来的工作进程。你不是主 Agent。
规则（不可协商）：
1. 不能再 Fork（调用 Agent 工具会被拦截）。
2. 不要对话、不要提问、不要请求确认。
3. 直接使用工具：读文件、搜索代码、做修改。
4. 严格限制在你被分配的任务范围内。
5. 最终报告以 "Scope:" 开头，500 字以内。
</fork_boilerplate>`
```

每一条都是在解决真实问题：
- 第 1 条 → 防无限嵌套（配合 `ALL_AGENT_DISALLOWED_TOOLS = ["Agent"]` 硬约束）
- 第 2 条 → 子 Agent 没有人跟它对话，问问题等于卡死
- 第 5 条 → 结果要回灌给父 Agent，必须控制长度（否则子 Agent 的上下文直接污染父 Agent）

**`IsForkContext` 的兜底作用**：

```go
// fork.go:60
func IsForkContext(msgs []llm.Message) bool {
    // 扫描所有消息（含 ToolCalls.Input / ToolResults.Content）找 <fork_boilerplate> 标签
}
```

当 caller 链丢失（比如 Hook 触发的动作不知道自己在哪个上下文）时，靠**扫内容里的标签**来判定。这是一个很实用的工程技巧：**用带标签的不可见标记在数据里传递来源信息**，比到处传 context 参数更鲁棒。

---

## 四、面试官可能追问（Q&A）

### L1 基础理解

**Q1：什么是 ReAct？你这个项目和教科书上的 ReAct 有什么不同？**
> A：ReAct = Reason + Act，模型交替进行推理和行动。教科书版是单轮的 `Thought → Action → Observation → Thought...` 文本 prompt 循环。本项目的不同有四点：① 用 **Function Calling** 而不是文本解析（`tool_use` 结构化块，不用正则抠 JSON）；② **保序分批并发**执行工具，不是一次一个；③ 有**五类显式停止条件**和安全网；④ 每次迭代前做**上下文管理**（压缩），教科书版假设上下文无限。

**Q2：Agent Loop 和 while(true) 调 API 的区别是什么？**
> A：三个关键区别：① **状态管理**——每轮要把 assistant 回合（含 tool_calls）和 tool 结果按序写回历史，且必须保持协议合法性；② **安全边界**——权限判定在工具执行前，人在回路要能阻塞循环；③ **收敛保证**——必须有停止条件和幻觉检测，否则会无限烧钱。

**Q3：为什么用 channel 而不是回调函数返回事件？**
> A：① channel 天然支持「生产者比消费者快」的背压语义（无缓冲 channel 会在消费者慢时阻塞生产者）；② 调用方用 `for ev := range ch` 消费，代码线性可读；③ `close(ch)` 天然表达「循环结束」；④ 配合 `ctx.Done()` 在 select 里能同时处理取消。回调的话需要自己管理并发和结束信号。

### L2 深挖实现

**Q4：一次回复里模型请求了 5 个 read_file，你的执行耗时是多少？**
> A：约等于**单个** read_file 的耗时（最慢的那个），因为连续的只读调用会被合并成一个并发批，用 `sync.WaitGroup` 并发执行。每个工具仍然受 `tool.DefaultTimeout = 30s` 的独立超时约束（`context.WithTimeout(ctx, tool.DefaultTimeout)`），所以总耗时上限是 30s 而不是 150s。

**Q5：那如果请求是 `write_file → bash → read_file` 呢？**
> A：全部串行。因为 `write_file` 和 `bash` 都不是只读（`ReadOnly()` 返回 false），每一个都会单独走串行路径；`read_file` 虽然只读，但它前面紧邻的不是只读调用，所以它自成一批。总耗时是三者之和。

**Q6：工具结果回灌后，模型看到的是什么格式？**
> A：模型看到的是 `role: tool` 的消息，里面是 `tool_result` 块：`{tool_use_id, content, is_error}`。MewCode 的 `llm.ToolResult` 是协议无关的中间表示，转换发生在适配层——Anthropic 映射到 `tool_result` block 的 `is_error` 字段，OpenAI 映射到 `role:"tool"` 消息（错误时在 content 前加标记）。

**Q7：`maxIterations = 25` 这个数字怎么定的？**
> A：25 是在「复杂任务给的预算」和「失控成本上限」之间的折中。Claude Code 量级的任务（重构一个模块）典型需要 8-15 轮；25 给了约 2 倍余量。成本上界是可控的：25 轮 × 平均 3 个工具 × 每次全量历史 ≈ 有明确上限。**更好的做法是 token 预算控制**（比如 `max_total_tokens = 2M`）而不是轮数——因为单轮成本会随上下文膨胀，轮数固定不代表成本固定。这是本项目可以改进的点。

**Q8：模型连续请求未知工具，为什么是 3 轮而不是 1 轮？**
> A：1 轮太激进——模型偶尔会拼错工具名（比如 `read` vs `read_file`），报错回灌后它下一轮通常能自我修正。3 轮是在「给足自我修正机会」和「及时止损」之间的折中。而且判定条件是 **`allUnknown`（整轮全是未知工具）**，只要有一个合法工具就清零计数，避免误杀混合调用。

### L3 边界与故障

**Q9：Agent 跑到一半用户按 ESC，接下来会发生什么？**
> A：① TUI 调 `cancel()`；② `ctx.Done()` 关闭；③ 正在执行的工具因为继承了 ctx 而被取消（`context.WithTimeout(ctx, ...)` 链式传递）；④ `executeBatched` 检测到 `ctx.Err() != nil`，给所有未完成的调用补 `{Content: "（已取消。）", IsError: true}`；⑤ `Run` 仍然把这批结果 `AddToolResults` 回灌（保证 `tool_use`/`tool_result` 配对）；⑥ `ensureAssistantTail` 补一条 assistant 收尾消息；⑦ goroutine 退出、`close(ch)`；⑧ TUI 收到 channel 关闭，回 idle 态。**整个过程的终点是「历史合法、可以继续对话」**。

**Q10：如果 provider 流式响应中途断了（网络抖动），怎么办？**
> A：`streamOnce` 收到 `ev.Err != nil` 立即返回错误。`Run` 判定 `ctx.Err() == nil`（不是用户取消）→ 派发 `Notification` Hook → emit `Event{Err: sErr}` → `ensureAssistantTail` 补收尾 → 结束本轮。**程序不退出**，用户可以重新发消息。已累积的 `textBuilder` 内容会被丢弃（没有写入历史），这是一个取舍：保留半截文本会导致历史里出现不完整回复。

**Q11：如果一轮里既有合法工具又有幻觉工具，会怎样？**
> A：合法工具正常执行，幻觉工具会走 `registry.Execute` 的 not-found 分支返回结构化错误（`{"error": "unknown tool: xxx"}`），一起回灌给模型。`unknownRun` 计数清零（因为 `allUnknown` 返回 false）。模型看到「其中一个是 unknown tool」，通常下一轮就修正了。

**Q12：并发批里两个工具同时写同一个文件会怎样？**
> A：**不会发生**——`write_file` 的 `ReadOnly()` 返回 false，它永远走串行路径，不存在两个写操作并发的情况。只读批里只有 `read_file`/`glob`/`grep`，它们不改文件系统。这是「按副作用分类」这个设计的直接收益。

**Q13：`a.runMu` 和 `atomic.running` 是什么关系？会不会冗余？**
> A：职责不同。`atomic.running` 是**状态标志**，给 TUI 读（`IsRunning()`），决定 ESC 键语义；`runMu` 是**互斥锁**，保证 `Run` 和 `RunForceCompact` 不并发执行（两个都会改 `Compact` 状态和 `Conversation`）。`Run` 内部的迭代是串行的，所以不需要额外的锁。`SessionRuntime.mu` 保护的是跨 goroutine 访问的锚点/计数（记忆更新的异步 goroutine 也会读）。

### L4 设计与权衡

**Q14：为什么工具执行不放在独立的 worker pool 里，而是同一轮内起 goroutine？**
> A：① 并发度天然很小（一轮通常 2-5 个工具），起 goroutine 的开销可忽略；② `sync.WaitGroup` 的语义就是「等这批全完」，比 pool 的任务提交/回收更简单；③ 每批都要等结果才能回灌历史，没有跨批并行的空间。如果将来要支持「后台任务」（本项目已通过 `task.Manager` 实现），那是另一个维度——跨轮并行，用独立的 task 抽象。

**Q15：如果让你重新设计 Agent Loop，你会改什么？**
> A：四点：① **抽出公共循环体**（现在 `Run` 和 `RunToCompletion` 重复了约 120 行）；② **工具 panic 兜底**（`recover` 转成结构化错误）；③ **token 预算替代轮数上限**（成本更可控）；④ **把「停止原因」做成结构化枚举**而不是字符串常量，方便上层做策略（比如「因上下文超限停止」可以自动触发压缩后继续）。

**Q16：你的循环是「同步 ReAct」，有没有考虑过 Plan-and-Execute？**
> A：考虑过，而且项目里有**Plan Mode**（`/plan`）作为轻量版本——只放开只读工具让模型先产出计划，`/do` 切回全工具立即执行。真正的 Plan-and-Execute 会先把计划落成结构化任务列表（如 `{steps: [{id, desc, status}]}`），再逐步执行并更新状态。**我选择不做完整版的原因**：Coding Agent 的任务边界在执行中才会明确（读完 A 才知道要改 B），一次性规划容易过时。Claude Code 也是 ReAct 为主、Plan Mode 为辅。Plan-and-Execute 更适合步骤可预知的场景（数据管道、批处理）。

---

## 五、企业级方案对照

| 问题 | MewCode 的做法 | 企业级做法 | 差距与补齐 |
|---|---|---|---|
| **循环驱动** | 同步 ReAct，进程内 goroutine | 事件驱动 + 持久化状态机（Temporal / Durable Execution）；每步落盘，进程崩溃可恢复 | 需要把「Agent 状态」外置（Redis/DB），支持断点续跑 |
| **并发执行** | 同轮内按副作用分批并发 | 工具调度器 + 优先级队列 + 租户级配额；长任务异步化（提交任务 ID 轮询） | 需要工具级 SLA、超时分级、熔断降级 |
| **停止控制** | 轮数上限 + 幻觉检测 | **预算控制**（token/金额/时长三维预算）+ 目标达成度评估 + 用户可中断 | 需要 cost tracker + 每次 LLM 调用前的预算检查 |
| **故障恢复** | 出错即停本轮，不恢复 | Checkpoint + 重放；幂等工具设计（key 去重） | 需要 step-level checkpoint 与 idempotency key |
| **子 Agent** | 进程内 goroutine，共享 registry | 独立服务 / 容器沙箱，通过 A2A 或内部 RPC 通信 | 需要 Agent 注册中心、服务发现、隔离执行环境 |
| **可观测** | TUI 事件流 + stderr 日志 | OpenTelemetry Trace（每个 tool call 一个 span）+ Prometheus 指标 + 结构化日志；按 session 聚合 | 需要接入 OTEL，定义 Agent 语义约定（span: agent.run / llm.request / tool.call） |
| **成本控制** | 只统计 token 展示 | Prompt Cache 命中率监控、模型路由（简单任务走小模型）、结果缓存 | 需要 LLM 网关统一计费与路由 |

### 重点讲一个：企业级为什么必须做「三维预算控制」

MewCode 用轮数（25）做上界，但**轮数和成本不是线性关系**：

```
第 1 轮：input 10K tokens
第 5 轮：input 60K tokens（历史累积）
第 15 轮：input 180K tokens（上下文接近上限）
```

15 轮的成本可能是 5 轮的 5 倍以上。企业级方案通常是：

```go
type Budget struct {
    MaxTotalTokens   int64   // 会话级 token 预算
    MaxCostUSD       float64 // 会话级金额预算
    MaxWallClock     time.Duration
    MaxToolCalls     int
}

// 每轮 LLM 调用前检查
func (b *Budget) Check(estimated int64) error {
    if b.UsedTokens + estimated > b.MaxTotalTokens {
        return ErrBudgetExhausted
    }
    ...
}
```

超预算时不是硬停，而是**降级**：切换到更便宜的模型 / 触发压缩 / 提示用户「预算即将耗尽，是否继续」。

---

## 六、本章速记卡（面试前 5 分钟看）

```
核心文件     agent.go (Run / executeBatched / streamOnce)
循环上限     maxIterations = 25
幻觉熔断     maxUnknownRun = 3（allUnknown 整轮判定）
工具超时     tool.DefaultTimeout = 30s
并发策略     连续只读 → 并发批；有副作用 → 串行；结果按原始 index 回灌
停止条件     自然完成 / 上限 / 幻觉 / 取消 / 出错
历史一致性   ensureAssistantTail + 取消时补 tool_result + 无论取消都回灌
事件解耦     Event 结构体 + 非零字段分派；Notice 不入历史
取消传播     emit 内 select ctx.Done() 防 goroutine 泄漏
子 Agent     RunToCompletion 复用循环（maxTurns / systemPrompt / allowedTools 可配）
Fork         cloneMessages + fixPendingToolCalls + ForkBoilerplate（借 Prompt Cache）
```

---

- [上一篇：架构分层与依赖设计](/phase3/mewcode/02-架构分层与依赖设计)
- [下一篇：工具系统与执行编排](/phase3/mewcode/04-工具系统与执行编排)
