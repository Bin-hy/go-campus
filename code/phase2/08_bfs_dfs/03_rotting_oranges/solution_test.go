package rotting_oranges

import "testing"

// --- 辅助函数 ---

func copyGrid(grid [][]int) [][]int {
	cp := make([][]int, len(grid))
	for i := range grid {
		cp[i] = make([]int, len(grid[i]))
		copy(cp[i], grid[i])
	}
	return cp
}

// --- 测试用例 ---

func TestOrangesRotting(t *testing.T) {
	tests := []struct {
		name     string
		grid     [][]int
		expected int
	}{
		{
			name:     "标准示例-4分钟",
			grid:     [][]int{{2, 1, 1}, {1, 1, 0}, {0, 1, 1}},
			expected: 4,
		},
		{
			name:     "不可能全部腐烂",
			grid:     [][]int{{2, 1, 1}, {0, 1, 1}, {1, 0, 1}},
			expected: -1,
		},
		{
			name:     "没有新鲜橘子",
			grid:     [][]int{{0, 2}},
			expected: 0,
		},
		{
			name:     "全是新鲜-无腐烂源",
			grid:     [][]int{{1, 1, 1}, {1, 1, 1}},
			expected: -1,
		},
		{
			name:     "全空",
			grid:     [][]int{{0, 0, 0}, {0, 0, 0}},
			expected: 0,
		},
		{
			name:     "全腐烂",
			grid:     [][]int{{2, 2, 2}, {2, 2, 2}},
			expected: 0,
		},
		{
			name:     "单格新鲜相邻腐烂",
			grid:     [][]int{{2, 1}},
			expected: 1,
		},
		{
			name:     "一行传播",
			grid:     [][]int{{2, 1, 1, 1, 1}},
			expected: 4,
		},
		{
			name:     "多源同时传播",
			grid:     [][]int{{2, 1, 1, 1, 2}},
			expected: 2,
		},
		{
			name:     "L形传播",
			grid:     [][]int{{2, 1, 0}, {1, 0, 0}, {1, 1, 1}},
			expected: 4,
		},
		{
			name:     "单格新鲜被隔离",
			grid:     [][]int{{0, 1, 0}, {0, 0, 0}, {0, 2, 0}},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grid := copyGrid(tt.grid)
			got := orangesRotting(grid)
			if got != tt.expected {
				t.Errorf("orangesRotting(%v) = %d, 期望 %d",
					tt.grid, got, tt.expected)
			}
		})
	}
}
