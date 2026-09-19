# BinRag 检索与向量存储链路 · 深度技术档案（只读源码分析）

> 分析对象：`/Users/binhy/Binhy-Projects/GoCampus/projects/docs-rag`
> 分析方式：**只读**源码逐函数阅读；分析阶段未修改、未创建任何文件（未运行 build/test，避免任何写入）。本档案为分析结论的落盘副本。
> 所有结论均标注 `文件:行号`；凡代码中未找到的，一律显式写「代码中未找到」，不臆造参数、常量或性能数字。

---

## 0. 阅读范围与文件清单

| 文件 | 行数 | 是否精读 |
|---|---|---|
| `internal/retriever/retriever.go` | 373 | ✅ 全文 |
| `internal/retriever/bm25.go` | 237 | ✅ 全文 |
| `internal/retriever/rrf.go` | 106 | ✅ 全文 |
| `internal/retriever/tokenizer.go` | 181 | ✅ 全文 |
| `internal/retriever/types.go` | 56 | ✅ 全文 |
| `internal/retriever/retriever_test.go` | 667 | ✅ 全文 |
| `internal/retriever/tokenizer_test.go` | 82 | ✅ 全文 |
| `internal/vectorstore/qdrant.go` | 276 | ✅ 全文 |
| `internal/vectorstore/store.go` | 37 | ✅ 全文 |
| `internal/reranker/reranker.go` | 217 | ✅ 全文 |
| `internal/reranker/reranker_llm.go` | 248 | ✅ 全文 |
| `internal/reranker/reranker_test.go` | 141 | ✅ 全文 |
| `internal/reranker/reranker_llm_test.go` | 213 | ✅ 全文 |
| `internal/embedding/embedder.go` | 184 | ✅ 全文 |
| `internal/embedding/embedder_test.go` | 195 | ✅ 全文 |

为讲清「参数从哪来、索引何时建、谁调用」，额外只读引用了装配/调用方文件：`internal/config/config.go`、`internal/config/manager.go`、`internal/pipeline/pipeline.go`、`internal/app/app.go`、`internal/app/rebuild.go`、`internal/rag/engine.go`、`internal/rag/routing.go`、`internal/rag/context.go`、`internal/rag/decompose.go`、`internal/api/handler_doc.go`、`internal/api/handler_chunk.go`、`internal/api/handler_config.go`、`internal/eval/evaluator.go`、`cmd/eval/main.go`、`docker-compose.yml`、`configs/config.yaml`、`configs/config.local.yaml`、`go.mod`，以及依赖库 `github.com/qdrant/go-client@v1.19.0`（用于确认库默认值，均标注为「库默认」）。

**一个关键上下文事实**：`internal/vectorstore/` 目录下**只有** `qdrant.go` 与 `store.go` —— **没有任何 `_test.go`**。因此第 2 节向量层结论全部来自实现阅读，无单测佐证（第 8、10 节会作为挑刺项列出）。

---

## 1. 模块职责与文件结构

### 1.1 分层总览

```
                    ┌─────────────────────────────┐
   rag.Engine ─────▶│ retriever.Retriever (接口)   │
                    │  Search / SearchMulti        │
                    │  SearchByVector / Rerank     │
                    └──────┬───────────┬───────────┘
                           │           │
              embedding.Embedder   ┌───┴────────────────────────┐
                           │     │                            │
                           ▼     ▼                            ▼
                 vectorstore.VectorStore              retriever.BM25Index
                 (qdrantStore, gRPC)                  (defaultBM25Index, 内存)
                           │                                  │
                           └────────► FuseRRF / FuseMultiQuery ◄┘
                                              │
                                              ▼
                                    reranker.Reranker (HTTP API)
```

接口定义位置：
- `retriever.Retriever` — `internal/retriever/retriever.go:16-26`
- `vectorstore.VectorStore` — `internal/vectorstore/store.go:27-37`
- `reranker.Reranker` — `internal/reranker/reranker.go:35-37`
- `embedding.Embedder` — `internal/embedding/embedder.go:17-19`
- `retriever.BM25Index` — `internal/retriever/bm25.go:15-27`
- `retriever.Tokenizer` — `internal/retriever/tokenizer.go:8-10`

### 1.2 各文件职责一句话

| 文件 | 职责 |
|---|---|
| `retriever.go` | 检索编排器：查询 embedding → 向量检索（并发）+ BM25（并发）→ RRF → rerank；多查询并行检索与跨路融合；Trace 埋点 |
| `bm25.go` | 纯自研内存倒排索引 + BM25 打分 + 按 kb 过滤；增删改查与 Rebuild |
| `rrf.go` | 两路加权 RRF（`FuseRRF`）+ 多路无权重 RRF（`FuseMultiQuery`） |
| `tokenizer.go` | 自研分词器：英文/数字按词小写化聚合，中文 bigram（可选 unigram），可选英文停用词 |
| `types.go` | 请求/结果/追踪数据结构 |
| `qdrant.go` | Qdrant gRPC 实现：集合创建、Upsert、Query、Delete、DeleteByFilter、Get、payload 双向转换、filter 构造 |
| `store.go` | 向量存储接口与 DTO（`VectorRecord` / `SearchRequest` / `SearchResult`） |
| `reranker.go` | Reranker 工厂 + api 模式（`/v1/rerank`）：限流、指数退避重试、index→候选映射 |
| `reranker_llm.go` | llm/ollama 模式：用 `chat/completions` 逐文档打分 0-10，正则解析 + 收敛 |
| `embedder.go` | OpenAI 兼容 `/v1/embeddings` 客户端：批切分、限流、重试、按 index 回填 |

### 1.3 关键类型与字段（原样）

```go
// internal/retriever/types.go:3-10
type RetrieveRequest struct {
	Query      string
	TopK       int
	Filter     map[string]any
	Trace      func(t RetrieveTrace) // 思考链路回调，nil=关闭（N2 零开销）
	SkipRerank bool                  // true=本检索不做 rerank（由调用方在融合/汇总后统一整体重排）
}
```

```go
// internal/retriever/types.go:38-49
type RetrieveResult struct {
	ID       string
	Content  string
	Score    float32
	Metadata map[string]any
}

// BM25Result BM25 检索结果
type BM25Result struct {
	ID    string
	Score float32
}
```

`BM25Result` **只有 ID 与 Score，没有 Content/Metadata** —— 这是第 4.6 节「BM25 独有命中正文为空」缺陷的类型级根源。

```go
// internal/retriever/bm25.go:29-44
type posting struct {
	docID string
	tf    int
}

type defaultBM25Index struct {
	mu        sync.RWMutex
	tokenizer Tokenizer
	inverted  map[string][]posting // term -> postings
	docLen    map[string]int       // docID -> token count
	docTerms  map[string][]string  // docID -> terms (用于 Remove)
	docKB     map[string]string    // docID -> kbID
	docChunks map[string][]string  // documentID -> chunkIDs（RemoveByDoc 用）
	totalLen  int
	avgLen    float64
}
```

`internal/vectorstore/store.go:5-24`：`VectorRecord{ID, Vector []float32, Payload map[string]any}`、`SearchRequest{Vector, TopK, Filter}`、`SearchResult{ID, Score float32, Payload}`。

`internal/reranker/reranker.go:18-32`：`RerankCandidate{ID, Content, Score, Metadata}` / `RerankResult{...}`（同构），以及 `apiReranker{config, client *http.Client, limiter *rate.Limiter}`（`reranker.go:39-43`）、`llmReranker{config, client, limiter, prompt}`（`reranker_llm.go:34-39`）。

线上协议（`internal/reranker/reranker.go:90-102`）：

```go
type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n"`
}

type rerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}
```

---

## 2. 向量检索细节（Qdrant）

### 2.1 gRPC 还是 HTTP？——gRPC

`internal/vectorstore/qdrant.go:8-10` 导入 `pb "github.com/qdrant/go-client/qdrant"`；`go.mod` 依赖 `github.com/qdrant/go-client v1.19.0`。

```go
// internal/vectorstore/qdrant.go:18-33
func NewQdrantStore(cfg config.VectorStoreConfig) (VectorStore, error) {
	host, port := parseHostPort(cfg.Host)

	client, err := pb.NewClient(&pb.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("连接 Qdrant 失败: %w", err)
	}

	return &qdrantStore{
		client: client,
		config: cfg,
	}, nil
}
```

```go
// internal/vectorstore/qdrant.go:268-276
func parseHostPort(host string) (string, int) {
	parts := strings.Split(host, ":")
	if len(parts) == 2 {
		port := 6334
		fmt.Sscanf(parts[1], "%d", &port)
		return parts[0], port
	}
	return host, 6334
}
```

**默认 6334 = Qdrant gRPC 端口**（REST 是 6333），与 `configs/config.yaml:24-25` 注释「Qdrant gRPC 地址 / host: "localhost:6334"」一致；`docker-compose.yml:44-49` 也显式设置 `QDRANT__SERVICE__GRPC_PORT=6334`。检索走新版 Query API `client.Query(ctx, &pb.QueryPoints{...})`（`qdrant.go:115`），不是老 `SearchPoints`，更不是 HTTP。

### 2.2 连接复用与未配置项

- `pb.NewClient` 只在 `NewQdrantStore` 调用一次，`qdrantStore` 长期持有 `client`（`qdrant.go:12-15`）→ **连接复用**（单 client 实例，随 `App` 生命周期）。
- 未配置项（源码只传 Host/Port）：**APIKey 未设置、UseTLS 未启用、无 GrpcOptions、无 RetryConfig**。库默认（`github.com/qdrant/go-client@v1.19.0/qdrant/config.go:29-68`）：`PoolSize` 默认 3（连接池 round-robin）、`KeepAliveTime` 默认 10s、`RetryConfig` 为 nil 时**不自动重试**、`APIKey` 默认 `""`、`UseTLS` 默认 false。
- **项目自身没有为 Qdrant 调用设置任何超时或重试**：`Search/Upsert/Delete/Get` 全部直接透传调用方 `ctx`（`qdrant.go:98-142`、`66-96`、`144-165`、`187-204`）。全仓库 `WithTimeout` 在 `internal/api|rag|app|task` 中仅 `app.go:284` 命中（优雅关闭时给审计 5s）→ **检索链路无超时兜底**：「超时与重试」在向量层属于「代码中未找到」，只有一个未启用的库能力。
- 关闭：`qdrant.go:206-208` 实现 `Close()`；但 `App.Close()`（`app.go:281-291`）只关 worker/audit/store，**没有调用 `vs.Close()`**（仅错误分支调用，如 `app.go:131`）。

### 2.3 集合命名/创建、维度、距离

```go
// internal/vectorstore/qdrant.go:35-64
func (s *qdrantStore) EnsureCollection(ctx context.Context) error {
	exists, err := s.client.CollectionExists(ctx, s.config.CollectionName)
	if err != nil {
		return fmt.Errorf("检查 Collection 失败: %w", err)
	}
	if exists {
		return nil
	}

	distance := pb.Distance_Cosine
	switch strings.ToLower(s.config.Distance) {
	case "euclid":
		distance = pb.Distance_Euclid
	case "dot":
		distance = pb.Distance_Dot
	}

	err = s.client.CreateCollection(ctx, &pb.CreateCollection{
		CollectionName: s.config.CollectionName,
		VectorsConfig: pb.NewVectorsConfig(&pb.VectorParams{
			Size:     uint64(s.config.Dimension),
			Distance: distance,
		}),
	})
	if err != nil {
		return fmt.Errorf("创建 Collection 失败: %w", err)
	}

	return nil
}
```

- **命名**：来自 `vectorstore.collection_name`，示例均为 `binrag`（`configs/config.yaml:27`、`configs/config.local.yaml:12`）。**单一集合**，不是「一库一集合」。
- **创建时机**：启动时 `vs.EnsureCollection(ctx)`（`internal/app/app.go:96`）；评测装配路径同样调用（`app.go:372`）。幂等：先 `CollectionExists`，存在即 return（`qdrant.go:36-42`）。
- **维度**：`vectorstore.dimension`，`<=0` 回退 `embedder.dimension`，后者默认 1536：

```go
// internal/config/config.go:390-398
	if c.Embedder.Dimension <= 0 {
		c.Embedder.Dimension = 1536
	}
	if c.VectorStore.Distance == "" {
		c.VectorStore.Distance = "cosine"
	}
	if c.VectorStore.Dimension <= 0 {
		c.VectorStore.Dimension = c.Embedder.Dimension
	}
```
  实际部署为 1024（`configs/config.local.yaml:6,13`，模型 `bge-m3`；`configs/config.local-copy.local.yaml:6,13` 为 `text-embedding-v4`/1024）。
- **距离**：默认 `Distance_Cosine`（`qdrant.go:44`）；`euclid`→`Euclid`、`dot`→`Dot`（`:45-50`）；部署均为 `cosine`（`config.local.yaml:14`）。
  ⚠️ **未知字符串静默落到 Cosine**（只有 `euclid`/`dot` 两个字面量被识别）。
- **未创建的优化项**：全仓库 grep `CreateFieldIndex|FieldIndex|Quantization|HnswConfig|OptimizersConfig` → **0 命中**：**没有为 `kb_id` 建 payload index**，未配 HNSW 参数、未开量化、未设 on_disk。

### 2.4 写入：批处理、payload 字段、维度校验

**批处理**：向量层**没有**分批逻辑，`Upsert` 把整个 `records` 一次性组装成 `points`，一次 RPC：

```go
// internal/vectorstore/qdrant.go:66-96（节选）
func (s *qdrantStore) Upsert(ctx context.Context, records []VectorRecord) error {
	if len(records) == 0 {
		return nil
	}

	points := make([]*pb.PointStruct, 0, len(records))
	for _, r := range records {
		payload := make(map[string]*pb.Value)
		for k, v := range r.Payload {
			payload[k] = toQdrantValue(v)
		}

		points = append(points, &pb.PointStruct{
			Id:      pb.NewID(r.ID),
			Vectors: pb.NewVectorsDense(r.Vector),
			Payload: payload,
		})
	}

	waitTrue := true
	_, err := s.client.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: s.config.CollectionName,
		Points:         points,
		Wait:           &waitTrue,
	})
	...
}
```
批大小实际由上游 Embedding 的 `batch_size`（默认 100）间接决定（`pipeline.go:105` 一次 Embed 全部 chunk，再一次 Upsert 全部）。`Wait: &waitTrue` = 同步等待（`qdrant.go:85-90`）。Point ID 为 UUID：`pb.NewID(uuid)`→`pb.NewIDUUID`（库 `oneof_factory.go:319-330`），与 `pipeline.go:118` 的 `uuid.New().String()`、`handler_chunk.go:29-31` 的 UUID 预校验一致。

**payload 字段（唯一写入点）**：

```go
// internal/pipeline/pipeline.go:120-138
		records[i] = vectorstore.VectorRecord{
			ID:     chunkID,
			Vector: vectors[i],
			Payload: map[string]any{
				"kb_id":           req.KBID,
				"document_id":     req.DocumentID,
				"chunk_id":        chunkID,
				"filename":        c.Metadata.DocFilename,
				"heading_context": c.Metadata.HeadingContext,
				"chunk_index":     c.Index,
				"content":         c.Content,
				"source_type":     c.Metadata.SourceType,
				"start_ms":        c.Metadata.StartMs,
				"end_ms":          c.Metadata.EndMs,
				"page_number":     c.Metadata.PageNumber,
				"heading":         c.Metadata.Heading,
				"anchor":          c.Metadata.Anchor,
			},
		}
```
共 **13 个字段**；类型：`start_ms`/`end_ms` 为 `int64`（`internal/chunker/types.go:56-57`）、`page_number` 为 `int`（`types.go:58`）、`chunk_index` 为 `int`，其余 string。转换见 `qdrant.go:233-248`：string/int/int64/float64/bool → 对应 `pb.Value`，**其他类型 fallback 成 `fmt.Sprintf("%v", val)` 字符串**；反序列化 `fromQdrantValue`（`:250-266`）**不支持 list/struct，返回 nil**。

**维度校验**：只校验数量：

```go
// internal/pipeline/pipeline.go:110-112
	if len(vectors) != len(chunks) {
		return nil, nil, fmt.Errorf("向量数量(%d)与 Chunk 数量(%d)不匹配", len(vectors), len(chunks))
	}
```
grep `Dimension` 全仓库非测试代码仅 `config.go:138/148/390-397` 与 `qdrant.go:55` → **代码中未找到「embedding 返回维度 == 配置/集合维度」的运行时校验**。

### 2.5 检索：Query 构造、topK、score 阈值

```go
// internal/vectorstore/qdrant.go:98-118
func (s *qdrantStore) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	topK := uint64(req.TopK)
	if topK == 0 {
		topK = 10
	}

	queryReq := &pb.QueryPoints{
		CollectionName: s.config.CollectionName,
		Query:          pb.NewQueryDense(req.Vector),
		Limit:          &topK,
		WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
	}

	if len(req.Filter) > 0 {
		queryReq.Filter = buildFilter(req.Filter)
	}

	scored, err := s.client.Query(ctx, queryReq)
```

- **topK**：直接用 `req.TopK`，`0`→兜底 10（`:99-102`）。⚠️ `uint64(req.TopK)` 在判空之前转换，**负数会变成极大值**（-1 → 18446744073709551615），`topK == 0` 分支永不触发。
- **score 阈值：代码中未找到**。`pb.QueryPoints` 未设 `ScoreThreshold`；全仓库 grep `Threshold|MinScore|score_threshold` 在检索链路 **0 命中**（唯二命中是多媒体场景检测 `config.go:83`、`scene_sampler.go:53`）。下游也不按分数过滤：`internal/rag/context.go:35-81` 的 `buildContext` 只按 `maxChunks` 与 token 预算截断。
- 未请求 `WithVector`（不回传向量，省带宽）；无分页/offset。
- 结果 ID 只认 UUID 分支：

```go
// internal/vectorstore/qdrant.go:127-132
		id := ""
		if sp.Id != nil {
			if uuid, ok := sp.Id.PointIdOptions.(*pb.PointId_Uuid); ok {
				id = uuid.Uuid
			}
		}
```
  若返回 `Num` 型 ID 会静默变 `""`（本项目恒写 UUID，实际不发生，但缺兜底）。

### 2.6 按 kb_id 过滤的具体写法（must / should）

```go
// internal/vectorstore/qdrant.go:210-231
func buildFilter(filter map[string]any) *pb.Filter {
	var conditions []*pb.Condition
	for key, val := range filter {
		switch v := val.(type) {
		case string:
			conditions = append(conditions, pb.NewMatchKeyword(key, v))
		case []string:
			// 多值匹配（如多知识库范围）：MatchKeywords = MatchAny(keywords...)
			if len(v) > 0 {
				conditions = append(conditions, pb.NewMatchKeywords(key, v...))
			}
		case int:
			conditions = append(conditions, pb.NewMatchInt(key, int64(v)))
		case int64:
			conditions = append(conditions, pb.NewMatchInt(key, v))
		}
	}
	if len(conditions) == 0 {
		return nil
	}
	return &pb.Filter{Must: conditions}
}
```

- **只用 `Must`，完全不用 `Should`/`MustNot`**。多个过滤键之间是 AND；多值 kb 是**单个 MatchKeywords 条件**（库 `conditions.go:76-84` → `Match_Keywords`，Qdrant 语义为「命中任一」，等价 MatchAny），所以多库是 OR、与其他键 AND。
- **支持类型只有 4 种**：`string`、`[]string`、`int`、`int64`。**`int32`/`float64`/`bool`/`[]any` 被静默忽略**（无 default 分支）——不报错也不过滤。
- 构造方：`internal/rag/engine.go:1044-1066`

```go
// internal/rag/engine.go:1058-1065
	switch len(ids) {
	case 0:
		return nil
	case 1:
		return map[string]any{"kb_id": ids[0]}
	default:
		return map[string]any{"kb_id": ids}
	}
```
  单库→`MatchKeyword`；多库→`MatchKeywords`（OR）；都不指定→`nil`（不调 `buildFilter`，「系统级不限」，`retriever.go:111-113`）。评测路径另一处构造：`internal/eval/evaluator.go:121-124`。

### 2.7 删除与按 ID 取回

- `Delete`：`pb.NewPointsSelector(pointIds...)`，`Wait=true`（`qdrant.go:144-165`），空 ids 直接 nil。
- `DeleteByFilter`：`pb.NewPointsSelectorFilter(buildFilter(filter))`，`Wait=true`（`:168-184`），用于入库重试补偿按 `document_id` 清理（`pipeline.go:66-73`）。
- `Get`：`client.Get` + `WithPayload`，不存在返回 `(nil,false,nil)`（`:187-204`），用于「引用来源查看原文」（`handler_chunk.go:33-48`，另有 UUID 预校验与文档访问权二次校验）。

### 2.8 向量层事实速查表

| 维度 | 事实 | 依据 |
|---|---|---|
| 协议 | gRPC（go-client v1.19.0，Query API） | `qdrant.go:8-10,115`；`go.mod` |
| 默认端口 | 6334（gRPC） | `qdrant.go:268-276` |
| 集合数 | 1 个（`collection_name`） | `configs/config.yaml:27` |
| 创建 | 启动 `EnsureCollection`，幂等 | `qdrant.go:35-42`；`app.go:96,372` |
| 维度 | `vectorstore.dimension`，回退 `embedder.dimension`（默认 1536） | `config.go:390-398` |
| 距离 | 默认 Cosine；`euclid`/`dot` 可选；未知值静默 Cosine | `qdrant.go:44-50` |
| payload | 13 字段（见 2.4） | `pipeline.go:123-137` |
| kb 过滤 | `Filter.Must` + `MatchKeyword`（单）/`MatchKeywords`（多，OR） | `qdrant.go:210-231`；`engine.go:1048-1066` |
| score 阈值 | **代码中未找到** | grep 无命中 |
| 写入批大小 | 向量层不分批；由 embedder `batch_size`（默认 100）间接决定 | `qdrant.go:66-96`；`config.go:381-383` |
| 一致性 | Upsert/Delete `Wait=true` | `qdrant.go:85-90,154-159,173-178` |
| 超时 | **代码中未找到** | grep `WithTimeout` 无检索命中 |
| 重试 | **代码中未找到**（库 `RetryConfig` nil 即不重试） | `qdrant.go:21-24`；库 `config.go:61-64` |
| 连接复用 | 单 client 常驻，库默认池 3 | `qdrant.go:12-33`；库 `config.go:45-49` |
| payload 索引 | **未创建** | grep 0 命中 |
| 关闭 | `Close()` 存在，但 `App.Close()` 未调用 | `qdrant.go:206-208`；`app.go:281-291` |

---

## 3. BM25 细节

### 3.1 自研还是用库？——完全自研

`internal/retriever/bm25.go` 只 import `math`、`sort`、`sync`（`bm25.go:3-7`），无第三方 BM25/倒排库；`go.mod` 亦无 bleve/blugelabs 之类依赖。索引是手写 `map[string][]posting`（`bm25.go:37`）。

### 3.2 分词器（中文如何处理、有没有 jieba/词典、大小写/停用词）

自研 `simpleTokenizer`（`tokenizer.go:95-111`），**没有 jieba、没有词典、没有 HMM/CRF、没有词干化、没有中文停用词**。

规则（`tokenizer.go:113-181`）：
1. 分类 `classify`（`tokenizer.go:76-91`）：ASCII 字母/数字→`classAlnum`；`U+4E00–U+9FFF` 快速判定或 `unicode.Is(unicode.Han, r)`→`classHan`；其余→`classOther`（注释说明快路径是为避免每次查全局脚本表）。
2. 英文/数字：连续 alnum 聚合，**ASCII 位运算小写化**，再走可选停用词过滤：

```go
// internal/retriever/tokenizer.go:131-139
		for i, r := range enBuf {
			if r >= 'A' && r <= 'Z' {
				enBuf[i] = r + 32 // 'A'~'Z' 区间 +32 即转小写
			}
		}
		word := string(enBuf)
		if _, stopped := t.cfg.stopWords[word]; !stopped {
			tokens = append(tokens, word)
		}
```
3. 中文 bigram，长度 1 时输出单字，可选 unigram：

```go
// internal/retriever/tokenizer.go:144-162
	flushChinese := func() {
		n := len(zhBuf)
		switch {
		case n == 0:
			return
		case n == 1:
			tokens = append(tokens, string(zhBuf[0]))
		default:
			if t.cfg.cjkUnigram {
				for _, r := range zhBuf {
					tokens = append(tokens, string(r))
				}
			}
			for i := 0; i+1 < n; i++ {
				tokens = append(tokens, string(zhBuf[i:i+2]))
			}
		}
		zhBuf = zhBuf[:0]
	}
```
4. **unigram 默认关闭**（`tokenizer.go:23-28` 注释明确指出默认只输出 bigram，单字查询如「库」无法命中）；**生产装配已开启**：

```go
// internal/app/app.go:101
	bm25 := retriever.NewBM25Index(retriever.NewSimpleTokenizer(retriever.WithCJKUnigram()))
```
5. 英文停用词可选（`tokenizer.go:32-60`，内置 44 个），但**生产装配未启用**（`app.go:101` 只传 `WithCJKUnigram()`）→ 线上停用词 = 无。
6. 并发安全：`tokenizer.go:96` 注释「无内部可变共享状态，并发安全」。

### 3.3 k1 与 b

```go
// internal/retriever/bm25.go:9-12
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)
```
**硬编码常量，不可配置**（全仓库仅此定义 + `bm25.go:195-196` 使用）。经典默认值。

### 3.4 IDF 与文档长度归一化

```go
// internal/retriever/bm25.go:180-199
	for _, term := range tokens {
		postings, ok := idx.inverted[term]
		if !ok {
			continue
		}

		df := float64(len(postings))
		idf := math.Log((n-df+0.5)/(df+0.5) + 1)

		for _, p := range postings {
			// 知识库过滤（在锁内读取 docKB，安全）：空集合不过滤
			if len(allowed) > 0 && !allowed[idx.docKB[p.docID]] {
				continue
			}
			dl := float64(idx.docLen[p.docID])
			tfNorm := (float64(p.tf) * (bm25K1 + 1)) /
				(float64(p.tf) + bm25K1*(1-bm25B+bm25B*dl/idx.avgLen))
			scores[p.docID] += idf * tfNorm
		}
	}
```
- **IDF** = `log((N-df+0.5)/(df+0.5)+1)`，非负变体；`N=len(idx.docLen)`（`:168`），`df=len(postings)`（**全库 df，非 kb 内 df**）。
- **归一化**：分母 `tf + k1*(1-b+b*dl/avgLen)`；`avgLen=totalLen/文档数`（`:87`、`:139-143`）。
- **累加**：`scores[docID] += idf*tfNorm`；无 query term 加权、无同义词扩展。
- ⚠️ `avgLen` 除零**不会发生**：`n==0` 提前 return（`:168-171`），且空 token 文档不入索引（`:63-66`），故 `n>0 ⇒ totalLen>0 ⇒ avgLen>0`。这个不变式是自研实现必须能解释的点。
- 排序 `sort.Slice` 降序（`:206-208`，**非稳定**）；截断 `topK>0` 才截（`:210-212`，与向量层 `<=0→10` 语义不同）。

### 3.5 内存还是持久化？何时构建/更新/删除？

**纯内存，无任何持久化**（`:34-56` 全为 `make(map...)`，无序列化代码）。

| 操作 | 触发点 | 依据 |
|---|---|---|
| 创建 | 启动时 `NewBM25Index(NewSimpleTokenizer(WithCJKUnigram()))`，**空索引** | `app.go:101`（服务）、`app.go:377`（eval） |
| 增量添加 | 向量 Upsert 成功后逐 chunk `AddWithDocID` | `pipeline.go:145-152` |
| 重试补偿 | 每次 Ingest 先 `DeleteByFilter(document_id)` + `RemoveByDoc(documentID)` | `pipeline.go:65-73` |
| 删除文档 | 按 `doc.ChunkIDs` 逐个 `Remove` | `handler_doc.go:306-317` |
| Rebuild | **生产代码中未找到调用点**（仅 `retriever_test.go:115`） | grep `.Rebuild(` 仅测试命中 |
| 重启恢复 | **代码中未找到** | grep 无生产调用 |

`Rebuild` 本身语义正确（清空 + 逐条 Add，`:217-231`），但**没有任何启动回灌**→ 重启后 `DocCount()==0`，被门控跳过（见 3.9）。**这是全链路最严重的功能缺口**。

### 3.6 并发安全用什么锁

- `mu sync.RWMutex`（`bm25.go:35`）。
- 写：`AddWithDocID`/`Remove`/`RemoveByDoc`/`Rebuild` 清空段用 `Lock()`（`:73,98,105,218`）；读：`SearchFilteredByKBs`/`DocCount` 用 `RLock()`（`:165,234`）。
- `SearchFilteredByKBs` **先分词再加锁**（`:160-166`），纯 CPU 工作在锁外，缩短临界区。
- `AddWithDocID` 在锁内调 `removeLocked`（`:76-79`）保证同 ID 覆盖原子性。
- 真实并发来源：入库 worker 默认 2（`config.go:534-536`；`internal/task/worker.go:52`）。
- 潜在问题：读锁覆盖整个打分循环（O(Σpostings)），大库时写入延迟会被拖长。

### 3.7 内存占用如何控制

**代码中未找到任何上限、淘汰、TTL 或压缩**。两个「每文档副本」结构会放大占用：`docTerms[docID]=[]string{...}`（`:89-94`）、`docChunks[documentID]=append(...)`（`:83-85`）。开启 `WithCJKUnigram()` 后中文每字多一个 token（`tokenizer.go:152-156`），索引体积同步增大（注释 `tokenizer.go:23-25` 也承认）。**代码中无容量测试，故不给任何数字。**

两个真实小缺陷：
1. `removeLocked` 不清理 `docChunks`（对比 `:134-137` 只 delete 了 `docLen/docTerms/docKB`）→ `Remove(chunkID)` 后仍残留（`:114-144`），幂等无害但缓慢泄漏。
2. `Rebuild` 调 `idx.Add(doc.ID, doc.Content, doc.KBID)`（`:229`）→ `Add` 转 `AddWithDocID(..., docID="")`（`:58-60`）→ **`docChunks` 不被填充 ⇒ 一旦用过 Rebuild，`RemoveByDoc` 就再也删不掉东西**（映射丢失，`:83-85`）。

### 3.8 多知识库：共用索引还是一库一索引？

**单一全局索引 + 查询期按 kb 过滤**（一库一索引 = 否）：全进程只 `NewBM25Index` 两次，均在启动装配（`app.go:101`、`:377`），无按 kb 建索引的循环；归属记录在 `docKB`（`:82`，来源 `pipeline.go:149-150`）；检索时在打分循环内跳过非授权 kb：

```go
// internal/retriever/bm25.go:189-198
		for _, p := range postings {
			// 知识库过滤（在锁内读取 docKB，安全）：空集合不过滤
			if len(allowed) > 0 && !allowed[idx.docKB[p.docID]] {
				continue
			}
			dl := float64(idx.docLen[p.docID])
			tfNorm := (float64(p.tf) * (bm25K1 + 1)) /
				(float64(p.tf) + bm25K1*(1-bm25B+bm25B*dl/idx.avgLen))
			scores[p.docID] += idf * tfNorm
		}
```
⚠️ `df` 用**全库** postings（`:186`）→ 多租户下 A 库 IDF 受 B 库分布影响（跨库统计影响打分，但不跨库返回结果）。代码中未找到修正。
⚠️ **kb 过滤只认 `kb_id` 键**：

```go
// internal/retriever/retriever.go:144-160（节选）
func filterKBIDs(filter map[string]any) []string {
	raw, ok := filter["kb_id"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	default:
		return nil
	}
}
```
  → 若上层传 `{"document_id": ...}` 或 `{"source_type": ...}`，**向量路过滤、BM25 路不过滤**，两路范围不一致（当前上层只传 kb_id，属潜在风险）。

### 3.9 BM25 的启停门控

```go
// internal/retriever/retriever.go:102-110
	// BM25 检索（可选，按 kb_id 集合过滤）
	kbIDs := filterKBIDs(filter)
	if r.config.EnableBM25 && r.bm25Index != nil && r.bm25Index.DocCount() > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bm25Results = r.bm25Index.SearchFilteredByKBs(query, topK, kbIDs)
		}()
	}
```
三重门控：配置开关、索引非 nil、**索引非空**。语义上是优雅降级，副作用是**重启后（索引空）静默退化为纯向量**，`method` 变 `"vector"`（`:136-139`），**无独立告警日志**。

---

## 4. RRF 融合细节

### 4.1 公式与 k 实际取值

```go
// internal/retriever/rrf.go:12-28
func FuseRRF(vectorResults []RetrieveResult, bm25Results []BM25Result, allDocs map[string]RetrieveResult, cfg RRFConfig) []RetrieveResult {
	if cfg.K <= 0 {
		cfg.K = 60
	}

	scores := make(map[string]float64)

	// 向量检索结果按 rank 贡献分数
	for rank, r := range vectorResults {
		scores[r.ID] += float64(cfg.VectorWeight) / float64(cfg.K+rank+1)
	}

	// BM25 检索结果按 rank 贡献分数
	for rank, r := range bm25Results {
		scores[r.ID] += float64(cfg.BM25Weight) / float64(cfg.K+rank+1)
	}
```
- **公式**：`score(d) = Σ weight_route / (K + rank + 1)`（rank 从 0 起）。
- **K**：由配置 `r.config.RRFK` 传入（`retriever.go:128-132`），默认 **60**（`config.go:412-413`），部署 yaml 亦为 60（`config.local.yaml:65`）；函数内再兜底 `<=0→60`（`rrf.go:14-16`）。
- **权重**：默认 **0.7 / 0.3**（`config.go:415-420`），部署一致（`config.local.yaml:66-67`）。**来源 = 配置**；`ValidateConfig` 强制二者之和为 1（容差 0.001）：

```go
// internal/config/manager.go:116-125
	// 融合权重非负
	if cfg.Retriever.VectorWeight < 0 || cfg.Retriever.BM25Weight < 0 {
		return fmt.Errorf("融合权重不能为负: vector=%v bm25=%v", cfg.Retriever.VectorWeight, cfg.Retriever.BM25Weight)
	}
	// 算法要求：向量权重 + BM25 权重之和必须为 1（RRF 融合比例）
	const weightTolerance = 0.001
	if math.Abs(float64(cfg.Retriever.VectorWeight)+float64(cfg.Retriever.BM25Weight)-1.0) > weightTolerance {
		return fmt.Errorf("算法要求向量权重 + BM25 权重之和必须为 1，当前 vector=%v + bm25=%v = %v",
			cfg.Retriever.VectorWeight, cfg.Retriever.BM25Weight, cfg.Retriever.VectorWeight+cfg.Retriever.BM25Weight)
	}
```
  ⚠️ 该校验只在**热更新**路径执行（`manager.go:45-55`）；`config.Load` 只 `applyDefaults()`（`config.go:360`），**不调用 `ValidateConfig`**。

### 4.2 两路如何合并、去重、chunk_id 对齐

- **对齐键 = ID = chunk_id（UUID）**，两路同源：向量点 ID = chunkID（`pipeline.go:118-121`），BM25 docID = 同一 chunkID（`pipeline.go:150` 传 `rec.ID`）→ 无歧义。
- **去重/合并**：`scores map[string]float64` 天然去重，命中两路则累加（`rrf.go:18-28`）。
- **内容回填缺陷**见 4.6。

### 4.3 多路融合：无权重，k 硬编码 60

```go
// internal/retriever/rrf.go:65-79（节选）
func FuseMultiQuery(listOfResults [][]RetrieveResult, k, topK int) []RetrieveResult {
	if k <= 0 {
		k = 60
	}
	if topK <= 0 {
		topK = len(listOfResults)
	}

	scores := make(map[string]float64)
	// 每路结果按 rank 贡献分数
	for _, results := range listOfResults {
		for rank, r := range results {
			scores[r.ID] += 1.0 / float64(k+rank+1)
		}
	}
```
- **多路无权重**（固定 1.0），与加权版是两套逻辑（`rrf.go:22,27` vs `:77`）。
- **k 由调用方传常量 60，不走配置**：`retriever.go:351`、`routing.go:124`（HyDE）→ `RRFK` 只影响两路 RRF，多路/HyDE 的 60 硬编码。
- 多路会遍历各列表用 `findInVector` 取第一个命中的完整文档（`rrf.go:82-96`），故多路场景正文通常完整（HyDE 两路 payload 齐全）。

### 4.4 排序稳定性

```go
// internal/retriever/rrf.go:46-48
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
```
（`FuseMultiQuery` 同：`rrf.go:98-100`）

输入切片由**遍历 map 构建**（`rrf.go:32-44`、`:83-96`），配合**非稳定排序** ⇒ **同分名次在多次请求间不确定**。多路融合中极易同分（各路同 rank 的不同 chunk 分数都是 `1/61`）。代码中未找到 tie-break ⇒ **多查询融合顺序不可复现**（影响评测可比性）。

### 4.5 召回候选数 vs 最终 topK

```go
// internal/retriever/retriever.go:53-83（节选）
	topK := req.TopK
	if topK <= 0 {
		topK = r.config.TopK
	}

	fusedResults, method, err := r.searchFused(ctx, req.Query, topK, req.Filter)
	...
	if len(fusedResults) > topK {
		fusedResults = fusedResults[:topK]
	}

	// Reranker（可选）：单路场景路内重排；SkipRerank 时由调用方在融合/汇总后统一重排
	if !req.SkipRerank {
		fusedResults = r.rerankIfEnabled(ctx, req.Query, fusedResults, topK, req.Trace)
	}
```
- 两路都用**同一个 topK**（`retriever.go:99,108`），融合后**先截断到 topK**（`:73-75`），**然后**才 rerank（`:78-80`）。
- ⇒ **rerank 候选池 = 已截断的 topK**，**无 oversample**（没有「召回 50、重排取 10」的漏斗），reranker 无法捞回向量排 30 名的文档。
- 实际数值：`Retriever.TopK` 默认 10（`config.go:409-411`），部署写 **2**（`config.local.yaml:64`）；但上层传 `ragCfg.TopK`（RAG 默认 5，`config.go:464-466`）→ **RAG 链路实际生效的是 `rag.top_k`，`retriever.top_k` 只作兜底**。`Retriever.TopK` 有 [1,50] 校验（`manager.go:112-115`），**`RAG.TopK` 无校验**。
- 多路：每路各取 topK（`:314`）→ 融合再取 topK（`:347-351`）→ 统一 rerank 一次（`:365`）。
- 分解/Step-Back：`decompose.go:223,310` 用 `MaxChunks`（默认 5）作 topN。

### 4.6 ⚠️ BM25 独有命中的 Content/Metadata 为空（真实缺陷）

`allDocs` **只由向量结果构建**：

```go
// internal/retriever/retriever.go:118-133
	// 如果没有 BM25 结果，直接用向量结果
	var fusedResults []RetrieveResult
	if len(bm25Results) == 0 {
		fusedResults = vectorResults
	} else {
		allDocs := make(map[string]RetrieveResult, len(vectorResults))
		for _, vr := range vectorResults {
			allDocs[vr.ID] = vr
		}

		rrfCfg := RRFConfig{
			K:            r.config.RRFK,
			VectorWeight: r.config.VectorWeight,
			BM25Weight:   r.config.BM25Weight,
		}
		fusedResults = FuseRRF(vectorResults, bm25Results, allDocs, rrfCfg)
	}
```
`FuseRRF` 回填顺序「向量列表 → allDocs → 只留 ID」：

```go
// internal/retriever/rrf.go:30-44
	// 构建融合结果
	results := make([]RetrieveResult, 0, len(scores))
	for id, score := range scores {
		var result RetrieveResult
		// 优先从向量结果中获取完整信息
		if doc, ok := findInVector(vectorResults, id); ok {
			result = doc
		} else if doc, ok := allDocs[id]; ok {
			result = doc
		} else {
			result = RetrieveResult{ID: id}
		}
		result.Score = float32(score)
		results = append(results, result)
	}
```
**后果**：凡「关键词命中但向量 Top-K 未召回」的 chunk，进入融合结果时 `Content == ""`、`Metadata == nil`，会被计入 topK、挤掉有正文的候选，并在 `buildContext` 生成**空内容条目**（`internal/rag/context.go:52-57`；`metaString(nil,...)` 返回 `""`，`context.go:116-119`）→ 污染 token 预算、引用来源缺文件名。检索链路中**代码中未找到任何 hydration 步骤**（`VectorStore.Get` 唯一非测试调用点是 `handler_chunk.go:33`）。
测试**没有断言该行为**：`retriever_test.go:305` 特意加了 BM25 独有文档 `bm25.Add("b1", "BM25 独有结果", "")`，但断言只有 `len(results) == 0` 的反向判断（`:323-325`）——只验证「不空」，不验证内容完整性。

---

## 5. 重排序细节

### 5.1 API 调用还是本地模型？——远程 HTTP API

```go
// internal/reranker/reranker.go:54-71
// NewReranker 创建 Reranker
// mode=api（默认）：调用 /v1/rerank 专用重排接口（Jina/Cohere/vLLM 等）
// mode=llm / ollama：使用通用大模型 chat/completions 打分重排（无专用 rerank 端点的场景）
func NewReranker(cfg config.RerankerConfig) Reranker {
	switch strings.ToLower(cfg.Mode) {
	case "llm", "ollama":
		return NewLLMReranker(cfg)
	case "", "api":
		// 默认 api 模式
	default:
		slog.Warn("未知重排模式，回退 api 模式", "mode", cfg.Mode)
	}
	return &apiReranker{
		config:  cfg,
		client:  &http.Client{Timeout: 30 * time.Second},
		limiter: newLimiter(cfg.QPS),
	}
}
```
- 无本地模型加载（无 ONNX/torch 依赖）。「本地」只能把 `base_url` 指向本地服务：`configs/config.local.yaml:71-77` 正是 `http://localhost:11434` + `api_key: ollama` + `model: qwen2.5:7b`。
- HTTP 超时固定 **30s**（`reranker.go:68`、`reranker_llm.go:49`）。

### 5.2 请求/响应格式（api 模式）

- URL：`strings.TrimRight(BaseURL,"/") + "/v1/rerank"`（`reranker.go:149`）；Header：JSON + 可选 `Authorization: Bearer`（`:155-158`）。
- 请求体 `{model, query, documents:[全部候选 Content], top_n}`（`:90-95,137-142`）；响应 `{results:[{index,relevance_score}]}`（`:97-102`）。
- 映射：按 `relevance_score` 降序 + 用 `index` 回填：

```go
// internal/reranker/reranker.go:186-201
	sort.Slice(rrResp.Results, func(i, j int) bool {
		return rrResp.Results[i].RelevanceScore > rrResp.Results[j].RelevanceScore
	})

	results := make([]RerankResult, 0, len(rrResp.Results))
	for _, rr := range rrResp.Results {
		if rr.Index < len(candidates) {
			c := candidates[rr.Index]
			results = append(results, RerankResult{
				ID:       c.ID,
				Content:  c.Content,
				Score:    float32(rr.RelevanceScore),
				Metadata: c.Metadata,
			})
		}
	}
```
  ⚠️ 只判上界，**未判 `rr.Index >= 0`**：上游返回 `index:-1` 时 `candidates[-1]` 会 **panic**。
  ⚠️ `top_n` 只**转发**给远端，本地**不按 topN 截断** api 模式结果（llm 模式才截断，`reranker_llm.go:122-124`）→ 两模式行为不一致；远端返回条数少于候选数时也不报错、不补齐。

### 5.3 批量与截断长度

- **批量**：一次请求发送**全部候选**（`reranker.go:132-142`），不切子批、不并发。
- **截断长度：代码中未找到**（无 token/字符截断逻辑；`documents[i] = c.Content` 原样发送）。正文长度实际由 chunker `chunk_size`（默认 512，`config.go:399-401`）约束。
- **llm 模式输出截断**：`MaxTokens: 32`（`reranker_llm.go:164`），只约束打分输出，不截断输入。

### 5.4 超时/重试/失败降级

```go
// internal/reranker/reranker.go:104-129
func (r *apiReranker) doRerank(ctx context.Context, query string, candidates []RerankCandidate, topN int) ([]RerankResult, error) {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		results, err := r.doRequest(ctx, query, candidates, topN)
		if err == nil {
			return results, nil
		}

		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("重试 %d 次后仍失败: %w", r.config.MaxRetries, lastErr)
}
```
- 可重试（`:160-179`）：网络错误、HTTP 429、HTTP ≥500；**其他非 200（400/401/404）立即失败**。
- `MaxRetries` 默认 3（`config.go:428-430`）→ 最多 4 次尝试；退避 1s/2s/4s。
- 限流 + 兜底：

```go
// internal/reranker/reranker.go:45-52
// newLimiter 创建 QPS 限流器；qps<=0（配置缺省）时使用默认值 10，
// 避免 rate.NewLimiter(0,0) 导致 Wait 无限阻塞
func newLimiter(qps int) *rate.Limiter {
	if qps <= 0 {
		qps = 10
	}
	return rate.NewLimiter(rate.Limit(qps), qps)
}
```
  （注释说「无限阻塞」，库实际行为是 `Wait` 返回错误 `rate: Wait(n=1) exceeds limiter's burst 0`，见 `x/time/rate@v0.15.0/rate.go:259-261`；方向一致，措辞不精确。）
- **reranker 挂了会怎样**：

```go
// internal/retriever/retriever.go:164-183（节选）
func (r *defaultRetriever) rerankIfEnabled(ctx context.Context, query string, results []RetrieveResult, topN int, trace func(RetrieveTrace)) []RetrieveResult {
	if !r.config.EnableReranker || r.reranker == nil || len(results) == 0 {
		return results
	}
	...
	reranked, err := r.reranker.Rerank(ctx, query, candidates, topN)
	if err != nil {
		slog.Warn("重排失败，降级返回原结果", "query", query, "err", err)
		return results
	}
```
  全有或全无：一次超时/500 丢掉整批重排，无部分结果。
  ⚠️ 降级后 `Score` 仍是 **RRF 分**（≈0.7/(61) ≈ 0.01 量级），成功重排后是 **relevance_score**（0~1）→ **同一字段语义随 reranker 可用性跳变**。

### 5.5 llm/ollama 模式细节

```go
// internal/reranker/reranker_llm.go:96-116（节选）
	results := make([]RerankResult, 0, len(candidates))
	for i, c := range candidates {
		if err := r.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("限流等待失败: %w", err)
		}

		prompt := strings.ReplaceAll(r.prompt, "{query}", query)
		prompt = strings.ReplaceAll(prompt, "{document}", c.Content)

		score, err := r.scoreDocument(ctx, prompt)
		if err != nil {
			return nil, fmt.Errorf("文档打分失败(候选 %d): %w", i, err)
		}
```
- **逐文档串行打分**（N 个候选 = N 次 LLM 调用，重试更多），**任一失败即整体失败**（无部分结果）→ retriever 降级为不重排。
- prompt 内置中文模板（`reranker_llm.go:20-29`），可被 `llm_prompt_template` 覆盖（`:42-46`），占位符 `{query}`/`{document}`（`:102-103`）。
- 请求体 `{model, messages, temperature, max_tokens:32}`（`:56-66,157-165`）；`temperature` = `LLMTemperature`，Go 零值即 0（`config.go:128-129` 注释「默认 0」，`applyDefaults` 未动该字段）。
- 解析：三级正则 + 收敛：

```go
// internal/reranker/reranker_llm.go:77-84
var (
	// scoreAfterLabelPattern 匹配 "分数：8" 这类带标签的分数
	scoreAfterLabelPattern = regexp.MustCompile(`分数[：:]\s*(-?\d+(?:\.\d+)?)`)
	// scoreWithUnitPattern 匹配 "8分" 这类带单位结尾的分数（无负号，避免 "0-10" 中的 "-10" 被误捕获）
	scoreWithUnitPattern = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*分`)
	// scoreTrailingPattern 匹配文本末尾的数字（避免命中 "0-10 分制" 等说明文字中的数字）
	scoreTrailingPattern = regexp.MustCompile(`(-?\d+(?:\.\d+)?)[^0-9]*$`)
)
```
  优先级：标签 →「N分」取最后一个（`:222-226`）→ 文末数字；无法解析→0；`clampScore` 收敛 [0,10]（`:236-248`）。

### 5.6 与 RRF 结果的衔接

| 路径 | 候选来源 | 传入 topN | 返回条数 | 依据 |
|---|---|---|---|---|
| `Search`（单路） | RRF 后**已截断至 topK** | `topK` | api：远端决定（本地不截断）；llm：`min(N, topN)` | `retriever.go:73-80`；`reranker.go:190-203`；`reranker_llm.go:122-124` |
| `SearchMulti` | 跨路 RRF 后（已按 topK 截断） | `topK` | 同上，**整体重排恰好 1 次** | `retriever.go:347-365` |
| `SearchByVector`（HyDE 一路） | **不重排** | — | — | `retriever.go:270-277` |
| HyDE 整体 | HyDE 路 + 原查询路融合后 | `e.cfg.TopK` | 同上 | `routing.go:124-129` |
| 分解/Step-Back | 子查询汇总后 | `MaxChunks`（默认 5） | 同上 | `decompose.go:223,310` |
| `SkipRerank=true` | 调用方统一重排 | — | — | `retriever.go:77-80`；`decompose.go:120`；`routing.go:111` |

`config.Reranker.TopN`（默认 5，`config.go:425-427`）在上述所有路径都**不会被用到**（调用方总传 >0），只在 `topN<=0` 兜底（`reranker.go:78-81`、`reranker_llm.go:91-94`）→ **死配置**（仍在配置 API/前端暴露，`handler_config.go:51,248-249`）。

---

## 6. Embedding 细节

```go
// internal/embedding/embedder.go:27-40
func NewEmbedder(cfg config.EmbedderConfig) (Embedder, error) {
	switch strings.ToLower(cfg.Provider) {
	case "", "openai":
		// 默认 / openai 兼容实现
	default:
		return nil, fmt.Errorf("未知 embedding provider: %s", cfg.Provider)
	}
	return &openaiEmbedder{
		config:  cfg,
		client:  &http.Client{Timeout: 30 * time.Second},
		limiter: rate.NewLimiter(rate.Limit(cfg.QPS), cfg.QPS),
	}, nil
}
```
- 仅 OpenAI 兼容 `/v1/embeddings`（`:121`）；未知 provider **直接报错**，不静默回退。超时 **30s**（`:37`），无 per-request ctx 超时兜底。

### 6.1 批量大小与并发

```go
// internal/embedding/embedder.go:42-69
func (e *openaiEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var allVectors [][]float32

	for i := 0; i < len(texts); i += e.config.BatchSize {
		end := i + e.config.BatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		if err := e.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("限流等待失败: %w", err)
		}

		vectors, err := e.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("第 %d-%d 条 Embedding 失败: %w", i, end, err)
		}

		allVectors = append(allVectors, vectors...)
	}

	return allVectors, nil
}
```
- 批大小 `batch_size` 默认 **100**（`config.go:381-383`），部署写 **10**（`config.local.yaml:7`）。
- **并发：无**（串行 for，不 pipeline）；整批失败即整次失败（错误带批次下标）。
- ⚠️ **`BatchSize <= 0` 死循环**：`i += 0` 且 `batch = texts[i:i]` 恒空 → `embedBatch` 请求空 input → 返回空 → i 不变 → **无限循环**。`NewEmbedder` 无兜底（对比 reranker 有 `newLimiter`、qdrant 有 `topK==0→10`）。经 `config.Load` 有 `applyDefaults` 保护，但**热更新路径不调 `applyDefaults`**（`manager.go:45-73` 只 `ValidateConfig`，后者不查 `BatchSize`）；当前配置 API 的 Embedder 白名单只允许改 `Model`（`handler_config.go:230-232`）→ 现网不可触发，属脆弱点。

### 6.2 失败重试

```go
// internal/embedding/embedder.go:71-96
func (e *openaiEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var lastErr error

	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		vectors, err := e.doRequest(ctx, texts)
		if err == nil {
			return vectors, nil
		}

		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("重试 %d 次后仍失败: %w", e.config.MaxRetries, lastErr)
}
```
- `MaxRetries` 默认 3（`config.go:384-386`），退避 1s/2s/4s；可重试 = 网络/429/≥500（`:132-147`）；其他 4xx 立即失败。
- `RetryableError` 带 `Unwrap`（`:177-179`），是三个客户端里唯一实现 `Unwrap` 的（reranker 的 `retryableError` 没有，`reranker.go:206-212`）。

### 6.3 归一化 / 维度校验 / 缓存

- **归一化：代码中未找到**（无 `normalize`/`Norm`），向量原样入库与查询，相似度语义完全依赖 `Distance_Cosine`。
- **维度校验：代码中未找到**（见 2.4）；响应回填按 index 松散对齐：

```go
// internal/embedding/embedder.go:158-165
	vectors := make([][]float32, len(texts))
	for _, d := range embResp.Data {
		if d.Index < len(vectors) {
			vectors[d.Index] = d.Embedding
		}
	}

	return vectors, nil
```
  → API 少返回条目/`index` 越界时，对应位置为 `nil`，**函数不报错**；`pipeline.go:110-112` 只比长度（相等即通过），随后 `pb.NewVectorsDense(nil)` 写入 Qdrant → **静默脏数据入口**。⚠️ 同样只判上界，未判 `d.Index >= 0`（负值 panic）。
- **缓存：代码中未找到**（三个包 grep `cache` 0 命中）；每次检索都真实调用一次 Embedding（`retriever.go:207`），HyDE 再多一次（`routing.go:90`）。

### 6.4 Embedding 事实速查

| 项 | 事实 | 依据 |
|---|---|---|
| 批大小 | `batch_size`，默认 100，部署 10 | `config.go:381-383`；`config.local.yaml:7` |
| 并发 | 无（串行） | `embedder.go:49-66` |
| 重试 | 429/5xx/网络，退避 1s/2s/4s，默认 3 次 | `embedder.go:71-96,143-147`；`config.go:384-386` |
| 限流 | `NewLimiter(QPS, QPS)`，默认 10，**无 <=0 兜底** | `embedder.go:38`；`config.go:387-389` |
| 超时 | 30s，无 ctx 超时兜底 | `embedder.go:37` |
| 归一化 | 代码中未找到 | grep 无命中 |
| 维度校验 | 代码中未找到（仅数量校验） | `pipeline.go:110-112` |
| 缓存 | 代码中未找到 | grep 0 命中 |
| 空输入 | `(nil, nil)` | `embedder.go:43-45` |

---

## 7. 边界与异常：逐场景的代码行为

| # | 场景 | 实际行为 | 依据 |
|---|---|---|---|
| 1 | 空知识库 | Qdrant 返回空 → 融合结果空 → 返回 `[]`，不报错；上层 `buildContext` 得空 items | `retriever.go:118-121`；`qdrant.go:120-141` |
| 2 | 向量库不可用 | `vectorErr != nil` → **整体失败**（`向量检索失败`），**不降级为仅 BM25** | `retriever.go:114-116`；`engine.go:990-992` |
| 3 | BM25 未就绪/空（含重启） | 门控失败 → **静默退化为纯向量**，`method="vector"`，无告警 | `retriever.go:104,136-139` |
| 4 | BM25 空但配置开启 | 有专门单测断言退化为纯向量 | `retriever_test.go:356-381` |
| 5 | 超长 query | **代码中未找到任何长度上限/截断**；O(n) 分词 + O(Σpostings) 打分 + 直传远端 | `tokenizer.go:113-181`；`bm25.go:159-199`；`retriever.go:206-215` |
| 6 | 超大 topK | 配置值受 [1,50] 限制；**请求级无上限**（MCP `top_k` 自由、eval `KValues` 来自数据集）；向量层 `uint64()` 透传，**负值溢出成 1.8e19** | `manager.go:112-115`；`mcp/tools.go:53,285`；`qdrant.go:99-102` |
| 7 | 并发读写 BM25 | RWMutex；写锁/读锁分离；分词在锁外 | `bm25.go:35,73,98,105,165,218,234` |
| 8 | 并发入库同一文档 | 先按 document_id 清理再写；**无按 doc 互斥/幂等键**，理论可互删 | `pipeline.go:65-73`；`worker.go:52` |
| 9 | reranker 不可用 | 捕获 → warn → 返回 RRF 原序 | `retriever.go:179-183`；`retriever_test.go:383-408` |
| 10 | reranker 返回负 index | **panic 风险**（只判上界） | `reranker.go:192` |
| 11 | Embedding 少返回条目 | 不报错，留 `nil` 向量并入库 | `embedder.go:158-165`；`pipeline.go:110-112` |
| 12 | chunker 产出 0 chunk | Ingest 返回 `(nil, warnings, nil)`，视为成功 | `pipeline.go:94-97` |
| 13 | chunk 内容 tokenize 为空 | `AddWithDocID` 直接 return（**在 removeLocked 之前**）→ 不入索引；同 ID 旧版本**残留** | `bm25.go:62-66` |
| 14 | 多路部分失败 | 单路 warn 忽略；全败 → `多路检索全部失败`；全成功但全空 → `(nil,nil)` | `retriever.go:314-345` |
| 15 | SearchMulti 空查询列表 | 返回错误「多查询检索：查询列表为空」 | `retriever.go:282-284` |
| 16 | HyDE 各级失败 | 四级降级：渲染/生成/embedding/HyDE 检索失败 → 原查询；原查询路失败 → 只用 HyDE | `routing.go:68-116` |
| 17 | Get 不存在 chunk | `ok=false` → API 404 | `qdrant.go:196-198`；`handler_chunk.go:37-40` |
| 18 | 未知 distance 配置 | 静默 Cosine，不报错不日志 | `qdrant.go:44-50` |
| 19 | 未知 reranker mode | warn + 回退 api | `reranker.go:63-65` |
| 20 | QPS=0 的 Embedder | `Wait` 直接报错（`exceeds burst 0`）→ Embed 全失败（默认路径不会触发） | `embedder.go:38,56-58`；库 `rate.go:259-261` |
| 21 | ctx 取消/超时 | 三客户端都正确响应 ctx，但**无服务端主动超时** | `embedder.go:77-81`；`reranker.go:110-115`；`reranker_llm.go:136-141` |

---

## 8. 测试覆盖的证据（「我怎么验证的」）

统计：`retriever_test.go` 20 个 `Test*`（+4 个 `t.Run`）、`tokenizer_test.go` 6 个、`reranker_test.go` 3 个、`reranker_llm_test.go` 6 个、`embedder_test.go` 4 个；**`internal/vectorstore/` 0 个测试文件**。

### 8.1 `internal/retriever/retriever_test.go`

| 测试 | 行号 | 断言了什么（边界） |
|---|---|---|
| `TestSimpleTokenizerEnglish` | `:16-28` | `"Hello World"` → `["hello","world"]`（小写化 + 按词聚合） |
| `TestSimpleTokenizerChinese` | `:30-42` | `"向量数据库"` → 4 个 bigram（**默认无 unigram**） |
| `TestSimpleTokenizerMixed` | `:44-59` | `"使用Qdrant存储"` → `["使用","qdrant","存储"]` |
| `TestBM25Index` | `:63-88` | `DocCount()==3`；命中非空；**tf 更高者第一**；Score>0 |
| `TestBM25Remove` | `:90-105` | Remove 后 `DocCount()==1` 且检索不到已删文档 |
| `TestBM25Rebuild` | `:107-130` | Rebuild 后旧不可命中、新可命中、`DocCount()==2` |
| `TestBM25SearchFilteredByKB` | `:133-167` | **kb 隔离四态**：不过滤 3 条；kb-a 2 条且不含 kb-b；kb-b 仅 doc2；不存在 kb 0 条 |
| `TestFuseRRF` | `:171-204` | 去重为 3；**两路都命中者第一**；分数降序 |
| `TestFuseRRFWeightChange` | `:206-229` | 权重 0.9/0.1 ↔ 0.1/0.9 **必须改变第一名** |
| `TestRetrieverSearch` | `:296-326` | 全链路不报错、结果非空（**未断言内容完整性**） |
| `TestRetrieverDegradeNoReranker` | `:328-354` | BM25/reranker 关闭、reranker 传 nil → 1 条且 ID=`v1` |
| `TestRetrieverDegradeEmptyBM25` | `:356-381` | **BM25 空索引 → 退化为纯向量** |
| `TestRetrieverRerankFailDegrades` | `:383-408` | reranker error → `Search` 不报错且返回融合结果 |
| `TestFuseMultiQuery`（4 子测试） | `:411-461` | ① 交集文档第一 + **无重复 ID**；② 三路无交集 5 条；③ `topK=2`→2 条；④ Content/Metadata 保留 |
| `TestSearch_TraceCallback` | `:464-509` | 恰好 2 次 Trace；第 1 次 `Method=="hybrid"` 且 `Recalled>0`；第 2 次含前后列表；`Filename=="a.md"` |
| `TestSearch_TraceNil` | `:512-529` | Trace=nil 行为不变 |
| `TestSearchMulti_TracePerQueryOrder` | `:532-571` | 1 次融合回调；`Method=="multi_fusion"`；`PerQuery` 数量=3 且**顺序与 queries 一致** |
| `TestSearch_SkipRerank` | `:574-602` | `SkipRerank=true` → 0 次；默认 → **恰好 1 次**（`calls` 计数） |
| `TestSearchMulti_OverallRerank` | `:605-646` | 整体 rerank **恰好 1 次**；2 次 Trace |
| `TestSearchMulti_RerankFailDegrades` | `:649-667` | rerank 失败降级返回融合结果 |

Mock 基建：`mockEmbedder`（含**未被任何测试使用的 `delay` 字段**，`:233-247`）、`mockVectorStore`（`:249-272`）、`mockReranker`（`calls` 计数，`:274-294`）。

### 8.2 `tokenizer_test.go`

| 测试 | 行号 | 断言 |
|---|---|---|
| `TestTokenizerCJKUnigram` | `:9-22` | 开 unigram → `"数据库"`=`["数","据","库","数据","据库"]`（顺序：先 unigram 后 bigram） |
| `TestTokenizerCJKUnigramRecall` | `:25-36` | 单字查询 `"库"` 必命中文档 token 集合（召回实证） |
| `TestTokenizerStopWords` | `:39-51` | 停用词大小写不敏感：`"The cat IS cute"`→`["cat","cute"]` |
| `TestTokenizerEmpty` | `:54-62` | 空串 0 token；**纯标点空白 0 token** |
| `TestTokenizerExtHan` | `:65-72` | 扩展 B 区 `"𠀀文"` → 1 个 bigram（`unicode.Han` 兜底分支） |
| `TestTokenizerDigitMixed` | `:75-81` | `"BM25算法v2"`→`["bm25","算法","v2"]` |

### 8.3 `internal/reranker/*_test.go`（全部 `httptest`）

| 测试 | 行号 | 断言 |
|---|---|---|
| `TestRerankNormal` | `reranker_test.go:14-63` | 按 relevance_score 重排；分数精确 = 0.95 |
| `TestRerankRetry` | `:65-113` | 两次 429 → **恰好 3 次请求**后成功 |
| `TestRerankTimeout` | `:115-140` | ctx 100ms 超时 → **必须返回错误** |
| `TestLLMRerankerRerank` | `reranker_llm_test.go:26-90` | 路径 `/v1/chat/completions`、model/temperature=0/messages 长度；top2 只含 9 分文档 |
| `TestLLMRerankerCustomPrompt` | `:92-122` | 自定义模板替换生效；`"8"`→Score=8 |
| `TestLLMRerankerHTTPError` | `:124-147` | 404 报错且含 `HTTP 404`（非可重试） |
| `TestLLMRerankerZeroQPSSkipsLimiter` | `:149-173` | QPS 缺省 0 不卡死；Score=5 |
| `TestNewRerankerMode` | `:175-188` | `llm`/`OLLAMA`→`*llmReranker`；空/`api`→`*apiReranker` |
| `TestParseScore` | `:190-213` | 12 个 case：`"8"→8`、`"9分"→9`、`"7.5"→7.5`、`"分数：6"→6`、`"得分: 9.5"→9.5`、`"11"→10`、`"-1"→0`、`"在 0-10 分制中我给 8 分"→8`、`"我给 7 分，满分 10"→7`、无数字→0、空→0 |

### 8.4 `embedder_test.go`

| 测试 | 行号 | 断言 |
|---|---|---|
| `TestEmbedNormal` | `:15-69` | 10 条 → 10 个向量，**每个维度 = 4 = 配置 Dimension** |
| `TestEmbedBatchSplit` | `:71-116` | 100 条 + `BatchSize=20` → **HTTP 请求恰好 5 次** |
| `TestEmbedRetry` | `:118-168` | 两次 429 → **恰好 3 次尝试**后成功 |
| `TestEmbedTimeout` | `:170-194` | ctx 50ms 超时 → 返回错误 |

### 8.5 覆盖盲区（诚实清单）

1. **`internal/vectorstore` 零测试**：`buildFilter`/`toQdrantValue`/`fromQdrantValue`/`parseHostPort`/`EnsureCollection` 全无单测。
2. **`mockEmbedder.delay` 未被使用** → **「两路并行」无测试验证**；而 `docs/04-检索器与重排序/checklist.md` 却把「两路检索并行执行（通过 mock 延迟验证总耗时 < 两路之和）」列为验收项 → **文档承诺与测试实现不一致**（可主动承认的加分点）。
3. **BM25 并发 `-race` 测试缺失**：checklist 列了「并发 Add + Search 不 panic（-race 通过）」，但测试中无 `go`/`WaitGroup`/`t.Parallel`。
4. **BM25 独有命中正文完整性未断言**。
5. **`SearchByVector`/HyDE 融合路径无 retriever 层单测**（仅 rag 层覆盖「是否调用」）。
6. **`SkipRerank` 的上层组合语义**（`decompose.go:120`、`routing.go:111` + 汇总后 `Rerank`）只被 rag 层间接覆盖。
7. **无真实 Qdrant 的集成/契约测试**（filter 语义、payload 往返、维度不符报错均未验证）。
8. **无 benchmark / 容量基线**。

---

## 9. 面试官最可能深挖的 12 个点（问题 + 代码依据）

### Q1. 为什么 hybrid？BM25 什么时候会静默失效？
```go
// internal/retriever/retriever.go:102-110
	// BM25 检索（可选，按 kb_id 集合过滤）
	kbIDs := filterKBIDs(filter)
	if r.config.EnableBM25 && r.bm25Index != nil && r.bm25Index.DocCount() > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bm25Results = r.bm25Index.SearchFilteredByKBs(query, topK, kbIDs)
		}()
	}
```
门控含 `DocCount() > 0`（`bm25.go:233-237`）；内存索引无持久化、启动无回灌（`app.go:101` 空索引，`Rebuild` 仅测试调用 `retriever_test.go:115`）⇒ **重启后 BM25 整路失效、静默退化为纯向量**，仅 Trace 的 `Method`（`retriever.go:136-139`）能看出。

### Q2. RRF 的 k 与权重从哪来？合法性怎么保证？
```go
// internal/retriever/retriever.go:128-133
		rrfCfg := RRFConfig{
			K:            r.config.RRFK,
			VectorWeight: r.config.VectorWeight,
			BM25Weight:   r.config.BM25Weight,
		}
		fusedResults = FuseRRF(vectorResults, bm25Results, allDocs, rrfCfg)
```
默认 k=60、0.7/0.3（`config.go:412-420`），`rrf.go:14-16` 再兜底；权重和必须为 1 的校验在 `manager.go:120-125`，**只在热更新路径生效**（`config.Load` 只 `applyDefaults`，`config.go:360`）；多路融合的 60 是硬编码（`retriever.go:351`）。

### Q3. 只被 BM25 命中的 chunk 会怎样？
```go
// internal/retriever/rrf.go:33-44
		var result RetrieveResult
		// 优先从向量结果中获取完整信息
		if doc, ok := findInVector(vectorResults, id); ok {
			result = doc
		} else if doc, ok := allDocs[id]; ok {
			result = doc
		} else {
			result = RetrieveResult{ID: id}
		}
		result.Score = float32(score)
		results = append(results, result)
```
`allDocs` 只由向量结果构建（`retriever.go:123-126`）⇒ **Content 空、Metadata nil**，进入 `buildContext`（`context.go:52-57`）成空条目，无 hydration（`Get` 只在 `handler_chunk.go:33`）。

### Q4. 召回候选与最终 topK 是两个数吗？
```go
// internal/retriever/retriever.go:73-80
	if len(fusedResults) > topK {
		fusedResults = fusedResults[:topK]
	}

	// Reranker（可选）：单路场景路内重排；SkipRerank 时由调用方在融合/汇总后统一重排
	if !req.SkipRerank {
		fusedResults = r.rerankIfEnabled(ctx, req.Query, fusedResults, topK, req.Trace)
	}
```
候选池 = 已截断的 topK；两路同一 topK（`:99,108`）；无 oversample，rerank 只能重排不能捞回。

### Q5. 两路真并行吗？错误怎么传播？
```go
// internal/retriever/retriever.go:95-116（节选）
	wg.Add(1)
	go func() {
		defer wg.Done()
		vectorResults, vectorErr = r.vectorSearch(ctx, query, topK, filter)
	}()
	...
	wg.Wait()

	if vectorErr != nil {
		return nil, "", fmt.Errorf("向量检索失败: %w", vectorErr)
	}
```
并行成立；错误策略不对称：向量失败即整体失败（不降级 BM25-only），BM25 接口无 error；并行性**无测试证明**。

### Q6. 多路并发限流与取消语义？
```go
// internal/retriever/retriever.go:293-311
	errgroupCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, concurrency)
	...
			select {
			case sem <- struct{}{}:
			case <-errgroupCtx.Done():
				return
			}
			defer func() { <-sem }()
```
并发上限默认 3（`config.go:421-423`，函数内兜底 `:286-289`）；`errgroupCtx` 只在函数退出时取消——**单路失败不会提前取消其余路**（命名像 errgroup，语义却不是）。

### Q7. BM25 公式与「除零」问题
```go
// internal/retriever/bm25.go:186-196
		df := float64(len(postings))
		idf := math.Log((n-df+0.5)/(df+0.5) + 1)

		for _, p := range postings {
			// 知识库过滤（在锁内读取 docKB，安全）：空集合不过滤
			if len(allowed) > 0 && !allowed[idx.docKB[p.docID]] {
				continue
			}
			dl := float64(idx.docLen[p.docID])
			tfNorm := (float64(p.tf) * (bm25K1 + 1)) /
				(float64(p.tf) + bm25K1*(1-bm25B+bm25B*dl/idx.avgLen))
```
k1=1.2、b=0.75 硬编码（`:9-12`）；`avgLen` 不会为 0（空 token 不入索引 `:63-66` + `n==0` 提前 return `:168-171`）；`df` 是全库 df（多租户下 IDF 受跨库分布影响）。

### Q8. 删除路径的并发与一致性
```go
// internal/retriever/bm25.go:120-137（节选）
	terms := idx.docTerms[id]
	for _, term := range terms {
		postings := idx.inverted[term]
		for i, p := range postings {
			if p.docID == id {
				idx.inverted[term] = append(postings[:i], postings[i+1:]...)
				break
			}
		}
		if len(idx.inverted[term]) == 0 {
			delete(idx.inverted, term)
		}
	}

	idx.totalLen -= docLen
	delete(idx.docLen, id)
	delete(idx.docTerms, id)
	delete(idx.docKB, id)
```
写锁串行、term 空回收、`avgLen` 重算；两处可挑：**未清理 `docChunks`**（残留）、O(postings) 线性扫描 + 切片搬移。

### Q9. BM25 重启如何恢复？多库共用索引吗？
`app.go:101` 建空索引；`Rebuild`（`bm25.go:217-231`）生产无调用；`Rebuild` 走 `Add(...docID="")`（`:58-60`）导致 `docChunks`（`:83-85`）缺失、`RemoveByDoc` 失效。多库 = 单索引 + 查询期过滤（`:190-193`、`retriever.go:144-160`），且过滤只识别 `kb_id`。

### Q10. 向量层过滤如何下推？
```go
// internal/vectorstore/qdrant.go:210-231（节选）
		case string:
			conditions = append(conditions, pb.NewMatchKeyword(key, v))
		case []string:
			// 多值匹配（如多知识库范围）：MatchKeywords = MatchAny(keywords...)
			if len(v) > 0 {
				conditions = append(conditions, pb.NewMatchKeywords(key, v...))
			}
		case int:
			conditions = append(conditions, pb.NewMatchInt(key, int64(v)))
		case int64:
			conditions = append(conditions, pb.NewMatchInt(key, v))
		}
	...
	return &pb.Filter{Must: conditions}
```
只用 `Must`；多值 → `Match_Keywords`（OR，库 `conditions.go:76-84`）；**int32/float64/bool/[]any 静默忽略**；空 → 无 Filter（`retriever.go:111-113`）；**未建 kb_id payload index**（grep 0 命中）。

### Q11. reranker 的失败面与降级语义
```go
// internal/retriever/retriever.go:179-183
	reranked, err := r.reranker.Rerank(ctx, query, candidates, topN)
	if err != nil {
		slog.Warn("重排失败，降级返回原结果", "query", query, "err", err)
		return results
	}
```
api 模式：网络/429/5xx 重试（`reranker.go:160-179`），退避 1s/2s/4s、默认 3 次；4xx 立即失败 → 整批丢失；`index<0` 会 panic（`:192`）；api 模式本地不按 topN 截断；降级后 `Score` 从 relevance_score 变 RRF 分；`Reranker.TopN` 为死配置。

### Q12. Embedding 的批处理/维度/缓存
```go
// internal/embedding/embedder.go:49-66（节选）
	for i := 0; i < len(texts); i += e.config.BatchSize {
		end := i + e.config.BatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		if err := e.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("限流等待失败: %w", err)
		}
```
批次串行、默认批 100（部署 10）、`BatchSize<=0` 死循环、`QPS<=0` 使 `Wait` 报错（`embedder.go:38` 无兜底）；响应按 index 回填且**不校验条目数/负 index**（`:158-165`）→ nil 向量可入库；**无归一化、无维度校验、无缓存**。

---

## 10. 代码中明显缺失 / 可被挑刺的地方

### 10.1 功能性缺陷（有明确影响）

| # | 问题 | 依据 | 影响 | 改进方向 |
|---|---|---|---|---|
| 1 | **BM25 索引无持久化、重启无回灌**（`Rebuild` 仅测试调用） | `app.go:101`；`bm25.go:217-231`；grep 无生产命中 | 重启后 hybrid 静默退化为纯向量，召回质量降级；仅 `method` 字段变化是线索 | Postgres/Qdrant 回灌或倒排快照持久化；加「索引就绪」健康检查 |
| 2 | **BM25 独有命中正文/元数据为空** | `retriever.go:123-133` + `rrf.go:33-44` | 空上下文条目占 token 预算、引用缺文件名 | 融合后按 ID 批量 `Get`；或让 `BM25Result` 携带正文 |
| 3 | **过滤器类型不支持时静默失效** | `qdrant.go:213-225` 无 default | 传 `int32`/`bool` 过滤条件不生效，可能越范围返回 | fail-fast 报错或补类型 |
| 4 | **`index < 0` 未防御 → panic** | `reranker.go:192`；`embedder.go:160` | 畸形响应打崩请求 goroutine | 加 `>= 0` 下界判断 |
| 5 | **`uint64(TopK)` 前未判负 / 请求级 topK 无上限** | `qdrant.go:99-102`；`mcp/tools.go:53,285` | 负值溢出为超大 limit → 拉全库 | 入口 clamp（如 [1,200]） |
| 6 | **Embedding 响应条目数不校验 → nil 向量入库** | `embedder.go:158-165` + `pipeline.go:110-112` | 脏数据入向量库且难定位 | 校验条目数/index 唯一性/非空 |
| 7 | **无 score 阈值** | 检索链路 grep 0 命中；`context.go:35-81` | 弱相关 chunk 进上下文，稀释 prompt | 引入 `min_score`（Qdrant `ScoreThreshold` + 融合后相对阈值），用现有 eval 做 A/B |
| 8 | **无 payload 索引** | grep `CreateFieldIndex` 0 命中 | `kb_id` 过滤无索引加速 | 建 keyword index（建议 `is_tenant`） |
| 9 | **融合后 `Score` 语义被覆盖且随 reranker 可用性跳变** | `rrf.go:42` vs `reranker.go:197`；降级 `retriever.go:181-182` | 前端排序/展示/阈值不可靠 | 拆 `fusion_score`/`rerank_score` 或统一归一化 |

### 10.2 架构与工程性可挑刺项

| # | 问题 | 依据 |
|---|---|---|
| 10 | **召回无 oversample，rerank 池 = topK** | `retriever.go:73-80`；`routing.go:124-129`；`decompose.go:223` |
| 11 | **同分排序不确定（非稳定排序 + map 遍历）** | `rrf.go:31-48`、`83-100` |
| 12 | **`Reranker.TopN` 是死配置** | `config.go:425-427`；`handler_config.go:51,248-249` |
| 13 | **多路/HyDE 融合 k 硬编码 60，不走 `RRFK`** | `retriever.go:351`；`routing.go:124` |
| 14 | **`fusion` 策略声明但未接线** | `strategy.go:12,56`、`engine.go:519,664`（仅日志）vs `retriever.go:120-134` |
| 15 | **BM25 过滤只认 `kb_id`，与向量路能力不对等** | `retriever.go:144-160` vs `qdrant.go:210-231` |
| 16 | **`searchFused` 已建 `allDocs` 却仍线性 `findInVector`** | `rrf.go:35,53-60` |
| 17 | **`Rebuild` 丢失 `docChunks` → `RemoveByDoc` 失效** | `bm25.go:58-60,83-85,217-231` |
| 18 | **`removeLocked` 不清理 `docChunks`** | `bm25.go:114-144` |
| 19 | **空 token chunk 保留旧版本** | `bm25.go:62-66` |
| 20 | **`Rebuild` 非原子**（先解锁清空再逐条加锁） | `bm25.go:218-230` |
| 21 | **查询 embedding / rerank 无缓存** | 三包 grep `cache` 0 命中 |
| 22 | **检索链路无超时预算** | rag/api 层 `WithTimeout` 0 命中 |
| 23 | **`App.Close()` 不关 Qdrant client** | `app.go:281-291` vs `qdrant.go:206-208` |
| 24 | **`ConfigManager.Update` 不走 `applyDefaults`** | `manager.go:45-73`；`ValidateConfig` 不覆盖 `BatchSize/QPS/TopN` |

### 10.3 评测与验证相关

| # | 问题 | 依据 |
|---|---|---|
| 25 | **Go 内置评测 CLI 实际只测「纯向量」**：`AssembleEvalDeps` 新建空 BM25 且不回灌，被 `DocCount()>0` 门控跳过 → CLI 报告与线上 hybrid 不可比（RAGAS 路径不同，它调 Go `/api/v1/chat` 走线上链路） | `app.go:377`；`retriever.go:104`；`cmd/eval/main.go:57`；`services/ragas-eval/src/ragas_eval/core/collector.py:95` |
| 26 | **无 vectorstore 单测** | `ls internal/vectorstore/` 仅两个 .go |
| 27 | **「两路并行」验收项无测试**（`mockEmbedder.delay` 未使用） | `retriever_test.go:236,240-241`；`docs/04-检索器与重排序/checklist.md` |
| 28 | **无 BM25 `-race` 并发测试**（checklist 有此验收项） | grep 无命中 |
| 29 | **无真实 Qdrant 契约测试** | 测试全 mock/httptest |
| 30 | **无「BM25 独有命中内容完整性」断言** | `retriever_test.go:296-326` |
| 31 | **无性能/容量基线**（代码与测试中均无 benchmark；本档案亦不编造数字） | grep `func Benchmark` 无命中于这些包 |

### 10.4 面试话术建议（基于上述事实）

1. **最强主动承认**：BM25 纯内存且无回灌 → 重启即退化。给出三步改进（Postgres chunk 回灌 → 倒排快照 → 就绪健康检查 + `method` 指标告警），并说明这是「最小实现换迭代速度」的权衡。
2. **第二条**：BM25 独有命中正文为空 + 无 hydration → 修法明确且代价低。
3. **第三条**：召回预算 = topK（无 oversample）→ 先做 `recall_topk = 3×topK + rerank topK`，用 `internal/eval/evaluator.go` 的 Recall@K 量化收益。
4. **可坦然说「未做」的**：查询/重排缓存、score 阈值、payload 索引，以及**评测与线上不一致**（CLI eval 只测向量路）——主动讲出来能证明对自己系统的边界清楚。

---

## 附录 A：检索链路调用链（含行号）

**单路（默认）**
`rag.Engine`（`engine.go:983-988`）→ `Search`（`retriever.go:53`）→ `searchFused`（`:87`）→ 并发① `embedder.Embed`（`:207`）→ `vectorstore.Search`（`:225` → `qdrant.go:98`）；并发② `bm25Index.SearchFilteredByKBs`（`:108` → `bm25.go:159`）→ `FuseRRF`（`:133` → `rrf.go:13`）→ 截断 topK（`:73`）→ `rerankIfEnabled`（`:79` → `:164`）→ `reranker.Rerank`（`reranker.go:73` / `reranker_llm.go:86`）→ `rag.buildContext`（`context.go:35`）→ LLM。

**多查询**：`engine.go:895-900` → `SearchMulti`（`retriever.go:281`）→ 每路 `searchFused`（并发上限 `MultiQueryConcurrency`）→ `FuseMultiQuery(results, 60, topK)`（`:351`）→ 一次整体重排（`:365`）。

**HyDE**：`engine.go:980-981` → `routing.go:63` → LLM 生成假设文档 → `Embed` → `SearchByVector`（`retriever.go:272`）→ 原查询 `Search(SkipRerank=true)`（`routing.go:107-112`）→ `FuseMultiQuery(...,60,TopK)`（`:124`）→ `Rerank`（`:129`）。

**入库**：`task.WorkerPool`（并发 2）→ `pipeline.Ingest`（`pipeline.go:64`）→ 补偿清理（`:66-73`）→ load → chunk → `Embed`（`:105`）→ `Upsert`（`:141`）→ `AddWithDocID`（`:150`）。

**删除**：`handler_doc.go:306-317` → `vs.Delete(chunkIDs)` + `bm25.Remove(chunkID)` → 删记录。

## 附录 B：本链路相关配置项与默认值（全部来自代码）

| 配置 | 默认值 | 依据 | 部署样例 |
|---|---|---|---|
| `embedder.batch_size` | 100 | `config.go:381-383` | 10（`config.local.yaml:7`） |
| `embedder.max_retries` | 3 | `config.go:384-386` | 3 |
| `embedder.qps` | 10 | `config.go:387-389` | — |
| `embedder.dimension` | 1536 | `config.go:390-392` | 1024（`:6`） |
| `vectorstore.dimension` | 回退 embedder | `config.go:396-398` | 1024（`:13`） |
| `vectorstore.distance` | `cosine` | `config.go:393-395` | cosine（`:14`） |
| `retriever.top_k` | 10（校验 [1,50]） | `config.go:409-411`；`manager.go:113` | **2**（`:64`） |
| `retriever.rrf_k` | 60 | `config.go:412-414` | 60 |
| `retriever.vector_weight`/`bm25_weight` | 0.7/0.3（和须为 1） | `config.go:415-420`；`manager.go:120-125` | 0.7/0.3 |
| `retriever.enable_bm25` | bool，零值 false | `config.go:110` | true |
| `retriever.enable_reranker` | bool，零值 false | `config.go:111` | true |
| `retriever.multi_query_concurrency` | 3 | `config.go:421-423` | — |
| `reranker.top_n` | 5（主链路未使用，见 10.2#12） | `config.go:425-427` | 5 |
| `reranker.max_retries` | 3 | `config.go:428-430` | 3 |
| `reranker.qps` | 10 | `config.go:431-433` | 10 |
| `reranker.mode` | `""`→api | `reranker.go:57-65` | — |
| `rag.top_k`（决定实际召回数） | 5 | `config.go:464-466` | — |
| `rag.max_chunks` | 5 | `config.go:470-472` | — |
| `chunker.chunk_size` | 512 | `config.go:399-401` | — |
| BM25 `k1`/`b` | 1.2 / 0.75（常量，不可配） | `bm25.go:9-12` | — |

## 附录 C：结论可信度分级

- **A 级（多行代码直接证明）**：协议/端口、集合创建与距离、payload 字段、filter 构造、BM25 公式与常量、RRF 公式与权重来源、并发与锁、批大小、重试与退避、降级路径、单测断言清单。
- **B 级（代码缺失 + 全仓库 grep 无命中 ⇒ 未实现）**：无 score 阈值、无归一化、无维度校验、无缓存、无 payload 索引、无检索超时、无 BM25 持久化/回灌、无 vectorstore 单测、无 benchmark。
- **C 级（依赖第三方库默认值，已标注来源）**：Qdrant client 连接池 3 / keepalive 10s / `RetryConfig` nil 不重试（`go-client@v1.19.0/qdrant/config.go:29-68`）；`rate.Limiter` burst=0 时 `Wait` 报错而非阻塞（`x/time/rate@v0.15.0/rate.go:259-261`）。
- **明确写「代码中未找到」**：性能数字、内存占用估算等一切量化指标。
