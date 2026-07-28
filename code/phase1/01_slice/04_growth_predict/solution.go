package growth_predict

// PredictGrowth 预测 slice 扩容后的新 cap
// oldCap: 当前容量
// needCap: 需要的最小容量
// 返回扩容后的容量（Go 1.18+ 策略，不考虑内存对齐）
func PredictGrowth(oldCap, needCap int) int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// OptimalPrealloc 计算预分配的收益
// appendSizes: 每次 append 的元素数量序列
// 返回 (不预分配的扩容次数, 预分配后的扩容次数)
func OptimalPrealloc(appendSizes []int) (withoutPrealloc, withPrealloc int) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// BatchAppend 高效合并多个切片
// 要求：只进行一次内存分配
func BatchAppend(slices ...[]int) []int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
