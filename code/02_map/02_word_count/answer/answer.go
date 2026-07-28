//go:build ignore

package answer

import (
	"sort"
	"strings"
)

func WordCount(text string) map[string]int {
	result := make(map[string]int)
	for _, word := range strings.Fields(text) {
		result[strings.ToLower(word)]++
	}
	return result
}

func TopN(freq map[string]int, n int) []string {
	if n <= 0 {
		return []string{}
	}

	words := make([]string, 0, len(freq))
	for w := range freq {
		words = append(words, w)
	}

	sort.Slice(words, func(i, j int) bool {
		if freq[words[i]] != freq[words[j]] {
			return freq[words[i]] > freq[words[j]]
		}
		return words[i] < words[j]
	})

	if n > len(words) {
		n = len(words)
	}
	return words[:n]
}

func UniqueWords(text string) []string {
	freq := WordCount(text)
	var result []string
	for word, count := range freq {
		if count == 1 {
			result = append(result, word)
		}
	}
	sort.Strings(result)
	if result == nil {
		return []string{}
	}
	return result
}
