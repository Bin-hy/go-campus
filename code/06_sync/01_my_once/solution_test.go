package my_once

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestMyOnce_ExecutesOnce(t *testing.T) {
	var once MyOnce
	var count int64

	for i := 0; i < 100; i++ {
		once.Do(func() {
			atomic.AddInt64(&count, 1)
		})
	}

	if count != 1 {
		t.Errorf("f 应只执行1次，实际执行了 %d 次", count)
	}
}

func TestMyOnce_ConcurrentDo(t *testing.T) {
	var once MyOnce
	var count int64
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			once.Do(func() {
				atomic.AddInt64(&count, 1)
			})
		}()
	}

	wg.Wait()

	if count != 1 {
		t.Errorf("并发下 f 应只执行1次，实际执行了 %d 次", count)
	}
}

func TestMyOnce_Done(t *testing.T) {
	var once MyOnce

	if once.Done() {
		t.Error("未调用 Do 前 Done 应返回 false")
	}

	once.Do(func() {})

	if !once.Done() {
		t.Error("调用 Do 后 Done 应返回 true")
	}
}

func TestMyOnce_Reset(t *testing.T) {
	var once MyOnce
	var count int64

	once.Do(func() {
		atomic.AddInt64(&count, 1)
	})

	if count != 1 {
		t.Fatalf("第一次 Do 后 count 应为1，得到 %d", count)
	}

	once.Reset()

	if once.Done() {
		t.Error("Reset 后 Done 应返回 false")
	}

	once.Do(func() {
		atomic.AddInt64(&count, 1)
	})

	if count != 2 {
		t.Errorf("Reset 后第二次 Do 应执行，count 应为2，得到 %d", count)
	}
}

func TestMyOnce_DifferentFunctions(t *testing.T) {
	var once MyOnce
	var first, second bool

	once.Do(func() { first = true })
	once.Do(func() { second = true })

	if !first {
		t.Error("第一个函数应被执行")
	}
	if second {
		t.Error("第二个函数不应被执行（Once 已经 done）")
	}
}

func TestMyOnce_PanicStillCounts(t *testing.T) {
	var once MyOnce
	var count int64

	// 标准 sync.Once: panic 了也算执行过
	func() {
		defer func() { recover() }()
		once.Do(func() {
			atomic.AddInt64(&count, 1)
			panic("intentional")
		})
	}()

	// 再次调用不应执行
	once.Do(func() {
		atomic.AddInt64(&count, 1)
	})

	if count != 1 {
		t.Errorf("panic 后不应再次执行，count 应为1，得到 %d", count)
	}

	if !once.Done() {
		t.Error("panic 后 Done 应返回 true")
	}
}
