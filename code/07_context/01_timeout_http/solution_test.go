package timeout_http

import (
	"context"
	"errors"
	"testing"
	"time"
)

func mockFetcher(delay time.Duration, result string) func(ctx context.Context, url string) (string, error) {
	return func(ctx context.Context, url string) (string, error) {
		select {
		case <-time.After(delay):
			return result, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func TestFetchWithTimeout_Success(t *testing.T) {
	fetcher := mockFetcher(10*time.Millisecond, "OK")
	result, err := FetchWithTimeout(context.Background(), "http://example.com", time.Second, fetcher)
	if err != nil {
		t.Fatalf("不应超时: %v", err)
	}
	if result != "OK" {
		t.Errorf("期望 OK，得到 %s", result)
	}
}

func TestFetchWithTimeout_Timeout(t *testing.T) {
	fetcher := mockFetcher(time.Second, "slow")
	_, err := FetchWithTimeout(context.Background(), "http://slow.com", 50*time.Millisecond, fetcher)
	if err == nil {
		t.Fatal("应超时返回 error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("期望 DeadlineExceeded，得到 %v", err)
	}
}

func TestFetchWithTimeout_ParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fetcher := mockFetcher(time.Second, "slow")

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := FetchWithTimeout(ctx, "http://example.com", 5*time.Second, fetcher)
	if err == nil {
		t.Fatal("父 context 取消后应返回 error")
	}
}

func TestFetchMultiple_AllSuccess(t *testing.T) {
	fetcher := mockFetcher(10*time.Millisecond, "data")
	urls := []string{"http://a.com", "http://b.com", "http://c.com"}

	results := FetchMultiple(urls, time.Second, fetcher)
	if len(results) != 3 {
		t.Errorf("期望3个结果，得到 %d", len(results))
	}
}

func TestFetchMultiple_Timeout(t *testing.T) {
	fetcher := func(ctx context.Context, url string) (string, error) {
		if url == "http://slow.com" {
			time.Sleep(time.Second)
			return "slow", nil
		}
		return "fast", nil
	}
	urls := []string{"http://fast.com", "http://slow.com"}

	results := FetchMultiple(urls, 100*time.Millisecond, fetcher)
	// fast 应该成功
	if _, ok := results["http://fast.com"]; !ok {
		t.Error("快请求应成功")
	}
}
