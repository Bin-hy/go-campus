# Eino 事实核实笔记（T2 产出）

> 核实方式：GitHub API + raw.githubusercontent.com main/tag 源码 + pkg.go.dev + cloudwego.io 官方文档。核实时间 2026-09-01 前后。
> 写作规则：API 以本笔记为准；stars 写「约 1.3 万」；不确定项见文末。

## 1. 版本与规模

- 最新正式 release：**v0.9.18**（2026-08-31）；预发布 v0.10.0-alpha.x（agentic-runtime 重构）
- stars：**约 12.9k**；定位自述："The ultimate LLM/AI application development framework in Go."
- v0.9.x 引入 `model.AgenticModel`、`compose.AgenticToolsNode` 等 agentic 原语
- 来源：https://api.github.com/repos/cloudwego/eino 、https://github.com/cloudwego/eino/releases

## 2. compose 包核心 API（main 分支源码逐行核实）

```go
func NewChain[I, O any](opts ...NewGraphOption) *Chain[I, O]
func NewGraph[I, O any](opts ...NewGraphOption) *Graph[I, O]
func WithGenLocalState[S any](gls GenLocalState[S]) NewGraphOption

const START = "start"
const END   = "end"

// 加节点（方法名精确）：
AddChatModelNode(key string, node model.BaseChatModel, opts ...GraphAddNodeOpt) error
AddLambdaNode(key string, node *Lambda, opts ...GraphAddNodeOpt) error   // 不是 AddLambda
AddToolsNode(key string, node *ToolsNode, opts ...GraphAddNodeOpt) error // 复数 Tools
// 同类还有 AddChatTemplateNode/AddRetrieverNode/AddEmbeddingNode/AddGraphNode/AddPassthroughNode 等

// ToolsNode：
type ToolsNodeConfig struct {
    Tools []tool.BaseTool
    UnknownToolsHandler func(ctx context.Context, name, input string) (string, error) // 幻觉工具处理
    ExecuteSequentially bool  // 默认并行执行多个 tool call
}
func NewToolNode(ctx context.Context, conf *ToolsNodeConfig) (*ToolsNode, error)

// Lambda 四范式构造：
InvokableLambda / StreamableLambda / CollectableLambda / TransformableLambda / AnyLambda

// 边与分支：
func (g *Graph[I, O]) AddEdge(startNode, endNode string) error  // 上游出参类型须等于下游入参类型（编译期检查）
func NewGraphBranch[T any](condition GraphBranchCondition[T], endNodes map[string]bool) *GraphBranch
func (g *graph) AddBranch(startNode string, branch *GraphBranch) error
// GraphBranchCondition[T] func(ctx, in T) (endNode string, err error)

// 编译：
func (g *Graph[I, O]) Compile(ctx, opts...) (Runnable[I, O], error)
type Runnable[I, O any] interface { Invoke / Stream / Collect / Transform }
```

- **Chain 用 `Append*` 系列**（AppendChatModel/AppendLambda/AppendToolsNode/AppendBranch/AppendParallel），链式调用
- 来源：https://raw.githubusercontent.com/cloudwego/eino/main/compose/graph.go 等

## 3. 流式处理：四范式 + 流化/合包自动转换（官方术语）

| 范式 | 模式 | 交互名 | Lambda 构造 |
|---|---|---|---|
| Invoke | 非流入/非流出 | Ping-Pong | InvokableLambda |
| Stream | 非流入/流出 | Server-Streaming | StreamableLambda |
| Collect | 流入/非流出 | Client-Streaming | CollectableLambda |
| Transform | 流入/流出 | Bidirectional-Streaming | TransformableLambda |

- **流化（Streaming）**：T → 单 chunk 的 Stream[T]，俗称「假流」（只为满足签名，无首包延迟优势）
- **合包（Concat）**：Stream[T] → 完整 T
- 框架自动做这两个转换（官方原文："框架会自动将 StreamReader[T] concat 成 T" / "自动将 T 装箱成单帧流"）
- 自动 concat 需类型可合并，否则 `compose.RegisterStreamChunkConcatFunc[T](fn func([]T) (T, error))`；schema.Message 已预注册
- 流扇出靠 StreamReader copy 帧复制；callback 收到的流是副本，**必须 Close**，否则管道泄漏
- 来源：https://www.cloudwego.io/zh/docs/eino/core_modules/chain_and_graph_orchestration/stream_programming_essentials/

## 4. Callbacks 机制

五个注入点（callbacks.Handler 接口）：
- OnStart（开始前，非流式输入）
- OnEnd（成功后；错误不触发）
- OnError（返回非 nil error 时）
- OnStartWithStreamInput（Collect/Transform 流式输入）
- OnEndWithStreamOutput（Stream/Transform 流式输出）

语义要点：
- 同一 handler 各时机经返回 ctx 传状态；不同 handler 间无顺序保证
- 流式 handler 收到副本，**必须 Close**；不得修改 Input/Output 值（数据竞争）
- 可选 TimingChecker 跳过未关注时机，避免流复制开销
- 构造：`callbacks.NewHandlerBuilder().OnStartFn(...)...Build()`
- 注入：全局 `callbacks.AppendGlobalHandlers(...)`（推荐；InitCallbackHandlers 已 Deprecated）；请求级 `compose.WithCallbacks(...)`；编译级 `WithGraphCompileCallbacks`
- RunInfo 含 Name/Type/Component
- 来源：https://raw.githubusercontent.com/cloudwego/eino/main/callbacks/interface.go

## 5. 官方 ReAct Agent（flow/agent/react）

```go
func NewAgent(ctx context.Context, config *AgentConfig) (*Agent, error)
type AgentConfig struct {
    ToolCallingModel model.ToolCallingChatModel  // 推荐
    Model            model.ChatModel             // Deprecated
    ToolsConfig      compose.ToolsNodeConfig
    MessageModifier  MessageModifier   // 调模型前改写输入
    MessageRewriter  MessageModifier   // 改写 state 累积消息（压缩历史）
    MaxStep          int               // 默认 12
    ToolReturnDirectly map[string]struct{}
    StreamToolCallChecker func(...)    // 流式判断是否含 tool call，默认只查首帧
    GraphName / ModelNodeName / ToolsNodeName  // 默认 ReActAgent/ChatModel/Tools
}
```

- 本体 = 编译后的 `Runnable[[]*schema.Message, *schema.Message]`，暴露 Generate（Invoke）与 Stream
- 结构：ChatModel 节点 + ToolsNode + 「调工具还是结束」branch 构成的**循环图**
- 来源：https://raw.githubusercontent.com/cloudwego/eino/main/flow/agent/react/react.go

## 6. eino-ext 生态

- 顶层目录：`components/ callbacks/ devops/ libs/ adk/ acp/ skills/`
- 组件实现：ChatModel（OpenAI/Claude/Gemini/Ark/Ollama/DeepSeek/Qwen…）、Tool（GoogleSearch/DuckDuckGo）、Retriever、Embedding、Indexer、Document Loader/Transformer、Lambda
- Callback handlers：langfuse、apmplus、cozeloop、langsmith
- DevOps：Eino IDE 插件（可视化调试、UI 图编辑）
- 各组件为**独立 go module**，按需 import
- 来源：https://github.com/cloudwego/eino-ext/blob/main/README.md

## 7. State 机制（重要纠正：无 NewStateGraph！）

- **`NewStateGraph` 当前不存在**（v0.9.18 pkg.go.dev 无此符号；v0.3.7 起即无导出符号，仅历史过期注释）
- 当前形态（compose/state.go 核实）：

```go
// 1) 建图注册局部 state 生成函数
type GenLocalState[S any] func(ctx context.Context) (state S)
graph := compose.NewGraph[I, O](compose.WithGenLocalState(func(ctx) *MyState { return &MyState{} }))

// 2) 节点级 pre/post handler 读写 state
func WithStatePreHandler[I, S any](pre StatePreHandler[I, S]) GraphAddNodeOpt
func WithStatePostHandler[O, S any](post StatePostHandler[O, S]) GraphAddNodeOpt
// 流式变体：WithStreamStatePreHandler / WithStreamStatePostHandler

// 3) Lambda 内部推荐：并发安全的 ProcessState
func ProcessState[S any](ctx context.Context, handler func(context.Context, S) error) error
```

- 并发语义：state 由 `internalState{state, mu sync.Mutex, parent}` 承载，所有读写先加锁；嵌套图按词法作用域沿 parent 链查找，内层遮蔽外层
- **无 reducer/字段级合并**：多节点在锁保护下顺序读写同一实例，合并逻辑用户自实现——与 LangGraph 的 reducer 是不同哲学
- 节点触发模式：`AnyPredecessor`（默认）/ `AllPredecessor`；执行引擎是 pregel 风格 super-step
- 来源：https://raw.githubusercontent.com/cloudwego/eino/main/compose/state.go

## 不确定项

1. NewStateGraph 是否曾在 v0.3.7 之前真实存在 → 稳妥表述「当前版本不存在」
2. v0.10.0-alpha 的 breaking changes 未逐项核实 → 文档以 v0.9.18/main 为准
3. Compile 的编译期选项未枚举全
4. eino-ext 的 adk/acp/skills 目录内容未展开
5. flow/agent/multiagent 形态未核实
