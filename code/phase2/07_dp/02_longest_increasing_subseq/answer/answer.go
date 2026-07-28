//go:build ignore

package answer

import "sort"

// O(n*logn) 贪心 + 二分
func lengthOfLIS(nums []int) int {
	tails := make([]int, 0) // tails[i] = 长度为 i+1 的递增子序列的最小末尾
	for _, num := range nums {
		pos := sort.SearchInts(tails, num)
		if pos == len(tails) {
			tails = append(tails, num)
		} else {
			tails[pos] = num
		}
	}
	return len(tails)
}
