//go:build ignore

package answer

func maxSlidingWindow(nums []int, k int) []int {
	if len(nums) == 0 || k == 0 {
		return nil
	}
	deque := make([]int, 0)   // 存索引，单调递减队列
	result := make([]int, 0, len(nums)-k+1)

	for i := 0; i < len(nums); i++ {
		// 移除超出窗口范围的队首
		for len(deque) > 0 && deque[0] <= i-k {
			deque = deque[1:]
		}
		// 维护单调递减：移除队尾所有比当前值小的
		for len(deque) > 0 && nums[deque[len(deque)-1]] < nums[i] {
			deque = deque[:len(deque)-1]
		}
		deque = append(deque, i)
		// 窗口形成后收集结果
		if i >= k-1 {
			result = append(result, nums[deque[0]])
		}
	}
	return result
}
