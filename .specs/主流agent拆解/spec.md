# 主流 Agent 拆解模块 Spec

## 背景

当前 VitePress 文档站 `docs/phase3/`（第三阶段：AI 应用开发）混合了两类内容：

- **自研实战项目**：docs-rag（RAG 问答）、agent-harness（自研剪辑 Agent 运行时）
- **开源框架拆解**：pi-harness（对 pi 开源 Agent 工具链的深度拆解）

两者目标不同：实战项目是「动手做」，框架拆解是「读懂别人的设计」。混在一起会让读者对「第三阶段=项目实战」的定位产生困惑。

同时，目标岗位（字节剪映 AI 剪辑 Agent 开发，Go 方向）面试高频涉及主流 Agent 框架的设计理解，目前只有 pi 一篇拆解，缺少对 **Eino**（CloudWeGo 的 Go 语言 LLM 应用框架，与目标岗位技术栈直接匹配）和 **LangGraph**（Python 生态最主流的 Agent 编排框架，面试常考对照）的拆解。

## 目标

- 建立独立的「主流 Agent 拆解」顶层模块，专门承载开源 Agent 框架/工具链的深度拆解
- pi 拆解从第三阶段迁入该模块，第三阶段回归纯实战项目定位
- 新增 Eino、LangGraph 两篇高质量拆解，与 pi 拆解形成「三框架对照」知识体系
- 阅读体验目标：**无负担阅读**（问题先行、图解优先、单篇长度克制、随时可跳读）+ **吃透核心概念**（每个概念讲清"为什么存在"，并落到 Go/面试场景）

## 功能需求

- F1: 新建顶层文档目录「主流agent拆解」，包含模块首页（index），首页提供：模块定位说明、三个框架（pi / Eino / LangGraph）的导航卡片、三框架横向对照表（语言、编排范式、状态管理、持久化、适用场景）、推荐阅读顺序
- F2: 将 pi-harness 现有 5 个文档（index、核心机制、LLM抽象层、工程化落地、Go落地与面试）整体迁入新模块，更新所有内部链接（文档间互链、指向 phase3 的链接、VitePress 导航与侧边栏配置）
- F3: 新增 Eino 拆解，共 3 篇：
  - ① 全景图：Eino 是什么、解决什么问题、核心概念一张图（Component / Chain / Graph / Callback）
  - ② 核心机制拆解：Graph 编排与状态流转、ReAct Agent 实现、流式处理（流式拼接四大范式）、Callbacks 横切机制——每个机制按「问题 → 方案 → 图解」组织
  - ③ 对照与面试：Eino vs pi vs LangGraph 对照、字节系技术栈关联、面试速答与速记卡
- F4: 新增 LangGraph 拆解，共 3 篇：
  - ① 全景图：LangGraph 是什么、与 LangChain 的关系、核心概念一张图（StateGraph / Node / Edge / Checkpointer）
  - ② 核心机制拆解：图模型与 Pregel 执行模型（Super-step）、状态与 Reducer、持久化与断点续跑（Checkpointer/Interrupt）、人机协作——每个机制按「问题 → 方案 → 图解」组织
  - ③ 对照与面试：LangGraph vs Eino vs pi 对照、用 Go 思维理解 LangGraph、面试速答与速记卡
- F5: VitePress 配置更新：顶部导航新增「主流 Agent 拆解」入口；为新模块配置侧边栏；第三阶段侧边栏与导航移除 pi-harness 入口；phase3/index.md 项目列表移除 pi 拆解条目
- F6: 每篇拆解文档遵循统一的「无负担阅读」模板：
  - 开头一段「这篇讲什么 + 读完能回答什么问题」
  - 每个核心概念先讲「没有它会怎样」（问题先行），再讲方案
  - 关键机制配 mermaid 图解
  - 篇末 30 秒「速记卡」
  - 单篇正文控制在可一次读完的篇幅（约 200~300 行）

## 非功能需求

- N1: 流程图一律使用 mermaid 语法（遵循仓库编写规则）
- N2: 所有新增/修改文档为中文，代码示例中 Eino 用 Go、LangGraph 用 Python，均给出最小可懂的片段而非大段源码
- N3: 内容准确性：Eino 基于 cloudwego/eino 当前主版本（v0.x，含 compose/Graph、ReAct Agent、Callbacks）；LangGraph 基于 langchain-ai/langgraph 当前主版本（StateGraph、Pregel、checkpointer）。不确定的 API 细节需通过官方仓库/文档核实
- N4: 站内链接全部使用相对路径形式（VitePress `/路径` 形式），迁移后无死链
- N5: 风格与现有 pi 拆解一致：表格、mermaid、「面试价值」视角、速记卡

## 不做的事

- 不重写 pi 拆解正文内容（仅迁移 + 修链接 + 微调导航锚点）
- 不做 Eino/LangGraph 的源码逐行走读（只拆解设计机制与核心概念）
- 不新增除 pi/Eino/LangGraph 之外的框架拆解（如 AutoGen、CrewAI，留待后续）
- 不改动 docs-rag、agent-harness 两个实战项目的内容
- 不做英文版

## 验收标准

- AC1: 访问新模块首页能看到 pi / Eino / LangGraph 三个入口与三框架对照表（对应 F1）
- AC2: pi 拆解 5 篇在新路径下可访问，原 `/phase3/pi-harness/` 路径不再出现在导航与侧边栏，站内无指向旧路径的链接（对应 F2、F5）
- AC3: Eino 拆解 3 篇可访问，每篇符合 F6 模板（开头导读、问题先行、至少 1 个 mermaid 图、篇末速记卡）（对应 F3、F6）
- AC4: LangGraph 拆解 3 篇可访问，每篇符合 F6 模板（对应 F4、F6）
- AC5: `npm run docs:build` 构建成功，无死链报错（VitePress 构建会校验内部链接）（对应 N4）
- AC6: phase3/index.md 只保留 docs-rag 与 agent-harness 两个实战项目（对应 F5）
