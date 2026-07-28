package deep_copy

// DeepCopy 对 int 切片进行深拷贝
// 要求：
// - 返回全新切片，修改返回值不影响原切片
// - nil 输入返回 nil
// - 空切片输入返回空切片（非 nil）
func DeepCopy(src []int) []int {
	// TODO: 在这里实现你的代码
	// panic("not implemented")
	if src == nil {
		return nil
	}
	dist := make([]int, len(src))
	copy(dist, src)
	return dist
}
