package filter_map_reduce

// Filter 返回满足条件的元素切片
func Filter[T any](slice []T, fn func(T) bool) []T {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Map 对切片中每个元素应用转换函数
func Map[T any, U any](slice []T, fn func(T) U) []U {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Reduce 将切片归约为单个值
func Reduce[T any, U any](slice []T, init U, fn func(U, T) U) U {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Contains 判断切片是否包含某个元素
func Contains[T comparable](slice []T, target T) bool {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Unique 去重，保持原始顺序
func Unique[T comparable](slice []T) []T {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// GroupBy 按照 key 函数对元素分组
func GroupBy[T any, K comparable](slice []T, keyFn func(T) K) map[K][]T {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
