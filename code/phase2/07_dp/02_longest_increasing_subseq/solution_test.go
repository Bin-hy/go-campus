package longest_increasing_subseq

import "testing"

func TestLengthOfLIS(t *testing.T) {
	tests := []struct {
		name     string
		nums     []int
		expected int
	}{
		{"标准示例", []int{10, 9, 2, 5, 3, 7, 101, 18}, 4},
		{"有平台", []int{0, 1, 0, 3, 2, 3}, 4},
		{"全部相同", []int{7, 7, 7, 7, 7}, 1},
		{"严格递增", []int{1, 2, 3, 4, 5}, 5},
		{"严格递减", []int{5, 4, 3, 2, 1}, 1},
		{"单元素", []int{10}, 1},
		{"两个元素递增", []int{1, 2}, 2},
		{"两个元素递减", []int{2, 1}, 1},
		{"V形", []int{5, 3, 1, 2, 4, 6}, 4},
		{"锯齿形", []int{1, 3, 2, 4, 3, 5}, 4},
		{"含负数", []int{-5, -2, 0, 3, 1, 4}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lengthOfLIS(tt.nums)
			if got != tt.expected {
				t.Errorf("lengthOfLIS(%v) = %d, 期望 %d",
					tt.nums, got, tt.expected)
			}
		})
	}
}
