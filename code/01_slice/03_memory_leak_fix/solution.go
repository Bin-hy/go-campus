package memory_leak_fix

// GetFirstN 安全提取前 N 个字节，不持有原始大切片的引用
// - 如果 data 为 nil，返回 nil
// - 如果 n >= len(data)，返回 data 的完整拷贝
// - 如果 n <= 0，返回空切片 []byte{}
func GetFirstN(data []byte, n int) []byte {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// FilterLargeSlice 从大切片中筛选满足条件的元素
// 返回值不应引用原始 data 的底层数组
func FilterLargeSlice(data []int, predicate func(int) bool) []int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// TrimMessage 提取消息有效载荷（去掉前4字节头 + 后2字节尾）
// - 如果 msg 长度 < 6，返回 nil
// - 返回值不应持有 msg 的底层数组引用
func TrimMessage(msg []byte) []byte {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
