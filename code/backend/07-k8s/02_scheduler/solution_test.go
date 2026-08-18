package scheduler

import (
	"reflect"
	"testing"
)

func TestFilter(t *testing.T) {
	pod := Pod{CPUReq: 500, MemReq: 1024}
	nodes := []Node{
		{Name: "node-a", CPUCap: 2000, MemCap: 4096, CPUAlloc: 1600, MemAlloc: 2048}, // 剩余 CPU 400 < 500，剔除
		{Name: "node-b", CPUCap: 2000, MemCap: 4096, CPUAlloc: 1000, MemAlloc: 3100}, // 剩余内存 996 < 1024，剔除
		{Name: "node-c", CPUCap: 2000, MemCap: 4096, CPUAlloc: 1000, MemAlloc: 1024}, // 都够
	}
	got := Filter(pod, nodes)
	want := []Node{nodes[2]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Filter 应只剩 node-c，实际 %v", got)
	}
	t.Log("过滤：requests 不满足的节点被剔除验证通过")
}

func TestFilterEmpty(t *testing.T) {
	pod := Pod{CPUReq: 8000, MemReq: 1024}
	nodes := []Node{{Name: "node-a", CPUCap: 2000, MemCap: 4096}}
	if got := Filter(pod, nodes); len(got) != 0 {
		t.Fatalf("无节点满足时结果应为空（Pod 进入 Pending），实际 %v", got)
	}
	t.Log("无节点满足 requests → Pending 验证通过")
}

func TestScore(t *testing.T) {
	nodes := []Node{
		{Name: "node-a", CPUCap: 1000, MemCap: 1024, CPUAlloc: 500, MemAlloc: 512}, // 剩 50% + 50% = 1.0
		{Name: "node-b", CPUCap: 1000, MemCap: 1024, CPUAlloc: 200, MemAlloc: 200}, // 剩 80% + 80% ≈ 1.6
		{Name: "node-c", CPUCap: 1000, MemCap: 1024, CPUAlloc: 900, MemAlloc: 900}, // 剩 10% + 12% ≈ 0.22
	}
	if got := Score(nodes); got != "node-b" {
		t.Fatalf("应选剩余最多的 node-b，实际 %s", got)
	}
	t.Log("打分：剩余资源最多的节点胜出验证通过")
}

func TestScoreTieBreak(t *testing.T) {
	nodes := []Node{
		{Name: "node-b", CPUCap: 1000, MemCap: 1024, CPUAlloc: 500, MemAlloc: 512},
		{Name: "node-a", CPUCap: 1000, MemCap: 1024, CPUAlloc: 500, MemAlloc: 512},
	}
	if got := Score(nodes); got != "node-a" {
		t.Fatalf("分数相同应取字典序小的 node-a，实际 %s", got)
	}
	t.Log("平局按名字字典序验证通过")
}

func TestScoreEmpty(t *testing.T) {
	if got := Score(nil); got != "" {
		t.Fatalf("空节点列表应返回空串，实际 %q", got)
	}
	t.Log("空列表返回空串验证通过")
}
