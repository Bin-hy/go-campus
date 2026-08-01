# RAG 文档问答系统

> 基于 Go 自研的企业级文档 RAG 系统，不依赖 LangChain 等框架，从零实现完整的检索增强生成管线。

## 项目定位

面试时能清晰讲述每个环节的设计选型与权衡，展示对 RAG 内部原理的深入理解，而非简单的框架调用。

## 核心特性

| 特性 | 说明 |
|------|------|
| 多格式文档解析 | PDF、Markdown、HTML，提取文本 + 表格 + 元数据 |
| 智能分块 | 递归语义分块 + Markdown 层级感知 + 可配置 overlap |
| 混合检索 | 向量语义检索（ANN） + 关键词检索（BM25），RRF 融合排序 |
| 多路查询 | Multi-Query 改写 + 查询分解（Decomposition） |
| 重排序 | Cross-Encoder 精排 Top-K，提升精确度 |
| 幻觉抑制 | 引用溯源 + 置信度评分 + 相似度阈值拒答 |
| 流式输出 | SSE 流式生成，首 token 延迟优化 |
| 评估框架 | 自动化评测：Recall@K、MRR、Faithfulness、Answer Relevance |
| Agent 集成 | RAG 作为 Tool，支持 Function Calling 协议接入 Agent 循环 |
| 生产就绪 | 异步索引、查询缓存、全链路 Trace、优雅降级 |

## 技术栈

| 组件 | 选型 | 选型理由 |
|------|------|----------|
| 语言 | Go | 字节主力后端语言，高并发友好 |
| LLM | OpenAI 兼容协议 | 可切换豆包/通义/DeepSeek |
| Embedding | text-embedding-3-small / BGE | 支持自定义模型切换 |
| 向量存储 | Qdrant（开发）/ Milvus（生产） | ANN 索引 + 元数据过滤 |
| 关键词检索 | 自研倒排索引 + BM25 | 理解底层原理，面试可讲 |
| 重排序 | BGE-Reranker API | Cross-Encoder 精排 |
| 配置 | Viper + .env | 多环境适配 |
| CLI | Cobra | 多命令结构 |
| 可观测 | OpenTelemetry | 全链路 Trace |

## 项目架构

```
projects/docs-rag/
├── cmd/
│   └── rag/main.go                # CLI 入口（index/ask/chat/eval）
├── internal/
│   ├── loader/                    # 文档加载层
│   │   ├── loader.go             # Loader 接口定义
│   │   ├── pdf.go                # PDF 解析
│   │   ├── markdown.go           # Markdown 解析
│   │   └── html.go               # HTML 解析
│   ├── splitter/                  # 文档分块层
│   │   ├── splitter.go           # Splitter 接口
│   │   ├── recursive.go          # 递归字符分块
│   │   └── markdown_aware.go     # Markdown 层级感知分块
│   ├── embedding/                 # Embedding 层
│   │   ├── client.go             # Embedding 接口 + OpenAI 实现
│   │   └── batch.go              # 批量 Embedding（减少 API 调用）
│   ├── store/                     # 存储层
│   │   ├── vector.go             # VectorStore 接口
│   │   ├── memory.go             # 内存实现（开发/测试）
│   │   ├── qdrant.go             # Qdrant 实现
│   │   └── bm25.go              # BM25 倒排索引
│   ├── retriever/                 # 检索层
│   │   ├── retriever.go          # Retriever 接口
│   │   ├── hybrid.go            # 混合检索（向量 + BM25 + RRF 融合）
│   │   ├── multiquery.go        # Multi-Query 查询改写
│   │   └── reranker.go          # Cross-Encoder 重排序
│   ├── generator/                 # 生成层
│   │   ├── generator.go          # Generator 接口
│   │   ├── prompt.go            # Prompt 模板管理
│   │   ├── stream.go            # SSE 流式生成
│   │   └── citation.go          # 引用溯源 + 幻觉检测
│   ├── eval/                      # 评估框架
│   │   ├── evaluator.go          # 评估接口
│   │   ├── retrieval.go         # 检索质量：Recall@K, MRR, NDCG
│   │   └── generation.go        # 生成质量：Faithfulness, Relevance
│   └── agent/                     # Agent 集成层
│       ├── tool.go               # RAG 作为 Function Calling Tool
│       └── memory.go            # 对话记忆管理
├── pkg/
│   ├── llm/                       # LLM 通用客户端（可复用）
│   │   ├── client.go             # OpenAI 兼容 HTTP 客户端
│   │   ├── stream.go            # SSE 解析
│   │   └── retry.go             # 指数退避重试
│   └── similarity/                # 相似度算法
│       └── cosine.go             # 余弦相似度 / 点积 / 欧氏距离
├── config/
│   └── config.go                  # 配置结构体 + Viper 加载
├── eval/
│   └── testsets/                  # 评测数据集
│       └── rag_qa.json           # 问答对 + 标注来源
├── Makefile
├── go.mod
└── README.md
```

## 命令设计

```bash
# 索引构建
rag index ./docs/ --chunk-size=500 --overlap=50

# 单次问答
rag ask "RAG 系统中如何处理幻觉问题？" --top-k=5 --rerank

# 交互式对话
rag chat --stream --hybrid

# 质量评估
rag eval --testset=eval/testsets/rag_qa.json --metrics=recall,faithfulness

# Agent 模式（RAG 作为 Tool）
rag serve --port=8080  # 暴露 Function Calling 兼容接口
```

## 关键设计决策

### 为什么自研而不用 LangChain？

1. **面试可讲**：每个环节的实现细节都能深入讨论
2. **Go 生态**：LangChain 是 Python，字节后端用 Go
3. **可控性**：生产环境需要精细调优，框架黑盒不利于排查
4. **理解深度**：实现过程中深入理解 BM25 评分、ANN 索引、RRF 融合等原理

### 为什么混合检索而不只用向量？

- 向量检索擅长语义匹配（"怎么处理 JSON" ≈ "解析 JSON 的方法"）
- 关键词检索擅长精确匹配（专有名词、代码片段、API 名称）
- RRF 融合取两者之长，是业界标准做法（字节内部也这么做）

### 为什么需要评估框架？

- 没有量化指标 = 无法证明系统有效
- 面试时能给出具体数字："我的系统 Recall@5 = 0.87，Faithfulness = 0.92"
- 支持 A/B 对比：改了分块策略后，检索质量提升了多少？

## 文档导航

- [学习笔记](./学习笔记) — RAG 核心知识点整理（索引构建/检索/生成/优化策略）
- [项目设计](./项目设计) — 系统架构设计与接口定义

## 开发进度

- [x] RAG 核心知识学习与笔记整理
- [ ] 项目骨架搭建 + 接口定义
- [ ] 文档加载器（PDF / Markdown）
- [ ] 递归分块 + Markdown 感知分块
- [ ] Embedding 客户端 + 批量处理
- [ ] 内存向量存储 + BM25 倒排索引
- [ ] 混合检索 + RRF 融合
- [ ] Multi-Query 查询改写
- [ ] Cross-Encoder 重排序
- [ ] Prompt 模板 + 流式生成
- [ ] 引用溯源 + 幻觉抑制
- [ ] 评估框架 + 测试集
- [ ] Agent Tool 接口
- [ ] Qdrant 集成
- [ ] 全链路 Trace + 性能优化

## 面试亮点

1. **全链路自研**：不依赖框架，每个环节可深入讲解
2. **混合检索**：向量 + BM25 + RRF，对标生产标准
3. **量化评估**：有具体指标证明系统效果
4. **Agent 集成**：RAG 作为 Tool 接入 Agent 循环
5. **Go 实现**：对齐字节技术栈
6. **生产意识**：流式输出、缓存、重试、可观测
