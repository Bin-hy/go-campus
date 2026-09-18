package main

import (
	"bytes"
	"testing"
)

// sinkLen / sinkSum 吃掉返回值，防止编译器把计算优化掉。
var (
	sinkLen int
	sinkSum uint32
)

// benchThumb 是基准测试的公共驱动：调用与 handler 完全相同的实现入口。
func benchThumb(b *testing.B, fix bool) {
	b.ReportAllocs()
	b.SetBytes(outBytes)
	for i := 0; i < b.N; i++ {
		out, sum, pooled := renderThumb(7, fix)
		sinkLen, sinkSum = len(out), sum
		if pooled != nil {
			scratchPool.Put(pooled)
		}
	}
}

// BenchmarkThumbBug 测量问题实现：每请求 7MB 分配。
// 运行：go test ./cmd/l02-alloc-gc -bench=Thumb -benchmem
func BenchmarkThumbBug(b *testing.B) { benchThumb(b, false) }

// BenchmarkThumbFix 测量修复实现：sync.Pool 复用后分配接近 0。
func BenchmarkThumbFix(b *testing.B) { benchThumb(b, true) }

// TestThumbModesProduceSameBytes 验证池化缓冲不会带来脏数据（这是 sync.Pool 最典型的坑），
// 同时保证 fix 模式没有偷偷改变业务语义。
func TestThumbModesProduceSameBytes(t *testing.T) {
	const id = 42

	var bugBuf, fixBuf bytes.Buffer
	bugOut, bugSum, bugPool := renderThumb(id, false)
	if bugPool != nil {
		t.Fatal("bug 模式不应该使用池")
	}
	_, _ = bugBuf.Write(bugOut)

	// 连续两次 fix 调用，第二次会复用池内缓冲：最容易暴露"忘了清空"的问题
	for i := 0; i < 2; i++ {
		fixOut, fixSum, fixPool := renderThumb(id, true)
		if fixPool == nil {
			t.Fatal("fix 模式必须使用池")
		}
		if fixSum != bugSum {
			t.Fatalf("第 %d 次 fix 校验和不一致: bug=%d fix=%d", i, bugSum, fixSum)
		}
		if !bytes.Equal(bugOut, fixOut) {
			t.Fatalf("第 %d 次 fix 输出与 bug 不一致（池化缓冲可能残留脏数据）", i)
		}
		fixBuf.Reset()
		_, _ = fixBuf.Write(fixOut)
		scratchPool.Put(fixPool)
	}

	if bugBuf.Len() != outBytes {
		t.Fatalf("输出长度应为 %d，实际 %d", outBytes, bugBuf.Len())
	}
	t.Logf("id=%d 输出一致: %d 字节, checksum=%d", id, bugBuf.Len(), bugSum)
}
