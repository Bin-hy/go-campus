# 第三阶段：AI 应用开发

本阶段以项目为驱动，通过实战掌握 AI 应用开发全链路。每个项目包含三件套：

- **项目首页**：项目简介、目标、技术栈、进度
- **学习笔记**：相关知识点整理
- **项目设计**：系统架构与实现方案

> 开源 Agent 框架拆解（pi / Eino / LangGraph）已独立成模块，见 [主流 Agent 拆解](/主流agent拆解/)。实战项目与框架拆解配合食用：拆解指导自研，自研验证拆解。

---

## 项目列表

### 1. MewCode — 从零手写的终端 AI Agent ★ 面试主讲项目

使用 Go 从零实现的终端 Coding Agent（对标 Claude Code）：单二进制、双协议（Anthropic + OpenAI）、多轮 ReAct 循环、保序分批并发、五层权限护栏、两层上下文压缩、MCP 工具生态、Skill / Hook / SubAgent 三级扩展。**13 个功能章节、115 个 Go 文件、约 1.78 万行**，全部手写、不用任何 Agent 框架。

- [项目首页](/phase3/mewcode/)
- [项目设计（ADR）](/phase3/mewcode/项目设计)
- [面试讲解（01-11 章）](/phase3/mewcode/01-项目全景与面试开场)
- [面试追问题库（120 问）](/phase3/mewcode/12-面试追问题库)
- [企业级 Agent 平台方案](/phase3/mewcode/13-企业级Agent平台方案)
- 源码：[projects/EasyCoding](https://github.com/Bin-hy/EasyCoding)（git submodule）

### 2. RAG 文档问答系统

使用 Go 实现的文档 RAG 系统，支持对多类型文档内容的存储和知识问答。

- [项目首页](/phase3/docs-rag/)
- [学习笔记](/phase3/docs-rag/学习笔记)
- [项目设计](/phase3/docs-rag/项目设计)

### 3. AI Agent Harness — 智能视频剪辑 Agent

使用 Go 实现面向智能视频剪辑场景的 AI Agent 运行时框架，涵盖 ReAct 控制循环、工具编排、任务规划、记忆管理和安全护栏。模拟字节跳动剪映 CapCut AutoCut 的核心 Agent 能力。

> MewCode 是该蓝图的**已落地实现**（且更完整）：Harness 讲「应该怎么做」，MewCode 讲「我做到了哪一步、哪里没做到」。

- [项目首页](/phase3/agent-harness/)
- [学习笔记](/phase3/agent-harness/学习笔记)
- [项目设计](/phase3/agent-harness/项目设计)

---

> 后续项目将陆续添加到此页面。
