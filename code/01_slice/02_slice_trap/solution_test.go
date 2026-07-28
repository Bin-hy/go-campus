package slice_trap

import (
	"reflect"
	"testing"
)

func TestAppendNoEffect(t *testing.T) {
	tests := []struct {
		name     string
		s        []int
		val      int
		expected bool
	}{
		{
			name:     "有剩余容量-受影响",
			s:        make([]int, 3, 5), // len=3, cap=5
			val:      99,
			expected: false, // 不会扩容，原数组受影响
		},
		{
			name:     "无剩余容量-不受影响",
			s:        []int{1, 2, 3}, // len=cap=3
			val:      99,
			expected: true, // 会扩容，原数组不受影响
		},
		{
			name:     "空切片-不受影响",
			s:        []int{},
			val:      1,
			expected: true,
		},
		{
			name:     "容量刚好多1",
			s:        make([]int, 2, 3),
			val:      5,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AppendNoEffect(tt.s, tt.val)
			if result != tt.expected {
				t.Errorf("期望 %v，得到 %v (len=%d, cap=%d)",
					tt.expected, result, len(tt.s), cap(tt.s))
			}
		})
	}
}

func TestSafeSubSlice(t *testing.T) {
	original := []int{1, 2, 3, 4, 5, 6, 7, 8}

	sub := SafeSubSlice(original, 2, 5) // 期望 [3, 4, 5]

	if !reflect.DeepEqual(sub, []int{3, 4, 5}) {
		t.Errorf("截取结果错误，期望 [3,4,5]，得到 %v", sub)
	}

	// 对子切片 append 不应影响原切片
	sub = append(sub, 99)
	if original[5] == 99 {
		t.Error("SafeSubSlice 失败：对子切片 append 影响了原切片")
	}
}

func TestSafeSubSlice_EdgeCases(t *testing.T) {
	s := []int{1, 2, 3}

	// 截取全部
	sub := SafeSubSlice(s, 0, 3)
	if !reflect.DeepEqual(sub, []int{1, 2, 3}) {
		t.Errorf("截取全部失败，得到 %v", sub)
	}

	// 截取空
	sub = SafeSubSlice(s, 1, 1)
	if len(sub) != 0 {
		t.Errorf("截取空切片失败，得到 %v", sub)
	}
}

func TestRemoveElement(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		index    int
		expected []int
	}{
		{
			name:     "删除中间元素",
			input:    []int{1, 2, 3, 4, 5},
			index:    2,
			expected: []int{1, 2, 4, 5},
		},
		{
			name:     "删除第一个元素",
			input:    []int{1, 2, 3},
			index:    0,
			expected: []int{2, 3},
		},
		{
			name:     "删除最后一个元素",
			input:    []int{1, 2, 3},
			index:    2,
			expected: []int{1, 2},
		},
		{
			name:     "索引越界-负数",
			input:    []int{1, 2, 3},
			index:    -1,
			expected: []int{1, 2, 3},
		},
		{
			name:     "索引越界-超出长度",
			input:    []int{1, 2, 3},
			index:    5,
			expected: []int{1, 2, 3},
		},
		{
			name:     "单元素切片",
			input:    []int{42},
			index:    0,
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveElement(tt.input, tt.index)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("期望 %v，得到 %v", tt.expected, result)
			}
		})
	}
}
