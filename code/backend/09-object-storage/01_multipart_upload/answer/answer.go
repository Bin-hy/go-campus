package multipart_upload

import (
	"bytes"
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

// MultipartUpload 参考答案：用 minio.Core 低层 API 手写分片上传三步协议。
//
// 为什么要分片：
//   - 单个 PUT 请求对大文件不友好（S3 单请求上限 5GiB），大文件必须分片；
//   - 分片可以并行上传、失败只重传坏的那一片，是断点续传的基础；
//   - 每片独立计算 ETag，服务端在 Complete 时校验并按序拼接。
func MultipartUpload(ctx context.Context, core *minio.Core, bucket, object string, data []byte, partSize int) (string, int, error) {
	if partSize <= 0 {
		return "", 0, fmt.Errorf("partSize 必须为正数，当前 %d", partSize)
	}

	// 第一步：初始化分片上传，拿到 uploadID（后续所有分片都归属它）。
	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("初始化分片上传失败: %w", err)
	}

	// 第二步：按 partSize 切片，逐片上传，记录每片的 PartNumber 与 ETag。
	var completeParts []minio.CompletePart
	partNumber := 1 // S3 分片号从 1 开始
	for off := 0; off < len(data); off += partSize {
		end := off + partSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[off:end]

		part, err := core.PutObjectPart(ctx, bucket, object, uploadID, partNumber,
			bytes.NewReader(chunk), int64(len(chunk)), minio.PutObjectPartOptions{})
		if err != nil {
			// 失败要中止，否则残留分片会一直占用空间（生产上再叠加生命周期规则兜底清理）。
			_ = core.AbortMultipartUpload(ctx, bucket, object, uploadID)
			return "", 0, fmt.Errorf("上传第 %d 片失败: %w", partNumber, err)
		}
		completeParts = append(completeParts, minio.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
		partNumber++
	}

	// 第三步：提交，服务端按 PartNumber 顺序拼成完整对象。
	info, err := core.CompleteMultipartUpload(ctx, bucket, object, uploadID, completeParts, minio.PutObjectOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("完成分片上传失败: %w", err)
	}
	return info.ETag, len(completeParts), nil
}
