package three_sum

import (
	"sort"
	"testing"
)

// --- 辅助函数 ---

func sortResult(result [][]int) {
	for _, triplet := range result {
		sort.Ints(triplet)
	}
	sort.Slice(result, func(i, j int) bool {
		for k := 0; k < 3; k++ {
			if result[i][k] != result[j][k] {
				return result[i][k] < result[j][k]
			}
		}
		return false
	})
}

func equalResult(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	sortResult(a)
	sortResult(b)
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

// --- 测试用例 ---

func TestThreeSum(t *testing.T) {
	tests := []struct {
		name     string
		nums     []int
		expected [][]int
	}{
		{
			name:     "标准示例",
			nums:     []int{-1, 0, 1, 2, -1, -4},
			expected: [][]int{{-1, -1, 2}, {-1, 0, 1}},
		},
		{
			name:     "无解",
			nums:     []int{0, 1, 1},
			expected: [][]int{},
		},
		{
			name:     "全零",
			nums:     []int{0, 0, 0},
			expected: [][]int{{0, 0, 0}},
		},
		{
			name:     "多个零",
			nums:     []int{0, 0, 0, 0},
			expected: [][]int{{0, 0, 0}},
		},
		{
			name:     "只有两个元素",
			nums:     []int{0, 1},
			expected: [][]int{},
		},
		{
			name:     "含大量重复",
			nums:     []int{-2, -2, -1, -1, 0, 0, 1, 1, 2, 2},
			expected: [][]int{{-2, 0, 2}, {-2, 1, 1}, {-1, -1, 2}, {-1, 0, 1}},
		},
		{
			name:     "全正数无解",
			nums:     []int{1, 2, 3, 4, 5},
			expected: [][]int{},
		},
		{
			name:     "全负数无解",
			nums:     []int{-5, -4, -3, -2, -1},
			expected: [][]int{},
		},
		{
			name:     "三个元素刚好",
			nums:     []int{-1, 0, 1},
			expected: [][]int{{-1, 0, 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := threeSum(tt.nums)
			if len(tt.expected) == 0 {
				if len(got) != 0 {
					t.Errorf("threeSum(%v) = %v, 期望空", tt.nums, got)
				}
				return
			}
			if !equalResult(got, tt.expected) {
				t.Errorf("threeSum(%v) = %v, 期望 %v", tt.nums, got, tt.expected)
			}
		})
	}
}
