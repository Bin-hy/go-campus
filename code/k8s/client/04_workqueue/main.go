package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// go run .
// 演示：用 2 个 worker 消费队列，失败自动重试。Ctrl+C 退出。
func main() {
	q := newDemoQueue()
	for _, item := range []string{"default/nginx-1", "default/nginx-2", "default/nginx-3"} {
		q.Add(item)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; cancel() }()

	var wg sync.WaitGroup
	StartWorkers(ctx, q, 2, func(item string) error {
		fmt.Printf("处理 %s\n", item)
		return nil
	}, &wg)

	// 队列消费完后程序仍在等待；这里 1 秒后退出演示
	<-ctx.Done()
	cancel()
	wg.Wait()
	fmt.Println("worker 已退出")
}

// demoQueue 基于切片的最小队列实现（仅演示，真实场景用 k8s workqueue）。
type demoQueue struct {
	mu    sync.Mutex
	items []string
	done  map[string]bool
}

func newDemoQueue() *demoQueue { return &demoQueue{done: map[string]bool{}} }

func (d *demoQueue) Add(item string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.items = append(d.items, item)
}
func (d *demoQueue) Get() (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.items) == 0 {
		return "", true
	}
	item := d.items[0]
	d.items = d.items[1:]
	return item, false
}
func (d *demoQueue) Done(item string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.done[item] = true
}
func (d *demoQueue) ShutDown() {}
