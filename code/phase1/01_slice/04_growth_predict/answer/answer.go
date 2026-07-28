//go:build ignore

package answer

func PredictGrowth(oldCap, needCap int) int {
	if oldCap == 0 {
		return needCap
	}

	newcap := oldCap

	if needCap > 2*oldCap {
		return needCap
	}

	const threshold = 256
	if oldCap < threshold {
		newcap = oldCap * 2
	} else {
		for newcap < needCap {
			newcap += (newcap + 3*threshold) / 4
		}
	}

	if newcap < needCap {
		newcap = needCap
	}

	return newcap
}

func OptimalPrealloc(appendSizes []int) (withoutPrealloc, withPrealloc int) {
	if len(appendSizes) == 0 {
		return 0, 0
	}

	// 计算总量
	total := 0
	for _, size := range appendSizes {
		total += size
	}

	// 模拟不预分配
	currentLen := 0
	currentCap := 0
	growCount := 0
	for _, size := range appendSizes {
		currentLen += size
		if currentLen > currentCap {
			currentCap = PredictGrowth(currentCap, currentLen)
			growCount++
		}
	}

	// 预分配：cap = total，不需要扩容
	return growCount, 0
}

func BatchAppend(slices ...[]int) []int {
	totalLen := 0
	for _, s := range slices {
		totalLen += len(s)
	}

	result := make([]int, 0, totalLen)
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}
