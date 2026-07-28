package sliding_window_max

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

func TestMaxSlidingWindow(t *testing.T) {
	tests := []struct {
		name     string
		nums     []int
		k        int
		expected []int
	}{
		{
			name:     "标准示例",
			nums:     []int{1, 3, -1, -3, 5, 3, 6, 7},
			k:        3,
			expected: []int{3, 3, 5, 5, 6, 7},
		},
		{
			name:     "k等于数组长度",
			nums:     []int{1, 3, -1, -3, 5},
			k:        5,
			expected: []int{5},
		},
		{
			name:     "k等于1",
			nums:     []int{1, 3, -1, -3, 5},
			k:        1,
			expected: []int{1, 3, -1, -3, 5},
		},
		{
			name:     "单调递增",
			nums:     []int{1, 2, 3, 4, 5},
			k:        3,
			expected: []int{3, 4, 5},
		},
		{
			name:     "单调递减",
			nums:     []int{5, 4, 3, 2, 1},
			k:        3,
			expected: []int{5, 4, 3},
		},
		{
			name:     "全部相同",
			nums:     []int{7, 7, 7, 7, 7},
			k:        3,
			expected: []int{7, 7, 7},
		},
		{
			name:     "包含负数",
			nums:     []int{-7, -8, 7, 5, 7, 1, 6, 0},
			k:        4,
			expected: []int{7, 7, 7, 7, 7},
		},
		{
			name:     "k等于2",
			nums:     []int{1, -1},
			k:        2,
			expected: []int{1},
		},
		{
			name:     "单个元素",
			nums:     []int{1},
			k:        1,
			expected: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxSlidingWindow(tt.nums, tt.k)
			if !equal(got, tt.expected) {
				t.Errorf("maxSlidingWindow(%v, %d) = %v, 期望 %v",
					tt.nums, tt.k, got, tt.expected)
			}
		})
	}
}
