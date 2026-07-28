package top_k

import "testing"

func TestFindKthLargest(t *testing.T) {
	tests := []struct {
		name     string
		nums     []int
		k        int
		expected int
	}{
		{
			name:     "标准示例1",
			nums:     []int{3, 2, 1, 5, 6, 4},
			k:        2,
			expected: 5,
		},
		{
			name:     "标准示例2",
			nums:     []int{3, 2, 3, 1, 2, 4, 5, 5, 6},
			k:        4,
			expected: 4,
		},
		{
			name:     "最大的元素",
			nums:     []int{3, 2, 1, 5, 6, 4},
			k:        1,
			expected: 6,
		},
		{
			name:     "最小的元素",
			nums:     []int{3, 2, 1, 5, 6, 4},
			k:        6,
			expected: 1,
		},
		{
			name:     "全部相同",
			nums:     []int{5, 5, 5, 5, 5},
			k:        3,
			expected: 5,
		},
		{
			name:     "两个元素取大",
			nums:     []int{1, 2},
			k:        1,
			expected: 2,
		},
		{
			name:     "两个元素取小",
			nums:     []int{1, 2},
			k:        2,
			expected: 1,
		},
		{
			name:     "单元素",
			nums:     []int{42},
			k:        1,
			expected: 42,
		},
		{
			name:     "含负数",
			nums:     []int{-1, -5, 3, 0, -2, 4},
			k:        2,
			expected: 3,
		},
		{
			name:     "大量重复",
			nums:     []int{1, 1, 1, 2, 2, 2, 3, 3, 3},
			k:        4,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 复制避免修改原始测试数据
			nums := make([]int, len(tt.nums))
			copy(nums, tt.nums)

			got := findKthLargest(nums, tt.k)
			if got != tt.expected {
				t.Errorf("findKthLargest(%v, %d) = %d, 期望 %d",
					tt.nums, tt.k, got, tt.expected)
			}
		})
	}
}
