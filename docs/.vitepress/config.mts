import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'
import { buildExerciseSidebar } from './exercises'

export default withMermaid(defineConfig({
  lang: 'zh-CN',
  title: 'GoCampus',
  description: '面向 AI 应用开发实习的 Go 学习与实战手册',
  cleanUrls: true,
  lastUpdated: true,

  mermaid: {
    theme: 'base',
    themeVariables: {
      primaryColor: '#e6fcf5',
      primaryBorderColor: '#087f5b',
      primaryTextColor: '#212529',
      secondaryColor: '#f1f3f5',
      secondaryBorderColor: '#099268',
      tertiaryColor: '#fff9db',
      tertiaryBorderColor: '#f59f00',
      lineColor: '#099268',
      textColor: '#343a40',
      fontSize: '14px',
      fontFamily: 'system-ui, -apple-system, sans-serif',
      nodeTextColor: '#212529',
    },
  },
  mermaidPlugin: {
    class: 'mermaid',
  },

  head: [
    ['meta', { name: 'theme-color', content: '#087f5b' }],
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }]
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'GoCampus',

    nav: [
      { text: '首页', link: '/' },
      {
        text: '学习计划',
        items: [
          { text: '总体规划', link: '/学习计划安排/总体规划' },
          { text: '第一阶段 · Go 语言深入', link: '/学习计划安排/第一阶段-Go语言深入' },
          { text: '第二阶段 · 计算机基础', link: '/学习计划安排/第二阶段-计算机基础强化' },
          { text: '第三阶段 · AI 应用开发', link: '/学习计划安排/第三阶段-AI应用开发基础' }
        ]
      },
      {
        text: '导师学习路径',
        items: [
          { text: '总览与章节安排', link: '/导师学习路径/' },
          { text: '01 · RAG 原理与 LLM 基础', link: '/导师学习路径/01-RAG原理与LLM基础' },
          { text: '02 · RAG 完整原理与 Prompt 工程', link: '/导师学习路径/02-RAG完整原理与Prompt工程' },
          { text: '03 · Function Calling 与 MCP', link: '/导师学习路径/03-FunctionCalling与MCP协议' },
          { text: '04 · ReAct 与 Agent 规划', link: '/导师学习路径/04-ReAct与Agent规划' },
          { text: '05 · Multi-Agent 编排与 LangGraph', link: '/导师学习路径/05-Multi-Agent编排与LangGraph' },
          { text: '06 · Agent 记忆与评测体系', link: '/导师学习路径/06-Agent记忆与评测体系' },
          { text: '07 · 文生视频与 Diffusion', link: '/导师学习路径/07-JD专题-文生视频与Diffusion模型' },
          { text: '08 · LLM 推理优化', link: '/导师学习路径/08-JD专题-LLM推理优化' },
          { text: '09 · LLM 训练流程与微调', link: '/导师学习路径/09-JD专题-LLM训练流程与微调' },
          { text: '10 · AI 应用技术栈速通', link: '/导师学习路径/10-AI应用技术栈速通' }
        ]
      },
      {
        text: '路线专题',
        items: [
          { text: '30 天冲刺总览', link: '/路线专题/' },
          { text: '算法专项训练', link: '/路线专题/01-算法专项训练' },
          { text: '后端与计算机基础', link: '/路线专题/02-后端与计算机基础' },
          { text: '大模型与 Agent 核心能力', link: '/路线专题/03-大模型与Agent核心能力' },
          { text: '简历项目改造与面试实战', link: '/路线专题/04-简历项目改造与面试实战' }
        ]
      },
      {
        text: '知识详解',
        items: [
          { text: '第一阶段 · Go 语言深入', link: '/第一阶段-知识点详解' },
          { text: 'Slice、Map 与内存布局', link: '/第一阶段-知识详解/Slice-Map与内存布局' },
          { text: 'Interface 底层原理', link: '/第一阶段-知识详解/Interface底层原理' },
          { text: 'String 与字节切片', link: '/第一阶段-知识详解/String与字节切片' },
          { text: 'Go 函数调用与栈详解', link: '/第一阶段-知识详解/Go函数调用与栈详解' },
          { text: 'Go 内存分配详解', link: '/第一阶段-知识详解/Go内存分配详解' },
          { text: 'Go GC 详解', link: '/第一阶段-知识详解/Go GC 详解' },
          { text: 'Go 并发编程详解', link: '/第一阶段-知识详解/Go并发编程详解' },
          { text: 'Go GMP 调度详解', link: '/第一阶段-知识详解/Go GMP 调度详解' },
          { text: 'Go 并发编程问答集', link: '/第一阶段-知识详解/Go并发编程问答集' },
          { text: 'Go Context 详解', link: '/第一阶段-知识详解/Go Context 详解' },
          { text: '第二阶段 · 操作系统', link: '/第二阶段-知识详解/操作系统面试详解' },
          { text: '第二阶段 · 计算机网络', link: '/第二阶段-知识详解/计算机网络面试详解' },
          { text: '第二阶段 · 分布式系统', link: '/第二阶段-知识详解/分布式系统面试详解' },
          { text: '后端技术栈强化', link: '/后端技术栈强化/' },
          { text: 'S10 · 向量数据库 Milvus', link: '/后端技术栈强化/10-milvus/01-向量检索原理' }
        ]
      },
      {
        text: 'K8s Code 教程',
        link: '/k8s-code教程/'
      },
      {
        text: '项目实战',
        items: [
          { text: 'RAG 文档问答系统', link: '/phase3/docs-rag/' },
          { text: 'AI Agent Harness', link: '/phase3/agent-harness/' }
        ]
      },
      {
        text: '主流 Agent 拆解',
        items: [
          { text: '拆解总览与三框架对照', link: '/主流agent拆解/' },
          { text: 'pi · 开源 Agent Harness', link: '/主流agent拆解/pi/' },
          { text: 'Eino · Go 编排框架', link: '/主流agent拆解/eino/' },
          { text: 'LangGraph · Python 编排框架', link: '/主流agent拆解/langgraph/' }
        ]
      },
      {
        text: '代码练习',
        items: [
          { text: '练习指南', link: '/练习指南' },
          { text: '习题集和答案', link: '/习题集和答案/' }
        ]
      }
    ],

    sidebar: {
      '/路线专题/': [
        {
          text: '30 天冲刺训练营',
          items: [
            { text: '专题总览', link: '/路线专题/' },
            { text: '算法专项训练（Day 1-30）', link: '/路线专题/01-算法专项训练' },
            { text: '后端与计算机基础（Day 1-9）', link: '/路线专题/02-后端与计算机基础' },
            { text: '大模型与 Agent 核心能力（Day 8-17）', link: '/路线专题/03-大模型与Agent核心能力' },
            { text: '简历项目改造与面试实战（Day 18-30）', link: '/路线专题/04-简历项目改造与面试实战' }
          ]
        }
      ],
      '/导师学习路径/': [
        {
          text: '导师学习路径',
          items: [
            { text: '总览与章节安排', link: '/导师学习路径/' },
            { text: '01 · RAG 原理与 LLM 基础', link: '/导师学习路径/01-RAG原理与LLM基础' },
            { text: '02 · RAG 完整原理与 Prompt 工程', link: '/导师学习路径/02-RAG完整原理与Prompt工程' },
            { text: '03 · Function Calling 与 MCP', link: '/导师学习路径/03-FunctionCalling与MCP协议' },
            { text: '04 · ReAct 与 Agent 规划', link: '/导师学习路径/04-ReAct与Agent规划' },
            { text: '05 · Multi-Agent 编排与 LangGraph', link: '/导师学习路径/05-Multi-Agent编排与LangGraph' },
            { text: '06 · Agent 记忆与评测体系', link: '/导师学习路径/06-Agent记忆与评测体系' },
            { text: '07 · 文生视频与 Diffusion', link: '/导师学习路径/07-JD专题-文生视频与Diffusion模型' },
            { text: '08 · LLM 推理优化', link: '/导师学习路径/08-JD专题-LLM推理优化' },
            { text: '09 · LLM 训练流程与微调', link: '/导师学习路径/09-JD专题-LLM训练流程与微调' },
            { text: '10 · AI 应用技术栈速通', link: '/导师学习路径/10-AI应用技术栈速通' }
          ]
        }
      ],
      '/学习计划安排/': [
        {
          text: '学习计划',
          items: [
            { text: '总体规划', link: '/学习计划安排/总体规划' },
            { text: '第一阶段：Go 语言深入', link: '/学习计划安排/第一阶段-Go语言深入' },
            { text: '第二阶段：计算机基础强化', link: '/学习计划安排/第二阶段-计算机基础强化' },
            { text: '第三阶段：AI 应用开发基础', link: '/学习计划安排/第三阶段-AI应用开发基础' }
          ]
        }
      ],
      '/第一阶段-知识': [
        {
          text: '第一阶段 · 知识详解',
          items: [
            { text: '知识点总览', link: '/第一阶段-知识点详解' },
            { text: 'Slice、Map 与内存布局', link: '/第一阶段-知识详解/Slice-Map与内存布局' },
            { text: 'Interface 底层原理', link: '/第一阶段-知识详解/Interface底层原理' },
            { text: 'String 与字节切片', link: '/第一阶段-知识详解/String与字节切片' },
            { text: 'Go 函数调用与栈详解', link: '/第一阶段-知识详解/Go函数调用与栈详解' },
            { text: 'Go 内存分配详解', link: '/第一阶段-知识详解/Go内存分配详解' },
            { text: 'Go GC 详解', link: '/第一阶段-知识详解/Go GC 详解' },
            { text: 'Go 并发编程详解', link: '/第一阶段-知识详解/Go并发编程详解' },
            { text: 'Go GMP 调度详解', link: '/第一阶段-知识详解/Go GMP 调度详解' },
            { text: 'Go 并发编程问答集', link: '/第一阶段-知识详解/Go并发编程问答集' },
            { text: 'Go Context 详解', link: '/第一阶段-知识详解/Go Context 详解' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '阶段学习计划', link: '/学习计划安排/第一阶段-Go语言深入' },
            { text: '代码练习指南', link: '/练习指南' }
          ]
        }
      ],
      '/后端技术栈强化/': [
        {
          text: '开始',
          items: [
            { text: '模块总览', link: '/后端技术栈强化/' }
          ]
        },
        {
          text: 'S1 MySQL · 基础篇（先过一遍）',
          collapsed: false,
          items: [
            { text: 'MySQL 入门与库表操作', link: '/后端技术栈强化/01-mysql/基础-MySQL入门与库表操作' },
            { text: 'SQL 语法详解', link: '/后端技术栈强化/01-mysql/基础-SQL语法详解' },
            { text: '表设计与约束范式', link: '/后端技术栈强化/01-mysql/基础-表设计与约束范式' },
            { text: '事务与索引入门', link: '/后端技术栈强化/01-mysql/基础-事务与索引入门' }
          ]
        },
        {
          text: 'S1 MySQL · 深入篇',
          collapsed: false,
          items: [
            { text: '存储引擎与 B+ 树', link: '/后端技术栈强化/01-mysql/存储引擎与B+树' },
            { text: '事务与 MVCC', link: '/后端技术栈强化/01-mysql/事务与MVCC' },
            { text: '索引与 SQL 优化', link: '/后端技术栈强化/01-mysql/索引与SQL优化' },
            { text: '慢查询优化实战', link: '/后端技术栈强化/01-mysql/慢查询优化实战' },
            { text: '主从复制与高可用', link: '/后端技术栈强化/01-mysql/主从复制与高可用' },
            { text: '面试题集', link: '/后端技术栈强化/01-mysql/面试题集' }
          ]
        },
        {
          text: 'S2 Redis 深入',
          collapsed: false,
          items: [
            { text: '数据结构底层', link: '/后端技术栈强化/02-redis/数据结构底层' },
            { text: '持久化与高可用', link: '/后端技术栈强化/02-redis/持久化与高可用' },
            { text: '缓存问题与一致性', link: '/后端技术栈强化/02-redis/缓存问题与一致性' },
            { text: '分布式锁与场景', link: '/后端技术栈强化/02-redis/分布式锁与场景' },
            { text: '面试题集', link: '/后端技术栈强化/02-redis/面试题集' }
          ]
        },
        {
          text: 'S3 Kafka 深入',
          collapsed: false,
          items: [
            { text: '架构与存储', link: '/后端技术栈强化/03-kafka/架构与存储' },
            { text: '生产消费语义', link: '/后端技术栈强化/03-kafka/生产消费语义' },
            { text: '可靠性与积压', link: '/后端技术栈强化/03-kafka/可靠性与积压' },
            { text: '面试题集', link: '/后端技术栈强化/03-kafka/面试题集' }
          ]
        },
        {
          text: 'S4 Go 微服务',
          collapsed: false,
          items: [
            { text: '单体拆分与边界设计', link: '/后端技术栈强化/04-microservice/单体拆分与边界设计' },
            { text: '架构与 gRPC', link: '/后端技术栈强化/04-microservice/架构与gRPC' },
            { text: '治理与稳定性', link: '/后端技术栈强化/04-microservice/治理与稳定性' },
            { text: '面试题集', link: '/后端技术栈强化/04-microservice/面试题集' }
          ]
        },
        {
          text: 'S5 高并发场景题',
          collapsed: false,
          items: [
            { text: '一致性与故障边界（AP/CP 深挖）', link: '/后端技术栈强化/05-high-concurrency/一致性与故障边界' },
            { text: '场景题（上）：秒杀/红包/ID/一致性', link: '/后端技术栈强化/05-high-concurrency/场景题-上' },
            { text: '场景题（中）：锁/限流/幂等/消息', link: '/后端技术栈强化/05-high-concurrency/场景题-中' },
            { text: '场景题（下）：Feed/点赞/排行/短链', link: '/后端技术栈强化/05-high-concurrency/场景题-下' }
          ]
        },
        {
          text: 'S6 Agent Backend',
          collapsed: false,
          items: [
            { text: '系统设计与串联', link: '/后端技术栈强化/06-agent-backend/系统设计与串联' }
          ]
        },
        {
          text: 'S7 K8s 容器编排',
          collapsed: false,
          items: [
            { text: '架构与核心对象', link: '/后端技术栈强化/07-k8s/架构与核心对象' },
            { text: '调度与控制器', link: '/后端技术栈强化/07-k8s/调度与控制器' },
            { text: '网络与存储', link: '/后端技术栈强化/07-k8s/网络与存储' },
            { text: '高可用与故障排查', link: '/后端技术栈强化/07-k8s/高可用与故障排查' },
            { text: '面试题集', link: '/后端技术栈强化/07-k8s/面试题集' }
          ]
        },
        {
          text: 'S8 分布式理论',
          collapsed: false,
          items: [
            { text: '分布式系统能力全景', link: '/后端技术栈强化/08-distributed/分布式系统能力全景' },
            { text: 'CAP 与 BASE 理论', link: '/后端技术栈强化/08-distributed/CAP与BASE理论' },
            { text: '底层存储与数据同步', link: '/后端技术栈强化/08-distributed/底层存储与数据同步' },
            { text: 'Raft 算法详解', link: '/后端技术栈强化/08-distributed/Raft算法详解' },
            { text: 'etcd 详解与工程实践', link: '/后端技术栈强化/08-distributed/etcd详解与工程实践' },
            { text: '选主机制详解', link: '/后端技术栈强化/08-distributed/选主机制详解' },
            { text: 'gRPC 负载均衡详解', link: '/后端技术栈强化/08-distributed/gRPC负载均衡详解' },
            { text: '网络通信与降级方案', link: '/后端技术栈强化/08-distributed/网络通信与降级方案' },
            { text: '限流与熔断', link: '/后端技术栈强化/08-distributed/限流与熔断' },
            { text: '高可用机制全景', link: '/后端技术栈强化/08-distributed/高可用机制全景' },
            { text: '容灾与备份恢复', link: '/后端技术栈强化/08-distributed/容灾与备份恢复' },
            { text: '面试题集', link: '/后端技术栈强化/08-distributed/面试题集' }
          ]
        },
        {
          text: 'S9 对象存储',
          collapsed: false,
          items: [
            { text: '为什么需要对象存储', link: '/后端技术栈强化/09-object-storage/为什么需要对象存储' },
            { text: '架构与核心机制', link: '/后端技术栈强化/09-object-storage/架构与核心机制' },
            { text: 'S3 API 与 Go 实战', link: '/后端技术栈强化/09-object-storage/S3-API与Go实战' },
            { text: '安全与生产实践', link: '/后端技术栈强化/09-object-storage/安全与生产实践' },
            { text: 'STS 临时凭证', link: '/后端技术栈强化/09-object-storage/STS临时凭证' },
            { text: '面试题集', link: '/后端技术栈强化/09-object-storage/面试题集' }
          ]
        },
        {
          text: 'S10 向量数据库 Milvus',
          collapsed: false,
          items: [
            { text: '向量检索原理', link: '/后端技术栈强化/10-milvus/01-向量检索原理' },
            { text: 'Milvus 架构与核心概念', link: '/后端技术栈强化/10-milvus/02-Milvus架构与核心概念' },
            { text: 'Collection 设计与数据写入', link: '/后端技术栈强化/10-milvus/03-Collection设计与数据写入' },
            { text: '向量索引与检索', link: '/后端技术栈强化/10-milvus/04-向量索引与检索' },
            { text: '一致性、事务与数据管理', link: '/后端技术栈强化/10-milvus/05-一致性事务与数据管理' },
            { text: '部署、监控与生产实践', link: '/后端技术栈强化/10-milvus/06-部署监控与生产实践' },
            { text: 'Milvus Go 实战', link: '/后端技术栈强化/10-milvus/07-Milvus-Go实战' },
            { text: '面试题集', link: '/后端技术栈强化/10-milvus/08-面试题集' }
          ]
        }
      ],
      '/k8s-code教程/': [
        {
          text: 'K8s Code 教程',
          items: [
            { text: '教程首页与路线图', link: '/k8s-code教程/' },
            { text: '01 环境准备与 kubectl 速查', link: '/k8s-code教程/01-环境准备与kubectl速查' },
            { text: '02 Pod 与容器生命周期', link: '/k8s-code教程/02-Pod与容器生命周期' },
            { text: '03 Deployment 与滚动发布', link: '/k8s-code教程/03-Deployment与滚动发布' },
            { text: '04 Service/Ingress/DNS', link: '/k8s-code教程/04-Service-Ingress与DNS' },
            { text: '05 配置与存储', link: '/k8s-code教程/05-配置与存储-ConfigMap-Secret-PV' },
            { text: '06 资源管理与 HPA/Job', link: '/k8s-code教程/06-资源管理与HPA-Job' },
            { text: '07 client-go：连接与 CRUD', link: '/k8s-code教程/07-client-go编程-连接与CRUD' },
            { text: '08 client-go：Informer 与 Workqueue', link: '/k8s-code教程/08-client-go编程-Informer与Workqueue' },
            { text: '09 手写 Controller', link: '/k8s-code教程/09-手写Controller实战' },
            { text: '10 CRD 与 Operator', link: '/k8s-code教程/10-CRD与Operator开发' },
            { text: '11 调度器深入', link: '/k8s-code教程/11-调度器深入' },
            { text: '12 网络与安全深入', link: '/k8s-code教程/12-网络与安全深入' },
            { text: '13 故障排查与生产实践', link: '/k8s-code教程/13-故障排查与生产实践' },
            { text: '14 面试题集', link: '/k8s-code教程/14-面试题集' }
          ]
        }
      ],
      '/练习指南': [
        {
          text: '代码练习',
          items: [
            { text: '练习指南', link: '/练习指南' },
            { text: '习题集和答案', link: '/习题集和答案/' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '知识点总览', link: '/第一阶段-知识点详解' },
            { text: '第一阶段学习计划', link: '/学习计划安排/第一阶段-Go语言深入' }
          ]
        }
      ],
      '/第二阶段-知识详解/': [
        {
          text: '第二阶段 · 计算机基础',
          items: [
            { text: '操作系统面试详解', link: '/第二阶段-知识详解/操作系统面试详解' },
            { text: '计算机网络面试详解', link: '/第二阶段-知识详解/计算机网络面试详解' },
            { text: '分布式系统面试详解', link: '/第二阶段-知识详解/分布式系统面试详解' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '阶段学习计划', link: '/学习计划安排/第二阶段-计算机基础强化' },
            { text: '代码练习指南', link: '/练习指南' }
          ]
        }
      ],
      '/习题集和答案/': buildExerciseSidebar(),
      '/phase3/agent-harness/': [
        {
          text: 'AI Agent Harness',
          items: [
            { text: '项目首页', link: '/phase3/agent-harness/' },
            { text: '学习笔记', link: '/phase3/agent-harness/学习笔记' },
            { text: '项目设计', link: '/phase3/agent-harness/项目设计' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '阶段学习计划', link: '/学习计划安排/第三阶段-AI应用开发基础' },
            { text: '第三阶段总览', link: '/phase3/' },
            { text: 'RAG 文档问答系统', link: '/phase3/docs-rag/' }
          ]
        }
      ],
      '/主流agent拆解/': [
        {
          text: '主流 Agent 拆解',
          items: [
            { text: '拆解总览与三框架对照', link: '/主流agent拆解/' }
          ]
        },
        {
          text: 'pi · 开源 Agent Harness',
          collapsed: false,
          items: [
            { text: '项目全景', link: '/主流agent拆解/pi/' },
            { text: '01 核心机制：Agent Loop', link: '/主流agent拆解/pi/核心机制' },
            { text: '02 LLM 抽象层', link: '/主流agent拆解/pi/LLM抽象层' },
            { text: '03 工程化落地', link: '/主流agent拆解/pi/工程化落地' },
            { text: '04 Go 落地与面试', link: '/主流agent拆解/pi/Go落地与面试' }
          ]
        },
        {
          text: 'Eino · Go 编排框架',
          collapsed: false,
          items: [
            { text: '① 全景图', link: '/主流agent拆解/eino/' },
            { text: '② 核心机制', link: '/主流agent拆解/eino/核心机制' },
            { text: '③ 对照与面试', link: '/主流agent拆解/eino/对照与面试' }
          ]
        },
        {
          text: 'LangGraph · Python 编排框架',
          collapsed: false,
          items: [
            { text: '① 全景图', link: '/主流agent拆解/langgraph/' },
            { text: '② 核心机制', link: '/主流agent拆解/langgraph/核心机制' },
            { text: '③ 对照与面试', link: '/主流agent拆解/langgraph/对照与面试' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '第三阶段总览', link: '/phase3/' },
            { text: 'AI Agent Harness 项目', link: '/phase3/agent-harness/' },
            { text: 'RAG 文档问答系统', link: '/phase3/docs-rag/' }
          ]
        }
      ],
      '/phase3/docs-rag/': [
        {
          text: 'RAG 文档问答系统',
          items: [
            { text: '项目首页', link: '/phase3/docs-rag/' },
            { text: '学习笔记', link: '/phase3/docs-rag/学习笔记' },
            { text: '项目设计', link: '/phase3/docs-rag/项目设计' },
            { text: '多模态文档处理逻辑', link: '/phase3/docs-rag/多模态文档处理逻辑' },
            { text: 'Rag 的 13 种分块策略', link: '/phase3/docs-rag/Rag的13种分块策略' },
            { text: '落地化的 RAG 系统优化策略', link: '/phase3/docs-rag/落地化的RAG系统优化策略' }
          ]
        },
        {
          text: '相关链接',
          items: [
            { text: '阶段学习计划', link: '/学习计划安排/第三阶段-AI应用开发基础' },
            { text: '第三阶段总览', link: '/phase3/' }
          ]
        }
      ],
      '/phase3/': [
        {
          text: '第三阶段 · AI 应用开发',
          items: [
            { text: '阶段总览', link: '/phase3/' },
            { text: 'RAG 文档问答系统', link: '/phase3/docs-rag/' },
            { text: 'AI Agent Harness', link: '/phase3/agent-harness/' },
            { text: '主流 Agent 拆解', link: '/主流agent拆解/' }
          ]
        }
      ]
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/Bin-hy/go-campus' }
    ],

    search: {
      provider: 'local',
      options: {
        translations: {
          button: {
            buttonText: '搜索文档',
            buttonAriaLabel: '搜索文档'
          },
          modal: {
            noResultsText: '没有找到相关结果',
            resetButtonTitle: '清除查询条件',
            footer: {
              selectText: '选择',
              navigateText: '切换',
              closeText: '关闭'
            }
          }
        }
      }
    },

    outline: {
      level: [2, 3],
      label: '本页目录'
    },
    docFooter: {
      prev: '上一篇',
      next: '下一篇'
    },
    lastUpdated: {
      text: '最后更新于',
      formatOptions: {
        dateStyle: 'medium',
        timeStyle: 'short'
      }
    },
    returnToTopLabel: '返回顶部',
    sidebarMenuLabel: '目录',
    darkModeSwitchLabel: '主题',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
    footer: {
      message: '持续学习，持续构建。',
      copyright: 'GoCampus Learning Notes'
    }
  }
}))
