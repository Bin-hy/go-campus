// Milvus Go SDK 全链路演示（配套 docs/后端技术栈强化/10-milvus/07-Milvus-Go实战）
//
// 前置：docker compose -f ../docker-compose.milvus.yml up -d
//       （起 etcd + MinIO + Milvus standalone，等健康检查通过）
//
// 运行：cd code/backend/10-milvus && go run .
// 链路：连接 → 建 Collection(Schema) → 插入(列式) → 建索引(HNSW) → Load → Search(top5)
//       → 带过滤检索 → 清理(Delete/Release/Drop)
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	commonpb "github.com/milvus-io/milvus-proto/go-api/v2/commonpb"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	// 连接地址：Milvus standalone 的 gRPC 端口（9091 是指标/健康检查，不是 SDK 端口）
	milvusAddr = "localhost:19530"
	// 演示集合名（生产按业务命名，如 video_clip / doc_chunks）
	collName = "doc_chunks"
	// 演示用 8 维伪向量；生产用 embedding 模型的实际维度（768/1024），且维数写死成常量
	dim = 8
)

func main() {
	ctx := context.Background()

	// ① 连接
	c, err := client.NewClient(ctx, client.Config{
		Address: milvusAddr,
		// Username/Password: 开了鉴权才需要
	})
	if err != nil {
		log.Fatalf("连接失败（先确认 Milvus 已起：docker compose -f ../docker-compose.milvus.yml up -d）: %v", err)
	}
	defer c.Close()
	fmt.Println("① connected to milvus ✔")

	// 建集合前先查重，存在则重建（演示环境直接重建）
	has, err := c.HasCollection(ctx, collName)
	if err != nil {
		log.Fatalf("HasCollection 失败: %v", err)
	}
	if has {
		_ = c.DropCollection(ctx, collName)
	}

	// ② 定义 Schema 并建 Collection（向量数据库的"建表"）
	schema := entity.NewSchema().
		WithName(collName).
		WithDescription("文档切块向量（Go SDK 演示）").
		WithField(entity.NewField().
			WithName("id").WithDataType(entity.FieldTypeInt64).
			WithIsPrimaryKey(true).WithIsAutoID(false)).
		WithField(entity.NewField().
			WithName("text").WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(1024)). // VarChar 必须给 MaxLength，否则 SDK 校验报错
		WithField(entity.NewField().
			WithName("category").WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(64)).
		WithField(entity.NewField().
			WithName("vector").WithDataType(entity.FieldTypeFloatVector).
			WithDim(dim)) // 维度必须与插入向量一致

	if err := c.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		log.Fatalf("建集合失败: %v", err)
	}
	fmt.Println("② collection created ✔")

	// ③ 插入数据：列式插入（每个字段传一整个列）；生产用 embedding 模型产出真实向量
	n := 100
	ids := make([]int64, 0, n)
	texts := make([]string, 0, n)
	cats := make([]string, 0, n)
	vecs := make([][]float32, 0, n)
	for i := 0; i < n; i++ {
		ids = append(ids, int64(i))
		texts = append(texts, fmt.Sprintf("这是第 %d 篇文档的切块内容", i))
		cats = append(cats, []string{"go", "redis", "k8s"}[i%3])
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = rand.Float32()
		}
		vecs = append(vecs, vec)
	}

	// Insert 返回主键列（entity.Column），用 Len() 拿插入条数
	pkCol, err := c.Insert(ctx, collName, "",
		entity.NewColumnInt64("id", ids),
		entity.NewColumnVarChar("text", texts),
		entity.NewColumnVarChar("category", cats),
		entity.NewColumnFloatVector("vector", dim, vecs),
	)
	if err != nil {
		log.Fatalf("插入失败: %v", err)
	}
	fmt.Printf("③ inserted %d entities ✔\n", pkCol.Len())

	// Flush：内存数据落盘成 segment，数据才能被索引/稳定查询
	if err := c.Flush(ctx, collName, false); err != nil {
		log.Fatalf("Flush 失败: %v", err)
	}

	// ④ 建索引：HNSW + COSINE（文本语义检索最常用度量）
	idx, err := entity.NewIndexHNSW(entity.COSINE, 16, 64) // M=16, efConstruction=64
	if err != nil {
		log.Fatalf("构造索引失败: %v", err)
	}
	if err := c.CreateIndex(ctx, collName, "vector", idx, false); err != nil {
		log.Fatalf("建索引失败: %v", err)
	}
	// 索引构建是异步的：轮询到 Finished 再继续（否则 load 后可能还在暴力扫描）
	// 注意：GetIndexState 返回 entity.IndexState（底层是 commonpb 枚举，需显式转换比较）
	fmt.Println("④ index building...")
	for {
		state, err := c.GetIndexState(ctx, collName, "vector")
		if err != nil {
			log.Fatalf("GetIndexState 失败: %v", err)
		}
		if state == entity.IndexState(commonpb.IndexState_Finished) {
			break
		}
		if state == entity.IndexState(commonpb.IndexState_Failed) {
			log.Fatal("索引构建失败")
		}
		fmt.Printf("   index state: %v\n", state)
		time.Sleep(time.Second)
	}

	// ⑤ Load：把数据+索引进 querynode 内存，之后才能检索
	if err := c.LoadCollection(ctx, collName, false); err != nil {
		log.Fatalf("Load 失败: %v", err)
	}
	fmt.Println("⑤ loaded ✔")

	// ⑥ 检索 top5：拿第 0 条数据的向量去搜（生产：用同一 embedding 模型编码查询文本）
	queryVec := entity.FloatVector(vecs[0])
	sp, err := entity.NewIndexHNSWSearchParam(128) // ef=128：召回/延迟的平衡点
	if err != nil {
		log.Fatalf("构造检索参数失败: %v", err)
	}
	sRet, err := c.Search(ctx, collName, nil, "",
		[]string{"text", "category"}, // 输出字段：除了分数，还想带回哪些标量
		[]entity.Vector{queryVec},
		"vector", entity.COSINE, 5, sp)
	if err != nil {
		log.Fatalf("Search 失败: %v", err)
	}
	fmt.Printf("⑥ top5 hits:\n")
	printHits(sRet[0])

	// ⑦a 带过滤检索：向量相似 + 标量条件（expr 语法和 SQL where 很像）
	expr := `category == "go" and id > 10`
	sRet2, err := c.Search(ctx, collName, nil, expr,
		[]string{"text"}, []entity.Vector{queryVec},
		"vector", entity.COSINE, 5, sp)
	if err != nil {
		log.Fatalf("过滤检索失败: %v", err)
	}
	fmt.Printf("⑦a filtered search (category==go and id>10):\n")
	printHits(sRet2[0])

	// ⑦b 清理：删除 / 释放内存 / 删集合
	if err := c.Delete(ctx, collName, "", `id in [0, 1, 2]`); err != nil {
		log.Fatalf("Delete 失败: %v", err)
	}
	fmt.Println("⑦b deleted id in [0,1,2] ✔")
	if err := c.ReleaseCollection(ctx, collName); err != nil {
		log.Fatalf("Release 失败: %v", err)
	}
	if err := c.DropCollection(ctx, collName); err != nil {
		log.Fatalf("Drop 失败: %v", err)
	}
	fmt.Println("⑦c released & dropped ✔")
	fmt.Println("all done ✔")
}

// printHits 打印检索结果：主键 + 分数 + 输出字段
func printHits(res client.SearchResult) {
	// 主键列：int64 主键
	idCol := res.IDs.(*entity.ColumnInt64)
	// 输出字段按名字找
	var textCol *entity.ColumnVarChar
	for _, f := range res.Fields {
		if f.Name() == "text" {
			textCol = f.(*entity.ColumnVarChar)
		}
	}
	for i := 0; i < int(res.ResultCount); i++ {
		id, _ := idCol.ValueByIdx(i)
		t, _ := textCol.ValueByIdx(i)
		fmt.Printf("   id=%-4d score=%.4f text=%s\n", id, res.Scores[i], t)
	}
}
