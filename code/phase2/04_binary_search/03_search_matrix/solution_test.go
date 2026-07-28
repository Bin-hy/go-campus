package search_matrix

import "testing"

func TestSearchMatrix(t *testing.T) {
	tests := []struct {
		name     string
		matrix   [][]int
		target   int
		expected bool
	}{
		{
			name:     "找到-中间",
			matrix:   [][]int{{1, 3, 5, 7}, {10, 11, 16, 20}, {23, 30, 34, 60}},
			target:   3,
			expected: true,
		},
		{
			name:     "未找到",
			matrix:   [][]int{{1, 3, 5, 7}, {10, 11, 16, 20}, {23, 30, 34, 60}},
			target:   13,
			expected: false,
		},
		{
			name:     "找到-第一个元素",
			matrix:   [][]int{{1, 3, 5, 7}, {10, 11, 16, 20}, {23, 30, 34, 60}},
			target:   1,
			expected: true,
		},
		{
			name:     "找到-最后一个元素",
			matrix:   [][]int{{1, 3, 5, 7}, {10, 11, 16, 20}, {23, 30, 34, 60}},
			target:   60,
			expected: true,
		},
		{
			name:     "target小于最小值",
			matrix:   [][]int{{1, 3, 5, 7}, {10, 11, 16, 20}},
			target:   0,
			expected: false,
		},
		{
			name:     "target大于最大值",
			matrix:   [][]int{{1, 3, 5, 7}, {10, 11, 16, 20}},
			target:   100,
			expected: false,
		},
		{
			name:     "单行",
			matrix:   [][]int{{1, 3, 5, 7, 9}},
			target:   5,
			expected: true,
		},
		{
			name:     "单列",
			matrix:   [][]int{{1}, {3}, {5}, {7}},
			target:   3,
			expected: true,
		},
		{
			name:     "1x1找到",
			matrix:   [][]int{{1}},
			target:   1,
			expected: true,
		},
		{
			name:     "1x1未找到",
			matrix:   [][]int{{1}},
			target:   2,
			expected: false,
		},
		{
			name:     "找到行首",
			matrix:   [][]int{{1, 3, 5}, {7, 9, 11}, {13, 15, 17}},
			target:   7,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searchMatrix(tt.matrix, tt.target)
			if got != tt.expected {
				t.Errorf("searchMatrix(%v, %d) = %v, 期望 %v",
					tt.matrix, tt.target, got, tt.expected)
			}
		})
	}
}
