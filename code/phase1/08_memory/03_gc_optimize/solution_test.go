package gc_optimize

import (
	"reflect"
	"testing"
)

func TestProcessData_Good_Correctness(t *testing.T) {
	inputs := []int{1, 2, 3, 4, 5, 10, 100}
	bad := ProcessData_Bad(inputs)
	good := ProcessData_Good(inputs)

	if !reflect.DeepEqual(bad, good) {
		t.Errorf("Good 版本结果与 Bad 不一致：\nBad:  %v\nGood: %v", bad, good)
	}
}

func TestProcessData_Good_Empty(t *testing.T) {
	result := ProcessData_Good([]int{})
	if len(result) != 0 {
		t.Errorf("空输入应返回空结果，得到 %v", result)
	}
}

func TestConcatStrings_Good_Correctness(t *testing.T) {
	strs := []string{"hello", " ", "world", "!"}
	bad := ConcatStrings_Bad(strs)
	good := ConcatStrings_Good(strs)

	if bad != good {
		t.Errorf("结果不一致：Bad=%q, Good=%q", bad, good)
	}
}

func TestConcatStrings_Good_Empty(t *testing.T) {
	if ConcatStrings_Good(nil) != "" {
		t.Error("nil 输入应返回空字符串")
	}
	if ConcatStrings_Good([]string{}) != "" {
		t.Error("空切片应返回空字符串")
	}
}

func TestObjectPool_Demo(t *testing.T) {
	n := ObjectPool_Demo(100)
	if n != 100 {
		t.Errorf("应处理100个请求，实际 %d", n)
	}
}

func BenchmarkProcessData_Bad(b *testing.B) {
	inputs := make([]int, 10000)
	for i := range inputs {
		inputs[i] = i
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ProcessData_Bad(inputs)
	}
}

func BenchmarkProcessData_Good(b *testing.B) {
	inputs := make([]int, 10000)
	for i := range inputs {
		inputs[i] = i
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ProcessData_Good(inputs)
	}
}

func BenchmarkConcatStrings_Bad(b *testing.B) {
	strs := make([]string, 1000)
	for i := range strs {
		strs[i] = "x"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConcatStrings_Bad(strs)
	}
}

func BenchmarkConcatStrings_Good(b *testing.B) {
	strs := make([]string, 1000)
	for i := range strs {
		strs[i] = "x"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConcatStrings_Good(strs)
	}
}
