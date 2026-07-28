package valid_parentheses

import "testing"

func TestIsValid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"简单匹配", "()", true},
		{"多种括号", "()[]{}", true},
		{"嵌套", "{[()]}", true},
		{"不匹配", "(]", false},
		{"交叉", "([)]", false},
		{"空字符串", "", true},
		{"只有左括号", "(((", false},
		{"只有右括号", ")))", false},
		{"复杂嵌套", "{[]()({[]})}", true},
		{"右括号多余", "()}", false},
		{"单个左括号", "(", false},
		{"单个右括号", ")", false},
		{"大量嵌套", "((((((()))))))", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValid(tt.input)
			if got != tt.expected {
				t.Errorf("isValid(%q) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}
