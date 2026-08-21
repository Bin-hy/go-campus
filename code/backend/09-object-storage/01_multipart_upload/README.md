# 分片上传：用 minio.Core 手写三步协议

## 难度：⭐⭐⭐ 较难

## 考点
- 分片上传（multipart upload）为什么存在：大文件断点续传、并行上传、绕过单请求大小限制
- 三步协议：`NewMultipartUpload`（拿 uploadID）→ 循环 `PutObjectPart`（逐片传、记 ETag）→ `CompleteMultipartUpload`（按序拼接）
- 分片号 `PartNumber` 从 1 开始；除最后一片外每片 ≥ 5 MiB（S3/MinIO 硬性约束）
- 失败要 `AbortMultipartUpload`，否则残留分片长期占用空间

## 环境准备

```bash
cd code/backend && docker compose up -d minio
```

控制台 http://localhost:9001 （账号密码均为 `minioadmin`）可观察桶与对象。

## 题目描述

剪辑素材里有大量几百 MB 到数 GB 的原始视频，一个 `PUT` 请求扛不住，需要切片上传。请用 **`minio.Core` 低层 API** 手写分片上传的三步协议：

1. 用 `NewMultipartUpload` 初始化，拿到 `uploadID`；
2. 把 `data` 按 `partSize` 切成 N 片，逐片 `PutObjectPart`，收集每片返回的 `PartNumber` 与 `ETag`；
3. 用收集到的 `[]minio.CompletePart` 调用 `CompleteMultipartUpload` 提交，服务端按分片号顺序拼成完整对象。

返回最终对象的 `etag` 与实际上传的分片数 `parts`。测试会构造一个 12 MiB、`partSize=5 MiB` 的对象（预期 3 片），并断言对象大小、内容逐字节一致、分片数正确。

## 函数签名

```go
func MultipartUpload(ctx context.Context, core *minio.Core, bucket, object string, data []byte, partSize int) (etag string, parts int, err error)
```

## 提示

1. `core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})` 返回 `uploadID`
2. 切片循环里 `PutObjectPart(ctx, bucket, object, uploadID, partNumber, bytes.NewReader(chunk), int64(len(chunk)), minio.PutObjectPartOptions{})`，`partNumber` 从 1 递增
3. 每片返回的 `ObjectPart` 里取 `PartNumber` 和 `ETag`，塞进 `minio.CompletePart{}`
4. 循环结束后 `CompleteMultipartUpload(ctx, bucket, object, uploadID, completeParts, minio.PutObjectOptions{})`，其返回值 `UploadInfo.ETag` 就是最终 ETag
5. 任一分片失败先 `AbortMultipartUpload` 再返回错误
6. `partSize <= 0` 直接返回参数错误

## 运行测试

```bash
cd code/backend/09-object-storage/01_multipart_upload && go test -v
```
