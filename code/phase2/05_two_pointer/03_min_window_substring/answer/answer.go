//go:build ignore

package answer

func minWindow(s string, t string) string {
	need := make(map[byte]int)
	for i := 0; i < len(t); i++ {
		need[t[i]]++
	}

	window := make(map[byte]int)
	left := 0
	matched := 0
	start, minLen := 0, len(s)+1

	for right := 0; right < len(s); right++ {
		c := s[right]
		if _, ok := need[c]; ok {
			window[c]++
			if window[c] == need[c] {
				matched++
			}
		}

		for matched == len(need) {
			if right-left+1 < minLen {
				minLen = right - left + 1
				start = left
			}
			d := s[left]
			if _, ok := need[d]; ok {
				if window[d] == need[d] {
					matched--
				}
				window[d]--
			}
			left++
		}
	}

	if minLen == len(s)+1 {
		return ""
	}
	return s[start : start+minLen]
}
