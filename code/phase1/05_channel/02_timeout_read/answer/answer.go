//go:build ignore

package answer

import (
	"errors"
	"time"
)

var ErrTimeout = errors.New("read timeout")

func ReadWithTimeout(ch <-chan int, timeout time.Duration) (int, error) {
	select {
	case v := <-ch:
		return v, nil
	case <-time.After(timeout):
		return 0, ErrTimeout
	}
}

func ReadMultipleWithTimeout(ch <-chan int, n int, timeout time.Duration) []int {
	if n <= 0 {
		return []int{}
	}
	result := make([]int, 0, n)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for i := 0; i < n; i++ {
		select {
		case v := <-ch:
			result = append(result, v)
		case <-timer.C:
			return result
		}
	}
	return result
}

func FirstResult(fns ...func() int) int {
	ch := make(chan int, len(fns))
	for _, fn := range fns {
		go func(f func() int) {
			ch <- f()
		}(fn)
	}
	return <-ch
}
