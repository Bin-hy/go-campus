package memory_leak_fix

import (
	"reflect"
	"testing"
	"unsafe"
)

// sliceDataPtr 获取切片底层数组指针（用于检测是否共享底层数组）
func sliceDataPtr(s []byte) uintptr {
	if len(s) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&s[0]))
}

func sliceIntDataPtr(s []int) uintptr {
	if len(s) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&s[0]))
}

func TestGetFirstN(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		n        int
		expected []byte
		isNil    bool
	}{
		{"正常截取", []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 3, []byte{1, 2, 3}, false},
		{"n大于长度", []byte{1, 2, 3}, 10, []byte{1, 2, 3}, false},
		{"n等于长度", []byte{1, 2, 3}, 3, []byte{1, 2, 3}, false},
		{"n为0", []byte{1, 2, 3}, 0, []byte{}, false},
		{"n为负数", []byte{1, 2, 3}, -1, []byte{}, false},
		{"nil输入", nil, 5, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFirstN(tt.data, tt.n)

			if tt.isNil {
				if result != nil {
					t.Errorf("期望 nil，得到 %v", result)
				}
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("期望 %v，得到 %v", tt.expected, result)
			}
		})
	}
}

func TestGetFirstN_NoMemoryLeak(t *testing.T) {
	// 创建一个大切片
	bigData := make([]byte, 1024*1024) // 1MB
	for i := range bigData {
		bigData[i] = byte(i % 256)
	}

	result := GetFirstN(bigData, 10)

	// 验证内容正确
	if len(result) != 10 {
		t.Fatalf("长度错误：期望10，得到%d", len(result))
	}

	// 验证不共享底层数组（关键！）
	resultPtr := sliceDataPtr(result)
	originalPtr := sliceDataPtr(bigData)
	if resultPtr == originalPtr {
		t.Error("内存泄漏：返回值仍然引用原始大切片的底层数组")
	}

	// 验证 cap 合理（不应该是 1MB 的 cap）
	if cap(result) > 100 {
		t.Errorf("可能内存泄漏：cap=%d 过大", cap(result))
	}
}

func TestFilterLargeSlice(t *testing.T) {
	data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	isEven := func(n int) bool { return n%2 == 0 }

	result := FilterLargeSlice(data, isEven)

	expected := []int{2, 4, 6, 8, 10}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("期望 %v，得到 %v", expected, result)
	}
}

func TestFilterLargeSlice_NoMemoryLeak(t *testing.T) {
	bigData := make([]int, 100000)
	for i := range bigData {
		bigData[i] = i
	}

	// 只筛选出前3个
	count := 0
	result := FilterLargeSlice(bigData, func(n int) bool {
		count++
		return n < 3
	})

	if len(result) != 3 {
		t.Fatalf("长度错误：期望3，得到%d", len(result))
	}

	// 验证不共享底层数组
	if len(result) > 0 && sliceIntDataPtr(result) == sliceIntDataPtr(bigData) {
		t.Error("内存泄漏：返回值引用了原始大切片的底层数组")
	}
}

func TestFilterLargeSlice_Empty(t *testing.T) {
	data := []int{1, 3, 5, 7}
	isEven := func(n int) bool { return n%2 == 0 }

	result := FilterLargeSlice(data, isEven)

	if result == nil {
		// 允许返回 nil 或空切片
		return
	}
	if len(result) != 0 {
		t.Errorf("期望空结果，得到 %v", result)
	}
}

func TestTrimMessage(t *testing.T) {
	tests := []struct {
		name     string
		msg      []byte
		expected []byte
		isNil    bool
	}{
		{
			name:     "正常消息",
			msg:      []byte{0xAA, 0xBB, 0xCC, 0xDD, 'h', 'e', 'l', 'l', 'o', 0xFF, 0xFE},
			expected: []byte{'h', 'e', 'l', 'l', 'o'},
		},
		{
			name:     "最小有效消息(7字节)",
			msg:      []byte{1, 2, 3, 4, 'X', 5, 6},
			expected: []byte{'X'},
		},
		{
			name:     "刚好6字节-空载荷",
			msg:      []byte{1, 2, 3, 4, 5, 6},
			expected: []byte{},
		},
		{
			name:  "太短-返回nil",
			msg:   []byte{1, 2, 3},
			isNil: true,
		},
		{
			name:  "nil输入",
			msg:   nil,
			isNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimMessage(tt.msg)

			if tt.isNil {
				if result != nil {
					t.Errorf("期望 nil，得到 %v", result)
				}
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("期望 %v，得到 %v", tt.expected, result)
			}
		})
	}
}

func TestTrimMessage_NoMemoryLeak(t *testing.T) {
	bigMsg := make([]byte, 1024*1024) // 1MB 消息
	bigMsg[0], bigMsg[1], bigMsg[2], bigMsg[3] = 0xAA, 0xBB, 0xCC, 0xDD

	result := TrimMessage(bigMsg)

	if cap(result) > len(result)+100 {
		t.Errorf("可能内存泄漏：len=%d 但 cap=%d", len(result), cap(result))
	}
}
