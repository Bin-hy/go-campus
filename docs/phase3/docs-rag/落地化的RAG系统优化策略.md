# 检索器（Retriever）优化

## 用户查询理解与改写
> 目的： 让检索更贴合用户真实意图，尤其在口语化、模糊或复杂问题下召回质量。

- **关键词提取/扩展**
 - 使用 TF-IDF、MB25或更小型模型(如 spaCy、KeyBEART) 提取核心词。
 - 利用同义词库（WordNet）、领域术语表进行语义扩展。

- **查询重写（Query Rewriting）**
 - 用LLM将原市问题改写为更规范、完整、含潜在关键词的形式

- 多角度检索（Multi-query Retrieval）
 - LLM生成多个视角的文通。分别检索后融合结果

- 上下文感知（Context-aware Rewriting）
 - 在多伦对话中，结合历史对话重写当前问题，避免指代不清

- Step-Back Prompting
 - 让LLM先生成一个更高层次的抽象问题，用于检索更通用的知识。


## 嵌入模型（Embedding Model） 优化

- **微调嵌入模型**
 - 在特定领域语料上微调开源模型（如bge、text2vec、e5），提升语义匹配精度

- **模型选型**
 - 对比不同模型在领域数据上的MRR、Recall@K 表示。
 - 考虑多语言支持、模型大小（768d vs 1024d）、推理速度。

- **池化策略**
 - 尝试 Mean Pooling， CLS Token、 Last Token等方式聚合 token embeddings，影响最终向量质量。


## 文档分块与索引优化
### 分块策略（Chunking）

- 块大小：实验 128 / 256 / 512 tokens，平衡上下文完整性与噪声。

- 分块方法：
 - 固定滑动窗口（带重叠）
 - 按自然段落/句子边界分割
 - 语义分割（使用LLM 或 NLP 工具识别主题边界）
 - 结构化分块： 利用文档标题、章节、表格等元结构

- 重叠设计：10～20% 重叠防止关键信息呗阶段。

- 多粒度索引：同时索引句子级、段落级、小节级内容，按需检索。

### 元数据增强

- 为每个chunk 添加丰富元数据：文档ID、标题、章节、作者、日期、实体标签等。
- 检索时可基于元数据过滤（如”只查2023年后的政策文件“），或加权（如“优先返回权威来源”）

### 向量索引选型与调参

- 选型： FASISS（本地）、Annoy、HNSW（高效近邻）、Pinecone / Weaviate / Milvus / Qdrant（云原生向量数据库）

- 参数调优：
 - FAISS： nlist， nprobe
 - HNSW： M， efConstruction， efSearch
 - 目标： 在 **召回率（Recall）**与 **查询延迟** 延迟间取得平衡


## 检索算法优化
- 相似度度量: Cosine、 Dot Product（归一化向量）、 Euclidean（较少用）
- Top-K设置： 通常 3-10， 过大引入噪声，过小漏检关键信息。

- 混合检索（Hybrid Search）：
 - RRF： 融合关键词与相连检索的排序结果
 - 两阶段检索：先用BM25粗略，再向量精排；或反之。
 - 重排序（Re-ranker）：用 Cross-Encoder（如 bge-reranker）对初检结果重新打分。

- 多向量表示：
 - 单文档生成多个向量（标题、摘要、关键句），分别检索后融合。
 - 提升对长文档的覆盖能力。

# 生成器（Generator）优化
## 