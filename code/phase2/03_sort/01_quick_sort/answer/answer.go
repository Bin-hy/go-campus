//go:build ignore

package answer

import "math/rand"

func sortArray(nums []int) []int {
	quickSort(nums, 0, len(nums)-1)
	return nums
}

func quickSort(nums []int, left, right int) {
	if left >= right {
		return
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

	quickSort(nums, left, i-1)
	quickSort(nums, i+1, right)
}
