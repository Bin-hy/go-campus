# 主流 Agent 拆解模块 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 迁移 | `docs/phase3/pi-harness/*.md` → `docs/主流agent拆解/pi/*.md`（5 篇） | pi 拆解整体迁入（git mv） |
| 修改 | `docs/主流agent拆解/pi/index.md` | 4 处篇目导航链接改为新路径 |
| 修改 | `docs/主流agent拆解/pi/核心机制.md` 等 4 篇 | 自引用链接改为新路径（如有） |
| 修改 | `docs/主流agent拆解/pi/Go落地与面试.md` | 文末新增「延伸阅读 → Eino 拆解」 |
| 新建 | `docs/主流agent拆解/index.md` | 模块首页：定位 + 三框架对照表 + 导航 + 阅读顺序 |
| 新建 | `docs/主流agent拆解/eino/index.md` | Eino ① 全景图 |
| 新建 | `docs/主流agent拆解/eino/核心机制.md` | Eino ② 四大机制 |
| 新建 | `docs/主流agent拆解/eino/对照与面试.md` | Eino ③ 对照 + 面试速答 + 速记卡 |
| 新建 | `docs/主流agent拆解/langgraph/index.md` | LangGraph ① 全景图 |
| 新建 | `docs/主流agent拆解/langgraph/核心机制.md` | LangGraph ② 四大机制 |
| 新建 | `docs/主流agent拆解/langgraph/对照与面试.md` | LangGraph ③ 对照 + 面试速答 + 速记卡 |
| 修改 | `docs/.vitepress/config.mts` | nav + sidebar 更新 |
| 修改 | `docs/phase3/index.md` | 移除 pi 条目，加新模块指引 |
| 修改 | `docs/路线专题/04-简历项目改造与面试实战.md` | 3 处 pi-harness 文本提及更新措辞 |
| 删除 | `docs/phase3/pi-harness/` | 迁移后空目录清理（git mv 自动处理） |

## T1: pi 拆解迁移

**文件：** `docs/phase3/pi-harness/` → `docs/主流agent拆解/pi/`
**依赖：** 无
**步骤：**
1. `mkdir -p docs/主流agent拆解`
2. `git mv docs/phase3/pi-harness docs/主流agent拆解/pi`
3. 在 5 篇迁移文档中全局替换链接：`/phase3/pi-harness` → `/主流agent拆解/pi`
4. 确认对 `/phase3/agent-harness/` 等其他路径的引用保持不变

**验证：** `grep -rn "phase3/pi-harness" docs/主流agent拆解/` 输出为空；`ls docs/主流agent拆解/pi/` 有 5 个 md 文件；`git status` 显示 rename 而非 delete+add

## T2: Eino 事实核实

**文件：** 无（研究任务，产出笔记供 T4-T6 使用）
**依赖：** 无（可与 T1 并行）
**步骤：**
1. web_search 核实 cloudwego/eino 当前主版本号与仓库结构
2. 核实 compose 包核心 API：`NewChain`/`NewStateGraph`/`AddChatModelNode`/`AddToolNode`/`AddEdge`/`AddBranch`/`Compile` 的签名形态
3. 核实流式处理机制的官方表述（流式拼接/自动转换的术语与四种场景）
4. 核实 Callbacks 注入点（OnStart/OnEnd/OnError/OnStartWithStreamInput/OnEndWithStreamOutput）
5. 核实 eino-ext 生态与 ReAct Agent 的提供方式（`react.NewAgent` 所在包）
6. 把核实结果（版本号、API 签名、官方术语）记录到 `.specs/主流agent拆解/notes-eino.md`

**验证：** notes-eino.md 中每条 API 都有来源链接；无「凭记忆」未核实的签名

## T3: LangGraph 事实核实

**文件：** 无（研究任务，产出笔记供 T7-T9 使用）
**依赖：** 无（可与 T1/T2 并行）
**步骤：**
1. web_search 核实 langchain-ai/langgraph 当前主版本号
2. 核实 StateGraph API：`add_node`/`add_edge`/`add_conditional_edges`/`set_entry_point`/`compile`、START/END 常量
3. 核实 Pregel 执行模型的官方描述（super-step、消息传递）
4. 核实 Reducer 机制（Annotated + add_messages）与并行写合并语义
5. 核实 Checkpointer 生态（InMemorySaver/SqliteSaver/PostgresSaver 当前命名）与 thread_id 用法
6. 核实 interrupt/resume API 现状（`interrupt()` 函数 + `Command(resume=...)`）
7. 把核实结果记录到 `.specs/主流agent拆解/notes-langgraph.md`

**验证：** notes-langgraph.md 中每条 API 都有来源链接

## T4: Eino ① 全景图

**文件：** `docs/主流agent拆解/eino/index.md`
**依赖：** T2
**步骤：**
1. 按模板写开头导读（读完能回答：Eino 解决什么问题？核心概念有哪几个？和手写 Go 编排差在哪？）
2. 「为什么需要 Eino」节：Go 生态缺统一 LLM 编排抽象的问题描述（问题先行）
3. 核心概念一张图（mermaid）：Component → Chain/Graph → Callbacks 三层
4. 逐个概念一段话 + 一句话记住：Component（ChatModel/Tool/Retriever/Embedding）、Chain、Graph、Callbacks
5. 15 行内最小可懂示例（Chain 串 ChatModel，基于 T2 核实的 API）
6. 篇目导航表（指向核心机制、对照与面试）+ 速记卡

**验证：** 篇幅 ≤300 行；含 ≥1 个 mermaid；代码签名与 notes-eino.md 一致；开头有导读、篇末有速记卡

## T5: Eino ② 核心机制

**文件：** `docs/主流agent拆解/eino/核心机制.md`
**依赖：** T2、T4
**步骤：**
1. 开头导读（读完能回答：Graph 怎么表达分支循环？流式怎么自动转换？Callbacks 怎么不侵入业务？）
2. 机制一 Component 抽象与类型安全：问题（多供应商多工具接口不统一）→ 方案（统一接口 + 编译期类型检查）→ 图解 → 一句话记住
3. 机制二 Graph 编排与状态流转：问题（分支/循环/并行怎么表达）→ 方案（Node/Edge/Branch + State 合并）→ mermaid 图（一个带条件分支的 Agent 图）→ 一句话记住
4. 机制三 流式处理自动转换：问题（有的节点流式、有的非流式，怎么拼）→ 方案（四大转换场景自动处理）→ mermaid 时序图 → 一句话记住
5. 机制四 Callbacks 横切：问题（日志/埋点/trace 侵入业务代码）→ 方案（五个注入时机 + 全局/请求级注入）→ 一句话记住
6. 收尾节「ReAct Agent 是怎么拼出来的」：用四个机制组装 react.NewAgent 的构成图
7. 速记卡

**验证：** 每个机制含「没有它会怎样」段落；≥2 个 mermaid；篇幅 ≤300 行

## T6: Eino ③ 对照与面试

**文件：** `docs/主流agent拆解/eino/对照与面试.md`
**依赖：** T4、T5
**步骤：**
1. 开头导读
2. Eino vs pi vs LangGraph 三框架对照表（语言/定位/编排范式/状态/持久化/流式/扩展/适用场景）
3. Eino 与字节技术栈关联节（CloudWeGo 生态、Kitex/Hertz、目标岗位匹配点）
4. 面试速答 5-8 条（Q&A 形式：Eino Graph vs LangGraph 图的区别、流式自动转换原理、Callbacks 用在哪、类型安全的价值）
5. 速记卡 + 交叉链接（pi、langgraph 各篇）

**验证：** 对照表覆盖三个框架；面试题均有简洁答案；链接指向 `/主流agent拆解/pi/` 与 `/主流agent拆解/langgraph/`

## T7: LangGraph ① 全景图

**文件：** `docs/主流agent拆解/langgraph/index.md`
**依赖：** T3
**步骤：**
1. 开头导读（读完能回答：LangGraph 和 LangChain 什么关系？图模型解决什么？核心概念有哪几个？）
2. 「从 LangChain 到 LangGraph」节：链式编排（LCEL）表达不了循环的问题（问题先行）
3. 核心概念一张图（mermaid）：StateGraph / Node / Edge / START·END / Reducer / Checkpointer
4. 逐个概念一段话 + 一句话记住
5. 15 行内最小 Agent 示例（Python，基于 T3 核实的 API）
6. 篇目导航表 + 速记卡

**验证：** 篇幅 ≤300 行；≥1 个 mermaid；代码与 notes-langgraph.md 一致

## T8: LangGraph ② 核心机制

**文件：** `docs/主流agent拆解/langgraph/核心机制.md`
**依赖：** T3、T7
**步骤：**
1. 开头导读（读完能回答：Pregel 怎么跑图？并行写冲突怎么办？崩溃怎么恢复？人工确认怎么做？）
2. 机制一 图模型与 Pregel 执行：问题（agent 需要循环，DAG 表达不了）→ 方案（Super-step 消息传递）→ mermaid super-step 时序图 → 一句话记住
3. 机制二 State 与 Reducer：问题（多节点并行写同一字段）→ 方案（Annotated reducer、add_messages 设计意图）→ 最小代码示例 → 一句话记住
4. 机制三 Checkpointer 持久化：问题（崩溃恢复/多轮会话记忆）→ 方案（thread_id + checkpoint、event-sourcing 味道）→ mermaid → 一句话记住
5. 机制四 Interrupt 人机协作：问题（危险操作需人工确认）→ 方案（interrupt + Command(resume=)）→ 最小代码示例 → 一句话记住
6. 速记卡

**验证：** 每个机制含「没有它会怎样」段落；≥2 个 mermaid；篇幅 ≤300 行

## T9: LangGraph ③ 对照与面试

**文件：** `docs/主流agent拆解/langgraph/对照与面试.md`
**依赖：** T7、T8、T6（引用 Eino 篇）
**步骤：**
1. 开头导读
2. LangGraph vs Eino vs pi 对照表（与 T6 表格互补视角，突出语言生态差异）
3. 「用 Go 思维理解 LangGraph」节：super-step ≈ 轮次调度、Reducer ≈ 合并策略、Checkpointer ≈ event-sourcing、interrupt ≈ 阻塞等待审批 channel
4. 面试速答 5-8 条（Pregel 是什么、为什么 Reducer 用 add_messages、Checkpointer 怎么实现断点续跑、LangGraph vs LangChain）
5. 速记卡 + 交叉链接

**验证：** Go 思维对照节至少 4 条映射；面试题有答案；链接有效

## T10: 模块首页

**文件：** `docs/主流agent拆解/index.md`
**依赖：** T4-T9（需引用全部篇目）
**步骤：**
1. 模块定位节：为什么拆这三个框架（产品形态 / Go 落地 / 业界主流三视角）
2. 三框架横向对照表（语言、定位、编排范式、状态管理、持久化、流式、扩展机制、适用场景）
3. 三个框架导航卡片（篇目 + 面试价值星级）
4. 推荐阅读顺序节：完整路径（pi → Eino → LangGraph）+ 面试冲刺快路径
5. 与 phase3 实战项目的关系说明（拆解指导自研，自研验证拆解）

**验证：** 全部 11 个篇目链接有效；对照表与 T6/T9 内容一致

## T11: VitePress 配置更新

**文件：** `docs/.vitepress/config.mts`
**依赖：** T1、T10（路径全部确定后）
**步骤：**
1. nav「项目实战」下拉删除 `pi 开源 Harness 拆解` 项
2. nav 新增「主流 Agent 拆解」顶级下拉（总览 + 三框架入口，位于「项目实战」之后）
3. sidebar 删除 `'/phase3/pi-harness/'` 分组
4. sidebar 新增 `'/主流agent拆解/'`：三个 collapsed 分组（pi 5 篇 / eino 3 篇 / langgraph 3 篇）+ 模块首页链接 + 「相关」组（phase3 两个实战项目）
5. 检查 `'/phase3/agent-harness/'` 与 `'/phase3/docs-rag/'` sidebar 中「相关」列表，如有 pi 链接则更新为新路径

**验证：** `npm run docs:dev` 起服务后导航与侧边栏显示正确；配置无语法错误（构建通过）

## T12: 存量页面清理

**文件：** `docs/phase3/index.md`、`docs/路线专题/04-简历项目改造与面试实战.md`
**依赖：** T1
**步骤：**
1. `phase3/index.md`：删除 pi 拆解条目，文末「后续项目」前加一行指引（框架拆解已移至主流 Agent 拆解模块）
2. 开头三件套说明如有涉及 pi 的描述同步调整
3. `路线专题/04`：3 处 pi-harness 文本提及改为「主流 Agent 拆解 · pi」并附链接 `/主流agent拆解/pi/`
4. 全站兜底：`grep -rn "phase3/pi-harness" docs/` 确认输出为空

**验证：** grep 输出为空；phase3/index.md 只列 docs-rag 与 agent-harness 两个项目

## T13: 构建验收

**文件：** 全部
**依赖：** T1-T12
**步骤：**
1. `npm run docs:build` 构建
2. 检查构建输出无死链（dead link）报错
3. `npm run docs:preview` 或 dev 服务人工抽查：模块首页、pi 迁移页、Eino/LangGraph 各篇、mermaid 渲染

**验证：** 构建 exit code 0；无 dead link 警告；抽查页面渲染正常

## 执行顺序

```
T1（pi 迁移）──┐
T2（Eino 核实）─┼─→ T4 → T5 → T6 ──┐
T3（LG 核实）───┘         T7 → T8 → T9 ──┼─→ T10 → T11 → T12 → T13
                                          （T9 依赖 T6）
```
