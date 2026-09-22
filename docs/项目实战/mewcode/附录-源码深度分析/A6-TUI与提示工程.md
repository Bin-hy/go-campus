# MewCode TUI / 命令体系 / 提示词工程 深度分析笔记

> 阅读范围（全部逐行读完，行数为实测）：
> `internal/tui/`：`tui.go` 667、`view.go` 285、`commands.go` 310、`complete.go` 203、`hooks.go` 92、`resume.go` 279、`select.go` 120、`stream.go` 21、`tasks.go` 33
> `internal/command/`：`command.go` 31、`dispatch.go` 43、`registry.go` 118、`builtins.go` 20、`builtin_local.go` 139、`builtin_prompt.go` 24、`builtin_skill.go` 138、`builtin_ui.go` 40、`skills.go` 65、`ui.go` 77
> `internal/prompt/`：`prompt.go` 56、`modules.go` 83、`environment.go` 93、`reminder.go` 26、`skills_block.go` 50
> `cmd/mewcode/main.go` 174、`cmd/smoke/main.go` 116
> 为把桥接讲清楚，另外核对了被这些文件直接依赖的实现：`internal/agent/agent.go`、`runtime.go`、`run_to_completion.go`、`agent_tool.go`、`internal/llm/provider.go`、`anthropic.go`、`internal/permission/*`、`internal/session/writer.go`、`internal/skills/catalog.go`、`active.go`、`executor.go`、`internal/instructions/loader.go`、`internal/task/manager.go`、`internal/hook/event.go`。凡引用外部函数名均已实际读过其定义。

---

## main.go 启动装配顺序

`main()` 是"读配置 → 建资源 → 交给 TUI"的线性装配，没有 DI 容器、没有 `wire`，全部显式 new。顺序如下（1..20）：

1. **`resolveConfigPath()`**：两层 fallback。先看 `./.mewcode/config.yaml`（开发模式），不存在再看 `~/.mewcode/config.yaml`（安装模式）；两者都不存在时**返回用户级路径**（由后续 `config.Load` 报错并打印配置模板提示）。依赖：`os.Stat` / `os.UserHomeDir`。
2. **`config.Load(cfgPath)` → `cfg`**：失败即 `os.Exit(1)`，并在 stderr 打印一份 `providers: - name / protocol / api_key / model` 的 YAML 样例。此处是**唯一的硬性前置依赖**，之后所有组件都直接或间接依赖 `cfg.Providers`。
3. **`root, _ := os.Getwd()`**：项目根，供后续 instructions / memory / MCP / permission / session 使用。错误被丢弃（`_`）。
4. **`instructions.NewLoader(root).Load() → instructionText`**：三层 `MEWCODE.md`（`<root>/MEWCODE.md` → `<root>/.mewcode/MEWCODE.md` → `~/.mewcode/MEWCODE.md`），支持 `@include` 展开（`maxIncludeDepth = 5`，带路径逃逸检测）。失败只打印 `[instructions] 加载项目指令失败`，**不中断**。依赖：3。
5. **记忆管理器**：`userHome, _ := os.UserHomeDir()`；`projectMemDir = <root>/.mewcode/memory`、`userMemDir = ~/.mewcode/memory`；`memory.NewManager(projectMemDir, userMemDir, nil, "")` —— 第三个参数是 provider，第四个是 model，**此刻 provider 尚未选定**（provider 在 `tui.New` 内部才由 `llm.New` 构造），所以传 `nil, ""`。随后 `memMgr.LoadIndex() → memoryText`。依赖：3。
6. **`tool.NewDefaultRegistry() → reg`**：注册 6 个内置工具 `read_file / write_file / edit_file / bash / glob / grep`（`tool/registry.go:135`）。
7. **MCP 接入**：`mcp.LoadConfig(root) → mcpCfg`（错误丢弃）→ `mcp.NewManager(context.Background(), mcpCfg, version)`，`defer mgr.Close()`；遍历 `mgr.Tools()` 逐个 `reg.Register(t)`。依赖：3、6。注意这里用的是 `context.Background()`，MCP 生命周期与 TUI 的 per-turn context 无关。
8. **`permission.NewEngine(root) → eng`**：加载三层规则（local > project > user）、编译黑名单、解析 `defaultMode`。即使返回 err 也**继续运行**（降级为空规则安全引擎，`eng` 必非 nil）。依赖：3。
9. **`hook.Load(root) → hookEngine`**：错误被丢弃，因此 `hookEngine` **可能为 nil**；TUI 侧全部 hook 调用点都做了 nil 保护（`m.hookEngine == nil` → return）。依赖：3。
10. **`compact.NewSessionContext(root) → sessionCtx`**：生成 `SessionID`（`YYYYMMDD-HHMMSS-xxxx`）、`SessionDir = <root>/.mewcode/sessions/<SessionID>`、`SpillDir = SessionDir/tool-results`。**失败即 `os.Exit(1)`**（与 4、5 的"降级"策略不同：会话目录是持久化的刚需）。
11. **`agent.SessionRuntime{...}`**：`Replacement`（内容替换状态）、`Recovery`（文件读取恢复状态）、`AutoTracking`（自动压缩追踪）、`Session: sessionCtx`、`ContextWindow: cfg.Providers[0].EffectiveContextWindow()`。依赖：2、10。**这里直接索引 `Providers[0]`，源码未体现空切片保护**（若 `config.Load` 允许零 provider 通过，则此处 panic；实际由第 2 步的 Load 校验兜住，属于隐性契约）。
12. **`session.NewWriter(sessionCtx.SessionDir) → writer`** + `defer writer.Close()`：`conversation.jsonl` 追加写（`O_CREATE|O_APPEND`，每条 `Encode` 后 `file.Sync()`）。依赖：10。
13. **后台清理**：`go session.CleanExpired(<root>/.mewcode/sessions, 30*24h)`。异步、不阻塞启动、失败只打 stderr。依赖：3。
14. **`conversation.NewWithHooks(writer.OnAppend(modelName), writer.OnReplace()) → conv`**：`modelName = cfg.Providers[0].Model`；`OnAppend` 只在**首条消息**写入 `model` 字段，`OnReplace` 先写一行 `{"type":"compact"}` 标记再 `AppendAll`。依赖：2、12。
15. **`subagent.LoadCatalog(root) → subagentCatalog`**：加载子 Agent 角色目录，打印 `[subagent] 已加载 N 个 Agent 角色`。依赖：3。
16. **`task.NewManager() → taskMgr`**：后台任务管理器（含 `done chan string`、`byName` 索引）。无外部依赖。
17. **`tui.New(...)` → `m`**：11 个参数的巨型构造函数（providers, version, reg, eng, runtime, writer, memMgr, instructionText, memoryText, hookEngine, taskMgr, subAgentCatalog）。TUI 内部继续二次装配，见下。
18. **`m.SetConversation(conv)`**：用第 14 步带回调的 Conversation **覆盖** TUI 内部 `New()` 里创建的空 `conversation.New()`。这是"先构造 TUI 再注入 conversation"的补丁式设计。
19. **`m.Run()`**：`tea.NewProgram(m).Run()`，无任何 `tea.WithXxx` 选项（无 AltScreen、无鼠标、无自定义输入输出）。错误 → `os.Exit(1)`。
20. **`hookEngine.Dispatch(ctx.Background(), EventSessionEnd, ...)`**：进程退出前兜底派发（`/clear`、`/resume` 路径另有各自的 SessionEnd）。

### `tui.New` 内部装配（第 17 步展开）

1. providers 为空则兜底为 `[{Name:"default", Protocol:"anthropic", Model:"unknown"}]`。
2. `textarea.New()`（`Prompt = "❯ "`、`Placeholder = "Send a message..."`、`ShowLineNumbers = false`、80×3）、`spinner.New()`（`spinner.Dot`）、`glamour.TermRenderer`（`style.json` 通过 `//go:embed` 注入零 margin 样式，失败回退 `light` 主题 + 同宽度 word wrap）。
3. `startMode = engine.StartMode()`（engine 为 nil 时 `ModeDefault`）。
4. `cwd, _ := os.Getwd()`；`sessionsDir = cwd + "/.mewcode/sessions"`（与 main 里 `filepath.Join` 写法不一致，字符串拼接）。
5. `command.New()` + `command.RegisterBuiltins(cmdReg)` → 13 条内置命令。
6. `skills.LoadCatalog(cwd) → cat`；`skills.NewActiveSkills()` → **写回 `runtime.ActiveSkills`**（跨轮保持激活状态，Agent 与 TUI 共享同一 runtime）。
7. 单 provider 分支：`llm.New(providers[0]) → m.provider`（失败则 `m.provider = nil`，`state = stateIdle`，后续 `m.ag.Run` 会 nil panic，源码未体现保护）→ `agent.New(provider, registry, version, engine, opts...)`，opts 依次为 `WithRuntime(runtime)`、`WithCatalog(cat)`、`WithHookEngine`、`WithMemoryManager`、`WithInstructionText`、`WithMemoryText` → `m.ag`。
8. `runtime.HookEngine = hookEngine`、`m.hookEngine = hookEngine`。
9. `skills.NewExecutor(cat, m.ag)`（`*agent.Agent` 实现 `SkillHost.ActivateSkill`）。
10. `command.RegisterSkillsAsCommands(cmdReg, cat, executor)`：每个 skill 注册为 `/<name>`，inline → `KindPrompt`，fork → `KindSkillFork`。
11. `tool.NewLoadSkillTool(executor)`（系统工具）→ `registry.Register`。
12. `tool.NewInstallSkillTool(cwd, cat, onInstalled)`（普通工具，受权限约束）；`onInstalled` 回调里**重新注册全部 skill 命令**（热更新）。
13. `cat.ValidateTools(...)` 校验 skill 的 `allowed_tools` 是否都存在，问题只打 stderr 警告。
14. `command.RegisterSkillCmd(cmdReg, SkillCommandDeps{Catalog, Executor, WorkDir, CmdReg})` → `/skill`。
15. `task.NewTaskListTool / TaskGetTool / TaskStopTool / SendMessageTool` → 注册 4 个任务工具。
16. `agent.NewAgentTool(subAgentCatalog, taskMgr, m.ag, bgEnabled=true)` → `registry.Register`；`agentTool.SetParentConvFn(m.conv.Messages)`。

多 provider 分支（`len(providers) > 1`）：`state = stateSelecting` + `initList()`，**不构造 provider、不构造 agent、不注册 skill/task/agent 工具**。`selectProvider` 只设置 `m.provider` 与 `state = stateIdle`，**源码未体现重新构造 Agent 的逻辑**——多 provider 路径下 `submitMessage` 会调用 nil 的 `m.ag`，属未完成实现。

---

## Bubble Tea 架构

### Model：单结构体 + 字段分区

`Model` 是一个 60+ 字段的扁平结构体（`tui.go:55`），没有拆成 Elm 风格的 sub-model，可读性靠注释分区：

- **UI 原语**：`textarea`、`spinner`、`list`（provider 选择用）、`resumeList`（会话恢复用）、`renderer`（glamour）、`width/height`。
- **依赖注入**：`providers`、`provider`、`registry`、`conv`、`engine`、`runtime`、`ag`、`writer`、`memMgr`、`instructionText`、`memoryText`、`sessionsDir`、`cmdRegistry`、`skillCatalog`、`skillExecutor`、`skillDeps`、`hookEngine`、`taskMgr`、`subAgentCatalog`、`agentTool`、`version`、`cwd`。
- **流式状态**：`cancel`（程序级取消，**从未被赋值**）、`turnCancel`（per-turn）、`events <-chan agent.Event`、`curReply strings.Builder`、`curTools []toolDisplay`、`turnStart time.Time`、`mode`、`iter`、`usageIn/usageOut`。
- **人在回路**：`pending *agent.ApprovalRequest`、`approveCursor int`。
- **命令/输出缓冲**：`completion completionMenu`、`pendingPrintln []string`、`pendingCmd tea.Cmd`。

`toolDisplay` 是 `{name, args}` 的极小值对象；`curTools` 是**切片而非单值**，因为 agent 的 `executeBatched` 会对连续只读工具批量并发，Start 事件先按序全部发出，所以 TUI 必须能同时展示多行 `● name(args) Running…`。

### 状态机：5 个状态，没有独立的 tool-running

```go
const (
    stateSelecting sessionState = iota // 多 provider 选择
    stateIdle                          // 等待输入
    stateStreaming                     // 等待/接收模型流（含工具执行期）
    stateApproving                     // 人在回路待批准
    stateResuming                      // 会话恢复列表
)
```

关键设计：**没有 `tool-running` 状态**。工具执行期间仍处于 `stateStreaming`，UI 靠 `len(m.curTools) > 0` 分支渲染"多行工具 + Running…"，否则渲染"spinner + Imagining… (Ns · 第 N 轮)"（`renderStreamingReply`）。也没有独立的"权限询问"以外的模态状态；`stateResuming`/`stateSelecting` 是"覆盖全屏"的列表态。

转移图（含触发）：
- `stateIdle --Enter(非空且非 / 命令)--> stateStreaming`（`submitMessage`）
- `stateIdle --/do、/review、skill(inline)--> stateStreaming`（`InjectAndSend → startStreaming`）
- `stateIdle --/resume--> stateResuming`（`OpenResumeMenu`，同时发出 `beginResume` 异步 Cmd）
- `stateSelecting --Enter--> stateIdle`（`selectProvider`）
- `stateStreaming --Event{Approval}--> stateApproving`（`handleAgentEvent` 中**不**重新发 `waitForEvent`）
- `stateApproving --Enter/1/2/3--> stateStreaming`（`commitApproval`）
- `stateStreaming --Event{Done}/Event{Err}--> stateIdle`（`finishTurn`）
- `stateResuming --Esc--> stateIdle`；`--Enter--> stateResuming`（等 `resumeDoneMsg` 才回 idle）；`resumeListMsg` 空列表 → 直接 `stateIdle`
- 取消（Esc / Ctrl+C）：`stateStreaming|stateApproving → turnCancel()` 后**仍是 stateStreaming**，靠 agent 回灌的 Done/Err 收尾。

### Update 分发：三段式优先级

```
WindowSizeMsg → 更新 width/height、textarea.SetWidth(w-4)、重建 renderer(w-4)
KeyPressMsg:
   ① ctrl+c 硬编码分支（全局最高优先级）
   ② Esc 硬编码分支
   ③ shift+tab（仅 stateIdle）切换权限模式
   ④ switch m.state → handleSelectingKey / handleIdleKey / (streaming: 吞掉) / updateApproving / updateResuming
agentEvent:
   state == stateApproving → updateApproving（实际只处理 KeyPressMsg，事件被忽略）
   否则 → handleAgentEvent
spinner.TickMsg → handleSpinnerTick（非 streaming 直接丢弃）
resumeListMsg / resumeDoneMsg → 仅 stateResuming 处理
default → 透传给 textarea.Update（光标移动、插入、删除等）
```

`handleAgentEvent` 内部用**无 tag 的 switch + case 布尔表达式**排定事件优先级，顺序即语义优先级：

```
Compact > Err > Approval > Tool(PhaseStart) > Tool(PhaseEnd) > Usage > Iter > Notice > Text > Done
```

细节：`Tool PhaseStart` 时若 `len(curTools)==0 && curReply.Len()>0`，会先把已累积的 preamble 作为 assistant 块 `tea.Println` 出去（分离"思考性前导文本"与后续工具轮），然后追加本次工具；`Tool PhaseEnd` 走 **FIFO 弹出队首**（agent 保证 Start/End 都按调用序 emit，所以队首就是本次结束的工具）。`Done` 时用 glamour 渲染整段 `curReply` 再打印。

### View：scrollback 模式，View 只画"最后几行"

`Run()` 用 `tea.NewProgram(m)`，**没有 `tea.WithAltScreen()`** → bubbletea 的 scrollback 模式：

- 所有对话内容（banner、用户消息、assistant 回复、工具行、工具结果、notice、错误）通过 `tea.Println(...)` 输出到终端历史，永久可滚。
- `View()` 只负责屏幕底部的一小块：`viewChat`（流式回复 + 带圆角边框的输入框 + 补全菜单 + 状态栏）、`viewApproving`（审批块 + 状态栏）、`viewResuming`（list.View）、`viewSelecting`（list.View + 提示行）。
- `View()` 返回 `tea.View`（v2 API：`tea.NewView(s string)`）。

渲染函数族：`toolLine`（`● name(args)`，青色）、`toolResultSummary`（`  ⎿  ` 缩进、**UI 侧截断 8 行**、错误红色）、`renderUserBlock`（`● text`）、`renderAssistantBlock`（`●\n` + 正文）、`renderErrorBlock`、`renderNoticeBlock`、`renderApprovalBlock`。样式全部是包级 `lipgloss.NewStyle()` 变量（硬编码 hex 颜色，无主题切换）。状态栏左模式右模型 + `↑in ↓out tok`（`formatCompact` 到 k/M），模式标签按 Mode 着色（default 绿 / acceptEdits 青 / plan 黄 / bypass 红）。

### 自定义 Msg 清单

| Msg 类型 | 定义处 | 语义 |
|---|---|---|
| `agentEvent` | `stream.go:9` `type agentEvent agent.Event`（**新类型而非别名**，故可在 type switch 中与 `agent.Event` 区分） | agent 事件流元素 |
| `resumeListMsg` | `resume.go:39` `{infos []session.SessionInfo; err error}` | 会话列表异步加载完成 |
| `resumeDoneMsg` | `resume.go:105` `{conv, writer, sessionCtx, sessionID, msgCount, err}` | 会话恢复（含压缩）完成 |

框架 Msg：`tea.WindowSizeMsg`、`tea.KeyPressMsg`、`spinner.TickMsg`，以及 textarea / list 内部产生的 Msg（走 default 分支透传）。

Command（Cmd）形式：
- `waitForEvent(m.events)`（agent 事件接力，唯一的数据入口）
- `tea.Println(...)` / `tea.Batch(...)` / `tea.Quit`（`Quit()` 里由 `pendingCmd` 携带，经 `flushPending` 交给框架）
- `m.spinner.Tick`、`m.textarea.Focus()`、`m.list.Update(...)` 返回的 Cmd
- `beginResume()` / `doResumeSession(info)`（`resume.go` 里的闭包 Cmd，返回自定义 Msg）

### 并发事件如何在 Update 中处理

三条并发的"外部世界"通道，处理策略各不相同：

1. **agent 事件流**：唯一被正规"用 Cmd 拉取"的通道。`waitForEvent` 每次 Update 只发一个读取 Cmd，形成**读一个 → 处理 → 再补一个**的接力，天然把并发收敛到 Update 的单线程里（bubbletea 保证 Update 串行）。
2. **后台任务完成通知**（`tasks.go`）：**不走 Update，不走 tea.Msg**。`Init()` 里 `go m.consumeTaskDone()` 常驻 goroutine，直接 `range m.taskMgr.SubscribeDone()`，把 `<task-notification>` 原文 `m.runtime.AppendReminders(...)`，由 agent 下一轮 `buildReminder → TakeReminders` 注入。跨 goroutine 写共享状态由 `SessionRuntime.mu` 保护（`runtime.go:31`），是安全的；但这条路径**没有任何 UI 提示**（用户看不到"某个后台任务完成了"，只有模型会看到）。
3. **异步命令 Cmd**（resume 列表加载、恢复、强制压缩）：Cmd 在 bubbletea 的 goroutine 里跑，返回 Msg 回到 Update；`doResumeSession` 里还做了 token 估算 + `RunForceCompact`，属于"重活在 Cmd 里"的正确用法。

### Esc / Ctrl+C 取消如何传播到 context

```
Esc (stateStreaming|stateApproving)
  ├─ 若 stateApproving 且 pending != nil：
  │     select { case pending.Respond <- OutcomeDenyOnce: default: }   // 非阻塞，先解开 agent 的 select
  ├─ m.turnCancel()                                                    // 取消 per-turn context
  └─ return m, waitForEvent(m.events)                                  // 继续读残余事件，等 Done/Err

Ctrl+C (stateStreaming|stateApproving)  同上
Ctrl+C (其他状态)                       m.cancel()（nil，no-op）+ tea.Quit
```

`context.WithCancel(context.Background())` 在 `submitMessage` / `startStreaming` 里**每轮新建**，存在 `m.turnCancel`，`finishTurn` 里置 nil。取消后 agent 侧通过三层感知：`emit()` 的 `select { ch <- e; case <-ctx.Done() }` 返回 false → 立刻回灌"（已取消。）"占位结果 → `ensureAssistantTail(conv, noticeCancelled)` 保证历史以 assistant 结尾（合法历史，smoke 场景 2 专门验证这一点）→ 关闭 channel → TUI 收到 `Done`（`!ok` 分支）→ `finishTurn`。

**注意**：`m.cancel` 字段在 `tui.go` 与 `commands.go` 中被读取（`Ctrl+C` 退出分支、`Quit()`），但**全仓没有任何地方赋值**（grep 确认），因此"程序级取消"实际不存在——退出完全依赖 `tea.Quit`。这是把 per-turn cancel 做扎实后遗留的死字段。

### 竞态防护现状

已具备：
- bubbletea 的 `Update` 单 goroutine 串行化，所有 UI 状态读写都在这一条线程上。
- agent 侧：`runMu sync.Mutex`（保证 `Run` 与 `RunForceCompact` 不并发）+ `running int32` 原子标记（`IsRunning`，供 TaskManager/AgentTool 判断）。
- `SessionRuntime.mu`：保护 `UsageAnchor / AnchorMsgLen / TurnCount / PendingReminders`（`UpdateAnchor/ResetAnchor/GetAnchor/IncTurn/AppendReminders/TakeReminders` 全部加锁）。
- `session.Writer.mu`：JSONL 单行写入 + `Sync` 串行。
- `skills.Catalog.mu`（RWMutex，`Reload` 先无锁做 I/O 再写锁原子替换）、`skills.ActiveSkills.mu`。
- `ApprovalRequest.Respond` 是**缓冲 1** 的 channel → TUI 侧发送永不阻塞。

源码未体现防护 / 潜在风险：
- **Esc 取消时可能存在两个 `waitForEvent` 读者**：上一个 `waitForEvent` Cmd 的 goroutine 仍阻塞在 `<-ch`，Update 又补一个新的 → 两个 goroutine 竞争同一个无缓冲 channel，事件到达顺序可能与 agent 的 emit 顺序不一致（不会崩溃，但理论上可乱序）。正常路径（收到事件 → 处理 → 补读）维持"同时最多一个读者"的不变式，取消路径打破它。
- `finishTurn` 把 `m.events = nil`；此后若任何路径调用 `waitForEvent(m.events)` 会**永久阻塞在 nil channel**（当前所有调用点都在 `m.events` 非 nil 的状态下，属隐性契约）。
- `consumeTaskDone` 的 goroutine 依赖 `SubscribeDone()` 永不关闭（无退出路径），随进程退出而终止，可接受但无优雅关闭。
- `Tool PhaseEnd` 的 FIFO 弹出依赖 agent"Start/End 都按序 emit"的承诺；若 agent 侧改为并发 emit End，TUI 会错配工具名。

---

## agent Event → bubbletea Msg 的桥接

桥接代码只有 21 行（`stream.go`），是整个 TUI 的"心脏瓣膜"：

```go
type agentEvent agent.Event          // 新类型：把 agent 事件包装成可识别的 tea.Msg

func waitForEvent(ch <-chan agent.Event) tea.Cmd {
    return func() tea.Msg {
        ev, ok := <-ch
        if !ok {
            return agentEvent{Done: true}   // channel 关闭 == 本轮结束
        }
        return agentEvent(ev)
    }
}
```

要点：

1. **Cmd 闭包 = 一次阻塞读**。`tea.Cmd` 就是 `func() tea.Msg`，bubbletea 在自己的 goroutine 中执行它；闭包里的 `<-ch` 阻塞期间不会卡住 UI（Update/View 仍在跑，spinner 靠 `spinner.TickMsg` 自驱动）。这是"把阻塞式 channel 读接进 Elm 架构"的标准做法。
2. **一次只读一个事件**。handler 每处理完一个事件就 `return m, waitForEvent(m.events)` 补一个读，形成接力。因为补读发生在 Update 里（串行），所以正常路径下永远只有一个读者。
3. **channel 关闭 → Done**。agent 侧 `Run` 的 goroutine `defer close(ch)`，所以"close"被复用为"Done 信号"的兜底：即便 agent 异常返回而没有 emit `Done`，TUI 也会收到 `agentEvent{Done:true}` 并 `finishTurn`（此时 `curReply` 可能为空 → 不打印任何块）。
4. **channel 生命周期**：
   - 创建：`m.events = m.ag.Run(turnCtx, m.conv, m.mode)`（`submitMessage` / `startStreaming`），只有一处来源。
   - 消费：`waitForEvent` 接力，直到收到 `Done`/`Err`/关闭。
   - 释放：`finishTurn()` 里 `m.events = nil`、`turnCancel = nil`、`curReply.Reset()`、`curTools = nil`、`iter = 0`、`state = stateIdle`；**保留 `mode` 与 `usageIn/usageOut`**（跨轮累计语义）。
   - 无缓冲 channel（`ch := make(chan Event)`，`agent.go:165`）是**背压阀门**：TUI 不读 → agent 的 `emit` 阻塞 → 整个 ReAct 循环暂停。这正是"人在回路"能阻塞 Agent 的机制基础。
5. **事件字段互斥由 handler 的 case 顺序裁决**，而不是由结构体保证（见上文优先级列表）。
6. `Err` 事件的语义是"本轮中断但不结束会话"：TUI 打印红色错误块并 `finishTurn()`，**不再补读 channel**，残余事件被丢弃；agent 在错误路径统一 `ensureAssistantTail(conv, noticeStreamErr)` 后 return → `close(ch)`，无泄漏。
7. `event.Done` 与 `event.Err` 可能都出现吗？看 `Run`：错误路径 `emit(Event{Err})` 后 `return`（**不 emit Done**），自然结束路径 emit `Done` 后 return。两条路径互斥，符合 `llm.StreamEvent` 注释里的"Done/Err 互斥"约定。
8. **Notice 事件**（`noticeMaxIter` / `noticeUnknownTools` / 压缩进度）只进 UI 不进历史，历史侧由 `ensureAssistantTail` 写入对应兜底文本，两处文案刻意保持同源常量。

---

## 人在回路弹窗

### 触发条件

不是 TUI 主动弹窗，而是 **agent 在权限判定为 `Ask` 时通过事件流请求**：

1. `agent.executeBatched` 走**非只读、串行**分支（`a.registry.IsReadOnly(name) == false`，即 write_file / edit_file / bash / MCP 及未知工具）。
2. HOOK `PreToolUse` 未被拦截（被拦截则直接产出 hook block 结果，不进入审批）。
3. `a.eng.Check(mode, call, false)` 返回 `permission.Ask`。`modeFallback`（`engine.go:143`）只产 Allow/Ask，绝不产 Deny：default/plan 的 Write/Exec、acceptEdits 的 Exec → Ask；`reason` 形如 `"default 模式下 文件写入 类操作需确认"`。
4. 子 Agent 的两条旁路：`a.dontAsk == true` 直接 Allow（不弹窗）；`a.approvalUpgrader != nil` 走升级回调（见下文风险）。
5. 构造 `&ApprovalRequest{Name, Args: argPreview(call.Input), Reason: reason, Respond: make(chan permission.Outcome, 1)}`，`emit(Event{Approval: req})`，然后：

```go
select {
case o := <-respond:  return o, true
case <-ctx.Done():    return 0, false     // ok=false → 全部剩余工具回灌"（已取消。）"
}
```

### 三选一如何回写

TUI 侧 `handleAgentEvent` 收到 `Approval` → `m.pending = ev.Approval; m.approveCursor = 0; m.state = stateApproving; return m, nil`（**刻意不补 `waitForEvent`**，让 agent 停在 `emit` 或 `select`）。

`viewApproving` + `renderApprovalBlock` 渲染三行菜单：

| 索引 | 标签 | Outcome | 语义（源码文案） |
|---|---|---|---|
| 0 | `1. 允许本次` | `OutcomeAllowOnce` | 仅本次放行，不记录规则 |
| 1 | `2. 永久允许（写入本地配置）` | `OutcomeAllowForever` | 记录精确 allow 规则，跨会话生效 |
| 2 | `3. 拒绝本次` | `OutcomeDenyOnce` | 回灌错误给模型，让其调整策略 |

输入：`↑/k`、`↓/j` 移动 cursor（钳制 0..2）；`enter` / `space` 提交 `outcomeForIndex(cursor)`；数字 `1/2/3` 直达。`commitApproval(outcome)`：`m.pending.Respond <- outcome`（缓冲 1，不阻塞）→ 清 `pending`/`cursor` → `state = stateStreaming` → 补 `waitForEvent`。

agent 侧收到 outcome 的处理（`agent.go:847`）：
- `DenyOnce` → `llm.ToolResult{Content: "用户拒绝执行：" + reason, IsError: true}` 回灌，**模型看到一条工具错误并可能改变策略**（这是"拒绝"被设计成可自愈的关键）；不 emit PhaseStart，只 emit PhaseEnd。
- `AllowOnce` → 只 emit PhaseStart → `registry.Execute(ctx-with-`tool.DefaultTimeout`)` → emit PhaseEnd。
- `AllowForever` → 先 `eng.PersistLocalAllow(call)`（失败则 emit 一条 Notice `（写入本地规则失败: %v）`，**仍然执行**），再走 Execute。

`PersistLocalAllow`（`permission/persist.go`）的规则生成有三条硬约束：`ruleFor` 生成**不含通配的精确规则** `FriendlyName(pattern)`；命令串用 `escapeGlob` 转义 `\ * ? [ ]`（防止"永久允许"被泛化成通配放行）；去重幂等（已存在同名规则直接 return），写 `<root>/.mewcode/settings.local.yaml` 并同步更新内存 `e.local.allow`。

### 如何阻塞 Agent 协程 + 超时处理

阻塞链路：`requestApproval` 的 `select` ↔ TUI 不发 `waitForEvent`。两端任意一端不动作，另一个就永远等待——**没有任何超时**（源码未体现 timeout / 默认拒绝 / 倒计时）。唯一的解阻塞路径：

1. 用户按键 → `commitApproval` 发送 outcome。
2. 用户按 Esc / Ctrl+C → Update 先把 `OutcomeDenyOnce` 塞进缓冲 channel（`select` + `default` 防阻塞），再 `turnCancel()`；agent 侧 `select` 会随 `ctx.Done()` 返回 `ok=false`，把剩余工具全部标记为"（已取消。）"，然后沿取消路径收尾。
3. ctx 被外部取消（同一 `turnCtx`）。

风险与缺口（源码未体现）：
- **子 Agent 的 Ask 无人应答**：`ApprovalUpgrader` 类型、`WithApprovalUpgrader` option、`agent.go:787` 的升级分支都已实现，但 **TUI/agent_tool 侧从未调用 `WithApprovalUpgrader`**（全仓 grep 只有定义与内部使用）。子 Agent（前台）的 Approval 事件被投到 `AgentTool` 内部的 `events := make(chan Event, 32)`，而 `task.aggregateEvent` 只统计 Tool/Usage，**忽略 Approval** → 子 Agent 会一直阻塞在该 `select` 上，直到前台 120s 超时（`autoBackgroundDuration`）转后台，转后台后仍无人应答，最终只能靠父 ctx 取消才解开。
- `/clear`、`/resume` 等路径若在 pending 状态下被触发，`pending` 不会被清（`finishTurn` 也不清），存在脏状态可能。
- 审批过程**不写入 conversation/JSONL**，也无 hook 事件（`EventNotification` 的注释说"权限 Ask 弹出审批时"派发，但实际只在 stream 出错时派发，见 `agent.go:320`），因此审批记录不可审计、不可回放。

---

## 命令体系

### 注册表结构（`registry.go`）

```go
type Registry struct {
    byName  map[string]*Command // 主名 + 别名都指向同一个 *Command，key 已小写
    visible []*Command          // 排除 Hidden，按 Name 字典序（SliceStable 维护）
}
```

- `Register` 的三道校验：Name 非空、Name 必须全小写、`Name + Aliases` 中任一 key 与已有冲突 → **`panic`**。这是刻意的"启动期快速失败"：命令表是编译期知识，写错了不该等到运行时。
- `Lookup` 大小写不敏感（内部 `strings.ToLower`），别名与主名等价。
- `Visible()` 返回副本（防外部改内部切片），供 `/help` 与补全菜单。
- `PrefixMatch(prefix)`：trim 掉前导 `/`、转小写、**只匹配 `Name` 前缀**（不匹配 Aliases、不匹配 Description），空前缀返回全部 visible。
- `RemoveNames(names)`：skill reload 时清理旧命令，同时重建 `byName` 与 `visible`（`visible` 会因删除而变成"非连续切片"，但仍保持有序）。

`Command` 结构：`Name`（不含 `/`）、`Aliases []string`、`Description`、`Kind`、`Hidden`（`/help` 与补全都不显示，但 dispatch 仍可命中——`registry_test.go` 的 `TestVisible_ExcludesHidden` 明确断言这一点）、`Handler func(ctx, UI) error`。

### 三类（实为四类）Kind 的区分

```go
KindLocal     // 纯本地：只打印信息，不改 Model，不进对话历史
KindUI        // 影响界面：可改 Model 状态，不进对话历史
KindPrompt    // 提示词：向对话注入 user 消息 + 触发 LLM 回合
KindSkillFork // Skill fork：异步执行后以 assistant 消息写入对话
```

- **local vs prompt 的判别点不在注册表，而在 `Handler` 的实现**：Kind 是**声明式的元数据**，真正分流的是 handler 调用了 `UI` 的哪个方法。`KindPrompt` 的 handler 一定调 `ui.InjectAndSend(label, preset)`；`KindUI` 的 handler 调 `SetMode/Quit/ForceCompact/OpenResumeMenu/ClearAndNewSession`；`KindLocal` 只调 `Println/Error` 与只读查询。
- **Idle 守护**（`commands.go:236`）：`(Kind == KindUI || Kind == KindPrompt || Kind == KindSkillFork) && !m.Idle()` → `Error("请等待当前任务完成")`。也就是说 5 条 `KindUI` 命令（`/exit`、`/clear`、`/plan`、`/resume`、`/compact`）在 streaming 期间**一律被拒**，必须先用 Esc 取消本轮；`KindLocal` 不受限（`/status`、`/help` 随时可查）。
- **依赖倒置**：handler 只依赖 `command.UI` 接口（`ui.go` 22 个方法 + `NopUI()` 测试桩），`*tui.Model` 实现它。这样 command 包对 tui 包**零依赖**（否则会形成 import cycle），也让 handler 可以用 `recordingUI` 单测（`builtins_test.go`）。
- `KindSkillFork` 的实际执行**没有走 handler**：`makeSkillHandler` 对 fork 直接返回错误 `"fork skill %q 暂不支持通过命令直接调用，请使用自然语言触发 LoadSkill"`（`skills.go:59`），只有 `KindPrompt`（inline）真正工作。

### 命令解析（`dispatch.go` 的 `Parse`）

返回 `(name, isSlash)` 四态：
- `("", false)`：非 `/` 开头 → 送 LLM。
- `("/", true)` 实为 `("", true)` + 仅 `/`：Lookup 必 miss。
- `(name, true)`：合法 `/name`。
- `("", true)`：`//double`、`"/ help"`、`"/help xx"`（**有尾随参数**）→ 一律 `("", true)`，Lookup 必 miss。

`Parse` 是**刻意保守**的：它只认"纯命令名"，参数解析被推迟到 TUI 层。因此 `dispatchSlash` 里有一段补丁逻辑——当 `name == ""` 时手动从原文再切一次命令名（`strings.IndexByte(raw, ' ')`）：

```go
name, isSlash := command.Parse(text)
if name == "" { /* 手动提取 "/cmd args" 的 cmd */ }
```

结果是 `/help xx` 这类输入不会走"未命中提示"，而会命中 `help` 并**忽略参数**（带参数命令除 `/skill` 外都由 TUI 特判）。这是"解析层极简 + 上层补丁"的痕迹。

`/skill` 是唯一有专属分发路径的命令：`dispatchSlash` 里 `if name == "skill"` → `handleSkillCmd(text)` → 解析出子命令串 → `command.HandleSkillSub(ui, args, deps)`（`builtin_skill.go:50`）。

### 全部内置命令清单（13 条 + 动态 skill 命令）

`RegisterBuiltins` 注释写"一次性注册 12 条内置命令"，但**实际注册 13 条**，`builtins_test.go` 断言 `len(visible) != 13` 即失败——注释与代码不一致（文档债）。

| # | 命令 | Kind | Handler | 职责与实现要点 |
|---|---|---|---|---|
| 1 | `/clear` | KindUI | `handleClear` | `ClearAndNewSession()` + `ClearActiveSkills()` + 提示。内部顺序：派发 SessionEnd → 关旧 writer → `compact.NewSessionContext` → 新 writer → `bindConversation` → `runtime.ResetForNewSession` → 归零 iter/usage → 派发 SessionStart。任一失败只 `Error` 并 return（可能留下半初始化状态） |
| 2 | `/compact` | KindUI | `handleCompact` | `ForceCompact()`：按 mode 取工具集（Plan 只读）→ `ag.RunForceCompact`（内部 `runMu` 串行）→ 打印 `已压缩，token 从 X 降至 Y` |
| 3 | `/do` | KindPrompt | `handleDo` | `SetMode(ModeDefault)` + `InjectAndSend("/do", prompt.ExecuteDirective)`，`ExecuteDirective = "请按上面的计划开始执行。"`——Plan Mode 的出口 |
| 4 | `/exit` | KindUI | `handleExit` | `ui.Quit()` → `cancel()`(no-op) + `pendingCmd = tea.Quit` |
| 5 | `/help` | KindLocal | `handleHelp(reg)` | **闭包捕获 registry**（注册顺序依赖），遍历 `Visible()` 计算最长名对齐，逐行 `/<name>   desc`，一次 `Println`（多行文本靠 `joinLines`） |
| 6 | `/hooks` | KindLocal | `handleHooks` | 按 event 分组打印已加载 hook 规则（`[once]`/`[async]` 标记）+ `Loaded from: <sources>`；`HookRules() []interface{}` 是因为 command 包不想依赖 hook 包，TUI 侧做类型断言回转 `hook.Rule` |
| 7 | `/memory` | KindLocal | `handleMemory` | 列出 `memMgr.ListFiles()`（project + user 合并） |
| 8 | `/permission` | KindLocal | `handlePermission` | 打印 `ui.Mode().String()`（`default` / `acceptEdits` / `plan` / `bypassPermissions`） |
| 9 | `/plan` | KindUI | `handlePlan` | `SetMode(ModePlan)` + 提示。真正的只读约束在 agent 侧：Plan 模式 `registry.ReadOnlyDefinitions()` 收窄工具集 + 每轮注 reminder |
| 10 | `/resume` | KindUI | `handleResume` | `OpenResumeMenu()`：`state = stateResuming` + `textarea.Reset()` + `pendingCmd = beginResume()`（异步 `session.ListSessions`） |
| 11 | `/review` | KindPrompt | `handleReview` | 注入 `reviewDirective` 常量文本（"请审查当前上下文中的代码变更…"）并发起回合 |
| 12 | `/session` | KindLocal | `handleSession` | 打印 `Session: <SessionID>` 与 `Path: <writer.Path()>` |
| 13 | `/status` | KindLocal | `handleStatus` | 6 项 `keyWidth=11` 对齐输出：Mode / Tokens(累计 in/out) / Tools(registry.Count) / Memories(文件数) / Model / Directory |
| + | `/skill` | KindLocal | `handleSkill`→`HandleSkillSub` | `list`（`Executor.ListSummaries()`，格式 `  /%-20s [source] mode - desc`）/ `info <name>`（Meta 详情：Mode/ForkContext/Model/AllowedTools/Source）/ `reload`（`Executor.ReloadCatalog` → `CmdReg.RemoveNames(removed)` → 重新 `RegisterSkillsAsCommands`） |
| + | `/<skill-name>` | KindPrompt / KindSkillFork | `makeSkillHandler` | 每个 skill 动态注册；inline 走 `exec.Execute` → `InjectAndSend("/"+name, body)`；fork 返回"暂不支持"错误 |

另外 `InstallSkillTool` 安装 skill 后的 `onInstalled` 回调会**重新注册全部 skill 命令**（新装的 skill 立即可用，无需重启）；`RemoveNames` 的 `visible` 重建保证了 `/help` 不漏不重。

### 补全（`complete.go`）如何实现

`completionMenu{items, cursor, offset, active}` + `completionMaxRows = 8`：

1. **触发与刷新**：`handleIdleKey` 里每次 textarea 变更后调用 `syncCompletionFromInput()` → `completion.Update(textarea.Value(), cmdRegistry)`。`Update` 里：空串或不以 `/` 开头 → `Hide()`；否则 `items = reg.PrefixMatch(input)`、`active = true`，并修正 `cursor`（越界回退）与 `clampOffset()`（滚动窗口，保证 cursor 可见）。
2. **窗口滚动**：`clampOffset` 双向钳制：`offset > cursor` 时上移，`cursor >= offset+8` 时下移，渲染最多 8 行，上/下溢出分别显示 `↑ N more` / `↓ N more`。
3. **渲染**：`Render(width)` 计算最长名做 padding，选中行黑底白字高亮，普通行灰色；无匹配时显示 `无匹配命令`（注意：`active` 仍为 true，因为 `/do xx` 这种带参数输入会走进"无匹配"分支）。
4. **按键拦截**：`handleCompletionKey(msg) (tea.Cmd, bool)` 在 `handleIdleKey` 最前面调用，`consumed = true` 则短路：
   - `KeyUp/KeyDown` → `MoveUp/MoveDown`，消费。
   - `KeyTab` → 取 `Selected()` → `textarea.SetValue("/"+name)` → `Reset()` → `Hide()` → **立刻 `dispatchSlash` 执行**（Tab = 选中并执行）。
   - `KeyEnter` → 把选中项写入 textarea、`Hide()`、**返回 consumed=false**，让 `handleIdleKey` 的 Enter 分支继续走正常提交路径（等价于执行，但走的是"文本 → 提交"链路）；若 `items` 为空则 `Hide()` + 消费（**吞掉这次回车**）。
   - `KeyEscape` → `Hide()`，但**这条分支实际是死代码**：`Update` 顶部的全局 Esc 分支在 `stateIdle` 时直接 `return m, nil`，根本不会下传到 `handleIdleKey`（受影响的还有"Esc 无法撤销已弹出的补全菜单"）。
5. 补全只做**命令名前缀匹配**：无模糊匹配、无描述匹配、无别名匹配（`Aliases` 字段已实现于 Registry 但 13 条内置命令**一条都没用**）、无参数补全（`/skill <TAB>`、`@file` 均不支持）。

---

## 系统提示工程化

### 七个固定模块 + 三个可选槽（`modules.go`）

`Module{Name, Priority, Content}`，`FixedModules()` 返回 7 个固定模块（Priority 10..70，内容内置常量）：

| Priority | Name | 内容要点（原文摘要） |
|---|---|---|
| 10 | 身份 | "你是 MewCode，一个终端 AI 编程助手（类似 Claude Code），使用 Go 实现…" |
| 20 | 系统约束 | 操作边界：文件操作限定工作目录、**密钥绝不回显**、破坏性操作先确认、拒绝恶意请求 |
| 30 | 任务模式 | ReAct 多步自主循环：分析→选工具→执行→观察→决定下一步；编辑前必读；完成后简洁收尾 |
| 40 | 动作执行 | 工具调用策略：主动调用、**多个只读工具可并发**、有副作用工具谨慎、基于实际结果回答 |
| 50 | 工具使用 | **工具选择优先级**：read_file/glob/grep 优先于 bash 拼凑；编辑前必须 read_file 并确认 `old_string` 唯一 |
| 60 | 语气风格 | 简洁直接、不奉承、用中文 |
| 70 | 文本输出 | 代码块标注语言、Markdown 结构化、最终答复精炼 |

`OptionalModules(instructions, memory, skillsCatalog)` 返回 3 个可选槽：

| Priority | Name | 数据来源 | 空时行为 |
|---|---|---|---|
| 80 | 自定义指令 | `instructionText`（三层 `MEWCODE.md` + `@include`） | 空则跳过 |
| 90 | 可用 Skill 列表 | `RenderSkillsCatalog(catalog.ToPromptItems())` | 空则跳过 |
| 100 | 长期记忆 | `memoryText`（`memMgr.LoadIndex()`） | 空则跳过 |

**注意槽位顺序与参数顺序不一致**：函数签名是 `(instructions, memory, skillsCatalog)`，但 Priority 让 skills(90) 排在 memory(100) 之前。因为最终顺序由 `AssembleSystem` 的 Priority 排序决定，参数顺序不影响结果——这是"排序即真相、参数序不可信"的隐式约定，读代码时容易误判。

### 如何保证逐字节稳定（Prompt Cache 友好）

`AssembleSystem(mods)`（`prompt.go:11`）三条规则：
1. **先防御性拷贝再排序**（`copy` + `sort.SliceStable`，避免修改调用方切片）；
2. 按 `Priority` 升序，`SliceStable` 保证同优先级按其输入顺序（确定性）；
3. 跳过 `Content == ""` 的模块，用 `"\n\n"` 连接 —— 不留占位、不产生 `\n\n\n`（`TestAssembleSystem_SkipEmpty` 专门断言）。

测试层面的稳定性契约：`TestBuildSystemPrompt_Deterministic` 断言连续两次 `BuildSystemPrompt("","","")` **逐字节相等**（注释直接点名 "N1 cache stability"）；`TestAssembleSystem_Extensible` 断言新增模块只需进列表、优先级 5 能插到最前。

运行时的稳定性来源：
- `BuildSystemPrompt` 每次 `Run` 开头调用一次（`agent.go:193`、`run_to_completion.go:59`），输入 `instructionText` / `memoryText` 在 `main` 里只加载一次 → 同进程内跨轮**完全一致**；`memoryText` 是索引快照，只有 `UpdateAsync` 落盘后下次进程重启才会变。
- `RenderSkillsCatalog` 的顺序来自 `Catalog.Names()` → `order` 由 `sort.Strings` 维护 → 字典序稳定。
- **变化因子被刻意排除出 Stable**：时间、日期、git 状态、模型名都在 `Environment` 段；活跃 Skill（用户中途激活）也只拼到 `envText`。因此"稳定段逐字节不变"这一前提成立。

### 缓存断点的真实落点（cache_control）

`llm.System{Stable, Environment}` 两段在 `toAnthropicSystem`（`anthropic.go:123`）里被拆成两个 `TextBlockParam`：

```go
if sys.Stable != "" {
    blocks = append(blocks, anthropic.TextBlockParam{Text: sys.Stable, CacheControl: anthropic.NewCacheControlEphemeralParam()})  // 打点
}
if sys.Environment != "" {
    blocks = append(blocks, anthropic.TextBlockParam{Text: sys.Environment})                                                       // 不打点
}
```

注释里写明了踩过的坑：**必须用 `NewCacheControlEphemeralParam()` 构造器，空字面量会被 `omitzero` 丢掉**。全项目**只用 1 个缓存断点**（Anthropic 允许 4 个）：工具定义（`toAnthropicTools`）与消息历史均未打点，`reminder` 也不打点（它被并入最后一条 user 消息，天然位于缓存前缀之后，不破坏前缀）。命中效果由 `Usage.CacheRead/CacheWrite` 上报（`anthropic.go:229`），但 **TUI 只在状态栏展示 `↑in ↓out`，缓存读写字段被 `agent.Event.Usage` 收集后未渲染**（`handleAgentEvent` 里只累加 Input/Output）。

### environment 段采集哪些字段（`environment.go`）

`GatherEnvironment(version, model)` 采集 6 个字段，`Render()` 生成 `"环境信息\n" + 逐行`，**空值项直接省略**：

| 字段 | 采集方式 | 稳定性 |
|---|---|---|
| `WorkingDir` | `os.Getwd()`（失败留空） | 会话内稳定 |
| `Platform` | `runtime.GOOS` | 稳定 |
| `Date` | `time.Now().Format("2006-01-02")` | **每天变** |
| `GitStatus` | `git status --porcelain`（`context.WithTimeout(2s)`），空输出 → `"clean"`，否则 `"%d 个文件有改动"`；**命令失败/非 git 目录 → 留空（N4 降级）** | 每次 Run 可变 |
| `Version` | 传入的 `version`（ldflags 注入） | 稳定 |
| `Model` | `provider.Model()` | 稳定 |

两条明确的设计约束写在注释里：**"不读环境变量（N5）"**（避免 API Key 等敏感信息经 env 进入提示词——与固定模块"系统约束"里"密钥绝不回显"形成双重保障），**git 外调失败只降级不中断**。调用时机：每个 `Run` 开头一次（`env := prompt.GatherEnvironment(...)`），`envText` 每轮重建（为的是把中途激活的 Active Skill 拼进去）。

### reminder（Plan Mode）首轮 / 后续的注入策略

`reminder.go` 三个常量 + 两个包装函数：
- `planReminderFull`：完整版——"你当前处于计划模式…只能使用只读工具（read_file、glob、grep）…不能写文件、编辑文件或执行 shell 命令。请产出清晰分步的执行计划，然后停止，等待用户用 `/do` 批准"。
- `planReminderConcise`：精简版——"仍在计划模式。继续只读调研，产出计划后等待 /do。"
- `SystemReminder(body)`：统一包 `<system-reminder>\n…\n</system-reminder>`，让模型把注入理解为"系统补充上下文"而非用户提问（`TestSystemReminder_WrapsBody`）；`PlanReminder(full bool)` 二选一。

策略由 `agent.buildReminder(mode, iter)` 决定（`agent.go:1162`）：

```go
planReminderInterval = 4   // 常量，内置不可配
full := iter == 1 || (iter-1)%planReminderInterval == 0   // iter = 1, 5, 9, 13, ...
```

即**首轮给完整版**（建立约束），之后**每 4 轮重述一次完整版**（防长循环中的指令漂移），其余轮次给一句话精简版（省 token）。非 Plan 模式不注入 plan reminder。

同一函数还把 `runtime.TakeReminders()`（hook 注入的 `InjectedPrompts` + `TaskManager` 的 `<task-notification>`）取出并**清空**（一次性语义），与 plan reminder 用 `"\n\n"` 连接成最终 `reminder` 字符串。

注入位置：`llm.Request.Reminder` → `appendReminderAnthropic`（`anthropic.go:141`）：**并入最后一条 user 消息的 content 块**（末条非 user 时新起一条 user 消息）。这样做同时满足两件事——(a) 不污染 system 前缀，缓存友好；(b) 保证 user/assistant 角色交替合法（注释标注为 N3）。

### skills_block 注入格式

两阶段（两个函数、两个位置）：

1. **第一阶段·目录**（`RenderSkillsCatalog`，进 system prompt 的 priority 90 槽，每次 Run 头一次）：

```
## 可用 Skill（调用 LoadSkill 工具激活）

以下 Skill 可通过 LoadSkill 工具按需激活。激活后 SOP 将钉在环境上下文最显眼位置。

- **/<name>**: <description>
```

空列表返回 `""` → 整个槽被 `AssembleSystem` 跳过。

2. **第二阶段·正文**（`RenderActiveSkillsBlock`，拼到 `envText` 末尾，**每轮迭代重建**）：

```
## Active Skills

以下 Skill 已激活，其 SOP 指令优先于通用系统指令：

### Skill: <name>

<body>
```

末尾 `strings.TrimRight(sb.String(), "\n")` 收口。激活数据结构是 `skills.ActiveSkills`（`map[name]idx` + `[]ActiveEntry`，同名重复激活**覆盖原位置**而非追加），存续于 `SessionRuntime.ActiveSkills`（跨轮、跨 Run），`/clear` 通过 `ClearActiveSkills` 清空、新会话 `ResetForNewSession` 也会清。

这套"目录在 system（缓存段，稳定）+ 正文在 Environment（非缓存段，动态）"的切分，是提示词工程里非常正确的分层：**目录是"能力声明"必须常驻；SOP 正文是"当轮指令"必须最新**。

---

## 设计决策与权衡

1. **决策：TUI 用单个巨型 `Model` + 无 AltScreen 的 scrollback 模式。**
   为什么：终端 AI 编程助手的核心信息量在"历史对话 + 工具输出"，`tea.Println` 把内容交给终端原生 scrollback（可选中、可复制、可 `Cmd+F`），View 只画底部几行，`Update` 里不需要维护任何"历史行列表"。
   替代方案：AltScreen 全屏 + 自己维护 viewport（如 Claude Code/Ink 的部分做法）→ 需要实现滚动、选中、复制、性能裁剪（长输出截断、虚拟滚动），复杂度陡增；代价是本项目**无法做整体布局**（没有固定 header、没有侧栏、状态栏只能贴底）。

2. **决策：`waitForEvent` 一次一读接力，而不是在 `Init()` 里起 goroutine `p.Send(agentEvent)`。**
   为什么：`p.Send` 是并发入口，无法形成背压，事件顺序依赖 channel 本身就够但 UI 无法"暂停消费"；用 Cmd 拉取让"不读 channel"成为一个**可用的控制手段**（人在回路正是靠它挂起 agent）。
   替代方案：`tea.Program.Send` + 缓冲 channel → 实现更短，但会丢掉"暂停消费"这一最优雅的阻塞机制，还得自己处理"程序退出后仍在 send"的 panic 风险。

3. **决策：人回审批的阻塞放在 agent 侧（`select { respond; ctx.Done() }`），TUI 只负责渲染与回传。**
   为什么：Agent 是"知道自己在等什么"的一方，阻塞点离业务最近，取消语义天然统一（同一个 `ctx`）；Agent 可以独立于 TUI 被消费（`cmd/smoke` 用同一个 `Run` 跑非交互流程，`task.Manager` 也直接消费事件流做后台任务）。
   替代方案：TUI 侧状态机挂起 + agent 侧轮询/回调 → 需要双向回调接口，取消路径要手写两套；或在 agent 侧轮询 `Respond`（忙等）→ 浪费 CPU 且取消延迟大。

4. **决策：`command.Registry.Register` 冲突直接 `panic`。**
   为什么：命令表是静态知识（内置 13 条 + skill 动态），冲突一定是编码错误；启动期 panic 让 CI/单测立刻暴露（`registry_test.go` 三个 panic 用例），避免"某个命令静默失效"。
   替代方案：返回 error 让调用方决定 → 调用方（`RegisterBuiltins`）大概率会忽略，变成运行期才发现的诡异行为；或"后者覆盖前者" → skill 与内置命令同名时会意外劫持。

5. **决策：`Parse` 严格拒绝带参数的命令名，只把"纯 `/name`"当作命令。**
   为什么：解析层保持极简且无歧义，参数解析的责任交给"知道自己要不要参数"的上层（只有 `/skill` 需要）；同时保证"未知输入必须给用户明确提示"而非静默当提示词发给模型。
   替代方案：Parse 直接返回 `(name, args)` → 更通用，但会引入 `strings.Fields` 的引号/转义语义问题，且当时的命令集里只有 `/skill` 需要参数；代价是现在多了一段 TUI 层的补丁式二次提取。

6. **决策：命令用 `Kind` 元数据 + `UI` 接口双重约束，而不是 TUI 里 `switch commandName`。**
   为什么：依赖倒置让 `command` 包零依赖 `tui`，handler 可单测（`NopUI`/`recordingUI`）；新增命令不需要改 dispatch 代码。
   替代方案：TUI 内 `switch` → 命令逻辑与 UI 逻辑纠缠，无法测试；代价是 `UI` 接口膨胀到 22 个方法（`NopUI` 也要跟着改，测试桩维护成本上升），且 Kind 与 handler 实际行为之间**没有编译期强制**（可以声明 KindLocal 却调 `InjectAndSend`）。

7. **决策：系统提示用"带 Priority 的模块列表 + 稳定装配"，而非拼接字符串。**
   为什么：模块可独立评审/测试/复用（子 Agent 走 `RunToCompletion` 时也能复用 `BuildSystemPrompt`）；Priority 让"插件式扩展"成为可能（测试 `TestAssembleSystem_Extensible` 就是这条承诺）。稳定性则由"跳过空模块 + 稳定排序 + 固定分隔符"三件事保证。
   替代方案：单个大 const 字符串 → 改动一行就要重新审查整段，且无法做"环境段与稳定段分离"；代价是多了一层间接，且 `OptionalModules` 的参数顺序与 Priority 顺序不一致，容易被误读（见前述）。

8. **决策：稳定段（Stable）与环境段（Environment）在协议层就分开，只有 Stable 打缓存断点。**
   为什么：Anthropic 的 cache 是**前缀缓存**，只要断点之前的内容逐字节不变就命中；环境段含日期与 git 状态（每轮/每天变），必须放在断点**之后**才能不污染缓存。把这条不变量推到 `llm.System` 结构上（而不是靠调用方自觉拼字符串），是最不容易被后续改坏的落点。
   替代方案：把环境和稳定段拼成一个字符串（早期做法）→ 每次 git 状态变化都让整段缓存失效，成本高一个数量级；代价是"唯一断点"没用满 4 个配额（tools、messages 未打点），长会话下仍有较大未优化空间。

9. **决策：reminder 走 messages 尾部（并入最后一条 user 消息），不进 system。**
   为什么：一是缓存（system 断点之后的内容尽量少动）；二是合法性（Anthropic 要求 user/assistant 严格交替，把 reminder 作为"新 content block"追加到现有 user 消息，比"新起一条 user 消息"更不容易触发连续同角色）；三是语言习惯上"当轮指令"更像用户消息的补充。
   替代方案：追加到 system 末尾 → 每轮 system 变化，缓存直接失效；或在 system 里维护一个 `<system-reminder>` 槽 → 同样破坏缓存稳定性。

10. **决策：每轮新建 `context.WithCancel(context.Background())`（per-turn context），而非全局 ctx。**
    为什么：一轮对话是天然的生命周期单元；取消本轮后 `m.events` 归零、`turnCancel` 归零，下一轮完全干净（不会因上一轮取消而连坐）；`finishTurn` 的清理边界与 ctx 边界重合，好推理。
    替代方案：单程序 ctx 派生子 ctx（`WithCancel(parent)`）→ 语义更统一，但当前实现里 `m.cancel` 根本没人赋值，"程序级取消"这条线本来就是空的；代价是 `UserPromptSubmit`/hook/`RunForceCompact` 都用 `context.Background()`，无法从 TUI 侧统一取消（例如压缩中按 Esc 无效）。

11. **决策：agent 事件 channel 无缓冲，把它当成背压阀门。**
    为什么：ReAct 循环与 UI 渲染的速度差极大（模型流式 vs 终端重绘），无缓冲天然实现"UI 处理完才继续"，也让"人在回路"的挂起零成本；`emit` 的 `select { ch <- e; case <-ctx.Done() }` 保证取消能穿透发送点。
    替代方案：带缓冲 channel（如 32）→ 吞吐更好，但会引入"事件在缓冲区里积压，用户看到的是过期状态"的问题，且人在回路的挂起会延后生效；代价是无缓冲下若消费者死亡（TUI 退出）agent 会阻塞在 `emit`（靠 ctx 取消兜住）。

12. **决策：`SessionRuntime` 作为跨 Run 的长生命周期状态容器，Agent 实例在 TUI 里常驻复用。**
    为什么：上下文管理的锚点（`UsageAnchor/AnchorMsgLen`）、内容替换状态、文件读取恢复状态、激活的 Skill、PendingReminders 都必须在多轮之间保持；把这些从 `Agent` 抽到 `runtime` 后，`Agent` 可以 `New` 一次反复 `Run`（`tui.New` 里就只 `agent.New` 一次）。
    替代方案：每轮重建 Agent → 每轮都要重灌所有状态，且 `runtime` 里的 `mu` 就白加了；代价是 `Agent`/`runtime` 的责任边界模糊（`runtime.HookEngine` 字段集两处赋值：`New()` 里与 `main` 里都设了 `HookEngine`），`ResetForNewSession` 需要手工枚举所有子状态（新增字段容易漏）。

13. **决策：Plan 模式用"工具集收窄 + 每轮 reminder"实现，而不是在权限层真正拒绝写操作。**
    为什么：`ModePlan` 的权限矩阵沿用 default（`modeFallback` 注释："矩阵同 default 作防御兜底"），真正的约束是 `registry.ReadOnlyDefinitions()`（模型**看不见**写工具，从源头减少违规）+ reminder（行为引导）。双保险、且不依赖模型"记忆模式"。
    替代方案：在 `Check` 里对 Plan 模式的 Write/Exec 直接 Deny → 更硬，但模型会反复尝试并收到一堆错误结果，浪费迭代（`unknownRun`/`maxIterations` 更容易触顶）；代价是"工具不可见"依赖 provider 正确传递工具集，若某个 provider 忽略 `Tools` 字段则 Plan 模式形同虚设（源码未体现防护）。

14. **决策：UI 侧工具结果一律截断 8 行（`truncateLines` / `toolResultSummary`），历史里保留全文。**
    为什么：终端渲染成本与可读性；agent 侧 `truncateLines(resultSummary, 8)` 只截"发到 UI 的那份"，`conv.AddToolResults(results)` 保留完整内容，模型仍能拿到全文。
    替代方案：UI 也显示全文 → 大文件读取会把屏幕刷爆；代价是 UI 与历史的"内容不等价"，用户按屏幕内容与模型讨论时可能对不上（压缩时另有 `SpillDir` 溢出机制兜底）。

15. **决策：Skill 采用"目录常驻 system + 正文动态注入 env"的两阶段激活。**
    为什么：Skill 数量增加时，把全部 SOP 都塞进 system 会让每轮成本爆炸；"按需激活"用一次 `LoadSkill` 工具调用换取后续每轮的正文注入，同时利用"system 缓存段"装目录（稳定、可缓存）。
    替代方案：全量注入 → 简单但不可扩展；或完全不注入正文、每轮靠模型自己读 SKILL.md → 多一次工具往返且容易漂移；代价是"激活"是会话级粘性状态（`/clear` 才清），用户不主动清理会一直占用上下文。

---

## 面试官可能追问（20 条）

**Q1：`waitForEvent` 为什么能保证事件不丢、不乱序？它在什么情况下会失灵？**
A：正常情况下失灵不了——每个事件处理完后 Update 恰好补发一个 `waitForEvent`，bubbletea 保证"产生该 Msg 的 Cmd 已经结束"，所以任一时刻最多一个读者（`stream.go:13`）。乱序/丢失的风险窗口在**取消路径**：Esc/Ctrl+C 分支在返回 `waitForEvent(m.events)` 时，上一个读取 Cmd 仍阻塞在 `<-ch`（Cmd 不可取消），于是两个 goroutine 抢同一个无缓冲 channel，可能把两个事件分给两个 Msg 且到达顺序不确定。源码未体现对这一情况的防护（没有 reader token / 没有把旧 Cmd 结果丢弃）。

**Q2：为什么 `View()` 里看不到历史对话？**
A：因为 `Run()` 里没有 `tea.WithAltScreen()`，程序跑在 scrollback 模式：历史由 `tea.Println` 追加到终端原生缓冲区，`View()` 只返回底部区域（`viewChat` 的输入框/补全/状态栏，或 `viewApproving` 的审批块）。这带来可选中可复制的好处，代价是无法做整体布局，且**弹窗内容（如审批菜单）切态后不留痕迹**（它只存在于 View，不进 scrollback）。

**Q3：审批期间 Agent 到底是"阻塞在哪一行"？**
A：阻塞在 `agent.requestApproval`（`agent.go:994`）的 `select { case o := <-respond; case <-ctx.Done() }`；而 TUI 侧 `handleAgentEvent` 在 `StateApproving` 分支 `return m, nil` **故意不发** `waitForEvent`，于是 agent 之前那次 `emit(Event{Approval: req})` 也早已返回——真正让整个 ReAct 循环停住的是"TUI 不再读 channel"这一行为本身（无缓冲 channel 提供了背压）。

**Q4：用户选"永久允许"之后，规则写到哪、会不会被泛化？**
A：写到 `permission.Engine.localPath` = `<root>/.mewcode/settings.local.yaml`（`PersistLocalAllow`），规则由 `ruleFor` 生成：`friendlyName(pattern)`，命令串先经 `escapeGlob` 把 `\ * ? [ ]` 转义成字面量，**不含任何通配**；写入前做幂等去重，写完同步更新内存 `e.local.allow`。所以"永久允许 `git status`"不会连带放行 `git status --hard`（精确匹配）。

**Q5：审批有超时吗？用户挂机不管会怎样？**
A：没有超时，源码未体现 timeout/默认拒绝/倒计时。用户不管 = Agent 永远卡在 `select`，spinner 会一直转（`spinner.TickMsg` 照常驱动），TUI 其他按键除 Esc/Ctrl+C 外都被 `stateApproving` 分支吞掉。唯一的出路是用户按键或取消 ctx。

**Q6：子 Agent 触发权限 Ask 会怎样？**
A：目前会**卡住**。`ApprovalUpgrader`（`agent/permission_upgrade.go`）与 `WithApprovalUpgrader`、`agent.go:787` 的升级分支都实现了，但 TUI 从未接线（`tui.New` 造 `AgentTool` 时只 `SetParentConvFn`）；子 Agent 的 Approval 事件落到 `AgentTool` 内部的 `events chan Event`，而 `task.aggregateEvent` 只统计 `Tool`/`Usage`，**丢弃 Approval**，无人回传 `Respond`。只能等 120s（`autoBackgroundDuration`）转后台 + 最终靠 ctx 取消解开。属明确的待补齐点。

**Q7：`FinishTurn` 为什么保留 `mode` 和 usage，却清空 `iter`/`curTools`？**
A：生命周期不同：`mode` 是用户档位（`Shift+Tab` 切换、跨轮保持，`finishTurn` 刻意不动）、`usageIn/usageOut` 是会话累计值（`/status` 要显示累计 token）；而 `iter`（当前轮迭代数，来自 `Event{Iter}`）、`curTools`（本轮工具展示队列）、`curReply`、`events`、`turnCancel` 都是**本轮临时态**，必须清干净，否则下一轮会串味（例如 `curTools` 残留会让 PhaseEnd 的 FIFO 弹错工具）。

**Q8：`m.cancel` 有什么用？**
A：**没有实际作用**——`tui.go`/`commands.go` 三处读取它（Ctrl+C 退出分支、`Quit()`），但全仓没有任何赋值点；`Quit()` 真正生效的是 `m.pendingCmd = tea.Quit`。它反映的是一段未完成的设计（本来想做"程序级 ctx，用于取消入参给 hook/MCP/ForceCompact 的 `context.Background()`"）。这是 Cancelled 语义只做到 per-turn 的死字段。

**Q9：`dispatchSlash` 里为什么又写了一遍命令名提取？**
A：因为 `command.Parse` 刻意只认"纯 `/name`"，`"/help xx"` 会返回 `("", true)`。TUI 层为了不让带参数的命令落到"未知命令"提示，在 `name == ""` 时手动 `TrimPrefix` + `IndexByte(' ')` 再切一次。这是"解析层极简、上层补丁"的折中，副作用是除 `/skill` 外所有命令的参数都被静默忽略。

**Q10：Idle 守护为什么只拦 KindUI/KindPrompt/KindSkillFork，不拦 KindLocal？**
A：因为 `KindLocal` 只读不写（`Println` + 只读查询），并发执行不会破坏状态（`/status`、`/help`、`/memory` 在流式中查询反而更有用）；而 `KindUI` 会改 Model 状态（`/clear` 要重建 writer/conversation/runtime，`/resume` 要换 Session），必须串行。（副作用：流式期间 `/exit` 也被拒，用户得先 Esc。）

**Q11：`MewCode` 的系统提示如何做到"两次调用逐字节相同"？**
A：`AssembleSystem` 用 `copy` + `sort.SliceStable` 按 `Priority` 排序（同优先级保持输入顺序）、跳过空 `Content`、固定用 `"\n\n"` 连接；`FixedModules()` 是常量切片；三个可选槽的输入（`instructionText`/`memoryText`/`skillsCatalog`）在 `main`/`Catalog` 里都是"进程内加载一次、字典序稳定"。单测 `TestBuildSystemPrompt_Deterministic` 直接断言字符串相等。

**Q12：为什么环境信息不放进 system 的稳定段？**
A：`Environment.Date`（每天变）和 `GitStatus`（`git status --porcelain`，随工作区变化）都**不满足逐字节稳定**。一旦放进断点之前，任何文件改动都会让整个 system 前缀缓存失效。所以协议层就把 `llm.System` 拆成 `Stable`（打 `CacheControl: NewCacheControlEphemeralParam()`）与 `Environment`（不打点），环境段永远排在断点之后。

**Q13：Plan Mode 的"只读"是怎么保证的？**
A：两层：① 工具集侧 `registry.ReadOnlyDefinitions()`（模型看不到 write_file/edit_file/bash，`agent.go:208`）；② 提示词侧 `buildReminder` 注入 plan reminder——首轮与"每 4 轮"给 `planReminderFull`（明确列出 read_file/glob/grep、禁止写/编辑/shell、产出计划后停止等 `/do`），其余轮给 `planReminderConcise`。`ModePlan` 在 `modeFallback` 里沿用 default 矩阵，是防御兜底而非主约束。

**Q14：`planReminderInterval = 4` 的 `full` 判定为什么是 `(iter-1)%4 == 0`？**
A：这样 iter = 1,5,9,13… 命中完整版：**首轮**（iter==1）与**每 4 轮**重述一次完整约束。若写成 `iter%4==0` 则首轮反而是精简版（1%4≠0），与"首轮必须建立完整约束"的设计意图冲突。测试 `TestReAct_PlanReminderByIter` 覆盖这一行为。

**Q15：Skill 正文注入会不会破坏 Prompt Cache？**
A：不会破坏**缓存前缀**：目录（`RenderSkillsCatalog`）进 system 的 priority 90 槽（在缓存断点内，内容随目录变化而非随对话变化），正文（`RenderActiveSkillsBlock`）拼到 `Environment` 段（断点之后、不打点）。激活/切换 Skill 时只有环境段变化，稳定段与原断点仍然命中。

**Q16：命令补全 `Tab` 和 `Enter` 的行为差异是什么？**
A：`Tab`：把选中命令 `SetValue("/"+name)` 后立即 `dispatchSlash` **执行**（`complete.go:168`）。`Enter`：把选中项写入 textarea、关菜单、返回 `consumed=false`，交由 `handleIdleKey` 的 Enter 分支走"提交"链路（效果同样是指令执行，但多了一次 textarea 往返）；当 `items` 为空时 Enter 被吞掉（`consumed=true`）。另外 Esc 关菜单的分支是**死代码**——`Update` 顶部全局 Esc 处理在 `stateIdle` 就 `return m, nil` 了。

**Q17：`agentEvent` 为什么是 `type agentEvent agent.Event` 而不是 `type agentEvent = agent.Event`？**
A：Go 里 `type A B` 是**新定义类型**（不同 identity），`type A = B` 是别名。用新类型后 `Update` 的 `case agentEvent:` 才能精确匹配到桥接消息，同时保留字段访问（底层类型相同、可转换），也避免 agent 包的事件类型被 tea 框架"误认"为其他消息。

**Q18：`resume` 恢复会话时做了哪些额外处理？**
A：`doResumeSession`（`resume.go:45`）四步：① `session.LoadSession(info.Dir)` 读消息；② `estimateTokens` 粗估（`chars * 0.25`）若超 `ContextWindow - 8000` 就先 `RunForceCompact` 压缩；③ 若 `time.Since(info.ModifiedAt) > 6h` 追加一条 user 消息 `"[系统提示] 本会话已暂停 X小时。部分上下文可能已过时…"`；④ `compact.OpenSessionContext` 打开原会话目录 + `session.OpenWriter` 追加模式重开 JSONL（**必须在构造 Conversation 之前**，否则 `OnAppend` 会绑到旧 writer），最后在 scrollback 里 `renderHistoryMessages` 回放历史（tool 消息截断 200 字符）。

**Q19：TUI 的密钥安全怎么保证？**
A：三层：① `environment.go` 注释明确"不读环境变量（N5）"，环境段只采 6 个白名单字段；② 系统提示固定模块"系统约束"明确"API 密钥等敏感信息绝不回显到对话区或任何输出"；③ 权限引擎的黑名单（`blacklist`，不可配）在 `Check` 第①层拦截危险命令，`bypassPermissions` 也拦。此外 API Key 只存在于 `config.ProviderConfig` 与 SDK client，未进入任何 prompt/日志路径。

**Q20：这套 TUI/命令/提示词里最该先补的三个洞是什么？**
A：① **子 Agent 审批链路**：`WithApprovalUpgrader` 未接线 + `task.aggregateEvent` 忽略 Approval → 子 Agent 遇 Ask 必卡；② **多 provider 路径未完成**：`selectProvider` 后不构造 Agent/skill/task 工具，`/`命令可用但 `submitMessage` 会碰 nil `m.ag`；③ **缓存断点只用 1 个**（tools/messages 未打点）且 `CacheRead/CacheWrite` 已采集却不展示——长会话的成本优化空间与可观测量都还没兑现。（其他：`EventSessionResume` 定义但从未派发、`AgentTool.SetParentConvFn(m.conv.Messages)` 绑定的是 `SetConversation` 之前的旧 conversation 导致 Fork 拿到空父历史、`m.cancel` 死字段。）

---

## 企业级对应方案

对比对象：Claude Code（提示词分层 / CLAUDE.md / 斜杠命令 / 权限 UX）、Cursor（rules）、Anthropic Prompt Caching（cache_control 断点）、TUI 框架选型（Bubble Tea vs Ink vs Textual）。

### 1) 提示词分层与"项目指令"的发现机制

- **Claude Code**：分层提示（系统提示 + 工具描述 + 项目/用户/企业记忆）。`CLAUDE.md` 支持多级发现（`./CLAUDE.md`、`./CLAUDE.local.md`、父目录向上递归、`~/.claude/CLAUDE.md`、企业级策略），并支持 `@path` 导入；CLAUDE.md 内容是"能力 + 约定 + 命令"的长文档，且带记忆自动写入。
- **MewCode**：`instructions.NewLoader(root).Load()` 三层 `MEWCODE.md`（`<root>/MEWCODE.md` → `<root>/.mewcode/MEWCODE.md` → `~/.mewcode/MEWCODE.md`），`@include` 递归上限 `maxIncludeDepth = 5`，带"路径逃逸检测"边界；最终作为**单一文本**填入 priority 80 槽（无结构化段落）。
- **差异与补齐点**：① 缺"向上递归发现"（只查 projectRoot，不含父目录）；② 缺企业级/组织级策略层与本地私有层（`MEWCODE.local.md`）；③ 缺"本次改动涉及目录 → 就近注入对应规则"的能力；④ `@include` 展开后整段是裸文本，没有"来源标注"（Claude Code 会标注文件路径来源），模型无法区分约定来自哪里；⑤ 缺记忆自动写回 MEWCODE.md（有 `memory.Manager` 但落点在 `.mewcode/memory/*.md`，与指令文件不互通）。

### 2) Cursor rules 的"条件化注入"对比

- **Cursor**：`.cursor/rules/*.mdc`，frontmatter 支持 `description` / `globs` / `alwaysApply`，可"按文件 glob 命中才注入"或"让模型自行决定是否拉取（agent-requested）"，并且支持 Rule 与 tool 联动。
- **MewCode**：只有"全局一段静态文本"（priority 80）+ 会话级 Skill 两阶段（`RenderSkillsCatalog` 目录常驻 / `RenderActiveSkillsBlock` 正文按需）。**规则本身没有条件化触发**（无 glob、无路径、无关键词触发），也没有 agent-requested rules。
- **补齐点**：把 instructions 从"字符串"升级为"条目集合（含 glob/description/tier）"，在 `AssembleSystem` 前按"本轮工具目标路径"过滤；这正好能复用现有的 `ToolDefinition`/`argPreview` 里提取 `path` 的能力（`argPreview` 的 `preferKeys` 已经优先取 `path`）。

### 3) Prompt Cache 工程（Anthropic `cache_control` 断点设计）

- **Anthropic 规范**：最多 4 个 `cache_control: {type:"ephemeral"}` 断点，缓存查找顺序是 **tools → system → messages**；断点落在"长且稳定"的内容末尾才划算（最小可缓存长度如 1024/2048 token 视模型而定）。
- **MewCode**：`toAnthropicSystem` 只给 `System.Stable` 打 1 个断点（注释强调必须用 `NewCacheControlEphemeralParam()` 构造器，否则 `omitzero` 会吞掉），`Environment` 段不打点；`toAnthropicTools` 与 `toAnthropicMessages` **完全没有** `cache_control`。用量侧已经采集 `CacheWrite/CacheRead`（`Usage` + `Event.Usage`），但 TUI 只用 Input/Output。
- **差异与补齐点**：① 给 tools 数组末尾打第 2 个断点（工具定义通常 2k+ token，且 mode 不变时稳定）；② 给 messages 的"稳定前缀末条"打第 3 个断点，并在压缩（`compact`）或 `OnReplace` 后**移动断点**（Claude Code 的实践就是断点跟随 compact 迁移）；③ CAUTION：切 Plan 模式会换工具集 → 缓存必然失效，可考虑"工具集不变、靠系统提示声明只读"作为缓存友好方案；④ TUI 状态栏加 `cache R/W` 展示，让缓存收益可见（否则这个工程无从验证）。

### 4) TUI 框架选型

- **Bubble Tea（本项目）**：Elm 架构（`Init/Update/View`）+ `Cmd` 描述副作用；`bubbles` 提供 textarea/list/spinner/viewport；`lipgloss` 做样式、`glamour` 做 Markdown 渲染；Go 单二进制、无运行时依赖、`tea.Println` 天然利用终端 scrollback；并发模型统一（Cmd + Msg），跨平台（含 Windows）。
- **Ink（Node/React）**：`<Box>/<Text>` 组件化 + React 生态（`useInput`、状态管理、npm 复用），适合已有 Node 技术栈（Claude Code 即 Node/Ink 阵营）；代价是需要 Node 运行时、npm 依赖体积、`yoga` 布局在复杂树上性能与调试成本、以及"重绘阴影"（Ink 的静态输出 API 相对年轻）。
- **Textual（Python）**：CSS 式样式、DOM 查询、`reactive` 属性、成熟 widget（DataTable、Tree、TabbedContent），适合复杂面板式应用；代价是 Python 分发（打包体积/启动慢/依赖环境敏感），且 Elm-式单向数据流不如 Bubble Tea 显式。
- **MewCode 差异与补齐点**：当前没有 AltScreen/鼠标/固定布局/主题系统/焦点管理（焦点隐式由 `state` 决定，且 `stateApproving` 下 textarea 完全不可用）；若要企业级观感，需要 ① 引入 `viewport`/`AltScreen` + `lipgloss` 布局（header/sidebar/状态栏三段）；② 主题可配（当前颜色硬编码 hex：`#44CCCC`、`#FF4444`…）；③ 键位映射集中化（现在 ctrl+c/Esc/shift+tab 硬编码在 `Update` 顶部，且与补全菜单存在拦截顺序冲突）；④ 无障碍/窄终端降级（`width < 20` 的兜底散落在多处）。

### 5) 命令面板与补全设计

- **Claude Code**：斜杠命令来自 `~/.claude/commands/*.md` 与项目 `.claude/commands/`（frontmatter：`description` / `allowed-tools` / `argument-hint` / `model`），支持 `$ARGUMENTS`、`!bash` 内联、`@file` 引用；MCP server 命令自动变成 `/mcp__server__tool`；补全菜单支持模糊匹配、分类、参数提示。
- **Cursor**：命令面板（`Cmd+K`/`Cmd+L`）+ `@` 上下文引用（文件/符号/文档/web）统一在一套"引用语法"里。
- **MewCode**：`Registry`（`byName` + `visible` 有序）+ `PrefixMatch` 前缀匹配 + 8 行滚动菜单；Kind 四分类；`Aliases` 字段已实现但 13 条内置命令无一使用；`/skill list|info|reload` 是唯一的手写子命令。
- **差异与补齐点**：① 命令来源只有"编译进二进制"和"skill 目录"，缺**markdown 命令发现**（企业内最常用的"团队自定义命令"落地方式），可复用 `skills.LoadCatalog` 的双层扫描模式（`~/.mewcode/commands/` + `<root>/.mewcode/commands/`）；② 缺参数补全与 `$ARGUMENTS` 模板占位；③ 缺 `@file` 上下文引用（补全菜单目前只认 `/` 开头，`@` 无任何处理）；④ `PrefixMatch` 只匹配 Name，应扩展为 name + alias + description 的加权匹配（企业用户记不住命令名，靠描述搜索才是刚需）；⑤ 缺命令面板的"最近使用/收藏/危险操作二次确认"分级。

### 6) 人在回路的权限 UX 与审计

- **Claude Code**：权限模式（default/acceptEdits/plan/bypassPermissions）+ 每次询问带"always allow（可编辑具体规则）/ allow once / deny"，规则写入 `settings.local.json`；另有 hooks 做策略前置拦截；审批与工具调用全程进 transcript，可 `--resume` 回放。
- **MewCode**：五层防御（黑名单 → 沙箱 → 三级规则 → 模式兜底 → 人在回路）已经成型，`PersistLocalAllow` 的精确规则 + glob 转义比"always allow this tool"更保守（更安全），Esc/Ctrl+C 的兜底 denial（`select` + `default` 非阻塞回传）也很讲究。
- **补齐点**：① 审批**无超时**、**不入历史**、**无 hook 事件**（`EventNotification` 注释与实现不符），导致"谁批准了什么"不可审计；② 三级规则的写入目标只有 local 层（项目级/用户级要手改 YAML）；③ 缺"批量授权同类操作"（如"本次会话允许所有只读 bash"）与会话级临时规则；④ 子 Agent 审批升级未接线（见 Q6），企业多 Agent 场景下这是硬伤。

### 7) 状态机与可观测性（顺带对比）

- 本项目状态机只有 5 态且**没有 tool-running/compacting 态**：压缩进度靠 `Event{Compact}`（`CompactPhaseBeforeAuto/AfterAuto/BeforeEmergency/AfterEmergency`）走 Notice 渲染，压缩中按 Esc 无效（`RunForceCompact` 用 `context.Background()`）。
- 企业实现通常把"可观测性"做成一等公民：token/缓存读写（本项目已采集未展示）、每轮耗时（`turnStart` 已采集，仅用于 `Imagining… (Ns)`）、工具成功率、压缩次数、审批次数——目前只有 `/status` 的 6 项静态快照。
- **补齐点**：把 `Iter/Usage/CacheRead/CacheWrite/Compact` 事件汇入一个 `session stats` 面板（`cmdRegistry` 已有扩展位，加一条 `/stats` 即可），并把 `EventSessionResume` 真正派发出去（hook 侧的 11 个事件定义目前只用了 10 个），这样才能接入企业侧的埋点/审计管道。

---

*笔记基于上述源码在阅读时的真实状态；凡"源码未体现"处均已注明，未作推测性补全。*
