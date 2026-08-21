package multipart_upload

import (
	"context"

	"github.com/minio/minio-go/v7"
)

// MultipartUpload 用 minio.Core 低层 API 手写分片上传三步协议：
// NewMultipartUpload → 循环 PutObjectPart → CompleteMultipartUpload。
// 把 data 按 partSize 切分逐片上传，返回最终对象 ETag 与实际分片数。
func MultipartUpload(ctx context.Context, core *minio.Core, bucket, object string, data []byte, partSize int) (etag string, parts int, err error) {
	// TODO: 实现你的代码
	panic("not implemented")
	// 1.使用 NewMultipartUpload
	// 得到一个uploadID
	// 2.每次 使用PutObjectPart ，提交uploadID和partNumber和Data
	// 返回该片 ETag
	//3. 上传完成， CompleteMultipartUpload
	// core.NewMultipartUpload(bucket)
}
