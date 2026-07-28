//go:build ignore

package answer

import "unsafe"

type BadLayout struct {
	A bool
	B int64
	C int32
	D bool
	E int64
}

// 最优排列：大字段在前，小字段在后
type GoodLayout struct {
	B int64 // 8
	E int64 // 8
	C int32 // 4
	A bool  // 1
	D bool  // 1 + 2 padding
} // total = 24

type OptimizedMessage struct {
	ID        uint64 // 8
	Timestamp int64  // 8
	Length    uint32 // 4
	Flags     uint16 // 2
	Type      uint8  // 1
	Priority  uint8  // 1
	Valid     bool   // 1 + 7 padding
} // total = 32

func SizeOfBad() uintptr  { return unsafe.Sizeof(BadLayout{}) }
func SizeOfGood() uintptr { return unsafe.Sizeof(GoodLayout{}) }
func SizeOfOptimizedMessage() uintptr { return unsafe.Sizeof(OptimizedMessage{}) }
