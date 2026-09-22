package main

import (
	"bytes"
	"testing"
)

// TestExportModesProduceSameCSV 验证 fix 模式写出的 CSV 与 bug 模式逐字节一致，
// 避免"为了快把格式改坏了"。
func TestExportModesProduceSameCSV(t *testing.T) {
	const rows = 200

	var bugBuf, fixBuf bytes.Buffer
	bugN, err := exportCSV(&bugBuf, rows, false)
	if err != nil {
		t.Fatalf("bug 导出失败: %v", err)
	}
	fixN, err := exportCSV(&fixBuf, rows, true)
	if err != nil {
		t.Fatalf("fix 导出失败: %v", err)
	}

	if bugN != fixN {
		t.Fatalf("返回字节数不一致: bug=%d fix=%d", bugN, fixN)
	}
	if !bytes.Equal(bugBuf.Bytes(), fixBuf.Bytes()) {
		t.Fatalf("CSV 内容不一致: bug=%d 字节 fix=%d 字节", bugBuf.Len(), fixBuf.Len())
	}
	if got := bytes.Count(bugBuf.Bytes(), []byte("\n")); got != rows {
		t.Fatalf("行数应为 %d，实际 %d", rows, got)
	}
	t.Logf("rows=%d bytes=%d 两种实现输出一致", rows, bugBuf.Len())
}

// BenchmarkExportBug 测量问题实现的耗时与分配。
// 运行：go test ./cmd/l07-io-serialization -bench=Export -benchmem -benchtime=20x
func BenchmarkExportBug(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(defaultRows))
	for i := 0; i < b.N; i++ {
		if _, err := exportCSV(discard{}, defaultRows, false); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExportFix 测量修复实现的耗时与分配。
func BenchmarkExportFix(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(defaultRows))
	for i := 0; i < b.N; i++ {
		if _, err := exportCSV(discard{}, defaultRows, true); err != nil {
			b.Fatal(err)
		}
	}
}

// discard 是一个零分配的 io.Writer，用来模拟"下游很快"的理想写路径，
// 这样基准测试的差异主要来自格式化与缓冲，而不是下游。
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
