package timeout_read

import (
	"errors"
	"testing"
	"time"
)

func TestReadWithTimeout_Success(t *testing.T) {
	ch := make(chan int, 1)
	ch <- 42

	v, err := ReadWithTimeout(ch, time.Second)
	if err != nil {
		t.Fatalf("不应超时: %v", err)
	}
	if v != 42 {
		t.Errorf("期望 42，得到 %d", v)
	}
}

func TestReadWithTimeout_Delayed(t *testing.T) {
	ch := make(chan int)

	go func() {
		time.Sleep(50 * time.Millisecond)
		ch <- 99
	}()

	v, err := ReadWithTimeout(ch, time.Second)
	if err != nil {
		t.Fatalf("不应超时: %v", err)
	}
	if v != 99 {
		t.Errorf("期望 99，得到 %d", v)
	}
}

func TestReadWithTimeout_Timeout(t *testing.T) {
	ch := make(chan int) // 永远不会有数据

	start := time.Now()
	_, err := ReadWithTimeout(ch, 100*time.Millisecond)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrTimeout) {
		t.Errorf("期望 ErrTimeout，得到 %v", err)
	}

	if elapsed < 80*time.Millisecond || elapsed > 300*time.Millisecond {
		t.Errorf("超时时间不准确：%v", elapsed)
	}
}

func TestReadMultipleWithTimeout_AllAvailable(t *testing.T) {
	ch := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		ch <- i
	}

	result := ReadMultipleWithTimeout(ch, 3, time.Second)
	if len(result) != 3 {
		t.Fatalf("期望读取3个，得到 %d 个: %v", len(result), result)
	}
	// 应该是 1, 2, 3
	for i, v := range result {
		if v != i+1 {
			t.Errorf("result[%d] = %d，期望 %d", i, v, i+1)
		}
	}
}

func TestReadMultipleWithTimeout_PartialTimeout(t *testing.T) {
	ch := make(chan int, 2)
	ch <- 10
	ch <- 20

	// 只有2个值，但要读5个，会超时
	result := ReadMultipleWithTimeout(ch, 5, 200*time.Millisecond)
	if len(result) != 2 {
		t.Errorf("期望读取2个（超时前），得到 %d 个: %v", len(result), result)
	}
}

func TestReadMultipleWithTimeout_SlowProducer(t *testing.T) {
	ch := make(chan int)

	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(30 * time.Millisecond)
			ch <- i
		}
	}()

	// 200ms 内大约能读 6-7 个
	result := ReadMultipleWithTimeout(ch, 100, 200*time.Millisecond)
	if len(result) < 4 || len(result) > 8 {
		t.Errorf("慢生产者场景：期望读取 4~8 个，得到 %d 个", len(result))
	}
}

func TestReadMultipleWithTimeout_Zero(t *testing.T) {
	ch := make(chan int, 5)
	ch <- 1

	result := ReadMultipleWithTimeout(ch, 0, time.Second)
	if len(result) != 0 {
		t.Errorf("n=0 应返回空，得到 %v", result)
	}
}

func TestFirstResult_Fast(t *testing.T) {
	fast := func() int {
		time.Sleep(10 * time.Millisecond)
		return 1
	}
	slow := func() int {
		time.Sleep(500 * time.Millisecond)
		return 2
	}

	start := time.Now()
	result := FirstResult(fast, slow)
	elapsed := time.Since(start)

	if result != 1 {
		t.Errorf("应返回最快的结果 1，得到 %d", result)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("应在快函数完成后立即返回，耗时 %v", elapsed)
	}
}

func TestFirstResult_Single(t *testing.T) {
	fn := func() int { return 42 }
	result := FirstResult(fn)
	if result != 42 {
		t.Errorf("单函数应返回 42，得到 %d", result)
	}
}

func TestFirstResult_AllSameSpeed(t *testing.T) {
	fn := func() int {
		time.Sleep(50 * time.Millisecond)
		return 99
	}

	result := FirstResult(fn, fn, fn)
	if result != 99 {
		t.Errorf("期望 99，得到 %d", result)
	}
}
