package min_window_substring

import "testing"

func TestMinWindow(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		tStr     string
		expected string
	}{
		{
			name:     "标准示例",
			s:        "ADOBECODEBANC",
			tStr:     "ABC",
			expected: "BANC",
		},
		{
			name:     "完全相等",
			s:        "a",
			tStr:     "a",
			expected: "a",
		},
		{
			name:     "t比s长",
			s:        "a",
			tStr:     "aa",
			expected: "",
		},
		{
			name:     "无解",
			s:        "abc",
			tStr:     "d",
			expected: "",
		},
		{
			name:     "s等于t",
			s:        "abc",
			tStr:     "abc",
			expected: "abc",
		},
		{
			name:     "t中有重复字符",
			s:        "aaabbc",
			tStr:     "aab",
			expected: "aab",
		},
		{
			name:     "最小窗口在结尾",
			s:        "cabwefgewcwaefgcf",
			tStr:     "cae",
			expected: "cwae",
		},
		{
			name:     "单字符匹配",
			s:        "ab",
			tStr:     "b",
			expected: "b",
		},
		{
			name:     "s为空",
			s:        "",
			tStr:     "a",
			expected: "",
		},
		{
			name:     "t为单字符重复",
			s:        "bdab",
			tStr:     "ab",
			expected: "ab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minWindow(tt.s, tt.tStr)
			if got != tt.expected {
				t.Errorf("minWindow(%q, %q) = %q, 期望 %q",
					tt.s, tt.tStr, got, tt.expected)
			}
		})
	}
}
