# BinRag 文档入库链路 · 深度技术档案

> 分析范围：`internal/loader`、`internal/chunker`、`internal/task`、`internal/pipeline`、`internal/embedding`(入库侧)、`internal/store`、`internal/vectorstore`、`internal/api`(上传/任务 Handler)、`internal/app`(装配)、`internal/multimedia`(简述)
> 所有行号基于当前工作区文件快照。凡代码中无依据者，明确标注「代码中未找到」。

---

## 0. 结论速览（先看这 8 条）

| # | 事实 | 依据 |
|---|---|---|
| 1 | 上传是**同步落盘 + 同步写两条 PG 记录**，随后立即返回 `task_id`；真正的 Load→Chunk→Embed→Store 在 worker goroutine 里**异步**执行 | `internal/api/handler_doc.go:107-138`、`internal/task/worker.go:79-101` |
| 2 | 全链路**没有任何数据库事务**：`grep Begin(/pgx.Tx/BeginTx` 在 `internal/store`、`internal/pipeline`、`internal/task` 中零命中 | 表格见 §1.4 |
| 3 | chunk_id 是 **`uuid.New().String()`（v4 随机）**，不是内容 hash；因此**没有内容级幂等键** | `internal/pipeline/pipeline.go:116-119` |
| 4 | Qdrant payload 里存 chunk 全文；**PG 不存 chunk 表**，只在 `documents.chunk_ids TEXT[]` 里存一堆 UUID | `internal/pipeline/pipeline.go:120-138`、`internal/store/schema.go:15-27` |
| 5 | BM25 是**纯内存**索引，`Rebuild` 在全仓非测试代码中**没有任何调用点** → 进程重启后 BM25 为空，hybrid 检索静默退化为纯向量 | `internal/retriever/bm25.go:217-231`、`internal/retriever/retriever.go:104`、`internal/app/app.go:101` |
| 6 | 任务领取是 **`UPDATE ... WHERE id IN (SELECT ... LIMIT 1) RETURNING`**，**没有 `FOR UPDATE SKIP LOCKED`**；批次固定 1，空闲时每 worker 500ms 轮询一次 | `internal/store/task.go:73-96`、`internal/task/worker.go:18-22` |
| 7 | **没有 panic 恢复**（`grep recover()` 在 task/pipeline/api/app 零命中），业务 panic 会带走整个进程 | §9-6 |
| 8 | 退避**复用 `updated_at`**（写入未来时间），领取时用 `updated_at <= NOW()` 过滤实现延迟 | `internal/task/worker.go:154-155`、`internal/store/task.go:77` |

---

## 1. 完整入库数据流

### 1.1 同步段：HTTP 上传（请求线程内完成）

入口路由：`internal/api/router.go:122` `v1.POST("/documents/upload", h.UploadDocument)`，前置鉴权中间件 `internal/api/router.go:96-105`。

| 步骤 | 函数 / 位置 | 行为 | 失败返回 |
|---|---|---|---|
| 1 | `handler.UploadDocument` `handler_doc.go:38` | 入口 | — |
| 2 | `http.MaxBytesReader` `handler_doc.go:42-43` | **体积限制必须早于任何 body 读取**（`maxBytes = UploadMaxSizeMB × 1024 × 1024`） | 413/400 |
| 3 | kb_id 校验 `handler_doc.go:46-55` | 走 query，必须是合法 UUID（防路径穿越） | 400 |
| 4 | `ensureKBAccess` `handler_doc.go:57-60` | 越权/不存在统一 404 | 404 |
| 5 | `c.FormFile("file")` `handler_doc.go:62-71` | 解析 multipart；区分 `*http.MaxBytesError` | 400 |
| 6 | `registry.Support(info)` `handler_doc.go:74-78` → `support.go:21-32` | 格式识别 + 多媒体能力预检二合一 | 400（原因见 `SupportResult.Reason`） |
| 7 | `registry.Resolve` `handler_doc.go:82` → `registry.go:30-49` | 取 parser；非媒体则进入可读性预检 | — |
| 8 | `precheckReadable` `handler_doc.go:325-338` | **用 `ModeStrict` 完整解析一遍** + `ValidateReadable` | 400 |
| 9 | `os.MkdirAll` + `c.SaveUploadedFile` `handler_doc.go:96-105` | 落盘 `${FileStorageDir}/${kbID}/${docID}${ext}` | 500 |
| 10 | `taskID := uuid.New()` `handler_doc.go:108` | **先造 taskID 再写文档**（文档可反查任务） | — |
| 11 | `store.CreateDocument` `handler_doc.go:120` → `store/document.go:13-27` | INSERT documents，`status='pending'` | 500 |
| 12 | `store.CreateTask` `handler_doc.go:133` → `store/task.go:10-20` | INSERT ingest_tasks，`status='pending'` | 500 |
| 13 | `OK(c, {task_id, document_id})` `handler_doc.go:138` | **同步边界到此结束** | — |

> 注意步骤 11→12 是两次独立 `Exec`，**中间无事务**；若第 12 步失败，磁盘文件与 documents 记录会残留成孤儿（`handler_doc.go:133-136`）。

### 1.2 异步段：Worker 池消费

```
app.New → worker.Start(ctx)                      app.go:122-123
  └─ store.ResetProcessingTasks(ctx)             worker.go:45  → task.go:99-103
  └─ for i < WorkerCount { go workerLoop }       worker.go:52-55
workerLoop (worker.go:69)
  ├─ store.ClaimPendingTasks(ctx, 1)             worker.go:79  → task.go:73-96   [状态 pending→processing]
  ├─ 空 → time.After(500ms) 继续                  worker.go:89-96
  └─ process(context.WithoutCancel(ctx), t)       worker.go:100
       ├─ store.GetDocument(t.DocumentID)         worker.go:107 → document.go:51-68
       ├─ os.Open(doc.FilePath)                   worker.go:113
       ├─ pipeline.Ingest(...)                    worker.go:121 → pipeline.go:64
       │    ├─ DeleteByFilter{document_id}        pipeline.go:66-69   [Qdrant 旧向量清理]
       │    ├─ bm25.RemoveByDoc(document_id)      pipeline.go:70-72
       │    ├─ loader.Load(tolerant)              pipeline.go:76 → loader.go:42-66
       │    ├─ ValidateReadable                   pipeline.go:89 → validate.go:63-96
       │    ├─ chunker.Chunk                      pipeline.go:94 → chunker.go:41-105
       │    ├─ embedder.Embed(texts)              pipeline.go:105 → embedder.go:42-69
       │    ├─ 构造 VectorRecord(含 payload)      pipeline.go:115-139
       │    ├─ vectorstore.Upsert(wait=true)      pipeline.go:141 → qdrant.go:66-96
       │    └─ bm25.AddWithDocID(...)             pipeline.go:146-152 → bm25.go:62-95
       ├─ UpdateTask(completed)                   worker.go:132-139 → task.go:59-69
       └─ UpdateDocumentStatus(completed, IDs)    worker.go:140 → document.go:71-80
```

### 1.3 状态写入时机（完整状态机）

| 时机 | 位置 | ingest_tasks.status | documents.status | 其他字段 |
|---|---|---|---|---|
| 上传返回前 | `handler_doc.go:115,129` | `pending` | `pending` | — |
| worker 领取 | `task.go:75` | `processing` | **不变** | `updated_at=now()` |
| 成功 | `worker.go:132,137,140` | `completed` | `completed` | `warning_message`（若有）、`chunk_ids` 回填 |
| 失败未超限 | `worker.go:149-156` | `pending` | **不变** | `retry_count++`、`error_message`、`updated_at=now()+backoff` |
| 失败超上限 | `worker.go:158-165` | `failed` | `failed` | `chunk_ids` 被写成 `{}`（`document.go:72-74`） |
| 进程启动 | `worker.go:45` → `task.go:99-103` | `processing → pending` | 不变 | `updated_at=now()` |
| 手动重试 | `handler_task.go:78-80` | `failed → pending` | 不变（仍 `failed`） | `retry_count=0`、`error_message=""` |

> **`DocStatusProcessing` 是死常量**：`store.go:32` 定义了它，但全仓非测试代码从未写入（grep 只命中定义处）。因此文档在入库期间一直是 `pending`，前端无法区分「排队中」与「正在处理」。

### 1.4 同步 / 异步 / 事务边界总结

| 边界类型 | 位置 | 说明 |
|---|---|---|
| 同步→异步切换点 | `handler_doc.go:138` 返回 / `worker.go:79` 领取 | 由 PG `ingest_tasks` 表做队列，**不是 channel**（`internal/task` 无 channel 声明，仅 `sync.WaitGroup`） |
| DB 事务边界 | **不存在** | `grep "Begin(\|pgx.Tx\|BeginTx"` 在 `internal/store`、`internal/pipeline`、`internal/task` 零命中 |
| 外部存储边界 | `qdrant.go:85-90` `Wait: &waitTrue` | Qdrant 写是同步确认的（强一致读己写） |
| 一致性边界 | `pipeline.go:141`(Qdrant) 与 `pipeline.go:150`(BM25)、`worker.go:137/140`(PG) | 三处写入**无任何分布式事务/补偿日志**，只靠顺序 + 日志告警 |

### 1.5 入库侧 embedding 调用方式（只关注调用面）

- 接口：`Embed(ctx, texts []string) ([][]float32, error)`，`embedder.go:17-19`
- pipeline 一次把**全部 chunk 的 Content** 打包传入，`pipeline.go:100-105`
- 内部按 `BatchSize` 切片串行发 HTTP，`embedder.go:49-66`；每批前 `limiter.Wait(ctx)` 限流（`embedder.go:56-58`，`rate.NewLimiter(QPS, QPS)` 见 `embedder.go:38`）
- 单批失败按 `MaxRetries` 重试，退避 `1<<(attempt-1)` 秒，仅对 `*RetryableError`（网络错误 / 429 / 5xx）重试，`embedder.go:71-96`、`embedder.go:132-147`
- 返回向量按 `data[].index` 回填下标，`embedder.go:158-163`
- pipeline 侧校验 `len(vectors) == len(chunks)`，不等即报错，`pipeline.go:110-112`
- 默认值：`BatchSize=100`、`MaxRetries=3`、`QPS=10`、`Dimension=1536`，`config.go:381-392`；**`configs/config.yaml:16` 实际配置 `batch_size: 10`**，`configs/config.yaml:14` `dimension: 1024`
- HTTP client 超时 **30s 硬编码**（`embedder.go:37`，不读配置）

### 1.6 多媒体链路（pipeline 确实会走到）

`app.go:228-252 buildLoaderRegistry` 在配置就绪后用真实 provider 覆盖同名扩展名；`pipeline` 与 `api` **共用同一个 registry 实例**（`app.go:104,170`）。

- 图片：`parser_image.go:45-94`，视觉模型出描述 → 1 个 `BlockImageDescription`（含 width/height/source）
- 音频：`parser_audio.go:43-95`，ASR 出带时间戳分段 → N 个 `BlockAudioSegment`
- 视频：`parser_video.go:51-183`，**落临时文件**（`os.CreateTemp`，`parser_video.go:57`）→ `ffprobe` 探测（`parser_video.go:73`）→ `strategy.SampleFrames` 抽帧（`parser_video.go:82`）→ 逐帧 VLM（`parser_video.go:91`）→ 音轨独立拆流 + ASR（`parser_video.go:121-157`）
- ffmpeg / ffprobe 命令：`internal/multimedia/frame_extractor.go:45`（`exec.CommandContext`，数组传参防注入）、`internal/multimedia/audio_extractor.go:41`、`internal/multimedia/dashscope_speech.go:102`
- 能力缺失（`vision`/`speech` 的 `api_key` 为空 → provider 返回 nil，`multimedia/provider.go:17-36`）时 `CheckCapabilities` 返回 `*ErrMediaCapabilityMissing`，上传阶段 400 拒绝（`capability.go:64-76`、`support.go:26-30`）
- `scene` 策略缺 `vision_embedding` → `NewFrameStrategy` 返回 nil → `CheckCapabilities` 报文本错误，`multimedia/provider.go:40-51`、`parser_video.go:41-44`

---

## 2. Loader 深潜

### 2.1 统一结构体（`internal/loader/types.go`）

```go
type Block struct {                    // types.go:22-27
    Type     BlockType
    Content  string
    Level    int            // 仅标题有效，1-6
    Metadata map[string]any // page / start_ms / end_ms / media_type / source / width / height / frame_index
}
type Document struct { Blocks []Block; Metadata DocumentMeta }   // types.go:30-33
type DocumentMeta struct { Filename, Format, Title string; Size int64; PageCount int; Extra map[string]any } // types.go:36-43
type FileInfo struct { Filename, MIMEType string; Size int64 }   // types.go:46-50
type LoadOptions struct { Mode ErrorMode; Filename string }      // types.go:61-64
type LoadResult struct { Document *Document; Warnings []string } // types.go:67-70
```

`BlockType` 七种：`BlockParagraph / BlockHeading / BlockListItem / BlockCode / BlockTable / BlockImageDescription / BlockAudioSegment`（`types.go:11-19`）。
`ErrorMode` 两种：`ModeTolerant`(默认) / `ModeStrict`（`types.go:55-58`）。

### 2.2 每种格式的解析实现

| 格式 | 扩展名 | 依赖 | 关键行 | 产出 Block 类型与元数据 |
|---|---|---|---|---|
| PDF | `.pdf` | `github.com/pdfcpu/pdfcpu`（`api.PageCount` + `api.ExtractContent`） | `parser_pdf.go:44,61` | 每页一个 `BlockParagraph`，`Metadata["page"]=i`；`DocumentMeta.PageCount` |
| DOCX | `.docx` | `github.com/fumiama/go-docx` | `parser_docx.go:38` | 按 `Style.Val == Heading1..6`/`"1".."6"` 判层级 → `BlockHeading(Level)`，其余 `BlockParagraph`；`Level==1` 首个标题进 `Title` |
| Excel | `.xlsx/.xls` | `github.com/xuri/excelize/v2` | `parser_excel.go:29` | 每个 sheet 一个 `BlockHeading(Level=1)`，每行 join("\t") 成 `BlockTable` |
| CSV | `.csv` | 标准库 `encoding/csv`（`LazyQuotes=true`、`FieldsPerRecord=-1`） | `parser_csv.go:25-27` | 首行 join(", ") 成 `BlockHeading(Level=1)`，其余行 join("\t") 成 `BlockTable` |
| HTML | `.html/.htm` | `golang.org/x/net/html` | `parser_html.go:26` | 递归遍历：`h1-h6`→Heading(Level=数字)，`p`→Paragraph，`li`→ListItem，`pre/code`→Code；跳过 `script/style/nav/footer/head`（`parser_html.go:52-54`） |
| Markdown | `.md/.markdown` | `github.com/yuin/goldmark`（AST 遍历） | `parser_markdown.go:39-45` | `Heading`/`Paragraph`(排除 ListItem 内)/`ListItem`/`FencedCodeBlock` 四类；`WalkSkipChildren` 防重复 |
| TXT | `.txt` | 标准库 `bufio.Scanner` | `parser_txt.go:28-52` | 空行分段，每段一个 `BlockParagraph` |
| 图片 | `.png/.jpg/.jpeg/.webp/.gif/.bmp` | 标准库 `image.DecodeConfig` + `VisionProvider` | `parser_image.go:57,61` | 1 个 `BlockImageDescription`，metadata：`media_type/width/height/source` |
| 音频 | `.mp3/.wav/.m4a/.flac/.ogg/.aac` | `SpeechProvider`（OpenAI whisper 风格 / dashscope） | `parser_audio.go:53` | N 个 `BlockAudioSegment`，metadata：`start_ms/end_ms/source/media_type` |
| 视频 | `.mp4/.avi/.mkv/.mov/.webm` | ffmpeg/ffprobe + VLM + 可选 ASR | `parser_video.go:73,82,91,123` | `BlockImageDescription`(带 `timestamp_ms/frame_index/start_ms/end_ms`) + `BlockAudioSegment` |

> 编码兜底：loader 层**不做 BOM/GBK 检测**（`parser_txt.go` 直接用 `bufio.Scanner` 按字节扫行），非 UTF-8 文本会出现 U+FFFD，被 `ReadableCharCount` 计入「断词」逻辑（`validate.go:36-40`）——代码中未找到编码探测实现。

### 2.3 扫描件 / 加密 PDF / 超大文件如何识别与拒绝

**扫描件 / 空内容（可读文本量判定）**——双层防线：

1. **上传预检（同步）**：`handler_doc.go:82-90` 仅对**非多媒体** parser 生效，调用 `precheckReadable`（`handler_doc.go:325-338`），用 `ModeStrict` 解析后执行 `loader.ValidateReadable(doc, MinReadableChars)`，失败直接 400。
2. **入库兜底（异步）**：`pipeline.go:82-91`，先判 `result.Document == nil || len(Blocks)==0`，再 `ValidateReadable`。

判定算法（`validate.go:26-53`）：
```
可读量 = 汉字数(每个+1) + 真实单词数
真实单词 = 纯 A-Za-z 串，长度 ∈ [2,20]，且不在 pdfInstructionWords 表内
```
`pdfInstructionWords` 是一张 **49 个 PDF 内容流操作符白名单**（`q/Q/cm/re/BT/ET/Tf/TJ/Tj/Do/Im/BDC/EMC...`，`validate.go:9-18`），目的是把 `q 595.44 0 0 841.68 cm 1 g /Im10 Do Q` 这类内容流碎片判为 0 可读量。

`ValidateReadable` 的**双条件**（`validate.go:88`）：`total >= minChars` **且** `maxBlock >= minChars`，注释明确说这是为防止「大量低密度碎片累加绕过」；测试 `TestValidateReadableFragments`（`validate_test.go:102-119`）专门锁定这个行为。
多媒体豁免：`format ∈ {audio,image,video}` 直接 return nil（`validate.go:74-77`），与上传侧对齐。

**加密 PDF**：代码中**没有显式的加密检测**。`parser_pdf.go:40-42` 只是把校验模式放宽：
```go
conf := model.NewDefaultConfiguration()
conf.ValidationMode = model.ValidationRelaxed
```
加密文件会在 `api.PageCount`（`parser_pdf.go:44`）或 `api.ExtractContent`（`parser_pdf.go:61`）处报错；tolerant 模式下（pipeline 走默认 `ModeTolerant`，`loader.go:43`）返回**只有 Format 没有 Blocks 的 Document + Warnings**（`parser_pdf.go:49-53`、`70-76`），随后被 `pipeline.go:82-88` 的「无 Blocks」判据拒掉。也就是说：**加密 PDF 是"被间接拒绝"，错误信息是"文档无可读文本"，而不是"文件已加密"**——这是一个可被追问的措辞缺陷。

**超大文件**：
- 唯一硬限制来自 HTTP 层：`http.MaxBytesReader(..., UploadMaxSizeMB × 1024 × 1024)`，`handler_doc.go:42-43`
- 默认值 **50MB**（`config.go:531-533`），但 **`configs/config.yaml:242` 实际配成 `upload_max_size_mb: 1024`**（1GB）
- **loader 包内没有任何大小限制常量**，`parser_*` 全部 `io.ReadAll` 或流式读到底（`parser_pdf.go:29`、`parser_docx.go:27`、`parser_markdown.go:28`、`parser_image.go:50`、`parser_audio.go:48`），视频 `io.Copy` 到临时文件（`parser_video.go:64`）
- **页数 / 行数 / sheet 数上限：代码中未找到**。`PageCount` 只被读取并写入元数据（`parser_pdf.go:95`），未被用于任何校验；PDF 逐页 `ExtractContent` 循环（`parser_pdf.go:58-88`）没有上限保护。

### 2.4 错误类型与分类（`errors.go`，共 3 个，全部结构化）

| 类型 | 定义 | 关键字段 | `Unwrap` | 典型抛出点 |
|---|---|---|---|---|
| `ErrUnsupportedFormat` | `errors.go:6-16` | `Filename, MIMEType` | 无 | `registry.go:45-48` |
| `ErrParseFailed` | `errors.go:19-30` | `Format, Cause` | ✅ `errors.go:28-30` | 各 parser 的 `ModeStrict` 分支（如 `parser_pdf.go:32`） |
| `ErrNoReadableContent` | `errors.go:33-41` | `Format, Readable, MinChars` | 无 | `validate.go:68,89-93`、`parser_image.go:66`、`parser_audio.go:80`、`parser_video.go:160` |
| `*ErrMediaCapabilityMissing` | `capability.go:64-76` | `Capability("vision"/"speech")` | 自定义 `Is`（`capability.go:73-76`）支持 `errors.Is(err, &ErrMediaCapabilityMissing{})` | `parser_image.go:36`、`parser_audio.go:34`、`parser_video.go:38-43` |

容错语义：**只有 `ModeStrict` 会返回 error**；tolerant 模式下解析失败返回「空 Document + Warnings」（如 `parser_docx.go:42-46`、`parser_pdf.go:46-53`）。上传预检用 strict，入库用 tolerant。

### 2.5 支持格式如何注册（registry）

```go
func (r *defaultRegistry) Register(parser Parser) {          // registry.go:21-28
    for _, ext := range parser.SupportedExts()  { r.extMap[strings.ToLower(ext)] = parser }
    for _, mime := range parser.SupportedMIMEs() { r.mimeMap[strings.ToLower(mime)] = parser }
}
func (r *defaultRegistry) Resolve(info FileInfo) (Parser, error) {  // registry.go:30-49
    ext := strings.ToLower(filepath.Ext(info.Filename))
    if ext != "" { if p, ok := r.extMap[ext]; ok { return p, nil } }   // ① 扩展名优先
    if info.MIMEType != "" { if p, ok := r.mimeMap[...]; ok { return p } } // ② MIME 兜底
    return nil, &ErrUnsupportedFormat{...}
}
```
- 注册顺序：`NewDefaultRegistry()` 依序注册 txt/md/pdf/docx/csv/excel/html + 三个 nil 能力多媒体 parser（`loader.go:21-35`）
- **后注册覆盖先注册**（map 赋值），这是 `app.go:243-250` 能用真实 provider 覆盖同名扩展名的**机制基础**
- 无扩展名文件走 MIME 兜底（测试 `TestMIMEFallback`，`loader_test.go:222-236`）
- 消费者：pipeline 用 `NewLoaderWithRegistry(reg)`（`app.go:107`），API 用同一个 `reg` 做 `Support`/`SupportedTypes`（`app.go:170`）

### 2.6 能力声明（capability）机制

三个可选接口，用**类型断言**在 registry 层统一收敛：

```go
type MediaCapabilityChecker interface { CheckCapabilities() error }  // capability.go:54-56
type MediaCategory interface { MediaCategory() string }              // capability.go:59-61
```
`support.go:21-32 Support()`：Resolve 失败→不支持；实现了 `MediaCapabilityChecker` 且检查失败→不支持（Reason 取 error 文本）。
`support.go:36-53 SupportedTypes()`：枚举 `extMap`，Category 默认 `"text"`，未实现 `MediaCategory` 即视为文本；**结果按 ext 升序**（`support.go:51`）。
能力来源：`multimedia.NewVisionProvider/NewSpeechProvider/NewFrameStrategy` 在 `api_key` 为空时返回 **nil**（`multimedia/provider.go:17-22,26-36,40-51`），parser 持有 nil 即在 `CheckCapabilities` 报缺失——「能力声明」本质是「依赖注入 + nil 检查」，`loader` 包**不反向依赖** `multimedia`（设计注释见 `capability.go:7-9`）。

---

## 3. Chunker 三策略

### 3.1 默认常量与配置（**实测值，非文档值**）

| 参数 | 代码默认 | 生效值（YAML） | 依据 |
|---|---|---|---|
| `ChunkSize` | **512** token | **500**（`configs/config.yaml:88`） | `types.go:38-40`、`config.go:399-401` |
| `ChunkOverlap` | 注释写 50，**实际 `< 0` 才填 50** | **50**（`configs/config.yaml:90`） | `types.go:41-43`、`config.go:402-404` |
| `HeadingLevel` | **2** | **2** | `types.go:44-46`、`config.go:405-407` |
| `Strategy` | `recursive`（未知值/空值均回退） | **`recursive`**（`configs/config.yaml:86`） | `types.go:15-26` |

> 两个坑：① `ChunkOverlap` 判据是 `c.ChunkOverlap < 0`（不是 `<= 0`），**YAML 里省略该字段会得到 0 = 完全无重叠**，与「默认 50」的注释矛盾；② `config.go` 与 `chunker.WithDefaults` 两处判据必须同时改才生效。

### 3.2 tokenizer 的真实实现（`tokenizer.go:20-57`）

**它不是 BPE/tiktoken，是启发式估算器**：

```go
for _, r := range text {
    if unicode.Is(unicode.Han, r) {           // 汉字：每个计 2 token
        if wordBuf.Len() > 0 { count++; wordBuf.Reset() }
        count += 2
    } else if unicode.IsPunct(r) || unicode.IsSymbol(r) {  // 标点/符号：先结算单词，再 +1
        if wordBuf.Len() > 0 { count++; wordBuf.Reset() }
        count++
    } else if unicode.IsSpace(r) {            // 空白：结算单词，不加分
        if wordBuf.Len() > 0 { count++; wordBuf.Reset() }
    } else { wordBuf.WriteRune(r) }           // 其余连续字符累积为一个「词」
}
```

测试锁定：`{"你好", 4}`、`{"你好world", 5}`、`{"a,b", 3}`（`chunker_test.go:30-40`）。
**含义：`chunk_size=500` 对纯中文实际只约 250 个汉字**（汉字按 2 token 计），而 bge/text-embedding 的真实 BPE 对中文常是 1 字 ≈ 0.6~1 token —— 配置语义与模型上下文窗口**并不同源**。

`Tokenizer` 是接口（`tokenizer.go:9-11`），可注入（`NewChunker(nil)` 时用 `DefaultTokenizer`，`chunker.go:22-25`；测试用 `mockTokenizer` 每字符 1 token，`chunker_test.go:11-15`）。

### 3.3 策略选择：自动还是配置？

**是配置 + 硬编码兜底，且优先级凌驾于 strategy 字段之上**（`chunker.go:41-56`）：

| 条件 | 走哪条路 | 位置 |
|---|---|---|
| `doc == nil \|\| len(Blocks) == 0` | 返回 nil | `chunker.go:44-46` |
| **任一 Block 是 `ImageDescription` 或 `AudioSegment`** | `chunkMediaBlocks`（**每块一个 chunk，完全无视 chunk_size**） | `chunker.go:49-51,136-162` |
| **任一 Block 有 `Metadata["page"]`** | `chunkPagedBlocks`（**按页分块**，超长页才降级 recursive） | `chunker.go:54-56,176-222` |
| `config.Strategy == StrategyHeading` | `headingStrategy.SplitByBlocks` | `chunker.go:60-69` |
| 其他（fixed/recursive） | `blocksToText` 合并全文 → 单策略切分 | `chunker.go:70-85` |
| 策略未注册（如自定义类型未注册） | **静默回退 `StrategyRecursive`** | `chunker.go:71-74` |

**判定顺序即优先级：媒体 > PDF 分页 > 配置策略**。也就是说：对 PDF 配置 `heading` 是无效的，对 mp4 配置任何策略都无效。这是「自动」压过「配置」的隐式行为，文档里没写（代码注释 `chunker.go:48,53,164` 有说明）。

### 3.4 策略一：固定大小（`strategy_fixed.go`）

```
Split(text, cfg, tok)                                    :14-49
  totalTokens = tok.Count(text); if <= ChunkSize → 返回整段   :19-22
  runes = []rune(text); start = 0
  loop:
    end = findChunkEnd(runes, start, ChunkSize, tok)      :29
    chunk = TrimSpace(runes[start:end]); 非空则收           :30-33
    if end >= len(runes) → break                          :35-37
    overlapStart = findOverlapStart(runes, end, Overlap, tok)  :40
    start = (overlapStart <= start) ? end : overlapStart   :41-45

findChunkEnd(runes, start, chunkSize, tok)                :51-86
  estimatedEnd = start + chunkSize          ← 把「rune 数」当「token 数」估算！
  若 estimatedEnd >= len：整段够小则直接返回 len            :58-64
  向前收缩： end -= max(1,(end-start)/10) 直到 Count <= chunkSize  :68-70
  向后扩展： end++ 直到 Count(start:end+1) > chunkSize       :73-75
  回退到最近空白/中文标点断点（。！？，；）                    :78-83, 88-100
findOverlapStart: 从 end 向前找 Count(runes[i:end]) >= overlapTokens 的位置，
                  再向右对齐到最近的 ' ' 或 '\n'            :102-121
```
- 断点字符集：`' '`、`'\n'`、`'\t'`、`。！？，；`（`strategy_fixed.go:90-97`）——**没有英文逗号/分号**
- 首块之后每块由 `findOverlapStart` 决定起始，overlap 以「token 数」换算回「字符位置」
- 复杂度陷阱：`findChunkEnd` 的收缩/扩展循环里每步都 `Count(子串)`，`Count` 又是 O(len)，**单块接近 O(chunkSize²)**
- 测试：`TestFixedSizeStrategy`（断言每块 ≤ ChunkSize、≥4 块）、`TestFixedSizeOverlap`（**断言被刻意放宽到空操作**，`chunker_test.go:91-108` 里的 if 分支体是注释，实际不校验重叠）

### 3.5 策略二：递归字符（`strategy_recursive.go`）

分隔符列表（**硬编码，长度 10**）：
```go
var defaultSeparators = []string{"\n\n", "\n", "。", "！", "？", ".", "!", "?", " ", ""}  // :13
```
算法：
1. `Count(text) <= ChunkSize` → 整段返回（`:20-22`）
2. `splitRecursive(text, separators, ...)`（`:34-67`）：取首个分隔符 `strings.Split`，逐段 TrimSpace 丢弃空段；每段若超限则**用剩余分隔符递归**；`sep == ""` 或分隔符耗尽 → `hardSplit`
3. `mergeSegments`（`:69-99`）：贪心合并，用 `"\n\n"` 连接，`Count(current+"\n\n"+seg) <= ChunkSize` 才并入
4. `addOverlap`（`:101-120`）：**后处理式**，只给第 1..n 块**前缀**拼上一块尾部 `overlapTokens` 对应的文本，用 `"\n"` 连接
5. `hardSplit`（`:138-161`）：按 rune 硬切，`end = start + chunkSize`，再收缩到 token 限制内

**关键缺陷（必须知道）**：`strings.Split(text, sep)` **丢弃分隔符本身**，切分后没有回填。所以只要文档超过 `ChunkSize`，被切分处的 `。！？.!?` 会**永久丢失**（仅 `"\n\n"`/`"\n"` 因作为合并连接符而"看起来"还在）。测试 `TestRecursiveStrategy`（`chunker_test.go:111-137`）用的是 `strings.Repeat("a",15)` 这种无标点文本，**恰好绕过了这个问题**。

### 3.6 策略三：Markdown 标题（`strategy_heading.go`）

```
SplitByBlocks(blocks, cfg, tok)                    :28-85
  targetLevel = cfg.HeadingLevel (默认 2)
  headingStack []string; currentContent Builder
  flushSection():
     content = TrimSpace(currentContent)
     非空则 ctx = Join(headingStack, " > ")
            heading = headingStack[last]
            anchor  = slugifyHeading(heading)
            若 Count(content) > ChunkSize → fallback(Recursive).Split 降级，子块沿用同一 ctx/heading/anchor  :54-63
            否则整节一个 section                                                    :64-66
  遍历 blocks：
     block.Type==Heading && block.Level <= targetLevel → flushSection(); updateHeadingStack(stack, content, level); 把标题文本追加进 content
     否则 → 追加进 content
  flushSection()   ← 收尾
updateHeadingStack: for len(stack) >= level { pop }; push(title)   :87-93
```
- **breadcrumb 是否拼进 chunk**：分层处理——**正文里包含标题原文**（`blockToText` 把 Heading 渲染成 `"#"*Level + " " + Content`，`strategy_heading.go:97-99`），而 `"A > B"` 形式的路径**只进 metadata**（`Metadata.HeadingContext`），**不拼进 `Chunk.Content`**
- 同时写入 `Metadata.Heading`（最近一级标题文本）与 `Metadata.Anchor`（slug，供前端跳转）
- `slugifyHeading`（`chunker.go:263-280`）：去 ``#*_`[]()>``，空白→`-`，中文保留；测试 `TestSlugifyHeading` 期望 `"A  B"→"A--B"`、`"#*_x"→"x"`（`chunker_test.go:364-376`）
- **非 heading 策略的 breadcrumb 有 bug**：`extractFirstHeadingContext`（`chunker.go:245-259`）函数名叫 "First"，实现却是**遍历完全部 block 后取栈的最终状态**，即返回**文档最后一个标题路径**（如 `主标题 > 二级 > 三级`），并把它**赋给该文档的每一个 chunk**。⇒ recursive/fixed 策略下，所有 chunk 的 `heading_context` 都是同一个（且往往是错的）值。

### 3.7 表格 / 空白如何处理

| 场景 | 处理 | 依据 |
|---|---|---|
| `BlockTable`（csv/excel 行） | `blockToText` 走 `default` 分支 → 内容按原样（`\t` 分隔）参与拼接；**不做表头补全、不做 Markdown 表格化** | `strategy_heading.go:104-106`、`parser_csv.go:52`、`parser_excel.go:59` |
| 块间空白 | `blocksToText` 用 `"\n\n"` 连接；每块放入 section 时再补 `"\n\n"`，最后 `TrimSpace` | `chunker.go:115-121`、`strategy_heading.go:75-79` |
| 空白行/空块 | txt parser 已丢弃空段（`parser_txt.go:31-38`）；chunker 侧 `TrimSpace` 后为空则丢弃（`chunker.go:141-143`、`strategy_recursive.go:55-57`） | 同左 |
| 表格跨 chunk 断裂 | **无保护**：一张大表会被 recursive 策略从 `\n\n`/`\n` 处切开，表头不重复 | `strategy_recursive.go:13,52-64` |

### 3.8 chunk_id 如何生成（**必须看代码确认的项**）

**没有任何 hash。** `internal/pipeline/pipeline.go:116-119`：

```go
chunkIDs := make([]string, len(chunks))
for i, c := range chunks {
    chunkID := uuid.New().String()   // ← google/uuid v4，随机
    chunkIDs[i] = chunkID
```
该 UUID 同时作为：Qdrant point ID（`pipeline.go:121` `ID: chunkID`）、payload 里的 `chunk_id`（`pipeline.go:126`）、PG `documents.chunk_ids` 数组元素（`worker.go:140`）、BM25 的 docID（`pipeline.go:150`）。
旁证：`handler_chunk.go:28-31` 用 `uuid.Parse(id)` 做入参校验，注释写「chunk id 为 UUID」——**设计上就承认了 chunk_id 是 UUID 而非内容指纹**。

> 全仓 `sha256/md5` 的用途只有 API Key hash（`app.go:320`、`middleware.go:61`、`mcp/auth.go:58`、`handler_key.go:83`、`handler_mcp_my.go:120`），与 chunk 无关。
> ⇒ **同一份内容重复入库会得到**：不同 `document_id` 的**全新向量**（真重复）；同一 `document_id` 的**覆盖式重写**（靠 `pipeline.go:66-73` 的前置删除，见 §5）。

---

## 4. Worker 池（`internal/task/worker.go`）

### 4.1 数量与并发

| 项 | 值 | 依据 |
|---|---|---|
| worker 数量默认 | **2** | `config.go:534-536` |
| 生产 YAML 实际值 | **5** | `configs/config.yaml:244`；`deploy/configs/config.docker.yaml:220` |
| 每 worker 单次领取数 | **1**（`claimBatchSize = 1`） | `worker.go:21` |
| 空转轮询间隔 | **500ms** | `worker.go:19`，使用处 `worker.go:89-96` |
| 领取出错退避 | **1s** | `worker.go:20`，使用处 `worker.go:85` |
| 任务级超时 | **无** | 见下 |

```go
// worker.go:98-101  —— 关键：用 WithoutCancel，任务处理不随 Shutdown 取消
for _, t := range tasks {
    w.process(context.WithoutCancel(ctx), t)
}
```
⇒ `Shutdown()` 会 `cancel()` + `wg.Wait()`（`worker.go:60-66`），但正在跑的任务**不会被中断**，只会等它跑完；由于 `os.Open`/`http` 都没有 per-task deadline，**一个卡死的 embedding 请求会把 Shutdown 挂住**（唯一兜底是 embedder 的 30s HTTP timeout，`embedder.go:37`）。

### 4.2 任务领取方式：DB 轮询 + 原子 UPDATE，**不是 channel，也没有 SKIP LOCKED**

```go
// internal/store/task.go:73-96
rows, err := s.pool.Query(ctx,
    `UPDATE ingest_tasks SET status = 'processing', updated_at = now()
     WHERE id IN (
         SELECT id FROM ingest_tasks WHERE status = 'pending' AND updated_at <= NOW() ORDER BY created_at LIMIT $1
     )
     RETURNING id, kb_id, document_id, status, retry_count, error_message, warning_message, created_at, updated_at`,
    limit)
```
- 领取方式：**单条 SQL 的 `UPDATE ... WHERE id IN (SELECT ... LIMIT n) RETURNING`**，一条语句内完成「选 + 占」，PG 的行级锁保证同一行不会被两个事务同时更新（**实际并发安全性成立**）
- **但它没有 `FOR UPDATE SKIP LOCKED`**：子查询里的 `SELECT` 在并发下可能被多个 worker 选出同一批 id，落锁时表现为**串行等待 + 后续 worker 拿到 0 行**（空轮询），而不是「跳过锁行去取下一批」——多实例部署时会放大成明显的领取抖动
- **没有 channel 队列**：整个 `internal/task` 包只有 `sync.WaitGroup`，无 `chan`
- 顺序：`ORDER BY created_at`（**FIFO，且相同时间戳时顺序不确定**）
- 退避过滤：`updated_at <= NOW()`（`task.go:77`）——见 §4.4

### 4.3 任务状态机枚举（`internal/store/store.go:22-27`）

```go
TaskStatusPending    = "pending"
TaskStatusProcessing = "processing"
TaskStatusCompleted  = "completed"
TaskStatusFailed     = "failed"
```
文档状态枚举同值 `store.go:30-35`（其中 `DocStatusProcessing` **是死常量**，见 §1.3）。
状态迁移：`pending → processing → {completed | failed}`，`failed → pending`（仅手动重试），`processing → pending`（仅启动重置）。**没有 `cancelled` / `dead` / `retrying` 枚举**。

### 4.4 失败重试与退避（指数，且寄生在 `updated_at` 上）

```go
// worker.go:148-168
func (w *defaultWorkerPool) fail(ctx context.Context, t store.Task, err error) {
    if t.RetryCount < w.cfg.TaskMaxRetries {
        t.Status = store.TaskStatusPending
        t.RetryCount++
        t.ErrorMessage = err.Error()
        backoff := time.Second << uint(t.RetryCount)     // 1s/2s/4s/8s…
        t.UpdatedAt = time.Now().Add(backoff)            // ← 写入「未来时间」当 next_attempt_at
    } else {
        t.Status = store.TaskStatusFailed
        t.ErrorMessage = err.Error()
        w.store.UpdateDocumentStatus(ctx, t.DocumentID, store.DocStatusFailed, nil)
    }
    w.store.UpdateTask(ctx, t)
}
```
- 最大重试次数默认 **3**（`config.go:537-539`），YAML 实际 **3**（`configs/config.yaml:246`）
- 退避序列（**首败后再 +1 才算**）：第 1 次失败 → `RetryCount=1` → **2s**；第 2 次 → 4s；第 3 次 → 8s；之后再失败即 `failed`。⇒ `TaskMaxRetries=3` 时总共 4 次执行机会
- 延迟的落地机制：`UpdateTask` 把 `updated_at` 写成未来时间（`task.go:59-68`），`ClaimPendingTasks` 用 `updated_at <= NOW()` 过滤（`task.go:77`）
  - ✅ 不需要额外的调度器，纯 DB 实现
  - ⚠️ **语义混用**：`updated_at` 既是"最后更新时间"又是"下次可领取时间"，任何按 `updated_at` 做的排序/监控/清理都会被这个未来时间污染
- 退避只作用于**任务级**；embedding 自身还有一层 1s/2s/4s 的重试（`embedder.go:74-82`），两层叠加 = 最坏情况单任务重试 4 × 3 = 12 次外部调用

### 4.5 重启恢复：`processing → pending`（**无租约，无心跳**）

```go
// internal/task/worker.go:44-47
func (w *defaultWorkerPool) Start(ctx context.Context) {
    if err := w.store.ResetProcessingTasks(ctx); err != nil { slog.Warn("重置悬挂任务失败", "err", err) }
// internal/store/task.go:99-103
`UPDATE ingest_tasks SET status = 'pending', updated_at = now() WHERE status = 'processing'`
```
- 启动即**全表无条件**把 processing 打回 pending，不检查任务年龄、不检查所属实例
- ⇒ 单实例重启场景：正确恢复 ✅
- ⇒ **多实例滚动重启场景：会把另一个实例正在执行的任务重置为 pending**，同一 document 被两个 worker 并行入库（后果见 §5 的一致性分析）——**代码中未找到任何 owner / lease / heartbeat 字段**（`ingest_tasks` 表结构见 §6）

### 4.6 幂等性保证（重复入库会怎样）

| 场景 | 实际行为 | 依据 |
|---|---|---|
| 同一 `document_id` 重跑（worker 重试 / 手动重试） | Ingest 开头按 `document_id` 删掉 Qdrant 全部旧点 + `bm25.RemoveByDoc`，再重写 ⇒ **覆盖式幂等** | `pipeline.go:66-73`、`qdrant.go:168-184`、`bm25.go:104-112` |
| 同一文件第二次上传（新 `document_id`） | **完全重复入库**，向量库出现两份内容、不同 chunk_id | `handler_doc.go:92` `docID := uuid.New().String()` |
| 两个 worker 同时处理同一 document（重置误伤场景） | 两次 `DeleteByFilter` + 两次 `Upsert` 并发交错 → 可能出现「A 删完 B 写的点」或同一 document_id 下残留两套向量；BM25 因 `docChunks[docID]` 是 append 语义（`bm25.go:83-85`）会残留指向已删 chunk 的 docID | §4.5 + `pipeline.go:66-73` |
| 无内容变更检测 | 没有 `content_hash` / `updated_at` / `etag` 任何字段 | `schema.go:15-27` |
| 无唯一约束兜底 | `documents` 只对 `id` 有 PK，`(kb_id, filename)` 无唯一索引 | `schema.go:15-27` |

### 4.7 手动重试接口行为（`internal/api/handler_task.go:56-86`）

- 路由：`POST /api/v1/tasks/:id/retry`（`router.go:131`），需鉴权 + KB 越权校验（`handler_task.go:68-71`，越权一律 404）
- **仅 `failed` 可重试**，否则 400「仅 failed 状态的任务可重试」（`handler_task.go:73-76`）
- 重置：`Status=pending`、`RetryCount=0`、`ErrorMessage=""`（`handler_task.go:78-80`）
- **不重置**：`WarningMessage`（残留上轮告警）、**`documents.status`（保持 failed 直到下次成功）**、`updated_at`（由 `UpdateTask` 写 now，`task.go:60-63`）
- ⚠️ `RetryCount=0` 意味着**手动重试是一次全新循环**，可无限次触发（无重试次数上限、无频率限制）

### 4.8 panic 恢复

**代码中未找到**。`workerLoop`（`worker.go:69-103`）与 `process`（`worker.go:106-145`）均无 `defer recover()`；全包 `grep "recover()"` 零命中。
⇒ 若 `pipeline.Ingest` 或任何 parser 内部 panic（例如第三方库在畸形 PDF 上 panic、`frames[i+1]` 越界），**panic 会沿 goroutine 上抛并终止整个进程**；此时该任务永远停在 `processing`，要靠下次启动的 `ResetProcessingTasks` 才能回到 pending——**但重试会再次命中同一个 panic，形成崩溃循环**。

---

## 5. Pipeline 编排（`internal/pipeline/pipeline.go`）

### 5.1 每步的输入 / 输出

| 步骤 | 输入 | 输出 | 失败处理 | 位置 |
|---|---|---|---|---|
| 补偿清理 | `req.DocumentID` | 删 Qdrant 旧点 + BM25 旧条目 | **仅 Warn，继续**（`slog.Warn`） | `:66-73` |
| **Load** | `io.Reader`（worker 传 `*os.File`）、`loader.FileInfo{Filename}`、默认 `ModeTolerant` | `*LoadResult{Document, Warnings}` | 包成 `加载文档失败: %w` 上抛 | `:76-79`、`worker.go:121-126` |
| 可读校验 | `*Document`、`loaderCfg.MinReadableChars` | nil / `ErrNoReadableContent` | 上抛 | `:82-91` |
| **Chunk** | `*Document` + `chunkConfig` | `[]chunker.Chunk`（**0 个时返回 `nil, warnings, nil` 成功！**） | — | `:94-97` |
| **Embed** | `texts = 所有 c.Content` | `[][]float32` | 上抛；数量不匹配单独报错 | `:100-112` |
| **Store** | 组装 `[]vectorstore.VectorRecord` | — | 上抛 `向量入库失败` | `:115-143` |
| BM25 更新 | records（取 payload 的 content/kb_id） | 内存索引 | **无错误返回路径** | `:146-152` |

> ⚠️ `pipeline.go:95-97`：`chunks` 为空时返回 `(nil, warnings, nil)` —— worker 会把它当成功，任务 `completed`、`chunk_ids` 写成 `{}`、文档 `completed`，但**一条向量都没有**。这是「成功但空入库」的静默路径。

### 5.2 Qdrant payload 字段清单（`pipeline.go:120-138`）—— 检索与引用的唯一真相源

```
kb_id, document_id, chunk_id, filename, heading_context, chunk_index,
content, source_type, start_ms, end_ms, page_number, heading, anchor
```
- 全部走 `toQdrantValue`（`qdrant.go:233-248`）：string/int/int64/float64/bool 原生，**其他类型一律 `fmt.Sprintf("%v")` 转字符串**（如 `width/height` 若为 int 可原生，但 map/slice 会被字符串化）
- 写入是**单次 `Upsert` 全部点**，`Wait=true`（`qdrant.go:85-90`）
- 引用回看：`GET /api/v1/chunks/:id`（`router.go:137` → `handler_chunk.go:33-84`）从 Qdrant 取 payload，再回 PG 校验文档归属（`handler_chunk.go:59-71`，防跨租户读残留向量）

### 5.3 PG 与 Qdrant 双写的一致性处理

**结论：没有一致性协议，只有「顺序 + 日志告警 + 前置删除」。**

```
① Qdrant DeleteByFilter(document_id)   pipeline.go:67   ─┐ 无事务
② Load / Chunk / Embed                 pipeline.go:76-112 │ 纯内存
③ Qdrant Upsert (wait=true)            pipeline.go:141   ─┘ 外部写入点（唯一「落库」动作）
④ BM25 AddWithDocID                    pipeline.go:150      内存写入
⑤ PG: tasks → completed                worker.go:137        ┐ 两次独立 UPDATE
⑥ PG: documents → completed + chunk_ids worker.go:140       ┘ 失败仅 slog.Warn
```
失败行为矩阵：

| 失败点 | 数据后果 | 处理 |
|---|---|---|
| ①②③ 任一步失败 | 无新增向量（③ 之前失败）；③ 本身失败则旧向量已被删、新向量未写 ⇒ **该文档向量彻底消失** | 任务回 pending 重试；重试时再删一次（已空）再写 |
| ③ 成功、④ 成功、⑤ 失败 | Qdrant/BM25 有数据，但 PG 任务仍 `processing` | 只 `slog.Warn("更新任务状态失败")`（`worker.go:138`）；**重启后 `ResetProcessingTasks` 会把它打回 pending 再跑一遍**，靠 ① 的前置删除实现最终收敛 |
| ③ 成功、⑥ 失败 | Qdrant 有向量、PG 文档仍非 completed（chunk_ids 为空） | 只 `slog.Warn`（`worker.go:141`）；**chunk_ids 为空 ⇒ 后续 DELETE 文档时无法删除这些向量**（见下） |
| ① 删除失败（如 Qdrant 抖动） | 旧向量残留 + 新向量叠加 ⇒ **重复检索结果** | 只 `slog.Warn("清理旧 chunk 向量失败（继续入库）")`（`pipeline.go:68`） |
| 崩溃/断电（无事务、无 outbox） | 无法自动恢复，只有任务状态机兜底 | `ResetProcessingTasks` |

**脏数据清理机制（两处，且不对称）**：
1. **入库前清旧**（`pipeline.go:66-73`）：按 `document_id` 删 Qdrant + BM25 —— 这是"重试补偿"，只在 `DocumentID != ""` 时执行
2. **删除文档时清理**（`handler_doc.go:306-321`）：**只按 `doc.ChunkIDs` 删**（`vs.Delete(doc.ChunkIDs)` + 逐个 `bm25.Remove(chunkID)`），随后 `os.Remove(doc.FilePath)` + `DeleteDocument`
   ⇒ **如果 `chunk_ids` 为空（步骤⑥失败、或 `pipeline.go:95-97` 的零 chunk 路径），这些向量永远不会被删除**——`DeleteByFilter` 在删除路径上**没有被使用**（只在 pipeline 里用过）

### 5.4 BM25 索引更新时机

- 时机：**Qdrant Upsert 成功之后**（`pipeline.go:141` → `:146-152`），顺序保证「BM25 里的 id 一定在 Qdrant 里存在过」
- 内容：只索引 `payload["content"]`（chunk 原文），kb 与 doc 维度分别落 `docKB` / `docChunks`（`bm25.go:81-85`）
- 并发：`sync.RWMutex` 保护，读走 RLock（`bm25.go:73-74, 165-166`）
- 去重：同一 id 重复 Add 会先 `removeLocked(id)` 再插（`bm25.go:77-79`）⇒ **同 id 幂等** ✅
- **持久化：完全没有**。`docLen/docTerms/inverted/docKB/docChunks` 全是内存 map（`bm25.go:34-44`），无快照、无落盘、无启动预热。
- **`Rebuild([]BM25Doc)`（`bm25.go:217-231`）在全仓非测试代码中零调用点**（`grep "Rebuild("` 仅命中接口声明、实现、以及 `retriever_test.go:115`）⇒ **进程重启后 `DocCount()==0`**，`retriever.go:104` 的 `r.bm25Index.DocCount() > 0` 门禁直接跳过 BM25，`retriever.go:120-123` 回退纯向量结果，`method` 也报 `"vector"` 而非 `"hybrid"`——**用户无感知地损失了混合检索能力，直到文档被重新入库**。

---

## 6. 数据库表结构（`internal/store/schema.go`）

### 6.1 `documents`（`schema.go:15-27`）

```sql
CREATE TABLE IF NOT EXISTS documents (
    id          TEXT PRIMARY KEY,
    kb_id       TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,  -- 唯一外键
    filename    TEXT NOT NULL,
    format      TEXT NOT NULL DEFAULT '',       -- 存的是扩展名，如 ".pdf"（handler_doc.go:113）
    size        BIGINT NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'pending',
    chunk_ids   TEXT[] NOT NULL DEFAULT '{}',   -- 全部 chunk UUID
    file_path   TEXT NOT NULL DEFAULT '',       -- 本地磁盘绝对路径
    task_id     TEXT NOT NULL DEFAULT '',       -- 反向关联，无 FK
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_documents_kb_id ON documents(kb_id);
```

### 6.2 `ingest_tasks`（`schema.go:29-39` + 迁移 `schema.go:99-101`）

```sql
CREATE TABLE IF NOT EXISTS ingest_tasks (
    id            TEXT PRIMARY KEY,
    kb_id         TEXT NOT NULL,       -- 无外键
    document_id   TEXT NOT NULL,       -- 无外键！文档删了任务还在
    status        TEXT NOT NULL DEFAULT 'pending',
    retry_count   INT NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()   -- 同时承担 next_attempt_at 语义
);
CREATE INDEX IF NOT EXISTS idx_ingest_tasks_status ON ingest_tasks(status);
-- 迁移（幂等 ALTER，schema.go:99-101）
ALTER TABLE ingest_tasks ADD COLUMN IF NOT EXISTS warning_message TEXT NOT NULL DEFAULT '';
```

### 6.3 与用户问题的逐项对照

| 问题 | 答案 | 依据 |
|---|---|---|
| `metadata JSONB` | **代码中未找到**。全仓 `grep -i jsonb` 在 `internal/store` 零命中；文档元数据（mime、页数、媒体时长…）**不落 PG**，只有 `format TEXT` 存扩展名 | `schema.go:15-27` |
| `documents` 索引 | 仅 `idx_documents_kb_id(kb_id)` | `schema.go:27` |
| `ingest_tasks` 索引 | 仅 `idx_ingest_tasks_status(status)`；**`ClaimPendingTasks` 的 `WHERE status='pending' AND updated_at<=NOW() ORDER BY created_at` 没有对应的复合索引** | `schema.go:39` vs `task.go:77` |
| 外键 | 只有 `documents.kb_id → knowledge_bases(id) ON DELETE CASCADE`；`ingest_tasks.document_id`、`ingest_tasks.kb_id`、`documents.task_id` **均无外键** | `schema.go:17,24,31-32` |
| chunk 记录是否入 PG | **不入**。PG 只有 `chunk_ids TEXT[]`；chunk 正文/元数据只存在 Qdrant payload | `document.go:71-80`、`pipeline.go:120-138` |
| 迁移机制 | `Migrate()` 全 `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS` 幂等执行（`schema.go:120-146`），`app.go:70` 启动时调用；**无版本号、无 down 迁移** | `schema.go:120-146` |
| 其他表（同库） | `knowledge_bases`(`:7-13`)、`api_keys`(`:41-48`)、`chat_history`(`:50-57`)、`users`(`:59-67`)、`mcp_audit_logs`(`:104-118`) | 同文件 |
| 存储访问层 | 接口 `Store`（`store.go:113-158`），实现 `pgStore{pool queryExecutor}`（`store.go:160-162`），`queryExecutor` 抽象让 `pgxmock` 可测（`store.go:13-19`） | 同文件 |

---

## 7. 测试覆盖证据（可作「我怎么验证」的素材）

### 7.1 Loader（4 个测试文件，33 个测试函数）

| 测试 | 锁定的行为 | 位置 |
|---|---|---|
| `TestTxtParser` | 空行分段 → 恰好 3 个 `BlockParagraph`；`Metadata.Filename/Format` 被回填 | `loader_test.go:14-40` |
| `TestMarkdownParser` | Title=首个 H1；Heading 层级 1/2/3 递增；2 个 ListItem；1 个 CodeBlock | `loader_test.go:42-101` |
| `TestCsvParser` | 1 表头→`BlockHeading` + 2 行→`BlockTable`，共 3 Block | `loader_test.go:103-127` |
| `TestExcelParser` | **内存里用 excelize 现造 xlsx**；≥2 sheet Heading、≥2 Table Block | `loader_test.go:129-163` |
| `TestHtmlParser` | Title 取 h1；跳过 script/style；li 计 2；pre/code 计为 Code | `loader_test.go:165-203` |
| `TestUnsupportedFormat` | `errors.As` 到 `*ErrUnsupportedFormat` 且 `Filename` 正确 | `loader_test.go:205-220` |
| `TestMIMEFallback` | 无扩展名 + `text/plain` → 解析为 txt | `loader_test.go:222-236` |
| `TestTolerantMode` / `TestStrictMode` | 同一份坏 PDF：tolerant 返 `Warnings` 且 Document 非 nil；strict 返 error | `loader_test.go:238-261` |
| `TestCustomParserRegistration` | 自定义 parser + `NewLoaderWithRegistry` 可插拔 | `loader_test.go:263-292` |
| `TestReadableCharCount`（8 子用例） | `"支持多种文档格式解析"=10`；`"BinRag 支持 5 种格式"=6`；`"q g a"=0`；内容流指令 `q ... cm 1 g /Im10 Do Q` = 0 | `validate_test.go:17-39` |
| `TestValidateReadable`（6 子用例） | 正常文本通过；**图像指令乱码被拒且错误是 `*ErrNoReadableContent`**；空 Blocks / nil doc 被拒；`minChars=0` 禁用；错误文案含「无可读文本」「最低 20」 | `validate_test.go:41-100` |
| `TestValidateReadableFragments` | **4 个低密度 PDF 指令碎片累加仍被拒**（双条件 maxBlock 判定的核心证据） | `validate_test.go:102-119` |
| `TestValidateReadableMultiBlockPass` | 有 1 个达标 block 即通过 | `validate_test.go:121-130` |
| `TestSupportTextAlwaysSupported` / `TestSupportUnknownFormat` | 文本恒支持且 Reason 为空；`.exe` 不支持且 Reason 含「不支持的文件格式」 | `support_test.go:9-18,56-65` |
| `TestSupportMediaWithoutConfig` | mp3/png/mp4 未配能力时 `Supported=false`，Reason 分别含 `speech`/`vision`/`vision` | `support_test.go:21-43` |
| `TestSupportAudioWithSpeech` | 注入 speech 后音频变支持（能力覆盖注册生效） | `support_test.go:46-53` |
| `TestSupportedTypesCategories` | `.txt=text/.mp3=audio/.png=image/.mp4=video`；未配置多媒体不支持且有 Reason；**ext 严格升序** | `support_test.go:68-115` |
| `TestImageParserParse` | 1 个 `BlockImageDescription`；width/height=20/10；source 为原文件名；**传给 vision 的字节 == 原始图片字节** | `parser_image_test.go:80-109` |
| `TestImageParserProviderError` / `EmptyDescription` | 服务失败返回含「视觉理解」的错误；空描述拒绝（防脏数据） | `parser_image_test.go:111-130` |
| `TestAudioParserParse` | 2 段 → 2 个 `BlockAudioSegment`；时间戳 0-2500/2500-5000；`Extra.duration_ms=末段 EndMs=5000` | `parser_audio_test.go:59-96` |
| `TestAudioParserEmptySegments` / `ProviderError` / `CapabilityMissing` | 空转写拒绝；服务失败报错；无 speech 报 `ErrMediaCapabilityMissing` | `parser_audio_test.go:43-56,98-115` |
| `TestVideoParserParse2` | 2 帧 + 1 音轨 = 3 Block；**帧 0 = [0,10000) 取下一帧边界**；`frame_index`；音轨保留原始时间戳；`Extra` 含 duration/codec/has_audio；**完整能力时 Warnings 为空** | `parser_video_test.go:74-121` |
| `TestVideoParserSpeechMissing2` | 无 speech → 出 warning 但视觉帧照常产出（降级不阻断） | `parser_video_test.go:125+` |
| `TestVideoParserAudioExtractFail2` / `FrameFail2` / `VisionFail2` | 音轨拆流失败→warning 跳过；抽帧失败/视觉失败→error | `parser_video_test.go:144-180` |

### 7.2 Chunker（15 个测试）

| 测试 | 锁定的行为 |
|---|---|
| `TestDefaultTokenizer` | `hello=1`、`hello world=2`、`你好=4`、`你好world=5`、`a,b=3`（`chunker_test.go:27-48`） |
| `TestFixedSizeStrategy` | 100 字符 / ChunkSize=20 → 每块 ≤ 20 且 ≥ 4 块（`:50-70`） |
| `TestFixedSizeOverlap` | ⚠️ **断言被削弱成空操作**：`if !Contains(...)` 分支体内全是注释，不实际校验重叠（`:72-109`） |
| `TestRecursiveStrategy` | 3 段各 15 字符 / ChunkSize=20 → 每块 ≤ 20 且 ≥ 3 块，**在段落边界切分**（`:111-137`） |
| `TestHeadingStrategy` | 按 h2 切分 ≥3 chunk；**每个 chunk 的 `HeadingContext` 非空**；且包含「主标题」（breadcrumb 生效）（`:139-186`） |
| `TestHeadingStrategyFallback` | 超长 h2 节（100 字符 / ChunkSize=30）降级拆分且**子块仍 ≤ ChunkSize**（`:188-220`） |
| `TestChunkMetadata` | `Index` 严格 0..n-1；`DocFilename` 透传；`TokenCount > 0`（`:222-251`） |
| `TestCustomTokenizer` / `TestCustomStrategyRegistration` | 可注入 tokenizer；`RegisterStrategy(99, mock)` 后自定义策略**被调用**且输出 1 chunk（`:253-305`） |
| `TestChunkMediaBlocks` | 3 个媒体 block → 恰好 3 chunk；**时间戳 1:1 贯通**（0-10000 / 10000-20000 / 0-3000）；`SourceType` 正确（`:316-346`） |
| `TestChunkMediaVideoAudioTrackSourceType` | **视频文档里的音轨 chunk 也归为 `video`**（按 `doc.Metadata.Format` 优先判定）（`:426-447`） |
| `TestChunkMediaImageSourceType` | 纯图片 → `SourceType=image`（`:349-361`） |
| `TestChunkPagedBlocks` | 2 页 → 2 chunk，`PageNumber` = 1/2（`:379-398`） |
| `TestChunkMarkdownHeadingAnchor` | Heading/Anchor/HeadingContext 三者均为「模块设计」（`:401-423`） |
| `TestSlugifyHeading` | `"A  B"→"A--B"`、`"#*_x"→"x"`、`"标题 [x] (y)"→"标题-x-y"`、中文保留（`:364-376`） |

### 7.3 Task / Worker（7 个测试）

| 测试 | 锁定的行为 |
|---|---|
| `TestProcess_Success` | 真实临时文件 → 任务 `completed`，文档 `completed` 且 `ChunkIDs` 长度=2（`worker_test.go:135-162`） |
| `TestProcess_RetryNotExceeded` | `RetryCount=0` 失败 → 回 `pending`、`RetryCount=1`、`ErrorMessage` 非空（`:165-191`） |
| `TestProcess_RetryExceeded` | `TaskMaxRetries=1` 且 `RetryCount=1` → `failed` 且错误原文保留（`:194-219`） |
| `TestProcess_DocMissing` | 文档查不到 → 走到上限即 `failed`（`:222-241`） |
| `TestStart_CallsReset` | `Start()` 确实调用 `ResetProcessingTasks`（重启恢复）；`Shutdown()` 能正常返回（`:244-257`） |
| `TestProcess_Concurrent` | 20 个 goroutine 并发 `process` 无数据竞争（配 `-race` 有效）（`:260-285`） |
| `TestProcess_WarningMessage` | warnings 精确写入 `task.warning_message`（`:299-322`） |

> 缺口：**没有对 `workerLoop` 的轮询/领取/退避路径做集成测试**（`fakeStore.ClaimPendingTasks` 直接返回 `nil,nil`，`worker_test.go:113-115`），退避时间与空转行为**未被测试覆盖**。

### 7.4 Pipeline（6 个测试）

| 测试 | 锁定的行为 |
|---|---|
| `TestIngestSuccess` | 每个 record 都含 `filename/kb_id/document_id/chunk_id/heading_context/chunk_index/content`；向量维度=4；`record.ID` 非空（`pipeline_test.go:69-133`） |
| `TestIngestEmbedError` | Embed 失败必须上抛（`:135-159`） |
| `TestIngestUpsertError` | Upsert 失败必须上抛（`:161-185`） |
| `TestIngestNoReadableContent` | 无可读文本：**错误类型是 `*loader.ErrNoReadableContent`，且向量库 0 条记录**（拒后不写脏数据）（`:191-223`） |
| `TestIngestWarningsPassthrough` | fakeLoader 的 warning 原样透传（spec N4）（`:239-263`） |
| `TestIngestMultimediaEndToEnd` | **图片/视频经真实 loader + mock provider 走完整链路**：图片 chunk 含视觉描述文本、`filename` 为原文件名、`source_type=image`、`start_ms=0`；视频（无 speech）带音轨降级 warning（`:293-367`） |

### 7.5 Store / API（与本链路直接相关）

| 测试 | 锁定的行为 |
|---|---|
| `TestClaimPendingTasks` | 断言 SQL 是 `UPDATE ingest_tasks` 且**返回行状态为 `processing`**，单参数 `1`（`store_test.go:116-146`） |
| `TestResetProcessingTasks` | 断言执行了 `UPDATE ingest_tasks` 且影响 2 行（`:149-165`） |
| `TestMigrateIdempotentOnRealPG` | 对真实 PG 连续迁移两次不报错（`migrate_pg_test.go:13`） |
| `TestUploadReturnsTaskID` | 上传 200 且返回 `task_id`+`document_id`；**任务记录存在且为 `pending`**；文档列表可见（`api_test.go:569-623`） |
| `TestUploadUnsupportedFormat` | 不支持格式 → 400（`:626`） |
| `TestUploadTooLarge` | 超过 `UploadMaxSizeMB` → 400 且文案含大小上限（`:714`） |
| `TestUploadNoReadableContent` | 扫描件/纯指令文本 → 上传阶段 400（`:1119`） |
| `TestUploadMultimediaCapabilityMissing` | 未配 vision/speech 时 `.png`/`.mp3` 上传 400 且 Reason 指向能力（`:1493-1512`） |
| `TestUploadMultimediaImageSuccess` | 配了 vision 后图片上传成功（`:1515`） |
| `TestUploadMultimediaPrecheckSkipsParse` | **多媒体不做文本可读性预检**（避免解析两次）（`:1545-1553`） |

---

## 8. 面试官最可能深挖的 12 个点

### ① 上传为什么能立刻返回？同步/异步边界在哪，状态什么时候写？

**问**：一次上传请求里到底做了哪些事？任务状态在什么时刻变成 processing？

```go
// internal/api/handler_doc.go:108-138
taskID := uuid.New().String()
doc := store.Document{ ID: docID, KBID: kbID, Filename: file.Filename, Format: ext,
    Size: file.Size, Status: store.DocStatusPending, FilePath: filePath, TaskID: taskID, CreatedAt: time.Now() }
if err := h.store.CreateDocument(ctx, doc); err != nil { Fail(c, CodeInternal, "创建文档记录失败"); return }
task := store.Task{ ID: taskID, KBID: kbID, DocumentID: docID, Status: store.TaskStatusPending, ... }
if err := h.store.CreateTask(ctx, task); err != nil { Fail(c, CodeInternal, "创建任务记录失败"); return }
OK(c, gin.H{"task_id": taskID, "document_id": docID})
```
**要点**：同步段 = 限流体积 → 校验 kb → 格式/能力预检 → 可读性预检（strict 解析一遍）→ 落盘 → 写 2 张表；`processing` 由 worker 领取时写（`task.go:75`）。**PG 表就是队列**，不是 channel/MQ。

### ② Loader 如何做到「加格式不改调用方」？MIME 兜底与能力声明怎么协同？

```go
// internal/loader/registry.go:30-49
func (r *defaultRegistry) Resolve(info FileInfo) (Parser, error) {
	ext := strings.ToLower(filepath.Ext(info.Filename))
	if ext != "" { if p, ok := r.extMap[ext]; ok { return p, nil } }
	if info.MIMEType != "" { if p, ok := r.mimeMap[strings.ToLower(info.MIMEType)]; ok { return p, nil } }
	return nil, &ErrUnsupportedFormat{Filename: info.Filename, MIMEType: info.MIMEType}
}
```
**要点**：ext 优先 → MIME 兜底；`Register` 后注册覆盖先注册（`registry.go:22-27`），这是 `app.go:243-250` 用真实 provider 覆盖 nil 能力 parser 的机制；能力通过 `MediaCapabilityChecker` 类型断言统一收敛到 `Support/SupportedTypes`（`support.go:26-30,43-48`）。**追问**：pipeline 侧 `FileInfo` 只传了 Filename（`worker.go:125`），所以入库时**只按扩展名解析，MIME 兜底路径实际未启用**。

### ③ 扫描件 PDF 怎么识别？为什么用双条件？

```go
// internal/loader/validate.go:79-94
total := 0; maxBlock := 0
for _, b := range doc.Blocks {
	n := ReadableCharCount(b.Content)
	total += n
	if n > maxBlock { maxBlock = n }
}
if total < minChars || maxBlock < minChars {
	return &ErrNoReadableContent{Format: doc.Metadata.Format, Readable: maxBlock, MinChars: minChars}
}
```
**要点**：`ReadableCharCount` = 汉字数 + 纯字母 2-20 长度词数 − PDF 操作符白名单（`validate.go:9-18,26-53`）；**单 block 也必须达标**，防「大量 ≤15 字的指令碎片累加过阈值」（`TestValidateReadableFragments` 就是这个 case）；阈值 `min_readable_chars` 默认 20（`config.go:521-523`，YAML `configs/config.yaml:37`）；多媒体豁免（`validate.go:74-77`）。**追问**：三层防线位置 = 上传 strict 预检（`handler_doc.go:325-338`）、入库 tolerant 兜底（`pipeline.go:82-91`）、parser 自身空产检查（`parser_image.go:65-67` / `parser_audio.go:79-81` / `parser_video.go:159-161`）。

### ④ 三个 chunk 策略的默认值到底是几？token 怎么算的？

```go
// internal/chunker/types.go:37-48
func (c ChunkerConfig) WithDefaults() ChunkerConfig {
	if c.ChunkSize <= 0 { c.ChunkSize = 512 }
	if c.ChunkOverlap < 0 { c.ChunkOverlap = 50 }   // ← 注意是 < 0
	if c.HeadingLevel <= 0 { c.HeadingLevel = 2 }
	return c
}
// internal/chunker/tokenizer.go:28-35
for _, r := range text {
	if unicode.Is(unicode.Han, r) { if wordBuf.Len() > 0 { count++; wordBuf.Reset() }; count += 2 }
```
**要点**：代码默认 512，**生产配置 500**（`configs/config.yaml:88`）；overlap 判据 `< 0` 导致「省略=0」；`HeadingLevel=2`；tokenizer 是**启发式估算**：汉字 2 token、标点 1、连续非空白字符算 1 词。**追问**：`chunk_size=500` 对中文 ≈ 250 汉字，与模型真实 BPE 不同源。

### ⑤ chunk_id 是怎么生成的？有幂等键吗？

```go
// internal/pipeline/pipeline.go:116-127
chunkIDs := make([]string, len(chunks))
for i, c := range chunks {
	chunkID := uuid.New().String()          // ← 随机 UUID v4，无 hash
	chunkIDs[i] = chunkID
	records[i] = vectorstore.VectorRecord{
		ID: chunkID, Vector: vectors[i],
		Payload: map[string]any{ "kb_id": req.KBID, "document_id": req.DocumentID, "chunk_id": chunkID, ... },
	}
}
```
**要点**：`uuid.New()`；全仓 `sha256` 只用于 API Key（`app.go:320` 等）；幂等靠 `document_id` 维度前置删除（`pipeline.go:66-73`）而非内容 hash；旁证 `handler_chunk.go:28-31` 用 `uuid.Parse` 校验入参。

### ⑥ 任务领取用没用 SKIP LOCKED？多 worker 会不会抢同一任务？

```go
// internal/store/task.go:73-80
rows, err := s.pool.Query(ctx,
	`UPDATE ingest_tasks SET status = 'processing', updated_at = now()
	 WHERE id IN (
	     SELECT id FROM ingest_tasks WHERE status = 'pending' AND updated_at <= NOW() ORDER BY created_at LIMIT $1
	 )
	 RETURNING id, kb_id, document_id, status, retry_count, error_message, warning_message, created_at, updated_at`, limit)
```
**要点**：**没有 `FOR UPDATE SKIP LOCKED`**。单条 UPDATE...WHERE IN (SELECT) 在 PG 下能保证同一行不被重复置为 processing（`TestClaimPendingTasks` 断言返回状态为 processing），但并发下子查询可能选出已被锁的行，表现为**锁等待 + 部分 worker 白跑**；`claimBatchSize=1`（`worker.go:21`）+ 500ms 空轮询（`worker.go:19`）⇒ 5 个 worker 空闲时约 **10 QPS 的空查询**。

### ⑦ 失败了怎么重试？退避写在哪？

```go
// internal/task/worker.go:149-156
if t.RetryCount < w.cfg.TaskMaxRetries {
	t.Status = store.TaskStatusPending
	t.RetryCount++
	t.ErrorMessage = err.Error()
	backoff := time.Second << uint(t.RetryCount     // 2s/4s/8s…
	t.UpdatedAt = time.Now().Add(backoff)          // 未来时间当 next_attempt_at
}
```
配合 `task.go:77` 的 `updated_at <= NOW()` 过滤生效。**要点**：默认最多 3 次（`config.go:537-539`），实际退避序列 **2s/4s/8s**（因为先自增后移位）；`updated_at` 语义被复用（可被追问监控/清理脚本的正确性）；embedding 内部另有 1s/2s/4s 重试（`embedder.go:76`）叠加。

### ⑧ 重启后未完成的任务怎么办？多实例安全吗？

```go
// internal/task/worker.go:44-47 + internal/store/task.go:99-103
func (w *defaultWorkerPool) Start(ctx context.Context) {
	if err := w.store.ResetProcessingTasks(ctx); err != nil { slog.Warn("重置悬挂任务失败", "err", err) }
	...
// ResetProcessingTasks:
`UPDATE ingest_tasks SET status = 'pending', updated_at = now() WHERE status = 'processing'`
```
**要点**：启动即**无条件**全表重置，无年龄判断、无 owner/lease/heartbeat 字段（表结构 `schema.go:29-39` 里没有）。单实例正确恢复；**多实例滚动重启会把别人正在跑的任务抢回 pending**。

### ⑨ 重复入库会怎样？幂等性怎么保证的？

```go
// internal/pipeline/pipeline.go:66-73
if req.DocumentID != "" {
	if err := p.vectorstore.DeleteByFilter(ctx, map[string]any{"document_id": req.DocumentID}); err != nil {
		slog.Warn("清理旧 chunk 向量失败（继续入库）", "doc", req.DocumentID, "err", err)
	}
	if p.bm25Index != nil { p.bm25Index.RemoveByDoc(req.DocumentID) }
}
```
**要点**：**"删后写"式覆盖幂等**，且删除失败**只告警不中断**（⇒ 可能残留导致重复结果）；同文件二次上传是**新 document_id ⇒ 真重复**；BM25 侧同 id 重复 Add 会先去重（`bm25.go:77-79`）。

### ⑩ PG 与 Qdrant 怎么保证一致？

```go
// internal/pipeline/pipeline.go:141-152
if err := p.vectorstore.Upsert(ctx, records); err != nil { return nil, nil, fmt.Errorf("向量入库失败: %w", err) }
if p.bm25Index != nil {
	for _, rec := range records {
		content, _ := rec.Payload["content"].(string)
		kbID, _ := rec.Payload["kb_id"].(string)
		p.bm25Index.AddWithDocID(rec.ID, content, kbID, req.DocumentID)
	}
}
// internal/task/worker.go:137-142
if err := w.store.UpdateTask(ctx, t); err != nil { slog.Warn("更新任务状态失败", "task", t.ID, "err", err) }
if err := w.store.UpdateDocumentStatus(ctx, t.DocumentID, store.DocStatusCompleted, chunkIDs); err != nil { slog.Warn(...) }
```
**要点**：**无事务、无 outbox、无两阶段**；顺序 = 清旧 → 写 Qdrant(wait=true) → 写 BM25 → 写 PG；PG 更新失败仅告警，靠任务状态机 + 下次 `ResetProcessingTasks` + 前置删除收敛；「Upsert 成功但 PG 更新失败」会让 `chunk_ids` 为空 ⇒ **删除文档时向量删不掉**（`handler_doc.go:307-317` 只按 ChunkIDs 删）。

### ⑪ 为什么用 `context.WithoutCancel`？worker panic 了怎么办？

```go
// internal/task/worker.go:98-101
for _, t := range tasks {
	// 用不随 Shutdown 取消的 ctx 处理任务：保证失败时状态能落库（防残留 processing）
	w.process(context.WithoutCancel(ctx), t)
}
```
**要点**：设计意图是「Shutdown 时让当前任务跑完并把状态写进库」，代价是**没有取消能力、没有 per-task deadline**（唯一兜底是 embedder 硬编码 30s，`embedder.go:37`）。**panic：全包无 `recover()`**（`grep recover()` 在 `internal/task`、`internal/pipeline`、`internal/api`、`internal/app` 零命中）⇒ 业务 panic 直接终止进程，任务停在 processing，重启后重试并可能再次 panic 形成崩溃循环。

### ⑫ 没有背压、没有死信队列——队列会不会被打爆？

**依据**：
```go
// internal/api/handler_doc.go:133-138  上传只做「插一条 pending」，不关心队列深度
if err := h.store.CreateTask(ctx, task); err != nil { Fail(c, CodeInternal, "创建任务记录失败"); return }
OK(c, gin.H{"task_id": taskID, "document_id": docID})

// internal/task/worker.go:19-21  消费端固定速率
pollInterval   = 500 * time.Millisecond
claimBatchSize = 1
```
**要点**：入口无队列长度上限、无准入控制、无按 KB 限流；消费端只有固定数量 worker（默认 2，YAML 5）× 每次 1 条；失败超限直接 `failed` 停在表里，**没有 DLQ / 告警 / 人工干预通道**（只有 `POST /tasks/:id/retry` 手动重试）；任务表随上传无限增长（无 TTL/归档），`ingest_tasks` 只有 `status` 单列索引。

---

## 9. 可被挑刺的缺陷（诚实清单）

**并发与可靠性**

1. **worker 无 panic 恢复**：`worker.go:69-145` 无 `defer recover()`；单个畸形文件导致的第三方库 panic 会终止整个服务进程，且重试会再次命中（崩溃循环）。
2. **多实例不安全**：`ResetProcessingTasks`（`task.go:99-103`）无条件把 processing 打回 pending，**无 lease/owner/heartbeat/任务年龄判断**；表结构也没有这些字段（`schema.go:29-39`）。
3. **无任务级超时/取消**：`context.WithoutCancel`（`worker.go:100`）+ 无 deadline；Shutdown 会被卡住（`worker.go:60-66` 的 `wg.Wait()`），阻塞式下线的唯一上限是 embedder 的 30s HTTP 超时。
4. **领取 SQL 无 `FOR UPDATE SKIP LOCKED`**（`task.go:73-80`），并发下存在锁等待与领取抖动；`claimBatchSize=1` + 500ms 轮询 ⇒ 空转查询量大。
5. **`updated_at` 语义混用**：既是更新时间又是下次可领取时间（`worker.go:155` / `task.go:77`），破坏任何基于 `updated_at` 的监控与归档逻辑。
6. **无背压、无 DLQ、无队列深度上限、无任务 TTL**：上传接口只负责插 task（`handler_doc.go:133`），失败任务永久留在表里。

**一致性与幂等**

7. **无跨存储事务 / 无 outbox**：Qdrant（`pipeline.go:141`）、BM25（`:150`）、PG（`worker.go:137,140`）三处写入只有顺序，PG 更新失败仅 `slog.Warn`。
8. **chunk_id 是随机 UUID，不是内容哈希**（`pipeline.go:118`）⇒ 没有内容级幂等键；同一文件上传两次必然重复入库；删除后重传会产生全新向量。
9. **删除路径有漏洞**：`deleteDocument` 只按 `doc.ChunkIDs` 删（`handler_doc.go:307-317`）；若 `chunk_ids` 为空（PG 更新失败，或 `pipeline.go:95-97` 的零 chunk 成功路径），**向量永久残留**（`DeleteByFilter` 在这条路径上未使用）。
10. **零 chunk 被视为成功**：`pipeline.go:95-97` 返回 `nil, warnings, nil` ⇒ 任务 `completed`、文档 `completed` 但没有任何向量，用户看到"成功"却检索不到。
11. **上传写库无事务**：`CreateDocument` 与 `CreateTask` 两次独立 INSERT（`handler_doc.go:120,133`），第二步失败留下孤儿文件 + 孤儿文档记录。

**BM25 / 检索**

12. **BM25 无持久化、启动不重建**：`Rebuild` 零调用点（`bm25.go:217-231`），重启后 `DocCount()==0` 使 `retriever.go:104` 静默跳过 BM25，hybrid 降级为纯向量且 `method` 谎报逻辑也无提示。

**分块质量**

13. **递归策略丢失分隔符**：`strings.Split(text, sep)` 后不回填（`strategy_recursive.go:52-64`）⇒ 超过 `chunk_size` 的文档会**丢掉 `。！？.!?`**；现有测试用无标点文本（`chunker_test.go:116-121`）恰好绕过。
14. **`extractFirstHeadingContext` 名不副实**：遍历完全部 block 后取栈顶（`chunker.go:245-259`），返回的是**文档最后一个**标题路径，并被赋给该文档**每一个** chunk（recursive/fixed 路径，`chunker.go:77,82`）。
15. **`ChunkOverlap` 默认值失效**：判据 `< 0`（`types.go:41-43`、`config.go:402-404`），YAML 省略即 0 重叠，与「默认 50」注释矛盾。
16. **多媒体 chunk 完全绕过 `chunk_size`**：`chunkMediaBlocks` 不做任何切分（`chunker.go:136-162`），一段 10 分钟视频的长转写会成为一个超大 chunk（受 block 粒度限制，但 block 本身可很长）。
17. **表格/代码块无保护**：既无表头补全、也无跨 chunk 保护（`strategy_heading.go:104-106`、`strategy_recursive.go:13`）。
18. **tokenizer 是估算器**（`tokenizer.go:20-57`）：汉字计 2 token，与目标模型 BPE 不一致 ⇒ `chunk_size` 与上下文窗口预算不同源，可能低估或高估。

**性能**

19. **固定策略的 token 计数是 O(chunkSize²)**：`findChunkEnd` 收缩/扩展循环每步都全量子串 `Count`（`strategy_fixed.go:68-75`）；`findOverlapStart` 同理（`:108-119`）。
20. **上传预检把文件完整解析一遍**（`handler_doc.go:325-338`，strict 模式），入库时再解析一遍 ⇒ **CPU 双倍**；且预检用的是 `file.Open()` 新流，与随后 `SaveUploadedFile` 之间存在 TOCTOU 窗口（内容可不同）。
21. **无 loader 层体积/页数保护**：`io.ReadAll` 全量进内存（`parser_pdf.go:29`、`parser_docx.go:27`、`parser_markdown.go:28`、`parser_image.go:50`、`parser_audio.go:48`）；`PageCount` 只写元数据不设上限（`parser_pdf.go:44,95`）；**页数/行数/sheet 上限在代码中未找到**。
22. **Qdrant Upsert 无重试**：embedding 有 3 次重试（`embedder.go:74`），向量写失败直接让任务重跑（重跑意味着重新 embedding + 重新删除，成本高）。

**数据模型**

23. **PG 里没有 chunk 表**，内容只在 Qdrant payload（`pipeline.go:120-138`）⇒ 无法用 SQL 做 chunk 统计/一致性校验/重建；`documents.chunk_ids TEXT[]` 随 chunk 数线性膨胀且无上限。
24. **`documents` 无 `metadata JSONB`、无 `updated_at`**（`schema.go:15-27`）；文档页数、MIME、媒体时长等全部丢失在入库后（只存在于 `DocumentMeta`，未落库）。
25. **`ingest_tasks.document_id` / `kb_id` 无外键**（`schema.go:31-32`）⇒ 删除文档后任务记录成为悬挂数据；删除 KB 有 CASCADE 删 documents，但**不删 ingest_tasks**（级联不到）。
26. **索引不足**：`ClaimPendingTasks` 的过滤条件是 `status + updated_at + ORDER BY created_at`，但只有 `idx_ingest_tasks_status`（`schema.go:39`）。
27. **`DocStatusProcessing` 是死常量**（`store.go:32`，全仓非测试零写入）⇒ 入库中的文档对外一直显示 `pending`。

**其它**

28. **`ErrNoReadableContent.Format` 被填成文件名**（`pipeline.go:83-87` 用 `req.Info.Filename`），与同类型在 `validate.go:90` 的语义（格式名）不一致。
29. **`DocumentMeta.Size` 在入库路径恒为 0**：worker 只传 `FileInfo{Filename}`（`worker.go:125`），而 `loader.go:63` 用 `info.Size` 覆盖 ⇒ 解析结果里的大小信息丢失。
30. **加密 PDF 的失败原因被表述为「文档无可读文本」**，用户无法区分「加密」「损坏」与「扫描件」（`parser_pdf.go:46-53` + `pipeline.go:82-88`；`grep -i encrypt` 在 `internal/loader` 零命中）。
31. **手动重试可无限重置 `RetryCount=0`**（`handler_task.go:79`），无重试总次数/频率限制；且不清空 `warning_message`。
32. **`LoadOptions.Filename` 的传递不对称**：pipeline 调用 `loader.Load` 时不带 `LoadOptions`（`pipeline.go:76`），`opt.Filename` 只能从 `info.Filename` 回落（`loader.go:43-48`）——功能上正确，但意味着**无法在入库路径上切换 `ModeStrict`**（多媒体的 `source` 元数据依赖它，实际仍能工作）。

---

## 附：明确「代码中未找到」的项

- `documents`/`ingest_tasks` 的 **metadata JSONB 列**（grep `jsonb` 零命中）
- **PDF 页数上限、CSV 行数上限、Excel sheet 数上限**常量
- **加密 PDF 的显式检测**（无 `encrypt`/`password` 判定逻辑）
- **`FOR UPDATE SKIP LOCKED`**、任务 lease/owner/heartbeat 字段
- **业务 goroutine 的 `recover()`**
- **死信队列 / 任务归档 / 队列深度上限 / 背压**
- **chunk 内容哈希 / 内容去重表 / 文档 updated_at / 内容变更检测**
- **BM25 索引的持久化或启动重建调用点**（`Rebuild` 无生产调用者）
- **入库路径上的 MIME 类型传递**（worker 只传 Filename）
- **`internal/embedding` 的 HTTP 超时可配置项**（30s 硬编码于 `embedder.go:37`）

---

> 主要参考文件（均为只读分析，未做任何修改）：`internal/loader/`、`internal/chunker/`、`internal/task/worker.go`、`internal/pipeline/pipeline.go`、`internal/store/{document,task,schema,store}.go`、`internal/vectorstore/qdrant.go`、`internal/embedding/embedder.go`、`internal/api/{handler_doc,handler_task,handler_chunk,router}.go`、`internal/app/{app,rebuild}.go`、`internal/retriever/{bm25,retriever}.go`、`internal/multimedia/provider.go`、`configs/config.yaml`。
