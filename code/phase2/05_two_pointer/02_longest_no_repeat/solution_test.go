package longest_no_repeat

import "testing"

func TestLengthOfLongestSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		expected int
	}{
		{"标准示例1", "abcabcbb", 3},
		{"全部重复", "bbbbb", 1},
		{"标准示例3", "pwwkew", 3},
		{"空字符串", "", 0},
		{"单字符", "a", 1},
		{"两个不同字符", "ab", 2},
		{"两个相同字符", "aa", 1},
		{"无重复", "abcdef", 6},
		{"尾部最长", "aab", 2},
		{"含空格", "a b c", 3},
		{"含数字", "abc123abc", 6},
		{"交替重复", "abababab", 2},
		{"长串无重复在中间", "aaabcdefgaaa", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lengthOfLongestSubstring(tt.s)
			if got != tt.expected {
				t.Errorf("lengthOfLongestSubstring(%q) = %d, 期望 %d",
					tt.s, got, tt.expected)
			}
		})
	}
}
