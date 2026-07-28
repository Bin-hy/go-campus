//go:build ignore

package answer

import "time"

func FixedSearch(query string, backends ...func(string) string) string {
	ch := make(chan string, len(backends)) // buffered! 确保落选者不阻塞
	for _, backend := range backends {
		go func(fn func(string) string) {
			ch <- fn(query)
		}(backend)
	}
	return <-ch // 取第一个结果
}

func FixedGenerator(done <-chan struct{}) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		i := 0
		for {
			select {
			case ch <- i:
				i++
			case <-done:
				return
			}
		}
	}()
	return ch
}

func FixedWorker(done <-chan struct{}, interval time.Duration, task func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			task()
		case <-done:
			return
		}
	}
}
