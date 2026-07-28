package find_first_last

import "testing"

// --- 辅助函数 ---

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

func TestSearchRange(t *testing.T) {
	tests := []struct {
		name     string
		nums     []int
		target   int
		expected []int
	}{
		{
			name:     "标准示例-存在",
			nums:     []int{5, 7, 7, 8, 8, 10},
			target:   8,
			expected: []int{3, 4},
		},
		{
			name:     "标准示例-不存在",
			nums:     []int{5, 7, 7, 8, 8, 10},
			target:   6,
			expected: []int{-1, -1},
		},
		{
			name:     "空数组",
			nums:     []int{},
			target:   0,
			expected: []int{-1, -1},
		},
		{
			name:     "单元素-找到",
			nums:     []int{1},
			target:   1,
			expected: []int{0, 0},
		},
		{
			name:     "单元素-未找到",
			nums:     []int{1},
			target:   2,
			expected: []int{-1, -1},
		},
		{
			name:     "全部相同",
			nums:     []int{2, 2, 2, 2, 2},
			target:   2,
			expected: []int{0, 4},
		},
		{
			name:     "target在开头",
			nums:     []int{1, 1, 2, 3, 4},
			target:   1,
			expected: []int{0, 1},
		},
		{
			name:     "target在结尾",
			nums:     []int{1, 2, 3, 4, 4},
			target:   4,
			expected: []int{3, 4},
		},
		{
			name:     "target出现一次",
			nums:     []int{1, 2, 3, 4, 5},
			target:   3,
			expected: []int{2, 2},
		},
		{
			name:     "target小于所有元素",
			nums:     []int{2, 3, 4, 5},
			target:   1,
			expected: []int{-1, -1},
		},
		{
			name:     "target大于所有元素",
			nums:     []int{2, 3, 4, 5},
			target:   6,
			expected: []int{-1, -1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searchRange(tt.nums, tt.target)
			if !equal(got, tt.expected) {
				t.Errorf("searchRange(%v, %d) = %v, 期望 %v",
					tt.nums, tt.target, got, tt.expected)
			}
		})
	}
}
