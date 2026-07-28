package n_queens

import "testing"

// --- 辅助函数 ---

// isValidSolution 验证 N 皇后解是否合法
func isValidSolution(board []string) bool {
	n := len(board)
	for i := 0; i < n; i++ {
		if len(board[i]) != n {
			return false
		}
	}
	// 检查每行恰好一个 Q
	queens := make([][2]int, 0, n)
	for i := 0; i < n; i++ {
		count := 0
		for j := 0; j < n; j++ {
			if board[i][j] == 'Q' {
				count++
				queens = append(queens, [2]int{i, j})
			} else if board[i][j] != '.' {
				return false
			}
		}
		if count != 1 {
			return false
		}
	}
	// 检查无冲突
	for i := 0; i < len(queens); i++ {
		for j := i + 1; j < len(queens); j++ {
			r1, c1 := queens[i][0], queens[i][1]
			r2, c2 := queens[j][0], queens[j][1]
			if c1 == c2 {
				return false // 同列
			}
			if r1-c1 == r2-c2 {
				return false // 主对角线
			}
			if r1+c1 == r2+c2 {
				return false // 副对角线
			}
		}
	}
	return true
}

// --- 测试用例 ---

func TestSolveNQueens(t *testing.T) {
	tests := []struct {
		name          string
		n             int
		expectedCount int // 已知解的数量
	}{
		{"n=1", 1, 1},
		{"n=2", 2, 0},
		{"n=3", 3, 0},
		{"n=4", 4, 2},
		{"n=5", 5, 10},
		{"n=6", 6, 4},
		{"n=8", 8, 92},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := solveNQueens(tt.n)
			if len(got) != tt.expectedCount {
				t.Errorf("solveNQueens(%d) 返回 %d 个解, 期望 %d 个",
					tt.n, len(got), tt.expectedCount)
				return
			}
			// 验证每个解的合法性
			for i, solution := range got {
				if !isValidSolution(solution) {
					t.Errorf("solveNQueens(%d) 第 %d 个解不合法: %v",
						tt.n, i, solution)
				}
			}
		})
	}
}

func TestSolveNQueens_4_Content(t *testing.T) {
	got := solveNQueens(4)
	if len(got) != 2 {
		t.Fatalf("solveNQueens(4) 期望 2 个解, 得到 %d", len(got))
	}
	// 验证具体解
	expected1 := []string{".Q..", "...Q", "Q...", "..Q."}
	expected2 := []string{"..Q.", "Q...", "...Q", ".Q.."}

	found1, found2 := false, false
	for _, sol := range got {
		if equalBoard(sol, expected1) {
			found1 = true
		}
		if equalBoard(sol, expected2) {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Errorf("solveNQueens(4) 解不匹配，得到: %v", got)
	}
}

func equalBoard(a, b []string) bool {
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
