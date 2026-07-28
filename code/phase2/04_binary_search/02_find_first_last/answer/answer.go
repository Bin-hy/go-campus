//go:build ignore

package answer

func searchRange(nums []int, target int) []int {
	first := lowerBound(nums, target)
	if first == len(nums) || nums[first] != target {
		return []int{-1, -1}
	}
	last := lowerBound(nums, target+1) - 1
	return []int{first, last}
}

func lowerBound(nums []int, target int) int {
	left, right := 0, len(nums)-1
	for left <= right {
		mid := left + (right-left)/2
		if nums[mid] < target {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	return left
}
