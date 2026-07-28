package word_count

import (
	"reflect"
	"testing"
)

func TestWordCount(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected map[string]int
	}{
		{
			name:     "基本统计",
			text:     "hello world hello",
			expected: map[string]int{"hello": 2, "world": 1},
		},
		{
			name:     "大小写不敏感",
			text:     "Go go GO",
			expected: map[string]int{"go": 3},
		},
		{
			name:     "多空格分隔",
			text:     "a  b   c  a",
			expected: map[string]int{"a": 2, "b": 1, "c": 1},
		},
		{
			name:     "空文本",
			text:     "",
			expected: map[string]int{},
		},
		{
			name:     "单个单词",
			text:     "alone",
			expected: map[string]int{"alone": 1},
		},
		{
			name:     "含tab和换行",
			text:     "hello\tworld\nhello",
			expected: map[string]int{"hello": 2, "world": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WordCount(tt.text)
			if result == nil {
				t.Fatal("不应返回 nil")
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("期望 %v，得到 %v", tt.expected, result)
			}
		})
	}
}

func TestTopN(t *testing.T) {
	tests := []struct {
		name     string
		freq     map[string]int
		n        int
		expected []string
	}{
		{
			name:     "正常TopN",
			freq:     map[string]int{"apple": 5, "banana": 3, "cherry": 8, "date": 1},
			n:        2,
			expected: []string{"cherry", "apple"},
		},
		{
			name:     "频次相同按字母序",
			freq:     map[string]int{"banana": 3, "apple": 3, "cherry": 3},
			n:        3,
			expected: []string{"apple", "banana", "cherry"},
		},
		{
			name:     "n大于单词数",
			freq:     map[string]int{"a": 1, "b": 2},
			n:        10,
			expected: []string{"b", "a"},
		},
		{
			name:     "n为0",
			freq:     map[string]int{"a": 1, "b": 2},
			n:        0,
			expected: []string{},
		},
		{
			name:     "空map",
			freq:     map[string]int{},
			n:        5,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TopN(tt.freq, tt.n)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("期望 %v，得到 %v", tt.expected, result)
			}
		})
	}
}

func TestUniqueWords(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "有唯一词",
			text:     "the cat sat on the mat",
			expected: []string{"cat", "mat", "on", "sat"},
		},
		{
			name:     "全部唯一",
			text:     "a b c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "无唯一词",
			text:     "go go go",
			expected: []string{},
		},
		{
			name:     "空文本",
			text:     "",
			expected: []string{},
		},
		{
			name:     "大小写统一后重复",
			text:     "Go go Hello hello World",
			expected: []string{"world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UniqueWords(tt.text)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("期望 %v，得到 %v", tt.expected, result)
			}
		})
	}
}
