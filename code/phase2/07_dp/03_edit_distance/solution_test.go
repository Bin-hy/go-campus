package edit_distance

import "testing"

func TestMinDistance(t *testing.T) {
	tests := []struct {
		name     string
		word1    string
		word2    string
		expected int
	}{
		{"标准示例1", "horse", "ros", 3},
		{"标准示例2", "intention", "execution", 5},
		{"相同", "abc", "abc", 0},
		{"空到非空", "", "abc", 3},
		{"非空到空", "abc", "", 3},
		{"都为空", "", "", 0},
		{"单字符相同", "a", "a", 0},
		{"单字符不同", "a", "b", 1},
		{"插入一个字符", "ab", "abc", 1},
		{"删除一个字符", "abc", "ab", 1},
		{"完全不同", "abc", "xyz", 3},
		{"长度差异大", "a", "abcdef", 5},
		{"实际场景", "kitten", "sitting", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minDistance(tt.word1, tt.word2)
			if got != tt.expected {
				t.Errorf("minDistance(%q, %q) = %d, 期望 %d",
					tt.word1, tt.word2, got, tt.expected)
			}
		})
	}
}
