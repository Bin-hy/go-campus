package pipeline

// Generator 生成 start 到 end（包含）的整数序列
// done 关闭时停止生成
func Generator(done <-chan struct{}, start, end int) <-chan int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Square 将输入 channel 中的每个数字平方后输出
// done 关闭时停止处理
func Square(done <-chan struct{}, in <-chan int) <-chan int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Filter 过滤输入 channel，只输出满足 predicate 的值
// done 关闭时停止处理
func Filter(done <-chan struct{}, in <-chan int, predicate func(int) bool) <-chan int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Merge 将多个输入 channel 合并为一个输出 channel（Fan-in）
// 所有输入关闭后输出 channel 关闭
// done 关闭时停止合并
func Merge(done <-chan struct{}, channels ...<-chan int) <-chan int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Pipeline 构建完整流水线：生成 → 平方 → 过滤
// 返回最终结果切片
func Pipeline(start, end int, predicate func(int) bool) []int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
