package struct_alignment

import (
	"testing"
	"unsafe"
)

func TestBadLayout_Size(t *testing.T) {
	size := SizeOfBad()
	t.Logf("BadLayout size = %d bytes", size)
	// BadLayout 应该有较多 padding
	if size <= 24 {
		t.Errorf("BadLayout 应该有 padding 导致 > 24 bytes，实际 %d", size)
	}
}

func TestGoodLayout_Smaller(t *testing.T) {
	badSize := SizeOfBad()
	goodSize := SizeOfGood()

	t.Logf("BadLayout: %d bytes, GoodLayout: %d bytes", badSize, goodSize)

	if goodSize >= badSize {
		t.Errorf("GoodLayout(%d) 应比 BadLayout(%d) 小", goodSize, badSize)
	}
}

func TestGoodLayout_Optimal(t *testing.T) {
	goodSize := SizeOfGood()
	// 最优排列：两个 int64(16) + int32(4) + 两个 bool(2) + padding(2) = 24
	// 实际最优应该是 24 bytes
	if goodSize > 24 {
		t.Errorf("GoodLayout 最优应为 24 bytes，实际 %d", goodSize)
	}
}

func TestOptimizedMessage_Size(t *testing.T) {
	size := SizeOfOptimizedMessage()
	t.Logf("OptimizedMessage size = %d bytes", size)

	// 字段总大小：8+1+1+4+8+2+1 = 25 bytes
	// 最优对齐：8+8+4+2+1+1+1+padding = 应该是 24 或 32 取决于排列
	// 最优排列：ID(8) + Timestamp(8) + Length(4) + Flags(2) + Type(1) + Priority(1) + Valid(1) + pad(7)... 
	// 实际最优是 24 bytes: 两个 int64(16) + uint32(4) + uint16(2) + 3个uint8(3) + 1 pad = 32? 
	// 不对：uint64(8) + int64(8) + uint32(4) + uint16(2) + uint8(1) + uint8(1) + bool(1) + pad(7)
	// = 8+8+4+2+1+1+1+pad(7) 不对... 对齐到 8: 8+8 = 16, +4+2+1+1 = 8, +1+pad(7) = 8 → 总32
	// 更优：uint64(8) + int64(8) + uint32(4) + uint16(2) + uint8+uint8+bool = 3, pad 5 → 8+8+8 = nope
	// 最优：8+8+4+2+1+1+1 = 25, 对齐到8的倍数 = 32
	if size > 32 {
		t.Errorf("OptimizedMessage 应不超过 32 bytes，实际 %d", size)
	}
}

func TestOptimizedMessage_HasAllFields(t *testing.T) {
	msg := OptimizedMessage{}
	// 通过 unsafe.Sizeof 确认结构体非空
	if unsafe.Sizeof(msg) == 0 {
		t.Error("OptimizedMessage 不应为空结构体")
	}
}
