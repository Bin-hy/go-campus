package search_rotated

import "testing"

func TestSearch(t *testing.T) {
	tests := []struct {
		name     string
		nums     []int
		target   int
		expected int
	}{
		{
			name:     "标准示例-找到",
			nums:     []int{4, 5, 6, 7, 0, 1, 2},
			target:   0,
			expected: 4,
		},
		{
			name:     "标准示例-未找到",
			nums:     []int{4, 5, 6, 7, 0, 1, 2},
			target:   3,
			expected: -1,
		},
		{
			name:     "找左半部分元素",
			nums:     []int{4, 5, 6, 7, 0, 1, 2},
			target:   5,
			expected: 1,
		},
		{
			name:     "找右半部分元素",
			nums:     []int{4, 5, 6, 7, 0, 1, 2},
			target:   2,
			expected: 6,
		},
		{
			name:     "单元素-找到",
			nums:     []int{1},
			target:   1,
			expected: 0,
		},
		{
			name:     "单元素-未找到",
			nums:     []int{1},
			target:   0,
			expected: -1,
		},
		{
			name:     "未旋转",
			nums:     []int{1, 2, 3, 4, 5},
			target:   3,
			expected: 2,
		},
		{
			name:     "旋转一位",
			nums:     []int{5, 1, 2, 3, 4},
			target:   5,
			expected: 0,
		},
		{
			name:     "两个元素-旋转",
			nums:     []int{3, 1},
			target:   1,
			expected: 1,
		},
		{
			name:     "两个元素-未找到",
			nums:     []int{3, 1},
			target:   2,
			expected: -1,
		},
		{
			name:     "找最后一个元素",
			nums:     []int{6, 7, 1, 2, 3, 4, 5},
			target:   5,
			expected: 6,
		},
		{
			name:     "找第一个元素",
			nums:     []int{6, 7, 1, 2, 3, 4, 5},
			target:   6,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := search(tt.nums, tt.target)
			if got != tt.expected {
				t.Errorf("search(%v, %d) = %d, 期望 %d",
					tt.nums, tt.target, got, tt.expected)
			}
		})
	}
}
