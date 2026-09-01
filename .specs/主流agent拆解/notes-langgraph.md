# LangGraph 事实核实笔记（T3 产出）

> 核实方式：web_search + PyPI/GitHub API + 本地安装 langgraph 1.2.11 实测交叉验证。
> 写作规则：API 以本笔记为准；stars 写「约 4 万+」；不确定项见文末。

## 1. 版本与规模

- 主版本线：**v1.x**（2025-10 官宣 1.0 里程碑）
- 最新 PyPI 版本：**langgraph 1.2.11**（配套 langgraph-checkpoint 4.2.0、checkpoint-sqlite 3.1.1、checkpoint-postgres 3.1.2）
- GitHub stars：**约 40,800**（实时 API 值，随时间增长）
- 来源：https://pypi.org/project/langgraph/ 、https://github.com/langchain-ai/langgraph 、https://www.langchain.com/blog/langchain-langgraph-1dot0

## 2. StateGraph 核心 API（1.2.11 实测）

```python
from typing import TypedDict
from langgraph.graph import StateGraph, START, END

class State(TypedDict):
    count: int

builder = StateGraph(State)                 # StateGraph(state_schema)
builder.add_node("a", node_a)
builder.add_edge(START, "a")                # 等价于 set_entry_point("a")，官方推荐写法
builder.add_edge("a", "b")
builder.add_conditional_edges("b", router, {"x": "c", "y": END})
graph = builder.compile()                   # 可传 checkpointer
graph.invoke({"count": 0})
```

- START/END 从 `langgraph.graph` 导入，值为 `'__start__'`/`'__end__'`
- `set_entry_point`/`set_finish_point` 仍存在（等价 START/END 边），文档推荐直接用边
- `StateGraph.__init__(state_schema, context_schema, input_schema, output_schema, **kwargs)`
- 来源：https://reference.langchain.com/python/langgraph/ 、https://github.com/langchain-ai/docs/blob/main/src/oss/langgraph/use-graph-api.mdx

## 3. Pregel 执行模型

- 受 Google Pregel 论文启发，按 **BSP（Bulk Synchronous Parallel）** 模型执行，划分为离散 **super-step**
- 每个 super-step 内所有被激活节点**并行**执行；super-step 间是全局同步屏障
- 节点间**消息传递**：节点在 super-step N 写 channel，在 N+1 被触发消费（inbox 模型）
- checkpointer 在 **super-step 边界**落盘 → 断点续跑/容错的基础
- 来源：https://deepwiki.com/langchain-ai/langgraph/3.3-pregel-execution-engine 、https://mintlify.wiki/langchain-ai/langgraph/guides/persistence

## 4. State 与 Reducer（1.2.11 实测）

- 默认：节点返回字段**覆盖** state 同名字段；`Annotated[field, reducer]` 改为**合并**
- `add_messages`（`from langgraph.graph.message import add_messages`）docstring 原文："Merges two lists of messages, updating existing messages by ID... 'append-only', unless the new message has the same ID"——默认追加，同 id 原地更新，支持 RemoveMessage
- 并行写同一 channel：由各字段 reducer 合并；无 reducer 则报 InvalidUpdateError
- 实测：两并行节点 `{"vals": ["x"]}`/`{"vals": ["y"]}`（`Annotated[list, operator.add]`）→ `['x','y']`

```python
from typing import Annotated, TypedDict
from langgraph.graph.message import add_messages

class State(TypedDict):
    messages: Annotated[list, add_messages]
```

- 来源：https://mintlify.wiki/langchain-ai/langgraph/guides/state-management

## 5. Checkpointer 生态（1.2.11 实测）

- 当前类名：**`InMemorySaver`**（`from langgraph.checkpoint.memory import InMemorySaver`）；旧名 MemorySaver 仍是别名（向后兼容），新代码一律用 InMemorySaver
- `SqliteSaver`：独立包 **langgraph-checkpoint-sqlite**，`from langgraph.checkpoint.sqlite import SqliteSaver`
- `PostgresSaver`：独立包 **langgraph-checkpoint-postgres**，`from langgraph.checkpoint.postgres import PostgresSaver`
- thread_id：invoke/stream 的 config 传 `{"configurable": {"thread_id": "..."}}`

```python
from langgraph.checkpoint.memory import InMemorySaver
graph = builder.compile(checkpointer=InMemorySaver())
config = {"configurable": {"thread_id": "t-1"}}
graph.invoke(input, config)
```

- 来源：https://reference.langchain.com/python/langgraph/checkpoints 、https://pypi.org/project/langgraph-checkpoint-sqlite/ 、https://pypi.org/project/langgraph-checkpoint-postgres/

## 6. interrupt 人机协作（1.2.11 实测）

- 当前推荐：**函数式 `interrupt()`**（`from langgraph.types import interrupt`），节点内调用即暂停（首次调用抛 GraphInterrupt）；客户端用 **`Command(resume=...)`** 恢复；恢复后**节点从头重跑**，`interrupt()` 返回 resume 值
- 旧 `interrupt_before`/`interrupt_after` 是「静态断点」机制，定位调试/环外审批，与动态 interrupt() 并存

```python
from langgraph.types import interrupt, Command

def approval_node(state):
    answer = interrupt({"question": "approve?"})
    return {"messages": [{"role": "user", "content": str(answer)}]}

graph.invoke(input, {"configurable": {"thread_id": "t-1"}})   # 返回含 __interrupt__
graph.invoke(Command(resume="yes"), config)                    # 恢复
```

- 来源：https://reference.langchain.com/python/langgraph/types/interrupt 、https://docs.langchain.com/langsmith/add-human-in-the-loop

## 7. Command 对象（1.2.11 实测）

- 节点返回 `Command`：**一条命令同时完成状态更新与控制流路由**，免去条件边
- 字段：`graph, update, resume, goto, PARENT`
  - `update`：合并进 state（走 reducer）
  - `goto`：下一跳转目标（节点名或 Send 对象）
  - `graph=Command.PARENT`：跳转父图（子图/多智能体 handoff）
  - `resume`：恢复 interrupt

```python
from langgraph.types import Command
from typing import Literal

def router_node(state) -> Command[Literal["worker", "__end__"]]:
    if state["done"]:
        return Command(goto="__end__")
    return Command(goto="worker", update={"count": state["count"] + 1})
```

- 来源：https://www.langchain.com/blog/command-a-new-tool-for-multi-agent-architectures-in-langgraph

## 不确定项

1. stars 精确值随时变 → 写「约 4 万+」
2. MemorySaver 别名保留期限未明确 → 一律用 InMemorySaver
3. set_entry_point 是否正式 deprecated 未确认 → 只说「推荐直接用 START/END 边」
4. Pregel 官方概念页旧域名失效，BSP/super-step 描述来自 DeepWiki 与多份一致二手资料
5. 部分搜索快照停留在 1.0.x/1.1.0 → 以 PyPI 实测 1.2.11 为准
