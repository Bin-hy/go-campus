package bucket_policy

import (
	"context"

	"github.com/minio/minio-go/v7"
)

// ApplyPublicReadPrefix 给 bucket 下发一条最小权限桶策略：
// 仅允许匿名用户 GET <bucket>/<prefix>* 下的对象，其余一律拒绝。
func ApplyPublicReadPrefix(ctx context.Context, client *minio.Client, bucket, prefix string) error {
	// TODO: 实现你的代码
	panic("not implemented")
}
