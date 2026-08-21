# 预签名 URL：客户端直传/直下

## 难度：⭐⭐ 中等

## 考点
- 预签名 URL（presigned URL）的本质：后端用自己的密钥算出一个带签名、带过期时间的临时 URL，客户端凭它直连对象存储，**字节流不经过后端带宽**
- 直传（PUT）+ 直下（GET）往返：海量素材上传/下载的标准做法
- 签名边界：`expiry` 过期即失效——这是预签名 URL 的安全生命线
- 绝不能把长期 AK/SK 发给前端；只发短时效的预签名 URL

## 环境准备

```bash
cd code/backend && docker compose up -d minio
```

## 题目描述

剪辑客户端要上传一段预览视频，又要能下载回来。如果字节流都过后端中转，后端带宽会被打爆。正确做法是**预签名 URL**：后端只负责签发临时 URL，客户端直连 MinIO。

请实现一次完整往返：

1. 用 `PresignedPutObject` 生成上传 URL；
2. 用标准库 `http.Client` 把 `body` PUT 到该 URL（模拟客户端直传）；
3. 用 `PresignedGetObject` 生成下载 URL；
4. GET 下载回来，返回下载到的字节。

返回下载内容 `downloaded`，测试会断言它与上传的 `body` 逐字节一致。另有一个独立测试验证**极短过期的 URL 在过期后被拒绝**（非 200）。

## 函数签名

```go
func PresignedRoundTrip(ctx context.Context, client *minio.Client, bucket, object string, body []byte, expiry time.Duration) (downloaded []byte, err error)
```

## 提示

1. `client.PresignedPutObject(ctx, bucket, object, expiry)` 返回 `*url.URL`
2. `http.NewRequestWithContext(ctx, http.MethodPut, putURL.String(), bytes.NewReader(body))`，记得设 `req.ContentLength`，再 `http.DefaultClient.Do(req)`
3. 上传成功状态码是 `200`；读干净并关闭 `resp.Body`
4. `client.PresignedGetObject(ctx, bucket, object, expiry, url.Values{})` 生成下载 URL，`http.Get` 后 `io.ReadAll`
5. 过期是签名里的一部分：过期后服务端直接拒绝，不需要后端参与

## 运行测试

```bash
cd code/backend/09-object-storage/02_presigned_url && go test -v
```
