package presigned_url

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
)

// PresignedRoundTrip 参考答案：预签名直传 + 预签名直下的完整往返。
//
// 核心思想：后端只生成带签名的临时 URL，真正的字节流由 HTTP 客户端直连对象存储，
// 不经过后端带宽——这是海量素材直传/直下的标准做法。签名里内置了过期时间、
// 方法（PUT/GET）、桶与对象路径，服务端据此校验，无需后端参与鉴权。
func PresignedRoundTrip(ctx context.Context, client *minio.Client, bucket, object string, body []byte, expiry time.Duration) ([]byte, error) {
	// 1) 生成预签名 PUT URL（客户端凭它直传，无需拿到后端 AK/SK）。
	putURL, err := client.PresignedPutObject(ctx, bucket, object, expiry)
	if err != nil {
		return nil, fmt.Errorf("生成预签名上传 URL 失败: %w", err)
	}

	// 2) 用普通 HTTP 客户端 PUT 上去（模拟前端/客户端直传行为）。
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("预签名上传请求失败: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("预签名上传返回非 200: %d", resp.StatusCode)
	}

	// 3) 生成预签名 GET URL 并下载回来。
	getURL, err := client.PresignedGetObject(ctx, bucket, object, expiry, url.Values{})
	if err != nil {
		return nil, fmt.Errorf("生成预签名下载 URL 失败: %w", err)
	}
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL.String(), nil)
	if err != nil {
		return nil, err
	}
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		return nil, fmt.Errorf("预签名下载请求失败: %w", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("预签名下载返回非 200: %d", getResp.StatusCode)
	}
	return io.ReadAll(getResp.Body)
}
