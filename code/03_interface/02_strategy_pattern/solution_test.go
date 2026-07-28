package strategy_pattern

import (
	"reflect"
	"testing"
)

func TestBubbleSort(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{"正常", []int{5, 3, 1, 4, 2}, []int{1, 2, 3, 4, 5}},
		{"已排序", []int{1, 2, 3}, []int{1, 2, 3}},
		{"逆序", []int{5, 4, 3, 2, 1}, []int{1, 2, 3, 4, 5}},
		{"单元素", []int{1}, []int{1}},
		{"空", []int{}, []int{}},
		{"重复", []int{3, 1, 3, 1, 2}, []int{1, 1, 2, 3, 3}},
		{"负数", []int{-3, 0, -1, 2, 1}, []int{-3, -1, 0, 1, 2}},
	}
	s := BubbleSort{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := make([]int, len(tt.input))
			copy(original, tt.input)
			result := s.Sort(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("期望 %v，得到 %v", tt.expected, result)
			}
			if !reflect.DeepEqual(tt.input, original) {
				t.Error("不应修改原切片")
			}
		})
	}
}

func TestQuickSort(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{"正常", []int{5, 3, 1, 4, 2}, []int{1, 2, 3, 4, 5}},
		{"已排序", []int{1, 2, 3}, []int{1, 2, 3}},
		{"逆序", []int{5, 4, 3, 2, 1}, []int{1, 2, 3, 4, 5}},
		{"单元素", []int{1}, []int{1}},
		{"空", []int{}, []int{}},
		{"大量元素", func() []int {
			s := make([]int, 1000)
			for i := range s { s[i] = 1000 - i }
			return s
		}(), func() []int {
			s := make([]int, 1000)
			for i := range s { s[i] = i + 1 }
			return s
		}()},
	}
	s := QuickSort{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := make([]int, len(tt.input))
			copy(original, tt.input)
			result := s.Sort(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("期望 %v，得到 %v", tt.expected, result)
			}
			if !reflect.DeepEqual(tt.input, original) {
				t.Error("不应修改原切片")
			}
		})
	}
}

func TestSortWith_Interface(t *testing.T) {
	data := []int{5, 2, 8, 1, 9}
	expected := []int{1, 2, 5, 8, 9}

	sorters := []Sorter{BubbleSort{}, QuickSort{}}
	for _, s := range sorters {
		t.Run(s.Name(), func(t *testing.T) {
			result := SortWith(data, s)
			if !reflect.DeepEqual(result, expected) {
				t.Errorf("%s: 期望 %v，得到 %v", s.Name(), expected, result)
			}
		})
	}
}
