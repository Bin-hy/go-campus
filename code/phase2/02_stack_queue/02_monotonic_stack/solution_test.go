package monotonic_stack

import "testing"

// --- 辅助函数 ---

func equal(a, b []int) bool {
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

// --- 测试用例 ---

func TestDailyTemperatures(t *testing.T) {
	tests := []struct {
		name         string
		temperatures []int
		expected     []int
	}{
		{
			name:         "标准示例",
			temperatures: []int{73, 74, 75, 71, 69, 72, 76, 73},
			expected:     []int{1, 1, 4, 2, 1, 1, 0, 0},
		},
		{
			name:         "单调递增",
			temperatures: []int{30, 40, 50, 60},
			expected:     []int{1, 1, 1, 0},
		},
		{
			name:         "单调递减",
			temperatures: []int{60, 50, 40, 30},
			expected:     []int{0, 0, 0, 0},
		},
		{
			name:         "三个元素递增",
			temperatures: []int{30, 60, 90},
			expected:     []int{1, 1, 0},
		},
		{
			name:         "单个元素",
			temperatures: []int{50},
			expected:     []int{0},
		},
		{
			name:         "两个相同",
			temperatures: []int{50, 50},
			expected:     []int{0, 0},
		},
		{
			name:         "全部相同",
			temperatures: []int{70, 70, 70, 70},
			expected:     []int{0, 0, 0, 0},
		},
		{
			name:         "V形",
			temperatures: []int{80, 60, 40, 60, 80},
			expected:     []int{0, 2, 1, 1, 0},
		},
		{
			name:         "锯齿形",
			temperatures: []int{30, 50, 20, 60, 10, 70},
			expected:     []int{1, 2, 1, 2, 1, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dailyTemperatures(tt.temperatures)
			if !equal(got, tt.expected) {
				t.Errorf("dailyTemperatures(%v) = %v, 期望 %v",
					tt.temperatures, got, tt.expected)
			}
		})
	}
}
