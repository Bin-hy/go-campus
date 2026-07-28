package coin_change

import "testing"

func TestCoinChange(t *testing.T) {
	tests := []struct {
		name     string
		coins    []int
		amount   int
		expected int
	}{
		{"标准示例", []int{1, 5, 10, 25}, 30, 2},
		{"无解", []int{2}, 3, -1},
		{"金额为0", []int{1}, 0, 0},
		{"一种面额", []int{1}, 5, 5},
		{"经典示例", []int{1, 2, 5}, 11, 3},
		{"大额面值", []int{186, 419, 83, 408}, 6249, 20},
		{"面额刚好", []int{3, 7}, 7, 1},
		{"两种凑法取最小", []int{1, 3, 5}, 9, 3},
		{"单枚硬币等于金额", []int{2, 5, 10}, 10, 1},
		{"无解-面额太大", []int{5, 10}, 3, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coinChange(tt.coins, tt.amount)
			if got != tt.expected {
				t.Errorf("coinChange(%v, %d) = %d, 期望 %d",
					tt.coins, tt.amount, got, tt.expected)
			}
		})
	}
}
