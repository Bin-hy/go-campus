package word_break

import "testing"

func TestWordBreak(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		wordDict []string
		expected bool
	}{
		{
			name:     "可拆分-两段",
			s:        "leetcode",
			wordDict: []string{"leet", "code"},
			expected: true,
		},
		{
			name:     "可拆分-重复使用",
			s:        "applepenapple",
			wordDict: []string{"apple", "pen"},
			expected: true,
		},
		{
			name:     "不可拆分",
			s:        "catsandog",
			wordDict: []string{"cats", "dog", "sand", "and", "cat"},
			expected: false,
		},
		{
			name:     "单字符",
			s:        "a",
			wordDict: []string{"a"},
			expected: true,
		},
		{
			name:     "单字符不在字典",
			s:        "b",
			wordDict: []string{"a"},
			expected: false,
		},
		{
			name:     "完全匹配",
			s:        "hello",
			wordDict: []string{"hello"},
			expected: true,
		},
		{
			name:     "多种拆法",
			s:        "pineapplepenapple",
			wordDict: []string{"apple", "pen", "applepen", "pine", "pineapple"},
			expected: true,
		},
		{
			name:     "重复字符",
			s:        "aaaaaaa",
			wordDict: []string{"aaa", "aaaa"},
			expected: true,
		},
		{
			name:     "重复字符不可拆",
			s:        "aaaaaaa",
			wordDict: []string{"aa", "aaa"},
			expected: true,
		},
		{
			name:     "长串回溯陷阱",
			s:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaab",
			wordDict: []string{"a", "aa", "aaa", "aaaa", "aaaaa"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wordBreak(tt.s, tt.wordDict)
			if got != tt.expected {
				t.Errorf("wordBreak(%q, %v) = %v, 期望 %v",
					tt.s, tt.wordDict, got, tt.expected)
			}
		})
	}
}
