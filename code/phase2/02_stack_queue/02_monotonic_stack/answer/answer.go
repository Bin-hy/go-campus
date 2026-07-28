//go:build ignore

package answer

func dailyTemperatures(temperatures []int) []int {
	n := len(temperatures)
	result := make([]int, n)
	stack := make([]int, 0) // 存索引，保持栈内温度单调递减

	for i := 0; i < n; i++ {
		for len(stack) > 0 && temperatures[i] > temperatures[stack[len(stack)-1]] {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			result[top] = i - top
		}
		stack = append(stack, i)
	}
	return result
}
