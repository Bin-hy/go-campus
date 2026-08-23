package sts_assume_role

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	endpoint  = "127.0.0.1:9000"
	accessKey = "minioadmin"
	secretKey = "minioadmin"
)

func uniqueBucket(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// adminClient 用长期 root 凭证构建的全权限客户端——只在测试里扮演「后端」，
// 负责建桶、播种素材、清理。真正被考察的 scoped 客户端由 AssumeScopedRole 产出。
func adminClient(t *testing.T) *minio.Client {
	t.Helper()
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("创建 MinIO 客户端失败: %v", err)
	}
	if _, err := client.ListBuckets(context.Background()); err != nil {
		t.Fatalf("连接 MinIO 失败（确认已 docker compose up -d minio）: %v", err)
	}
	return client
}

func putObject(t *testing.T, client *minio.Client, bucket, object, content string) {
	t.Helper()
	_, err := client.PutObject(context.Background(), bucket, object,
		bytes.NewReader([]byte(content)), int64(len(content)), minio.PutObjectOptions{})
	if err != nil {
		t.Fatalf("放入对象 %s 失败: %v", object, err)
	}
}

func cleanupBucket(t *testing.T, client *minio.Client, bucket string) {
	t.Helper()
	ctx := context.Background()
	for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
		if obj.Err != nil {
			continue
		}
		_ = client.RemoveObject(ctx, bucket, obj.Key, minio.RemoveObjectOptions{})
	}
	_ = client.RemoveBucket(ctx, bucket)
}

// isAccessDenied 判定错误是否为「被授权策略拒绝」。首选解析 S3 结构化错误码，
// 再兜底匹配文本，避免因包装层差异漏判。
func isAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	if minio.ToErrorResponse(err).Code == "AccessDenied" {
		return true
	}
	return strings.Contains(err.Error(), "AccessDenied")
}

func TestAssumeScopedRole(t *testing.T) {
	ctx := context.Background()
	admin := adminClient(t)
	bucket := uniqueBucket("sts")
	if err := admin.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("建桶失败: %v", err)
	}
	defer cleanupBucket(t, admin, bucket)

	// 后端(admin)播种两个用户的素材：alice 与 bob。
	const aliceMaterial = "alice-take1-material"
	putObject(t, admin, bucket, "clips/alice/take1.mp4", aliceMaterial)
	putObject(t, admin, bucket, "clips/bob/take1.mp4", "bob-take1-material")

	// 后端向 STS 换一组「只圈在 clips/alice/ 内、会过期」的临时凭证交给 alice 客户端。
	// ttl 传 1h：注意 minio-go v7.3.0 会把 DurationSeconds clamp 到 >=3600s，传更短拿到的仍是 1h。
	scoped, err := AssumeScopedRole(endpoint, accessKey, secretKey, bucket, "clips/alice/", time.Hour)
	if err != nil {
		t.Fatalf("AssumeScopedRole 失败: %v", err)
	}

	// ---- 正向：alice 前缀内可读、可写 ----
	obj, err := scoped.GetObject(ctx, bucket, "clips/alice/take1.mp4", minio.GetObjectOptions{})
	if err != nil {
		t.Fatalf("scoped 读 clips/alice/take1.mp4(GetObject)失败: %v", err)
	}
	data, err := io.ReadAll(obj)
	obj.Close()
	if err != nil {
		t.Fatalf("scoped 读 clips/alice/take1.mp4(Read)失败: %v", err)
	}
	if string(data) != aliceMaterial {
		t.Errorf("alice 素材内容不一致: 期望 %q, 实际 %q", aliceMaterial, string(data))
	}
	if _, err := scoped.PutObject(ctx, bucket, "clips/alice/take2.mp4",
		bytes.NewReader([]byte("new-take")), int64(len("new-take")), minio.PutObjectOptions{}); err != nil {
		t.Errorf("scoped 写 clips/alice/ 前缀应成功，却失败: %v", err)
	}

	// ---- 反向：越权碰 bob 前缀，一律 AccessDenied ----
	// 关键坑：GetObject 是惰性的，GetObject() 本身不报错，错误要到首次 Stat()/Read() 才暴露。
	denied, err := scoped.GetObject(ctx, bucket, "clips/bob/take1.mp4", minio.GetObjectOptions{})
	if err == nil {
		_, err = denied.Stat() // 惰性错误在此触发
		denied.Close()
	}
	if !isAccessDenied(err) {
		t.Errorf("期望越权读 clips/bob/* 被拒(AccessDenied)，实际: %v", err)
	}
	// PutObject 是即时的，越权写直接返回 AccessDenied。
	if _, err := scoped.PutObject(ctx, bucket, "clips/bob/take2.mp4",
		bytes.NewReader([]byte("hack")), int64(len("hack")), minio.PutObjectOptions{}); !isAccessDenied(err) {
		t.Errorf("期望越权写 clips/bob/* 被拒(AccessDenied)，实际: %v", err)
	}

	t.Logf("scope 验证通过：临时凭证在 clips/alice/ 内可读写，越权碰 clips/bob/ 被拒")
}
