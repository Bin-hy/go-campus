package timeout_http

import (
	"context"
	"time"
)

// FetchWithTimeout 发起带超时的 HTTP GET 请求
// 如果超时返回 error
// 这里用模拟函数代替真实 HTTP（便于测试）
func FetchWithTimeout(ctx context.Context, url string, timeout time.Duration, fetcher func(ctx context.Context, url string) (string, error)) (string, error) {
	// TODO: 在这里实现你的代码
	// 提示：用 context.WithTimeout 包装 ctx，传给 fetcher
	panic("not implemented")
}

// FetchMultiple 并发请求多个 URL，任一失败则取消其余
// 返回所有成功的结果（失败的 URL 不在结果中）
func FetchMultiple(urls []string, timeout time.Duration, fetcher func(ctx context.Context, url string) (string, error)) map[string]string {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
