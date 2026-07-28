package pool_optimize

import (
	"strings"
	"testing"
)

func TestBufferPool_GetPut(t *testing.T) {
	pool := NewBufferPool()

	buf := pool.GetBuffer()
	if buf == nil {
		t.Fatal("GetBuffer 不应返回 nil")
	}

	buf.WriteString("hello")
	if buf.String() != "hello" {
		t.Errorf("Buffer 写入异常")
	}

	pool.PutBuffer(buf)

	// 再次获取，Buffer 应已被 Reset
	buf2 := pool.GetBuffer()
	if buf2.Len() != 0 {
		t.Errorf("归还后的 Buffer 应已 Reset，但 Len=%d", buf2.Len())
	}
}

func TestBufferPool_Reuse(t *testing.T) {
	pool := NewBufferPool()

	// 获取并归还多次
	for i := 0; i < 100; i++ {
		buf := pool.GetBuffer()
		buf.WriteString("test data")
		pool.PutBuffer(buf)
	}

	// 不应 panic
}

func TestProcessRequests(t *testing.T) {
	requests := [][]byte{
		[]byte("hello"),
		[]byte("world"),
		[]byte(""),
		[]byte("go"),
	}

	results := ProcessRequests(requests)

	if len(results) != 4 {
		t.Fatalf("期望4个结果，得到 %d", len(results))
	}

	for i, r := range results {
		if !strings.HasPrefix(r, "processed: ") {
			t.Errorf("results[%d] 格式错误：%s", i, r)
		}
	}

	// 验证内容
	if !strings.Contains(results[0], "68656c6c6f") { // "hello" in hex
		t.Errorf("results[0] 应包含 hello 的 hex，得到 %s", results[0])
	}
}

func TestProcessRequests_Empty(t *testing.T) {
	results := ProcessRequests(nil)
	if results == nil {
		results = []string{}
	}
	if len(results) != 0 {
		t.Errorf("空输入应返回空结果，得到 %v", results)
	}
}

func BenchmarkProcessRequests_WithPool(b *testing.B) {
	requests := make([][]byte, 1000)
	for i := range requests {
		requests[i] = []byte("benchmark test data that is reasonably long to simulate real workload")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ProcessRequests(requests)
	}
}
