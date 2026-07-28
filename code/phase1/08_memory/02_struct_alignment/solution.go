package struct_alignment

import "unsafe"

// BadLayout 内存布局不佳的结构体（有较多 padding）
type BadLayout struct {
	A bool    // 1 byte
	B int64   // 8 bytes
	C int32   // 4 bytes
	D bool    // 1 byte
	E int64   // 8 bytes
}

// GoodLayout 请重新排列 BadLayout 的字段，使其占用最小内存
// 要求：包含与 BadLayout 完全相同的字段（名称和类型都相同）
type GoodLayout struct {
	// TODO: 重新排列字段使 sizeof 最小
	A bool
	B int64
	C int32
	D bool
	E int64
}

// SizeOfBad 返回 BadLayout 的大小
func SizeOfBad() uintptr {
	return unsafe.Sizeof(BadLayout{})
}

// SizeOfGood 返回 GoodLayout 的大小
func SizeOfGood() uintptr {
	return unsafe.Sizeof(GoodLayout{})
}

// OptimizedMessage 设计一个消息结构体，包含以下字段，使内存占用最小：
// - ID uint64
// - Type uint8
// - Priority uint8
// - Length uint32
// - Timestamp int64
// - Flags uint16
// - Valid bool
type OptimizedMessage struct {
	// TODO: 以最优顺序排列以上字段
}

// SizeOfOptimizedMessage 返回 OptimizedMessage 的大小
func SizeOfOptimizedMessage() uintptr {
	return unsafe.Sizeof(OptimizedMessage{})
}
