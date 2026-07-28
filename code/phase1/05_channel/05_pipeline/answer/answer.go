//go:build ignore

package answer

import "sync"

func Generator(done <-chan struct{}, start, end int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for i := start; i <= end; i++ {
			select {
			case out <- i:
			case <-done:
				return
			}
		}
	}()
	return out
}

func Square(done <-chan struct{}, in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for v := range in {
			select {
			case out <- v * v:
			case <-done:
				return
			}
		}
	}()
	return out
}

func Filter(done <-chan struct{}, in <-chan int, predicate func(int) bool) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for v := range in {
			if predicate(v) {
				select {
				case out <- v:
				case <-done:
					return
				}
			}
		}
	}()
	return out
}

func Merge(done <-chan struct{}, channels ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup

	for _, ch := range channels {
		wg.Add(1)
		go func(c <-chan int) {
			defer wg.Done()
			for v := range c {
				select {
				case out <- v:
				case <-done:
					return
				}
			}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func Pipeline(start, end int, predicate func(int) bool) []int {
	done := make(chan struct{})
	defer close(done)

	gen := Generator(done, start, end)
	squared := Square(done, gen)
	filtered := Filter(done, squared, predicate)

	var result []int
	for v := range filtered {
		result = append(result, v)
	}
	if result == nil {
		return []int{}
	}
	return result
}
