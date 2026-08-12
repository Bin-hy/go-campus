//go:build ignore

package answer

import (
	"sync"
)

// Run 运行生产者-消费者模型
// numProducers: 生产者数量
// numConsumers: 消费者数量
// itemsPerProducer: 每个生产者产生的任务数
// 生产者 p 产生的值为: p*itemsPerProducer + 0, p*itemsPerProducer + 1, ..., p*itemsPerProducer + (itemsPerProducer-1)
// 返回所有任务值的总和
func Run(numProducers, numConsumers, itemsPerProducer int) int {
	// TODO: 在这里实现你的代码
	sum := 0
	mu := sync.Mutex{}
	// 任务队列
	taskQueue := make(chan func(), 1)
	consumerWG := sync.WaitGroup{}
	producerWG := sync.WaitGroup{}

	// 生成消费者
	for i := 0; i < numConsumers; i++ {
		consumerWG.Add(1)
		go func() {
			defer consumerWG.Done()
			for task := range taskQueue {
				func() {
					defer recover() // 保护consume 哪怕task失败也能继续for，保持consumer活着

					// 执行任务
					task()
				}()
			}
		}()
	}

	// 生成生产者
	for p := 0; p < numProducers; p++ {
		producerWG.Add(1)

		go func(p int) {
			defer producerWG.Done()

			for i := 0; i < itemsPerProducer; i++ {
				value := p*itemsPerProducer + i
				taskQueue <- func() {
					mu.Lock()
					sum += value
					mu.Unlock()
				}
			}
		}(p)

	}
	// 生产者生产完 任务
	producerWG.Wait()
	close(taskQueue)

	// 等待消费者处理完任务
	consumerWG.Wait()
	return sum
}
