# 10-milvus：Milvus 向量检索 Go 实验

配套文档：[S10 向量数据库 Milvus](/后端技术栈强化/10-milvus/01-向量检索原理)（8 篇，从向量检索原理到 Go 实战）。

## 快速开始

```bash
# 1) 起 Milvus standalone（etcd + MinIO + Milvus，首次拉镜像 1~3 分钟）
docker compose -f ../docker-compose.milvus.yml up -d

# 2) 等健康检查通过
curl http://localhost:9091/healthz        # 期望返回 {"status":"OK"}

# 3) 跑全链路演示（连接→建集合→插入→建索引→Load→检索→过滤→清理）
go run .

# 4) 跑集成测试（Milvus 未运行时自动跳过，不影响 go test ./...）
go test -v .
```

## 文件说明

| 文件 | 说明 |
|------|------|
| `main.go` | 七步全链路演示：NewClient → CreateCollection(Schema) → Insert(列式) → CreateIndex(HNSW) → Load → Search → 过滤检索 → 清理 |
| `milvus_test.go` | 集成测试：无 Milvus 时自动 Skip；有 Milvus 时验证完整链路 + 过滤正确性 |

## 常见坑

- **端口**：SDK 连 **19530**（gRPC）；9091 只是健康检查/指标端口。
- **维度**：插入向量维度必须与 Schema `WithDim` 一致，否则报 `dimension mismatch`。
- **索引异步**：`CreateIndex` 返回不代表建完，必须轮询 `GetIndexState` 到 `Finished` 再 Load，否则查询退回暴力扫描。
- **没 Load 就 Search**：报 `collection not loaded`，先 `LoadCollection`。
- **一致性**：默认 Bounded（容忍约 5 秒陈旧），写入后立即查可能看不到，需要时用 `ClStrong`/`ClSession`。
