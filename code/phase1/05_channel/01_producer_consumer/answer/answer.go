//go:build ignore

package answer

import (
	"sync"
	"sync/atomic"
)

func Run(numProducers, numConsumers, itemsPerProducer int) int {
	ch := make(chan int, numProducers*itemsPerProducer)

	// 启动生产者
	var prodWg sync.WaitGroup
	for p := 0; p < numProducers; p++ {
		prodWg.Add(1)
		go func(p int) {
			defer prodWg.Done()
			for i := 0; i < itemsPerProducer; i++ {
				ch <- p*itemsPerProducer + i
			}
		}(p)
	}

	// 生产者全部完成后关闭 channel
	go func() {
		prodWg.Wait()
		close(ch)
	}()

	// 启动消费者
	var sum int64
	var consWg sync.WaitGroup
	for c := 0; c < numConsumers; c++ {
		consWg.Add(1)
		go func() {
			defer consWg.Done()
			for task := range ch {
				atomic.AddInt64(&sum, int64(task))
			}
		}()
	}

	consWg.Wait()
	return int(sum)
}
