package odd_even_print

import (
	"testing"
	"time"
)

func TestPrintOddEven_Basic(t *testing.T) {
	result := PrintOddEven(10)
	expected := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	if len(result) != len(expected) {
		t.Fatalf("长度错误：期望 %d，得到 %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d，期望 %d", i, v, expected[i])
		}
	}
}

func TestPrintOddEven_Odd(t *testing.T) {
	result := PrintOddEven(7)
	expected := []int{1, 2, 3, 4, 5, 6, 7}

	if len(result) != len(expected) {
		t.Fatalf("长度错误：期望 %d，得到 %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d，期望 %d", i, v, expected[i])
		}
	}
}

func TestPrintOddEven_One(t *testing.T) {
	result := PrintOddEven(1)
	if len(result) != 1 || result[0] != 1 {
		t.Errorf("期望 [1]，得到 %v", result)
	}
}

func TestPrintOddEven_Two(t *testing.T) {
	result := PrintOddEven(2)
	if len(result) != 2 || result[0] != 1 || result[1] != 2 {
		t.Errorf("期望 [1,2]，得到 %v", result)
	}
}

func TestPrintOddEven_Zero(t *testing.T) {
	result := PrintOddEven(0)
	if len(result) != 0 {
		t.Errorf("期望空切片，得到 %v", result)
	}
}

func TestPrintOddEven_Large(t *testing.T) {
	n := 1000
	result := PrintOddEven(n)

	if len(result) != n {
		t.Fatalf("长度错误：期望 %d，得到 %d", n, len(result))
	}
	for i := 0; i < n; i++ {
		if result[i] != i+1 {
			t.Errorf("result[%d] = %d，期望 %d", i, result[i], i+1)
			break
		}
	}
}

func TestPrintOddEven_NoDeadlock(t *testing.T) {
	done := make(chan struct{})
	go func() {
		PrintOddEven(100)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("死锁：3秒内未完成")
	}
}
