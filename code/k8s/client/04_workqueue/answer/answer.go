// Package answer 参考答案（自包含：不依赖父包，可独立编译对照阅读）。
package answer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

// Queue 工作队列抽象（与父包定义一致）。
type Queue interface {
	Add(item string)
	Get() (item string, shutdown bool)
	Done(item string)
	ShutDown()
}

// ProcessFunc 处理一个工作项。
type ProcessFunc func(item string) error

func RunWorker(ctx context.Context, q Queue, n int, fn ProcessFunc) {
	var wg sync.WaitGroup
	StartWorkers(ctx, q, n, fn, &wg)
	wg.Wait()
}

func StartWorkers(ctx context.Context, q Queue, n int, fn ProcessFunc, wg *sync.WaitGroup) {
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			wait.UntilWithContext(ctx, func(ctx context.Context) {
				item, shutdown := q.Get()
				if shutdown {
					return
				}
				if err := fn(item); err != nil {
					fmt.Printf("worker%d 处理 %q 失败: %v，重新入队\n", workerID, item, err)
					q.Add(item)
				}
				q.Done(item)
			}, time.Second)
		}(i)
	}
}

// stringQueueAdapter 把真实 workqueue（Add 接收 any）适配成 Queue 接口（Add 接收 string）。
type stringQueueAdapter struct{ q workqueue.RateLimitingInterface }

func (a *stringQueueAdapter) Add(item string)     { a.q.Add(item) }
func (a *stringQueueAdapter) Get() (string, bool) { item, shutdown := a.q.Get(); return item.(string), shutdown }
func (a *stringQueueAdapter) Done(item string)    { a.q.Done(item) }
func (a *stringQueueAdapter) ShutDown()           { a.q.ShutDown() }

// verify: 真实 workqueue 经适配后满足 Queue 接口。
var _ Queue = &stringQueueAdapter{q: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())}
