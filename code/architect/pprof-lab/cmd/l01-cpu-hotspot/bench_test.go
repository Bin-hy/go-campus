package main

import (
	"testing"
)

// benchN 与接口默认值保持一致（/api/render?n=2000），这样基准测试的口径和压测一致。
const benchN = 2000

// sinkScore / sinkOut 用来吃掉返回值，防止编译器把整段计算优化掉。
var (
	sinkScore float64
	sinkOut   string
)

// BenchmarkRenderBug 测量问题实现的耗时与分配。
// 运行：go test ./cmd/l01-cpu-hotspot -bench=Render -benchmem -benchtime=10x
func BenchmarkRenderBug(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(benchN))
	for i := 0; i < b.N; i++ {
		sinkScore, sinkOut = render(benchN, false)
	}
}

// BenchmarkRenderFix 测量修复实现的耗时与分配，与 Bug 版本使用同一入口函数。
func BenchmarkRenderFix(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(benchN))
	for i := 0; i < b.N; i++ {
		sinkScore, sinkOut = render(benchN, true)
	}
}

// TestRenderModesProduceSameScore 保证两种实现的业务语义一致（分数必须相同），
// 避免「为了快把功能删了」式的假优化。
func TestRenderModesProduceSameScore(t *testing.T) {
	const n = 200
	bugScore, bugOut := render(n, false)
	fixScore, fixOut := render(n, true)

	if bugScore != fixScore {
		t.Fatalf("两种实现的 score 不一致: bug=%v fix=%v", bugScore, fixScore)
	}
	if bugOut == "" || fixOut == "" {
		t.Fatal("输出为空，实现有问题")
	}
	if len(bugOut) == 0 || len(fixOut) == 0 {
		t.Fatal("输出长度为 0")
	}
	t.Logf("n=%d score=%v bug_bytes=%d fix_bytes=%d", n, bugScore, len(bugOut), len(fixOut))
}
