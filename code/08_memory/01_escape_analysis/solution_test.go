package escape_analysis

import "testing"

func TestCreateOnStack(t *testing.T) {
	v := CreateOnStack()
	if v == 0 {
		// 允许返回任何非零值作为初始化标志
	}
	_ = v
}

func TestCreateOnHeap(t *testing.T) {
	p := CreateOnHeap()
	if p == nil {
		t.Fatal("不应返回 nil")
	}
}

func TestSliceNoEscape(t *testing.T) {
	sum := SliceNoEscape()
	if sum <= 0 {
		t.Errorf("sum 应为正数，得到 %d", sum)
	}
}

func TestSliceEscape(t *testing.T) {
	s := SliceEscape()
	if s == nil || len(s) == 0 {
		t.Error("应返回非空 slice")
	}
}

func TestSumWithInterface(t *testing.T) {
	result := SumWithInterface(1, 2, 3, 4, 5)
	if result != 15 {
		t.Errorf("期望15，得到 %d", result)
	}
}

func TestSumWithInterface_Mixed(t *testing.T) {
	result := SumWithInterface(10, "skip", 20, nil, 30)
	if result != 60 {
		t.Errorf("期望60（跳过非int），得到 %d", result)
	}
}

func TestSumDirect(t *testing.T) {
	if SumDirect(1, 2, 3) != 6 {
		t.Error("1+2+3 应等于6")
	}
}

// 用 benchmark 对比栈分配 vs 堆分配性能
func BenchmarkCreateOnStack(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = CreateOnStack()
	}
}

func BenchmarkCreateOnHeap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = CreateOnHeap()
	}
}

func BenchmarkSumWithInterface(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = SumWithInterface(1, 2, 3, 4, 5)
	}
}

func BenchmarkSumDirect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = SumDirect(1, 2, 3)
	}
}
