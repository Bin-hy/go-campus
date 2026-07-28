//go:build ignore

package answer

func permute(nums []int) [][]int {
	var result [][]int
	used := make([]bool, len(nums))
	var backtrack func(path []int)
	backtrack = func(path []int) {
		if len(path) == len(nums) {
			tmp := make([]int, len(path))
			copy(tmp, path)
			result = append(result, tmp)
			return
		}
		for i := 0; i < len(nums); i++ {
			if used[i] {
				continue
			}
			used[i] = true
			path = append(path, nums[i])
			backtrack(path)
			path = path[:len(path)-1]
			used[i] = false
		}
	}
	backtrack(nil)
	return result
}
