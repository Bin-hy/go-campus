//go:build ignore

package answer

type Sorter interface {
	Sort([]int) []int
	Name() string
}

type BubbleSort struct{}

func (b BubbleSort) Sort(data []int) []int {
	result := make([]int, len(data))
	copy(result, data)
	n := len(result)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-1-i; j++ {
			if result[j] > result[j+1] {
				result[j], result[j+1] = result[j+1], result[j]
			}
		}
	}
	return result
}
func (b BubbleSort) Name() string { return "bubble" }

type QuickSort struct{}

func (q QuickSort) Sort(data []int) []int {
	result := make([]int, len(data))
	copy(result, data)
	quickSort(result, 0, len(result)-1)
	return result
}
func (q QuickSort) Name() string { return "quick" }

func quickSort(arr []int, low, high int) {
	if low < high {
		p := partition(arr, low, high)
		quickSort(arr, low, p-1)
		quickSort(arr, p+1, high)
	}
}

func partition(arr []int, low, high int) int {
	pivot := arr[high]
	i := low - 1
	for j := low; j < high; j++ {
		if arr[j] <= pivot {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}
	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}

func SortWith(data []int, s Sorter) []int { return s.Sort(data) }
