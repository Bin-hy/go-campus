//go:build ignore

package answer

func lengthOfLongestSubstring(s string) int {
	window := make(map[byte]int) // 字符 -> 窗口内出现次数
	left := 0
	result := 0
	for right := 0; right < len(s); right++ {
		c := s[right]
		window[c]++
		for window[c] > 1 {
			d := s[left]
			window[d]--
			left++
		}
		if right-left+1 > result {
			result = right - left + 1
		}
	}
	return result
}
