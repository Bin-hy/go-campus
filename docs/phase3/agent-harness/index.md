# AI Agent Harness — 智能视频剪辑 Agent

## 项目简介

使用 Go 实现一个面向**智能视频剪辑**场景的 AI Agent Harness（Agent 运行时框架），模拟字节跳动剪映 CapCut AutoCut 的核心 Agent 能力。

**Agent Harness** 是 AI Agent 系统中 LLM 之外的运行时基础设施层——包含 Agent 控制循环（ReAct Loop）、工具注册与执行、记忆管理、规划器、护栏（Guardrails）和可观测性等核心组件。正如 MongoDB 所述："The LLM is the smallest part of your agent system"。

## 为什么做这个项目

1. **目标岗位直接匹配**：字节跳动剪映 CapCut 正在招聘 "Agent开发实习生（AI剪辑）"，负责 AutoCut、智能成片、GenAI 等方向
2. **技术栈对齐**：字节内部使用 [CloudWeGo Eino](https://github.com/cloudwego/eino) 作为 Go 语言 LLM/Agent 应用开发框架
3. **深度展现 Go 工程能力**：Go 并发模型、接口设计、错误处理、可观测性在 Agent Harness 中全部涉及
4. **从 RAG 项目自然衍生**：RAG 系统将作为 Agent 的一个 Tool 被编排调用

## 核心能力

| 能力 | 说明 |
|------|------|
| ReAct 控制循环 | Reason → Act → Observe 循环驱动，支持最大步数限制 |
| Tool Registry | 动态注册/发现工具，JSON Schema 描述参数 |
| Planner | Plan-and-Execute 模式，多步任务分解与执行 |
| Memory | 短期（对话上下文）+ 长期（向量检索）记忆 |
| Guardrails | 输入/输出安全校验、敏感内容过滤、Token 预算控制 |
| Observability | OpenTelemetry Trace + Prometheus Metrics + 结构化日志 |
| 视频剪辑 Tools | 场景分割、字幕生成、BGM 推荐、画面裁剪等模拟工具 |

## 技术栈

| 领域 | 选型 |
|------|------|
| 语言 | Go 1.22+ |
| Agent 框架参考 | CloudWeGo Eino / sausheong/harness |
| LLM 接口 | OpenAI-compatible API (function calling) |
| 向量存储 | Qdrant (复用 RAG 项目) |
| 可观测 | OpenTelemetry + Prometheus + Grafana |
| 测试 | go test + testcontainers |
| 构建 | Makefile + Docker |

## 项目进度

| 阶段 | 状态 | 目标 |
|------|------|------|
| Phase 1: Core Loop | 🔲 待开始 | 跑通 ReAct 循环 + 基础 Tool 调用 |
| Phase 2: Planning & Memory | 🔲 待开始 | Plan-and-Execute + 对话记忆 |
| Phase 3: Video Tools | 🔲 待开始 | 视频剪辑场景工具集成 |
| Phase 4: Production | 🔲 待开始 | Guardrails + 可观测 + HTTP 服务 |

## 参考资源

- [CloudWeGo Eino — 字节跳动 Go LLM 框架](https://github.com/cloudwego/eino)
- [ByteDance Eino in Practice](https://www.cloudwego.io/docs/eino/overview/bytedance_eino_practice/)
- [Eino ReAct Agent Manual](https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/)
- [sausheong/harness — Go Agent Framework](https://github.com/sausheong/harness)
- [The Agent Harness: Why the LLM Is the Smallest Part (MongoDB)](https://www.mongodb.com/company/blog/technical/agent-harness-why-llm-is-smallest-part-of-your-agent-system)
- [Agent Orchestration Patterns (Azure)](https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns)
- [AutoCut: End-to-end Video Editing (CVPR 2026)](https://arxiv.org/abs/2603.28366v1)
- [Prompt-Driven Agentic Video Editing](https://arxiv.org/html/2509.16811v1)
- [tRPC-Agent-Go Planner](https://github.com/trpc-group/trpc-agent-go/blob/main/docs/mkdocs/en/planner.md)
- [Engineering and Governing the Agent Harness (UN University)](https://unu.edu/publication/engineering-and-governing-agent-harness-technology-and-policy-framework-runtime-layer)

---

- [学习笔记](/phase3/agent-harness/学习笔记)
- [项目设计](/phase3/agent-harness/项目设计)
