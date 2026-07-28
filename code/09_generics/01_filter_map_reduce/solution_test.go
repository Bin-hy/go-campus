package filter_map_reduce

import (
	"reflect"
	"strings"
	"testing"
)

func TestFilter_Int(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5, 6}
	evens := Filter(nums, func(n int) bool { return n%2 == 0 })
	expected := []int{2, 4, 6}
	if !reflect.DeepEqual(evens, expected) {
		t.Errorf("期望 %v，得到 %v", expected, evens)
	}
}

func TestFilter_String(t *testing.T) {
	words := []string{"hello", "hi", "world", "hey"}
	hWords := Filter(words, func(s string) bool { return strings.HasPrefix(s, "h") })
	expected := []string{"hello", "hi", "hey"}
	if !reflect.DeepEqual(hWords, expected) {
		t.Errorf("期望 %v，得到 %v", expected, hWords)
	}
}

func TestFilter_Empty(t *testing.T) {
	result := Filter([]int{}, func(n int) bool { return true })
	if result == nil {
		result = []int{}
	}
	if len(result) != 0 {
		t.Errorf("空输入应返回空，得到 %v", result)
	}
}

func TestMap_IntToString(t *testing.T) {
	nums := []int{1, 2, 3}
	strs := Map(nums, func(n int) string {
		return strings.Repeat("*", n)
	})
	expected := []string{"*", "**", "***"}
	if !reflect.DeepEqual(strs, expected) {
		t.Errorf("期望 %v，得到 %v", expected, strs)
	}
}

func TestMap_Square(t *testing.T) {
	nums := []int{1, 2, 3, 4}
	squares := Map(nums, func(n int) int { return n * n })
	expected := []int{1, 4, 9, 16}
	if !reflect.DeepEqual(squares, expected) {
		t.Errorf("期望 %v，得到 %v", expected, squares)
	}
}

func TestReduce_Sum(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5}
	sum := Reduce(nums, 0, func(acc, v int) int { return acc + v })
	if sum != 15 {
		t.Errorf("期望15，得到 %d", sum)
	}
}

func TestReduce_Concat(t *testing.T) {
	words := []string{"hello", " ", "world"}
	result := Reduce(words, "", func(acc string, v string) string { return acc + v })
	if result != "hello world" {
		t.Errorf("期望 'hello world'，得到 %q", result)
	}
}

func TestReduce_Empty(t *testing.T) {
	result := Reduce([]int{}, 42, func(acc, v int) int { return acc + v })
	if result != 42 {
		t.Errorf("空切片应返回初始值42，得到 %d", result)
	}
}

func TestContains(t *testing.T) {
	if !Contains([]int{1, 2, 3}, 2) {
		t.Error("应包含2")
	}
	if Contains([]int{1, 2, 3}, 5) {
		t.Error("不应包含5")
	}
	if !Contains([]string{"a", "b"}, "a") {
		t.Error("应包含 a")
	}
	if Contains([]string{}, "x") {
		t.Error("空切片不应包含任何元素")
	}
}

func TestUnique(t *testing.T) {
	result := Unique([]int{1, 2, 2, 3, 1, 4, 3})
	expected := []int{1, 2, 3, 4}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("期望 %v，得到 %v", expected, result)
	}

	strResult := Unique([]string{"a", "b", "a", "c", "b"})
	strExpected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(strResult, strExpected) {
		t.Errorf("期望 %v，得到 %v", strExpected, strResult)
	}
}

func TestGroupBy(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}
	people := []Person{
		{"Alice", 20}, {"Bob", 30}, {"Charlie", 20}, {"Dave", 30}, {"Eve", 25},
	}
	grouped := GroupBy(people, func(p Person) int { return p.Age })

	if len(grouped[20]) != 2 {
		t.Errorf("20岁应有2人，得到 %d", len(grouped[20]))
	}
	if len(grouped[30]) != 2 {
		t.Errorf("30岁应有2人，得到 %d", len(grouped[30]))
	}
	if len(grouped[25]) != 1 {
		t.Errorf("25岁应有1人，得到 %d", len(grouped[25]))
	}
}
