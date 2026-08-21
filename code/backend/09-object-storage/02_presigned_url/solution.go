package presigned_url

import (
	"context"
	"time"

	"github.com/minio/minio-go/v7"
)

// PresignedRoundTrip 完成一次预签名直传 + 预签名直下的往返：
// PresignedPutObject 生成上传 URL → http PUT → PresignedGetObject 生成下载 URL → http GET。
// 返回下载回来的字节，应与上传的 body 一致。
func PresignedRoundTrip(ctx context.Context, client *minio.Client, bucket, object string, body []byte, expiry time.Duration) (downloaded []byte, err error) {
	// TODO: 实现你的代码
	panic("not implemented")
}
