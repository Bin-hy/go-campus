//go:build ignore

package answer

func CreateOnStack() int {
	x := 42
	return x
}

func CreateOnHeap() *int {
	x := 42
	return &x // 取地址 → 逃逸
}

func SliceNoEscape() int {
	s := [5]int{1, 2, 3, 4, 5} // 数组在栈上
	sum := 0
	for _, v := range s {
		sum += v
	}
	return sum
}

func SliceEscape() []int {
	s := make([]int, 10)
	for i := range s {
		s[i] = i + 1
	}
	return s // 返回 slice → 逃逸
}

func SumWithInterface(nums ...interface{}) int {
	sum := 0
	for _, n := range nums {
		if v, ok := n.(int); ok {
			sum += v
		}
	}
	return sum
}

func SumDirect(a, b, c int) int {
	return a + b + c
}
