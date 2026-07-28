//go:build ignore

package answer

func numIslands(grid [][]byte) int {
	if len(grid) == 0 {
		return 0
	}
	rows, cols := len(grid), len(grid[0])
	count := 0
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if grid[i][j] == '1' {
				count++
				dfs(grid, i, j, rows, cols)
			}
		}
	}
	return count
}

func dfs(grid [][]byte, i, j, rows, cols int) {
	if i < 0 || i >= rows || j < 0 || j >= cols || grid[i][j] != '1' {
		return
	}
	grid[i][j] = '0'
	dfs(grid, i+1, j, rows, cols)
	dfs(grid, i-1, j, rows, cols)
	dfs(grid, i, j+1, rows, cols)
	dfs(grid, i, j-1, rows, cols)
}
