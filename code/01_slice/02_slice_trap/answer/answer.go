//go:build ignore

package answer

func AppendNoEffect(s []int, val int) bool {
	return len(s) == cap(s)
}

func SafeSubSlice(s []int, start, end int) []int {
	sub := make([]int, end-start)
	copy(sub, s[start:end])
	return sub
}

func RemoveElement(s []int, index int) []int {
	if index < 0 || index >= len(s) {
		return s
	}
	return append(s[:index], s[index+1:]...)
}
