package strategy_pattern

// Sorter 排序策略接口
type Sorter interface {
	Sort([]int) []int
	Name() string
}

// BubbleSort 冒泡排序
type BubbleSort struct{}

// Sort 用冒泡排序对切片排序（不修改原切片，返回新切片）
func (b BubbleSort) Sort(data []int) []int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

func (b BubbleSort) Name() string { return "bubble" }

// QuickSort 快速排序
type QuickSort struct{}

// Sort 用快速排序对切片排序（不修改原切片，返回新切片）
func (q QuickSort) Sort(data []int) []int {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

func (q QuickSort) Name() string { return "quick" }

// SortWith 使用指定策略排序
func SortWith(data []int, s Sorter) []int {
	return s.Sort(data)
}
