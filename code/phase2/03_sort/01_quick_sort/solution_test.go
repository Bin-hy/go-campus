package quick_sort

import (
	"sort"
	"testing"
)

// --- 辅助函数 ---

func isSorted(nums []int) bool {
	for i := 1; i < len(nums); i++ {
		if nums[i] < nums[i-1] {
			return false
		}
	}
	return true
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- 测试用例 ---

func TestSortArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "标准示例",
			input:    []int{5, 2, 3, 1},
			expected: []int{1, 2, 3, 5},
		},
		{
			name:     "含重复",
			input:    []int{5, 1, 1, 2, 0, 0},
			expected: []int{0, 0, 1, 1, 2, 5},
		},
		{
			name:     "已排序",
			input:    []int{1, 2, 3, 4, 5},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "逆序",
			input:    []int{5, 4, 3, 2, 1},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "单元素",
			input:    []int{1},
			expected: []int{1},
		},
		{
			name:     "两个元素",
			input:    []int{2, 1},
			expected: []int{1, 2},
		},
		{
			name:     "空数组",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "全部相同",
			input:    []int{3, 3, 3, 3, 3},
			expected: []int{3, 3, 3, 3, 3},
		},
		{
			name:     "包含负数",
			input:    []int{-3, 5, -1, 0, 2, -8},
			expected: []int{-8, -3, -1, 0, 2, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 复制输入避免影响原数据
			input := make([]int, len(tt.input))
			copy(input, tt.input)

			got := sortArray(input)
			if !equal(got, tt.expected) {
				t.Errorf("sortArray(%v) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSortArray_Large(t *testing.T) {
	// 测试大数组（验证不会退化为 O(n²)）
	n := 100000
	nums := make([]int, n)
	for i := range nums {
		nums[i] = n - i // 完全逆序
	}
	result := sortArray(nums)
	if !sort.IntsAreSorted(result) {
		t.Error("大数组排序结果不正确")
	}
}
