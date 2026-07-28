package goroutine_leak_fix

import "time"

// FixedSearch 向多个 backend 并发搜索，返回第一个结果
// 要求：未被选中的 goroutine 必须能退出，不泄漏
func FixedSearch(query string, backends ...func(string) string) string {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// FixedGenerator 返回一个产生 0, 1, 2, 3, ... 的 channel
// 当 done 关闭时，生成器 goroutine 必须退出
func FixedGenerator(done <-chan struct{}) <-chan int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// FixedWorker 每隔 interval 执行一次 task
// 当 done 关闭时，worker 必须立即停止并退出
func FixedWorker(done <-chan struct{}, interval time.Duration, task func()) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
