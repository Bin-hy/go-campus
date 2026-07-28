package combination_sum

import (
	"sort"
	"testing"
)

// --- 辅助函数 ---

func sortResults(results [][]int) {
	for _, r := range results {
		sort.Ints(r)
	}
	sort.Slice(results, func(i, j int) bool {
		for k := 0; k < len(results[i]) && k < len(results[j]); k++ {
			if results[i][k] != results[j][k] {
				return results[i][k] < results[j][k]
			}
		}
		return len(results[i]) < len(results[j])
	})
}

func equalResults(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	sortResults(a)
	sortResults(b)
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

func TestCombinationSum(t *testing.T) {
	tests := []struct {
		name       string
		candidates []int
		target     int
		expected   [][]int
	}{
		{
			name:       "标准示例1",
			candidates: []int{2, 3, 6, 7},
			target:     7,
			expected:   [][]int{{2, 2, 3}, {7}},
		},
		{
			name:       "标准示例2",
			candidates: []int{2, 3, 5},
			target:     8,
			expected:   [][]int{{2, 2, 2, 2}, {2, 3, 3}, {3, 5}},
		},
		{
			name:       "无解",
			candidates: []int{2},
			target:     1,
			expected:   [][]int{},
		},
		{
			name:       "单一候选",
			candidates: []int{1},
			target:     3,
			expected:   [][]int{{1, 1, 1}},
		},
		{
			name:       "target等于候选",
			candidates: []int{3, 5, 7},
			target:     5,
			expected:   [][]int{{5}},
		},
		{
			name:       "多种组合",
			candidates: []int{2, 3, 5},
			target:     5,
			expected:   [][]int{{2, 3}, {5}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := combinationSum(tt.candidates, tt.target)
			if len(tt.expected) == 0 {
				if len(got) != 0 {
					t.Errorf("combinationSum(%v, %d) = %v, 期望空",
						tt.candidates, tt.target, got)
				}
				return
			}
			if !equalResults(got, tt.expected) {
				t.Errorf("combinationSum(%v, %d) = %v, 期望 %v",
					tt.candidates, tt.target, got, tt.expected)
			}
		})
	}
}
