//go:build ignore

package answer

func wordBreak(s string, wordDict []string) bool {
	wordSet := make(map[string]bool)
	for _, w := range wordDict {
		wordSet[w] = true
	}
	dp := make([]bool, len(s)+1)
	dp[0] = true
	for i := 1; i <= len(s); i++ {
		for j := 0; j < i; j++ {
			if dp[j] && wordSet[s[j:i]] {
				dp[i] = true
				break
			}
		}
	}
	return dp[len(s)]
}
