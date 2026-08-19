//go:build ignore

package answer

import (
	"context"
	"sync"
)

type JobFunc func(ctx context.Context, job int) (int, error)

// RunPool 参考答案：fan-out + fan-in + context 取消
func RunPool(ctx context.Context, jobs []int, workers int, fn JobFunc) ([]int, error) {
	if workers <= 0 {
		workers = 1
	}

	in := make(chan int)
	out := make(chan int)
	errCh := make(chan error, workers)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range in {
				v, err := fn(ctx, j)
				if err != nil {
					errCh <- err
					return
				}
				select {
				case out <- v:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()

	go func() {
		defer close(in)
		for _, j := range jobs {
			select {
			case in <- j:
			case <-ctx.Done():
				return
			}
		}
	}()

	var results []int
	for v := range out {
		results = append(results, v)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	return results, nil
}
