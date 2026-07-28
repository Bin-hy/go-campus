//go:build ignore

package answer

func orangesRotting(grid [][]int) int {
	rows, cols := len(grid), len(grid[0])
	queue := make([][2]int, 0)
	fresh := 0

	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if grid[i][j] == 2 {
				queue = append(queue, [2]int{i, j})
			} else if grid[i][j] == 1 {
				fresh++
			}
		}
	}

	if fresh == 0 {
		return 0
	}

	dirs := [4][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}
	minutes := 0

	for len(queue) > 0 {
		size := len(queue)
		for k := 0; k < size; k++ {
			cur := queue[0]
			queue = queue[1:]
			for _, d := range dirs {
				nx, ny := cur[0]+d[0], cur[1]+d[1]
				if nx >= 0 && nx < rows && ny >= 0 && ny < cols && grid[nx][ny] == 1 {
					grid[nx][ny] = 2
					fresh--
					queue = append(queue, [2]int{nx, ny})
				}
			}
		}
		minutes++
	}

	if fresh > 0 {
		return -1
	}
	if minutes == 0 {
		return 0
	}
	return minutes - 1
}
