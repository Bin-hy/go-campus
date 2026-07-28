package num_islands

import "testing"

// --- 辅助函数 ---

// makeGrid 从字符串数组构建 byte grid
func makeGrid(rows []string) [][]byte {
	grid := make([][]byte, len(rows))
	for i, row := range rows {
		grid[i] = []byte(row)
	}
	return grid
}

// --- 测试用例 ---

func TestNumIslands(t *testing.T) {
	tests := []struct {
		name     string
		grid     []string
		expected int
	}{
		{
			name:     "一个大岛",
			grid:     []string{"11110", "11010", "11000", "00000"},
			expected: 1,
		},
		{
			name:     "三个岛",
			grid:     []string{"11000", "11000", "00100", "00011"},
			expected: 3,
		},
		{
			name:     "全是陆地",
			grid:     []string{"111", "111", "111"},
			expected: 1,
		},
		{
			name:     "全是水域",
			grid:     []string{"000", "000", "000"},
			expected: 0,
		},
		{
			name:     "单格陆地",
			grid:     []string{"1"},
			expected: 1,
		},
		{
			name:     "单格水域",
			grid:     []string{"0"},
			expected: 0,
		},
		{
			name:     "棋盘格",
			grid:     []string{"101", "010", "101"},
			expected: 5,
		},
		{
			name:     "L形岛屿",
			grid:     []string{"110", "100", "100"},
			expected: 1,
		},
		{
			name:     "对角不相连",
			grid:     []string{"10", "01"},
			expected: 2,
		},
		{
			name:     "长条形",
			grid:     []string{"11111"},
			expected: 1,
		},
		{
			name:     "单列",
			grid:     []string{"1", "0", "1", "0", "1"},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grid := makeGrid(tt.grid)
			got := numIslands(grid)
			if got != tt.expected {
				t.Errorf("numIslands(%v) = %d, 期望 %d",
					tt.grid, got, tt.expected)
			}
		})
	}
}
