---
layout: home

hero:
  name: GoCampus
  text: 从 Go 底层到 AI Agent 与分布式架构
  tagline: Go 后端 + AI Agent 求职学习手册：原理、可跑练习、Docker 实验与面试追问链，一条路线走完。
  image:
    src: /logo.svg
    alt: GoCampus
  actions:
    - theme: brand
      text: 从总体规划开始
      link: /学习计划安排/总体规划
    - theme: alt
      text: 后端技术栈 S1~S12
      link: /后端技术栈强化/

features:
  - icon: 🧭
    title: 一条明确的主线
    details: 先定阶段与时间，再按 Go 底层 → 计算机基础 → 后端技术栈 → AI 应用 → 架构 → 面试收口推进。
    link: /学习计划安排/总体规划
    linkText: 查看总体规划
  - icon: 🐹
    title: Go 语言底层原理
    details: Slice/Map/Interface/String 内存布局、函数调用栈、内存分配、GC、GMP、并发与 Context，共 10 篇专题。
    link: /第一阶段-知识详解/
    linkText: 进入第一阶段
  - icon: 🏗️
    title: 后端技术栈 S1~S12
    details: MySQL、Redis、Kafka、微服务、高并发、K8s、分布式、对象存储、Milvus、Elasticsearch，以及 Kafka vs RabbitMQ 专题。
    link: /后端技术栈强化/
    linkText: 按依赖顺序学习
  - icon: 🧠
    title: AI 应用与 Agent
    details: RAG → Prompt → MCP → ReAct → Multi-Agent 的知识地图，配套 Agent 拆解与三个可写进简历的项目。
    link: /导师学习路径/
    linkText: 打开 Agent 路径
  - icon: 🏛️
    title: 架构师修炼
    details: QPS 六道坎驱动的演进决策、真实落地参数、故障兜底与 15 个可复现实验，另有 pprof 性能取证专题。
    link: /架构师修炼/
    linkText: 开始修炼
  - icon: 🧪
    title: 学会的标准是跑通
    details: 128 道可判练习 + Docker 中间件实验，go test 给出即时反馈，文档与代码自动同步。
    link: /练习指南
    linkText: 开始动手
---

## 从哪里开始

先花 30 分钟读[总体规划](/学习计划安排/总体规划)，确定目标岗位与时间预算，然后按阶段推进。**每个阶段都配了可跑的练习或实验，读完不算学会，跑通才算。**

| 阶段 | 学什么 | 入口 | 验收方式 |
|------|--------|------|----------|
| 0 · 规划 | 目标、时间、阶段划分 | [总体规划](/学习计划安排/总体规划) | 写下目标岗位与周计划 |
| 1 · Go 语言 | 底层原理 10 篇：内存、GC、GMP、并发 | [第一阶段总览](/第一阶段-知识详解/) | `code/phase1/` 34 道练习 |
| 2 · 计算机基础 | 操作系统、网络、分布式理论 | [第二阶段总览](/第二阶段-知识详解/) | 面试问答自测 |
| 3 · 后端技术栈 | S1 MySQL → S12 消息队列 | [模块总览](/后端技术栈强化/) | `code/backend/` 39 道实验 |
| 4 · AI 应用 | RAG / Prompt / MCP / ReAct / Multi-Agent | [导师学习路径](/导师学习路径/) | 项目实战 + 面试问答 |
| 5 · 工程与架构 | 微服务、K8s、架构演进、pprof | [架构师修炼](/架构师修炼/) | 15 个可复现实验 |
| 6 · 面试收口 | 项目深挖、白板设计、冲刺路线 | [面试项目深挖](/面试项目深挖/) | 三版口述稿 + 追问链 |

## 三条路线，按目标选

**A. Go 后端 / 基础架构**
`1 → 2 → 3 → 5 → 6`
重点补齐 S1~S12 组件原理，再用[架构师修炼](/架构师修炼/)把「会用」升级成「会选型、会兜底」；[K8s Code 教程](/k8s-code教程/) 与 [Go 微服务实战](/microservice-code教程/) 放在阶段 5 并行推进。

**B. AI 应用开发（Agent 方向）**
`1 → 2 → 4 → 3（选学）→ 6`
以[导师学习路径](/导师学习路径/)为主线，配合[主流 Agent 拆解](/主流agent拆解/)（pi / Eino / LangGraph）与[项目实战](/项目实战/)（MewCode / RAG / Harness）；后端技术栈至少学完 MySQL、Redis、Kafka。

**C. 已有后端基础，想转大模型算法**
直接进入[转岗大模型算法](/转岗大模型算法/)的 18 个月作战地图：先做差距诊断，再补数学 / PyTorch / Transformer / 多模态地基，最后落到单卡训练与技术报告。

## 每个阶段怎么验收

1. **读完一篇 → 做对应练习**：先独立完成 `solution.go`，再对照 `answer/`；`cd code && go test ./...` 应全绿。
2. **学完一个模块 → 跑一次实验**：MySQL 主从、Redis 哨兵、Kafka 消息可靠性等实验在 [`code/architect/`](https://github.com/Bin-hy/go-campus/tree/main/code/architect)，按[实验手册](/架构师修炼/18-实验手册-Go落地实验)操作。
3. **进入面试前 → 用自己的话复述**：每篇末尾的自测题与追问链就是最小验收清单；整站可用右上角搜索定位概念。
4. **卡住时**：先看[练习指南](/练习指南)，环境与部署问题看[本地开发与部署](/本地开发与部署)。

::: tip 学习建议
文档用于建立知识框架，代码练习用于暴露理解偏差。每学完一个小节，先独立完成练习，再查看参考答案；能连续回答三层追问，才算真正掌握。
:::
