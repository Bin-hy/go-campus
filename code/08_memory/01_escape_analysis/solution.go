package escape_analysis

// CreateOnStack 创建值并在栈上使用（不逃逸）
// 返回值类型是 int（值类型不逃逸）
func CreateOnStack() int {
	// TODO: 创建一个 int 变量并返回其值
	panic("not implemented")
}

// CreateOnHeap 创建值并返回指针（逃逸到堆）
func CreateOnHeap() *int {
	// TODO: 创建一个 int 变量并返回其指针
	panic("not implemented")
}

// SliceNoEscape 创建固定大小的 slice，不逃逸
// 提示：编译器可以将小的、确定大小的 slice 分配在栈上
func SliceNoEscape() int {
	// TODO: 创建一个固定小 slice，计算并返回其元素之和
	panic("not implemented")
}

// SliceEscape 创建会逃逸的 slice
// 提示：返回 slice 引用或使 slice 太大会导致逃逸
func SliceEscape() []int {
	// TODO: 创建并返回一个 slice
	panic("not implemented")
}

// SumWithInterface 使用 interface{} 导致逃逸
func SumWithInterface(nums ...interface{}) int {
	// TODO: 将 nums 中所有 int 值求和
	panic("not implemented")
}

// SumDirect 直接使用具体类型，不逃逸
func SumDirect(a, b, c int) int {
	// TODO: 返回 a+b+c
	panic("not implemented")
}
