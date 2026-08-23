package bucket_policy

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

// ApplyPublicReadPrefix 参考答案：下发一条「仅前缀匿名只读」的最小权限桶策略。
//
// 关键安全点：
//   - Principal {"AWS":["*"]} 表示匿名——任何人都能命中此条，所以 Resource 必须收窄；
//   - Resource 用 arn:aws:s3:::<bucket>/<prefix>* 把开放面严格限定在该前缀，
//     private/ 等其它前缀不在授权范围内，匿名访问会被默认拒绝；
//   - Action 只给 s3:GetObject（只读），既不放 List 也不放写，避免越权。
func ApplyPublicReadPrefix(ctx context.Context, client *minio.Client, bucket, prefix string) error {
	policy := fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
      {
        "Effect": "Allow",
        "Principal": {"AWS": ["*"]},
        "Action": ["s3:GetObject"],
        "Resource": ["arn:aws:s3:::%s/%s*"]
      }
  ]
}`, bucket, prefix)

	if err := client.SetBucketPolicy(ctx, bucket, policy); err != nil {
		return fmt.Errorf("设置桶策略失败: %w", err)
	}
	return nil
}
