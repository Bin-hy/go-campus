//go:build ignore

package answer

import "math/rand"

func findKthLargest(nums []int, k int) int {
	target := len(nums) - k // 第k大 = 排序后索引 len-k
	return quickSelect(nums, 0, len(nums)-1, target)
}

func quickSelect(nums []int, left, right, target int) int {
	if left == right {
		return nums[left]
	}
	pivotIdx := left + rand.Intn(right-left+1)
	nums[pivotIdx], nums[right] = nums[right], nums[pivotIdx]

	pivot := nums[right]
	i := left
	for j := left; j < right; j++ {
		if nums[j] < pivot {
			nums[i], nums[j] = nums[j], nums[i]
			i++
		}
	}
	nums[i], nums[right] = nums[right], nums[i]

	if i == target {
		return nums[i]
	} else if i < target {
		return quickSelect(nums, i+1, right, target)
	} else {
		return quickSelect(nums, left, i-1, target)
	}
}
