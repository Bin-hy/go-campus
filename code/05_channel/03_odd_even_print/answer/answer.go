//go:build ignore

package answer

func PrintOddEven(n int) []int {
	if n <= 0 {
		return []int{}
	}

	result := make([]int, 0, n)
	ch := make(chan int, n)
	oddTurn := make(chan struct{}, 1)
	evenTurn := make(chan struct{}, 1)
	done := make(chan struct{})

	// 奇数 goroutine
	go func() {
		for i := 1; i <= n; i += 2 {
			<-oddTurn
			ch <- i
			if i+1 <= n {
				evenTurn <- struct{}{}
			} else {
				close(done)
			}
		}
	}()

	// 偶数 goroutine
	go func() {
		for i := 2; i <= n; i += 2 {
			<-evenTurn
			ch <- i
			if i+1 <= n {
				oddTurn <- struct{}{}
			} else {
				close(done)
			}
		}
	}()

	// 启动奇数先行
	oddTurn <- struct{}{}

	<-done
	close(ch)
	for v := range ch {
		result = append(result, v)
	}
	return result
}
