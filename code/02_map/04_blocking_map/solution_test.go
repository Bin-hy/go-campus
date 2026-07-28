package blocking_map

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestBlockingMap_GetExistingKey(t *testing.T) {
	m := NewBlockingMap()
	m.Put("hello", "world")

	v, err := m.Get("hello", time.Second)
	if err != nil {
		t.Fatalf("不应返回错误: %v", err)
	}
	if v != "world" {
		t.Errorf("期望 world，得到 %v", v)
	}
}

func TestBlockingMap_BlockUntilPut(t *testing.T) {
	m := NewBlockingMap()

	var result interface{}
	var getErr error
	done := make(chan struct{})

	go func() {
		result, getErr = m.Get("key1", 5*time.Second)
		close(done)
	}()

	// 等一下确保 Get 已经在阻塞
	time.Sleep(100 * time.Millisecond)

	m.Put("key1", 42)

	select {
	case <-done:
		if getErr != nil {
			t.Fatalf("不应返回错误: %v", getErr)
		}
		if result != 42 {
			t.Errorf("期望 42，得到 %v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Get 没有被 Put 唤醒")
	}
}

func TestBlockingMap_Timeout(t *testing.T) {
	m := NewBlockingMap()

	start := time.Now()
	_, err := m.Get("nonexist", 200*time.Millisecond)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrTimeout) {
		t.Errorf("期望 ErrTimeout，得到 %v", err)
	}

	if elapsed < 150*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Errorf("超时时间不准确：%v", elapsed)
	}
}

func TestBlockingMap_MultipleWaiters(t *testing.T) {
	m := NewBlockingMap()
	var wg sync.WaitGroup

	results := make([]interface{}, 5)
	errs := make([]error, 5)

	// 5 个 goroutine 同时等待同一个 key
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = m.Get("shared", 5*time.Second)
		}(i)
	}

	time.Sleep(100 * time.Millisecond)
	m.Put("shared", "value")

	wg.Wait()

	for i := 0; i < 5; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d 得到错误: %v", i, errs[i])
		}
		if results[i] != "value" {
			t.Errorf("goroutine %d 得到 %v，期望 value", i, results[i])
		}
	}
}

func TestBlockingMap_PutBeforeGet(t *testing.T) {
	m := NewBlockingMap()

	// 先 Put 后 Get
	m.Put("pre", "existing")

	v, err := m.Get("pre", time.Millisecond)
	if err != nil {
		t.Fatalf("已存在的 key 不应超时: %v", err)
	}
	if v != "existing" {
		t.Errorf("期望 existing，得到 %v", v)
	}
}

func TestBlockingMap_PutOverwrite(t *testing.T) {
	m := NewBlockingMap()

	m.Put("key", "v1")
	m.Put("key", "v2") // 覆盖

	v, err := m.Get("key", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if v != "v2" {
		t.Errorf("期望 v2，得到 %v", v)
	}
}

func TestBlockingMap_ConcurrentPutGet(t *testing.T) {
	m := NewBlockingMap()
	var wg sync.WaitGroup

	// 并发写入 100 个不同的 key
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			m.Put(key, i)
		}(i)
	}

	// 并发读取
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			m.Get(key, 2*time.Second)
		}(i)
	}

	wg.Wait()
}
