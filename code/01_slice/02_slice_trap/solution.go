package slice_trap

// AppendNoEffect 判断向 s append val 后，原切片 s 是否不受影响
// 返回 true 表示原切片不受影响（发生了扩容）
// 返回 false 表示原切片受影响（未扩容，底层数组被修改）
func AppendNoEffect(s []int, val int) bool {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// SafeSubSlice 安全地截取子切片 s[start:end]
// 要求：对返回的子切片进行 append 操作不会影响原切片 s
func SafeSubSlice(s []int, start, end int) []int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// RemoveElement 从切片中删除指定索引的元素（保持顺序）
// 如果 index 越界，返回原切片不做修改
func RemoveElement(s []int, index int) []int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
