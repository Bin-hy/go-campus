//go:build ignore

package answer

func SequentialPrint(n int, max int) []int {
	if max <= 0 {
		return []int{}
	}

	result := make([]int, 0, max)
	ch := make(chan int, max)

	// 创建 n 个 token channel
	tokens := make([]chan struct{}, n)
	for i := range tokens {
		tokens[i] = make(chan struct{})
	}

	done := make(chan struct{})

	for i := 0; i < n; i++ {
		go func(id int) {
			current := id + 1 // 起始值
			for {
				select {
				case <-tokens[id]:
					if current > max {
						// 传递给下一个让它也检查退出
						next := (id + 1) % n
						if next != 0 {
							tokens[next] <- struct{}{}
						} else {
							close(done)
						}
						return
					}
					ch <- current
					current += n
					next := (id + 1) % n
					if current-n >= max && next == 0 {
						close(done)
						return
					}
					tokens[next] <- struct{}{}
				case <-done:
					return
				}
			}
		}(i)
	}

	// 启动
	tokens[0] <- struct{}{}
	<-done

	close(ch)
	for v := range ch {
		result = append(result, v)
	}
	return result
}
