package pipeline

import (
	"sort"
	"testing"
	"time"
)

func TestGenerator(t *testing.T) {
	done := make(chan struct{})
	defer close(done)

	ch := Generator(done, 1, 5)
	var result []int
	for v := range ch {
		result = append(result, v)
	}

	expected := []int{1, 2, 3, 4, 5}
	if len(result) != len(expected) {
		t.Fatalf("期望 %d 个元素，得到 %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d，期望 %d", i, v, expected[i])
		}
	}
}

func TestGenerator_Done(t *testing.T) {
	done := make(chan struct{})
	ch := Generator(done, 1, 1000000)

	// 读几个后取消
	<-ch
	<-ch
	close(done)

	// channel 应该很快关闭
	time.Sleep(100 * time.Millisecond)
	count := 0
	for range ch {
		count++
	}
	if count > 100 {
		t.Errorf("done 后不应继续产生大量数据，多读了 %d 个", count)
	}
}

func TestSquare(t *testing.T) {
	done := make(chan struct{})
	defer close(done)

	in := Generator(done, 1, 4)
	out := Square(done, in)

	var result []int
	for v := range out {
		result = append(result, v)
	}

	expected := []int{1, 4, 9, 16}
	if len(result) != len(expected) {
		t.Fatalf("期望 %d 个元素，得到 %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d，期望 %d", i, v, expected[i])
		}
	}
}

func TestFilter(t *testing.T) {
	done := make(chan struct{})
	defer close(done)

	in := Generator(done, 1, 10)
	isEven := func(n int) bool { return n%2 == 0 }
	out := Filter(done, in, isEven)

	var result []int
	for v := range out {
		result = append(result, v)
	}

	expected := []int{2, 4, 6, 8, 10}
	if len(result) != len(expected) {
		t.Fatalf("期望 %d 个元素，得到 %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d，期望 %d", i, v, expected[i])
		}
	}
}

func TestMerge(t *testing.T) {
	done := make(chan struct{})
	defer close(done)

	ch1 := Generator(done, 1, 3)
	ch2 := Generator(done, 4, 6)
	ch3 := Generator(done, 7, 9)

	merged := Merge(done, ch1, ch2, ch3)

	var result []int
	for v := range merged {
		result = append(result, v)
	}

	sort.Ints(result)
	expected := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	if len(result) != len(expected) {
		t.Fatalf("期望 %d 个元素，得到 %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d，期望 %d", i, v, expected[i])
		}
	}
}

func TestPipeline(t *testing.T) {
	// 1~5 平方后 = 1, 4, 9, 16, 25
	// 过滤 > 5 的 = 9, 16, 25
	greaterThan5 := func(n int) bool { return n > 5 }
	result := Pipeline(1, 5, greaterThan5)

	sort.Ints(result)
	expected := []int{9, 16, 25}

	if len(result) != len(expected) {
		t.Fatalf("期望 %d 个元素，得到 %d: %v", len(expected), len(result), result)
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d，期望 %d", i, v, expected[i])
		}
	}
}

func TestPipeline_AllFiltered(t *testing.T) {
	// 1~3 平方后 = 1, 4, 9，过滤 > 100 的 = 无
	result := Pipeline(1, 3, func(n int) bool { return n > 100 })
	if len(result) != 0 {
		t.Errorf("全部被过滤后应为空，得到 %v", result)
	}
}

func TestPipeline_NoDeadlock(t *testing.T) {
	done := make(chan struct{})
	go func() {
		Pipeline(1, 100, func(n int) bool { return n%2 == 0 })
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Pipeline 死锁或 goroutine 泄漏")
	}
}
