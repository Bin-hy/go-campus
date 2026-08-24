// Milvus 全链路集成测试。
// 无 Milvus 环境时自动跳过（t.Skip），保证 `go test ./...` 在任意环境通过；
// 本地起了 Milvus standalone 后（docker compose -f ../docker-compose.milvus.yml up -d）
// 会自动执行完整链路验证：建集合 → 插入 → 建索引 → Load → 检索 → 清理。
package main

import (
	"context"
	"math/rand"
	"testing"
	"time"

	commonpb "github.com/milvus-io/milvus-proto/go-api/v2/commonpb"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const testColl = "milvus_demo_test"

func TestMilvusEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := client.NewClient(ctx, client.Config{Address: milvusAddr})
	if err != nil {
		t.Skipf("Milvus 未运行，跳过集成测试（起法：docker compose -f ../docker-compose.milvus.yml up -d）: %v", err)
	}
	defer c.Close()

	// 清理历史残留
	if has, _ := c.HasCollection(ctx, testColl); has {
		_ = c.DropCollection(ctx, testColl)
	}

	// 建集合
	schema := entity.NewSchema().
		WithName(testColl).
		WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true)).
		WithField(entity.NewField().WithName("text").WithDataType(entity.FieldTypeVarChar).WithMaxLength(256)).
		WithField(entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(8))
	if err := c.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	t.Cleanup(func() { _ = c.DropCollection(ctx, testColl) })

	// 插入 3 条
	ids := []int64{1, 2, 3}
	texts := []string{"go 并发", "redis 缓存", "milvus 向量"}
	vecs := make([][]float32, 3)
	for i := range vecs {
		vecs[i] = []float32{rand.Float32(), rand.Float32(), rand.Float32(), rand.Float32(),
			rand.Float32(), rand.Float32(), rand.Float32(), rand.Float32()}
	}
	if _, err := c.Insert(ctx, testColl, "",
		entity.NewColumnInt64("id", ids),
		entity.NewColumnVarChar("text", texts),
		entity.NewColumnFloatVector("vector", 8, vecs)); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := c.Flush(ctx, testColl, false); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// 建索引 + 轮询到 Finished
	idx, err := entity.NewIndexHNSW(entity.COSINE, 16, 64)
	if err != nil {
		t.Fatalf("NewIndexHNSW: %v", err)
	}
	if err := c.CreateIndex(ctx, testColl, "vector", idx, false); err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}
	for {
		state, err := c.GetIndexState(ctx, testColl, "vector")
		if err != nil {
			t.Fatalf("GetIndexState: %v", err)
		}
		if state == entity.IndexState(commonpb.IndexState_Finished) {
			break
		}
		if state == entity.IndexState(commonpb.IndexState_Failed) {
			t.Fatal("index build failed")
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Load + Search
	if err := c.LoadCollection(ctx, testColl, false); err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	sp, err := entity.NewIndexHNSWSearchParam(64)
	if err != nil {
		t.Fatalf("NewIndexHNSWSearchParam: %v", err)
	}
	res, err := c.Search(ctx, testColl, nil, "",
		[]string{"text"}, []entity.Vector{entity.FloatVector(vecs[0])},
		"vector", entity.COSINE, 3, sp)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) == 0 || res[0].ResultCount == 0 {
		t.Fatal("Search 未返回任何结果")
	}
	t.Logf("检索成功，命中 %d 条，top1 score=%.4f", res[0].ResultCount, res[0].Scores[0])

	// 带过滤检索：应只剩 id=1
	res2, err := c.Search(ctx, testColl, nil, `id == 1`,
		[]string{"text"}, []entity.Vector{entity.FloatVector(vecs[0])},
		"vector", entity.COSINE, 3, sp)
	if err != nil {
		t.Fatalf("过滤 Search: %v", err)
	}
	if res2[0].ResultCount != 1 {
		t.Fatalf("过滤检索期望 1 条，实际 %d 条", res2[0].ResultCount)
	}
	idCol := res2[0].IDs.(*entity.ColumnInt64)
	got, _ := idCol.ValueByIdx(0)
	if got != int64(1) {
		t.Fatalf("过滤结果主键期望 1，实际 %v", got)
	}
	t.Log("过滤检索正确 ✔")

	// 删除
	if err := c.Delete(ctx, testColl, "", `id in [1, 2]`); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	t.Log("全链路通过：建集合 → 插入 → 建索引 → Load → 检索 → 过滤 → 删除 ✔")
}
