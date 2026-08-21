package presigned_url

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func mustMakeBucket(t *testing.T, client *minio.Client, bucket string) {
	t.Helper()
	if err := client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("建桶失败: %v", err)
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

func TestPresignedRoundTrip(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	bucket := uniqueBucket("presign")
	mustMakeBucket(t, client, bucket)
	defer cleanupBucket(t, client, bucket)

	object := "clips/preview.mp4"
	body := []byte("这是一段用于预签名往返测试的素材内容 hello presigned world")

	got, err := PresignedRoundTrip(ctx, client, bucket, object, body, 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignedRoundTrip 失败: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("往返内容不一致：期望 %q，实际 %q", body, got)
	}
	t.Logf("预签名往返验证通过：上传并下载回 %d 字节", len(got))
}

// TestPresignedExpiry 验证极短过期的预签名 URL 在过期后失效。
func TestPresignedExpiry(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	bucket := uniqueBucket("presign-exp")
	mustMakeBucket(t, client, bucket)
	defer cleanupBucket(t, client, bucket)

	object := "clips/short-lived.txt"
	content := []byte("expiry test")
	if _, err := client.PutObject(ctx, bucket, object, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{}); err != nil {
		t.Fatalf("预置对象失败: %v", err)
	}

	// 生成 1 秒过期的预签名 GET URL（minio-go 允许的最小过期为 1s）。
	getURL, err := client.PresignedGetObject(ctx, bucket, object, 1*time.Second, url.Values{})
	if err != nil {
		t.Fatalf("生成预签名 URL 失败: %v", err)
	}

	// 等待其过期后再访问。
	time.Sleep(2 * time.Second)

	resp, err := http.Get(getURL.String())
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("预期过期后被拒绝，却返回 200")
	}
	t.Logf("过期验证通过：过期 URL 返回状态码 %d（非 200，签名已失效）", resp.StatusCode)
}
