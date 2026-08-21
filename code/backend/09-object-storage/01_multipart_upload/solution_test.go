package multipart_upload

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

// uniqueBucket 生成带时间戳的唯一桶名，避免并发/重复运行相互污染。
// 注意：S3/MinIO 桶名只能小写字母、数字、连字符，3-63 字符，不能有下划线。
func uniqueBucket(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// newCore 建立 minio.Core（低层 API，含 multipart 三步）并探活。
func newCore(t *testing.T) *minio.Core {
	t.Helper()
	core, err := minio.NewCore(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("创建 MinIO Core 失败: %v", err)
	}
	if _, err := core.ListBuckets(context.Background()); err != nil {
		t.Fatalf("连接 MinIO 失败（确认已 docker compose up -d minio）: %v", err)
	}
	return core
}

func TestMultipartUpload(t *testing.T) {
	ctx := context.Background()
	core := newCore(t)

	bucket := uniqueBucket("mp-upload")
	if err := core.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatalf("建桶失败: %v", err)
	}
	object := "clips/big-material.bin"
	defer func() {
		_ = core.RemoveObject(ctx, bucket, object, minio.RemoveObjectOptions{})
		_ = core.RemoveBucket(ctx, bucket)
	}()

	// 构造预期 3 片：partSize=5MiB，总 12MiB → 5+5+2。
	// S3 规定除最后一片外每片 ≥ 5MiB，所以 partSize 不能小于 5MiB。
	const partSize = 5 << 20  // 5 MiB
	data := make([]byte, 12<<20) // 12 MiB
	for i := range data {
		data[i] = byte(i % 251) // 可校验的确定内容
	}

	etag, parts, err := MultipartUpload(ctx, core, bucket, object, data, partSize)
	if err != nil {
		t.Fatalf("MultipartUpload 失败: %v", err)
	}
	if etag == "" {
		t.Errorf("期望返回非空 ETag")
	}
	if parts != 3 {
		t.Errorf("期望 3 片，实际 %d 片", parts)
	}

	// 校验对象大小与源一致
	info, err := core.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
	if err != nil {
		t.Fatalf("StatObject 失败: %v", err)
	}
	if info.Size != int64(len(data)) {
		t.Errorf("对象大小不符：期望 %d，实际 %d", len(data), info.Size)
	}

	// 校验内容逐字节一致
	obj, _, _, err := core.GetObject(ctx, bucket, object, minio.GetObjectOptions{})
	if err != nil {
		t.Fatalf("GetObject 失败: %v", err)
	}
	defer obj.Close()
	got, err := io.ReadAll(obj)
	if err != nil {
		t.Fatalf("读取对象失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("内容与源不一致：下载 %d 字节", len(got))
	}
	t.Logf("分片上传验证通过：%d 字节分 %d 片上传，ETag=%s", len(data), parts, etag)
}
