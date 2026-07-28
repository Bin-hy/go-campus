//go:build ignore

package answer

// DeepCopy 参考答案
func DeepCopy(src []int) []int {
	if src == nil {
		return nil
	}
	dst := make([]int, len(src))
	copy(dst, src)
	return dst
}
