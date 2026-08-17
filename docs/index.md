---
layout: home

hero:
  name: GoCampus
  text: 从 Go 深入到 AI Agent 实战
  tagline: 为 AI 应用开发实习准备的一站式学习手册，用阶段计划组织知识，用代码练习检验掌握程度。
  image:
    src: /logo.svg
    alt: GoCampus
  actions:
    - theme: brand
      text: 开始学习
      link: /学习计划安排/总体规划
    - theme: alt
      text: Go 知识详解
      link: /第一阶段-知识点详解

features:
  - icon: 🧭
    title: 清晰的学习路线
    details: 按阶段推进，从 Go 语言、计算机基础逐步进入 AI 应用与 Agent 开发。
    link: /学习计划安排/总体规划
    linkText: 查看总体规划
  - icon: 🧠
    title: 面试级知识讲解
    details: 先看阶段总览，再通过 Slice、Map、Interface、String 专题理解设计原因和版本边界。
    link: /第一阶段-知识详解/String与字节切片
    linkText: 阅读 String 专题
  - icon: 🧪
    title: 配套代码练习
    details: 每个专题配有测试与参考答案，用 go test 形成即时反馈。
    link: /练习指南
    linkText: 开始动手练习
  - icon: 🔎
    title: 全文搜索
    details: 使用右上角搜索框快速定位概念、题目与面试要点，支持键盘操作。
---

## 推荐学习顺序

1. 先阅读[总体规划](/学习计划安排/总体规划)，了解阶段目标和时间分配。
2. 阅读[第一阶段知识点总览](/第一阶段-知识点详解)，建立 Go 核心知识框架。
3. 进入[Slice、Map 与内存布局专题](/第一阶段-知识详解/Slice-Map与内存布局)，理解底层设计和常见误区。
4. 继续学习[Interface 底层原理](/第一阶段-知识详解/Interface底层原理)和[String 与字节切片](/第一阶段-知识详解/String与字节切片)。
5. 学习[Go 函数调用与栈详解](/第一阶段-知识详解/Go函数调用与栈详解)，理解 PC/SP/Stack Frame、逃逸分析与 goroutine 栈，为并发与内存分配打基础。
6. 学习[Go 内存分配详解](/第一阶段-知识详解/Go内存分配详解)，掌握 mcache/mcentral/mheap 三层分配模型与 size class 的关系。
7. 学习[Go GC 详解](/第一阶段-知识详解/Go GC 详解)，掌握三色标记、混合写屏障与 GOGC/GOMEMLIMIT 调优。
8. 学习[Go 并发编程详解](/第一阶段-知识详解/Go并发编程详解)，掌握 Goroutine、Channel、sync 包与协程池实战；完成 channel 练习后对照[并发编程问答集](/第一阶段-知识详解/Go并发编程问答集)查漏补缺。随后学习 [Go Context 详解](/第一阶段-知识详解/Go Context 详解)，理解取消传播与超时控制的源码实现，并完成 07_context 练习。
9. 深入[Go GMP 调度详解](/第一阶段-知识详解/Go GMP 调度详解)，掌握 G 状态机、Park/gopark 与 Handoff，理解并发与并行的区别。
10. 补齐后端短板：进入[后端技术栈强化](/后端技术栈强化/)模块，按 MySQL → Redis → Kafka → 微服务 → 高并发场景题 → Agent Backend 顺序学习。
11. 按阶段同步完成对应代码练习，每周用自己的语言复述并记录薄弱项。

::: tip 学习建议
文档用于建立知识框架，代码练习用于暴露理解偏差。每学完一个小节，先独立完成练习，再查看参考答案。
:::
