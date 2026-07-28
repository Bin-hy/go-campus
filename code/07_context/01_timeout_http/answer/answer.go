//go:build ignore

package answer

import (
	"context"
	"sync"
	"time"
)

func FetchWithTimeout(ctx context.Context, url string, timeout time.Duration, fetcher func(ctx context.Context, url string) (string, error)) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fetcher(ctx, url)
}

func FetchMultiple(urls []string, timeout time.Duration, fetcher func(ctx context.Context, url string) (string, error)) map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var mu sync.Mutex
	results := make(map[string]string)
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			if result, err := fetcher(ctx, u); err == nil {
				mu.Lock()
				results[u] = result
				mu.Unlock()
			}
		}(url)
	}

	wg.Wait()
	return results
}
