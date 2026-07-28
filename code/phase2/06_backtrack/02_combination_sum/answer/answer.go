//go:build ignore

package answer

import "sort"

func combinationSum(candidates []int, target int) [][]int {
	sort.Ints(candidates)
	var result [][]int
	var backtrack func(start int, path []int, remain int)
	backtrack = func(start int, path []int, remain int) {
		if remain == 0 {
			tmp := make([]int, len(path))
			copy(tmp, path)
			result = append(result, tmp)
			return
		}
		for i := start; i < len(candidates); i++ {
			if candidates[i] > remain {
				break
			}
			path = append(path, candidates[i])
			backtrack(i, path, remain-candidates[i])
			path = path[:len(path)-1]
		}
	}
	backtrack(0, nil, target)
	return result
}
