# mapping 建错一次，整个索引要重建—— Mapping 设计与调优

> 属于 S11 Elasticsearch · 第六篇
> 上一篇：[近实时、并发控制与索引重建](./05-近实时并发控制与索引重建)
> 下一篇：[生产运维与故障排查](./07-生产运维与故障排查)

ES 的运维事故里，**mapping 设计错误**和**分片规划错误**占了大半。前者表现为"这个字段排不了序""这个查询怎么这么慢""为什么价格是字符串"，后者表现为"集群一重启恢复要 40 分钟""协调节点 OOM"。这两件事的共同点是：**它们在建索引那一天就定型了，后面只能 reindex 补救**。

这一篇是一份"建索引之前过一遍"的检查表：字段类型怎么选、`dynamic` 要不要关、`nested` 什么时候必须用、分片数怎么算、写入和查询分别怎么调。

## 一、字段类型：先分清 text 和 keyword

这是 ES mapping 的第一道分水岭，也是最常被问的：

| | `text` | `keyword` |
|--|--------|-----------|
| 是否分词 | ✔（倒排表存切好的词） | ✘（整串当一个词） |
| 适合 | 标题、描述、正文、ASR 字幕 | 状态、ID、SKU、标签、枚举、日期字符串 |
| 能排序/聚合吗 | ✘（没有 doc_values） | ✔（doc_values 默认开） |
| `term` 查询 | 查不到（除非整串恰好是一个 token） | ✔ 精确匹配 |
| 典型查询 | `match` / `match_phrase` / `multi_match` | `term` / `terms` / `prefix` |

一条商品文档的推荐写法（同时支持两种用法）：

```json
PUT /products
{
  "mappings": {
    "properties": {
      "title":      { "type": "text", "analyzer": "ik_max_word", "search_analyzer": "ik_smart",
                      "fields": { "kw": { "type": "keyword", "ignore_above": 256 } } },
      "sku":        { "type": "keyword" },
      "brand":      { "type": "keyword" },
      "status":     { "type": "keyword" },
      "price":      { "type": "scaled_float", "scaling_factor": 100 },
      "sales_30d":  { "type": "integer", "index": false },
      "created_at": { "type": "date", "format": "strict_date_optional_time||epoch_millis" },
      "desc":       { "type": "text", "analyzer": "ik_max_word" },
      "tags":       { "type": "keyword" },
      "image_url":  { "type": "keyword", "index": false, "doc_values": false }
    }
  }
}
```

几个容易选错的类型：

- **金额用 `scaled_float`（`scaling_factor: 100`）而不是 `float`**：`float` 有精度问题，`scaled_float` 内部存整数分，聚合和排序不出错；也可以用 `long` 存"分"；
- **ID/编号/手机号一律 `keyword`**：用 `long` 会丢前导零、超出精度；用 `text` 会被切碎；
- **不用索引的字段加 `"index": false`**：省倒排表和内存（如"销量"只用来排序/展示，不需要被搜）；
- **`doc_values: false`** 用于既不排序也不聚合的字段（如 URL），省列存；
- **日期统一用 `date`**：字符串日期（`keyword`）无法做 `range`/`date_histogram`；`strict_` 系列格式能拦住脏数据。

| 字段参数 | 默认 | 作用 |
|---------|------|------|
| `index` | true | 是否建倒排（false = 不可搜） |
| `doc_values` | 非 text 为 true | 列存，排序/聚合/脚本用 |
| `norms` | text 为 true | 字段长度归一化（BM25 的 `b` 用）；不排序的 keyword 可关，省内存 |
| `store` | false | 是否额外存一份可单独取（一般用 `_source` 就够） |
| `_source` | 启用 | 存原始 JSON，用于返回/更新/reindex；大字段可 exclude |
| `enabled` | true | 整个对象是否建索引（false = 只存不搜） |
| `ignore_above` | 无 | keyword 超长截断不索引（防超长字符串撑爆） |

## 二、dynamic 与 mapping explosion：别让字段无限膨胀

`dynamic` 决定"遇到未定义的字段怎么办"：

| 值 | 行为 | 场景 |
|----|------|------|
| `true`（默认） | 自动推断类型并加进 mapping | 开发期方便，**生产大忌** |
| `false` | 不索引新字段（进 `_source` 但不搜） | 日志/半结构化数据 |
| `strict` | 直接拒绝写入 | 核心业务索引（强 schema） |

**mapping explosion 是最真实的线上事故**：某个服务把 URL 的 query 参数、或用户标签（每个用户一个字段名）写进 ES，字段数从 50 涨到 5 万，`cluster state` 膨胀到几百 MB，master 推送状态把网络打满，整个集群"假死"。ES 自带两道闸：`index.mapping.total_fields.limit`（默认 1000）、`index.mapping.depth.limit`（默认 20）、`nested objects` 上限（默认 50）。

正确的兜底姿势：**`dynamic: strict` + `dynamic_templates` 兜住未知字段**：

```json
PUT /logs
{
  "mappings": {
    "dynamic": "strict",
    "dynamic_templates": [
      { "strings_as_keywords": { "match_mapping_type": "string",
                                 "mapping": { "type": "keyword", "ignore_above": 512 } } },
      { "labels": { "path_match": "labels.*",
                    "mapping": { "type": "keyword" } } }
    ],
    "properties": { "@timestamp": { "type": "date" }, "message": { "type": "text" } }
  }
}
```

半结构化 JSON（第三方回调、埋点属性）不想一个个建字段，用 `flattened` 类型：整个对象当一个字段，叶子值都按 keyword 索引，**不会产生 N 个字段**，代价是不能对子字段做精确类型操作（只能整体过滤/聚合）。

## 三、nested 还是 object：数组对象的经典坑

ES 的 `object` 类型会把**数组元素扁平化**，丢掉"哪个值与哪个值属于同一条记录"的边界。看这个例子：

```json
// 文档
{ "sku": "A1", "specs": [ { "color": "红", "size": "XL" }, { "color": "蓝", "size": "M" } ] }

// object 映射下实际索引成：
"specs.color": ["红", "蓝"],  "specs.size": ["XL", "M"]
```

于是查询 `specs.color = 红 AND specs.size = M` **会命中**——库里根本没有"红色 M 码"的组合。这是电商/商品规格的经典 bug。

| 方案 | 怎么建 | 优点 | 代价 |
|------|--------|------|------|
| `object` | 默认 | 快、省 | 丢失数组元素边界，跨字段组合条件会误命中 |
| `nested` | `"type": "nested"` | 每个元素独立成 Lucene 文档，组合条件正确 | 每个 nested 文档都进索引，**查询要 `nested` 包装**，跨 nested 排序/聚合受限，**写放大** |
| `flattened` | `"type": "flattened"` | 字段不膨胀 | 只能整体过滤，无类型区分 |

```json
GET /products/_search
{ "query": { "nested": { "path": "specs",
    "query": { "bool": { "must": [ { "term": { "specs.color": "红" } },
                                    { "term": { "specs.size": "M" } } ] } } } } }
```

选型口诀：**规格/属性数组、要做组合条件 → `nested`；只是整体展示/搜索 → `object`；字段名本身不可控 → `flattened`**。`nested` 文档数 = 父文档 × 数组长度，1 亿文档 × 5 个规格 = 5 亿 nested 文档，要算进容量里。

## 四、分片规划：先用公式算，再拍脑袋

第三篇给了经验值，这里给可执行的算法：

```text
① 日增数据量 ≈ 日增文档数 × 单文档大小
② 总容量   ≈ 日增 × 保留天数 × (1 + 副本数)
③ 主分片数 ≈ 总容量 / 单分片目标大小（10~50GB，日志类取大、商品类取小）
④ 堆内存   ≈ 主分片总数 × 每个分片内存开销（~1~2MB 元数据）+ 热点数据量 × 搜索缓存需求
⑤ 校验：分片总数（含副本）≤ 堆 GB 数 × 20
```

以"商品索引"为例：

| 输入 | 值 |
|------|-----|
| 文档数 | 500 万 |
| 单文档 | 1KB（含文本） |
| 副本 | 1 |
| 总容量 | 500 万 × 1KB × 2 ≈ 10GB |
| 单分片目标 | 30GB → **1 个主分片就够**，但为了并行与未来增长，取 **3 个主分片** |
| 结论 | 3 主 × 1 副 = 6 分片，2~3 台数据节点，堆 16GB |

**过度分片的代价**（面试常问"分片多有什么坏处"）：

- 每个分片是一个 Lucene 实例，**堆内存里有段元数据、filter cache、query cache**，分片多 → 每 X GB 堆承载的分片数超限 → 集群状态推送慢、master 变慢；
- 查询要扇出到所有分片，分片多 → 协调节点归并压力大、尾延迟升高；
- 恢复/重启时每个分片都要 rebalance，分片多 → 恢复时间线性增长。

反过来，分片太少则写入并行度不够、单分片过大导致恢复慢。**经验区间：单分片 10~50GB，集群分片总数（含副本）控制在"堆 GB × 20"以内。**

## 五、写入调优：批量导入快 5~10 倍的套路

批量导入（初始化、重建索引）时，按顺序做这几件事：

```json
// 1. 临时调低副本数（导入完改回）
PUT /products/_settings { "number_of_replicas": 0 }
// 2. 关闭定时 refresh
PUT /products/_settings { "refresh_interval": "-1" }
// 3. translog 改为异步（可重放的数据才这么干）
PUT /products/_settings { "translog.durability": "async" }

// 4. bulk 写入：单批 5~15MB 或 1000~5000 条，并发 2~4 路
POST /_bulk
// 5. 导入完成后
PUT /products/_settings { "refresh_interval": "1s", "number_of_replicas": 1 }
POST /products/_refresh
```

| 参数 | 建议 | 原因 |
|------|------|------|
| bulk 批大小 | 5~15MB / 1000~5000 条 | 太小网络往返多，太大内存压力与单批失败回滚成本高 |
| 并发度 | 2~4（不超过 CPU 核数的一半） | 太高会打满 merge 线程池 |
| `refresh_interval` | 导入期 `-1` | 少生成段，少 merge |
| `number_of_replicas` | 导入期 `0` | 少一次网络同步，速度翻倍 |
| `translog.durability` | 导入期 `async` | 少 fsync |

**注意 merge**：批量导入会产生大量小段，导入后主动 `_forcemerge`（只对**只读索引**做，不要对还在写的索引做）把段合并成 1 个，能显著降低后续查询和恢复的开销。

## 六、查询调优：哪些写法会拖垮集群

| 反模式 | 为什么慢 | 替代方案 |
|--------|---------|---------|
| `wildcard: "*关键词*"` / `regexp` | 前导通配要扫描全部词，无法用 FST 定位 | 分词字段用 `match`；前缀用 `edge_ngram`/`search_as_you_type`；`index_prefixes` |
| 大 `terms` 列表（几万个 ID） | 构造巨型过滤器，内存与 CPU 双杀 | `terms` lookup（预存 ID 列表到另一个索引）、或改成分批查询 |
| `script` 查询/排序 | 每个文档执行一次脚本，无法缓存 | 预处理成字段（写入时算好），用 `doc_values` |
| `nested` 聚合/排序大基数 | 每个 nested 文档单独扫 | 用 `nested` + 子聚合限制 `size`，或预先物化 |
| `size` 很大 + `from` 很深 | 每个分片都要凑候选 | `search_after` + PIT |
| 对 text 字段排序/聚合 | 无 doc_values，走 fielddata（堆内存） | 加 keyword 子字段 |
| 聚合不带 filter | 全量桶，触发 circuit breaker | 用 `filter` 收窄 + 调 `shard_size` 下限 |

诊断工具：

```json
GET /products/_search
{ "profile": true, "query": { ... } }   // 逐个子查询的耗时/文档数，定位慢在哪一层

PUT /products/_settings
{ "index.search.slowlog.threshold.query.warn": "1s",
  "index.search.slowlog.threshold.fetch.warn": "500ms",
  "index.indexing.slowlog.threshold.index.warn": "500ms" }
```

`profile: true` 的输出能看到每个 query/filter 命中多少文档、耗时多少——**filter 命中数巨大而返回很少，通常意味着过滤条件写得太宽或字段类型不对**。

## 七、模板、别名与索引生命周期

生产上索引不是手建的，都用模板：

```json
PUT /_index_template/products_template
{
  "index_patterns": ["products_v*"],
  "priority": 200,
  "template": {
    "settings": { "number_of_shards": 3, "number_of_replicas": 1,
                  "refresh_interval": "1s" },
    "mappings": { "dynamic": "strict", "properties": { "title": { "type": "text" }, "sku": { "type": "keyword" } } },
    "aliases": { "products": {} }
  }
}
```

- **`_index_template`**：按名字模式匹配，新建索引自动套用（组件模板 `_component_template` 用来拼公共片段，比如"所有索引都带 ILM 设置"）；
- **别名 `products`**：应用永远只读写别名，物理索引可随意版本化（`products_v1` → `products_v2`）；
- **`data stream`**：只追加的时间序列数据（日志、指标），自动滚动（rollover）+ 配合 ILM 冷热分层自动迁移与删除。日志场景用它，不要手建 365 个按天索引。

::: warning 面试追问（3 层）
**Q1：`text` 和 `keyword` 怎么选？**——要看查询方式：需要分词检索（match）用 text，需要精确匹配/排序/聚合用 keyword。标题这类"既要搜又要精确"的字段用 `fields` 建双份（`title` + `title.kw`）。注意 text 没有 doc_values，直接排序聚合会退化成 fielddata 吃堆内存。
**Q2：为什么 `dynamic: true` 在生产是风险？**——未定义字段会被自动推断类型并永久写入 mapping，一旦推断错（比如 `"code": "001"` 推断成 long 丢前导零，或嵌套结构展开成上万字段）就要 reindex；更糟的是 mapping explosion 会让 cluster state 膨胀、集群假死。生产一律 `strict` + `dynamic_templates` 兜底。
**Q3：什么时候必须用 `nested`，代价是什么？**——数组对象需要"组合条件精确匹配同一元素"时（红 + XL）；代价是每个 nested 元素是独立 Lucene 文档（写放大、存储变大），查询必须走 `nested` 查询（慢一些），且不能对 nested 字段直接排序/聚合，要用 `inner_hits` 或物化到父文档。
:::

---

## 串起来

mapping 的三个定调决策：**字段类型**（text/keyword/数值/日期）、**dynamic 策略**（生产 strict + 模板兜底）、**数组对象**（组合条件 → nested）。分片规划用"容量 → 主分片数 → 堆内存 → 分片总上限"四步公式，别凭手感。写入调优三板斧：临时降副本、关 refresh、bulk 批量；读调优先想"能 filter 就 filter、别做前导通配、别用 script、别深分页"。最后，把建索引这件事**模板化 + 别名化**，才有后面 reindex 零停机重建的余地。

下一篇是运维视角的实战：**集群变红、分片分配不上、写入被拒、磁盘打满、断路器触发**——怎么定位、怎么恢复。

> 下一篇：[生产运维与故障排查](./07-生产运维与故障排查)
