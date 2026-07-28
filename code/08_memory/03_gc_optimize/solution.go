package gc_optimize

// ProcessData_Bad 频繁分配内存的版本（GC 压力大）
// 对每个 input 数字转为字符串再转回，模拟频繁分配
func ProcessData_Bad(inputs []int) []int {
	// 已实现 — 这是"坏"的版本，用于对比
	results := make([]int, 0)
	for _, v := range inputs {
		tmp := make([]int, 1) // 每次循环都分配
		tmp[0] = v * 2
		results = append(results, tmp[0])
	}
	return results
}

// ProcessData_Good 优化版本，减少 GC 压力
// 要求：结果与 Bad 版本完全一致，但尽量减少堆分配
func ProcessData_Good(inputs []int) []int {
	// TODO: 在这里实现优化后的代码
	panic("not implemented")
}

// ConcatStrings_Bad 用 += 拼接字符串（每次分配新内存）
func ConcatStrings_Bad(strs []string) string {
	result := ""
	for _, s := range strs {
		result += s // 每次拼接都分配新字符串
	}
	return result
}

// ConcatStrings_Good 优化版本
func ConcatStrings_Good(strs []string) string {
	// TODO: 在这里实现优化后的代码
	panic("not implemented")
}

// ObjectPool_Demo 演示 sync.Pool 减少分配
// 接收 n 个请求，每个请求需要一个临时 buffer（1KB）
// 返回处理的请求数
func ObjectPool_Demo(n int) int {
	// TODO: 用 sync.Pool 复用 buffer 处理 n 个请求
	panic("not implemented")
}
