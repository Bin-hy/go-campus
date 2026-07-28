package permutations

import (
	"sort"
	"testing"
)

// --- 辅助函数 ---

func sortResults(results [][]int) {
	for _, r := range results {
		// 每个排列本身不排序（排列顺序有意义）
		_ = r
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

func TestPermute(t *testing.T) {
	tests := []struct {
		name     string
		nums     []int
		expected [][]int
	}{
		{
			name: "三个数",
			nums: []int{1, 2, 3},
			expected: [][]int{
				{1, 2, 3}, {1, 3, 2}, {2, 1, 3},
				{2, 3, 1}, {3, 1, 2}, {3, 2, 1},
			},
		},
		{
			name:     "两个数",
			nums:     []int{0, 1},
			expected: [][]int{{0, 1}, {1, 0}},
		},
		{
			name:     "单个数",
			nums:     []int{1},
			expected: [][]int{{1}},
		},
		{
			name: "四个数-验证数量",
			nums: []int{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := permute(tt.nums)

			if tt.expected != nil {
				if !equalResults(got, tt.expected) {
					t.Errorf("permute(%v) 结果不匹配\n得到: %v\n期望: %v",
						tt.nums, got, tt.expected)
				}
			} else {
				// 只验证数量 n!
				factorial := 1
				for i := 1; i <= len(tt.nums); i++ {
					factorial *= i
				}
				if len(got) != factorial {
					t.Errorf("permute(%v) 数量 = %d, 期望 %d",
						tt.nums, len(got), factorial)
				}
			}
		})
	}
}

func TestPermute_NoDuplicate(t *testing.T) {
	got := permute([]int{1, 2, 3})
	seen := make(map[string]bool)
	for _, perm := range got {
		key := ""
		for _, v := range perm {
			key += string(rune('0' + v))
		}
		if seen[key] {
			t.Errorf("发现重复排列: %v", perm)
		}
		seen[key] = true
	}
}
