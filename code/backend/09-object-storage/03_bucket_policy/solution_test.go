package bucket_policy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
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

func newClient(t *testing.T) *minio.Client {
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

// anonymousStatus 用裸 http.Get（不带任何凭据）访问对象直链，返回状态码。
func anonymousStatus(t *testing.T, bucket, object string) int {
	t.Helper()
	directURL := fmt.Sprintf("http://%s/%s/%s", endpoint, bucket, object)
	resp, err := http.Get(directURL)
	if err != nil {
		t.Fatalf("匿名访问 %s 失败: %v", directURL, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func TestApplyPublicReadPrefix(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	bucket := uniqueBucket("policy")
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("建桶失败: %v", err)
	}
	defer cleanupBucket(t, client, bucket)

	putObject(t, client, bucket, "public/logo.png", "fake-png-bytes")
	putObject(t, client, bucket, "private/secret.mp4", "top-secret-material")

	if err := ApplyPublicReadPrefix(ctx, client, bucket, "public/"); err != nil {
		t.Fatalf("ApplyPublicReadPrefix 失败: %v", err)
	}

	// 正向：public/ 前缀匿名可读。
	if code := anonymousStatus(t, bucket, "public/logo.png"); code != http.StatusOK {
		t.Errorf("期望 public/logo.png 匿名可读(200)，实际 %d", code)
	}
	// 反向：private/ 前缀匿名被拒（最小权限）。
	if code := anonymousStatus(t, bucket, "private/secret.mp4"); code == http.StatusOK {
		t.Errorf("期望 private/secret.mp4 匿名被拒(非 200)，却返回 200——权限过宽！")
	}
	t.Logf("最小权限验证通过：public/ 匿名可读，private/ 匿名被拒")
}
