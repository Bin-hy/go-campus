package sequential_print

import (
	"testing"
	"time"
)

func TestSequentialPrint_Basic(t *testing.T) {
	result := SequentialPrint(3, 10)
	if len(result) != 10 {
		t.Fatalf("期望长度10，得到 %d", len(result))
	}
	for i, v := range result {
		if v != i+1 {
			t.Errorf("result[%d] = %d，期望 %d", i, v, i+1)
			break
		}
	}
}

func TestSequentialPrint_SingleGoroutine(t *testing.T) {
	result := SequentialPrint(1, 5)
	expected := []int{1, 2, 3, 4, 5}
	if len(result) != 5 {
		t.Fatalf("期望长度5，得到 %d", len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d，期望 %d", i, v, expected[i])
		}
	}
}

func TestSequentialPrint_ManyGoroutines(t *testing.T) {
	result := SequentialPrint(10, 30)
	if len(result) != 30 {
		t.Fatalf("期望长度30，得到 %d", len(result))
	}
	for i, v := range result {
		if v != i+1 {
			t.Errorf("result[%d] = %d，期望 %d", i, v, i+1)
			break
		}
	}
}

func TestSequentialPrint_MaxLessThanN(t *testing.T) {
	result := SequentialPrint(5, 3)
	if len(result) != 3 {
		t.Fatalf("期望长度3，得到 %d", len(result))
	}
	for i, v := range result {
		if v != i+1 {
			t.Errorf("result[%d] = %d，期望 %d", i, v, i+1)
		}
	}
}

func TestSequentialPrint_Zero(t *testing.T) {
	result := SequentialPrint(3, 0)
	if len(result) != 0 {
		t.Errorf("max=0 应返回空切片，得到 %v", result)
	}
}

func TestSequentialPrint_NoDeadlock(t *testing.T) {
	done := make(chan struct{})
	go func() {
		SequentialPrint(5, 100)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("死锁：3秒未完成")
	}
}
