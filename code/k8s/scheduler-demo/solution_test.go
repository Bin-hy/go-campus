package scheduler

import (
	"testing"
)

func TestFilterResource(t *testing.T) {
	nodes := []Node{
		{Name: "small", CPU: 100, Memory: 128},
		{Name: "big", CPU: 1000, Memory: 1024},
	}
	pod := Pod{NeedCPU: 500, NeedMem: 256}
	got := Filter(pod, nodes)
	if len(got) != 1 || got[0].Name != "big" {
		t.Fatalf("Filter 应只剩 big（small 资源不够），实际 %+v", got)
	}
	t.Log("资源过滤验证通过")
}

func TestFilterGPU(t *testing.T) {
	nodes := []Node{
		{Name: "cpu-only", CPU: 1000, Memory: 1024, GPUCapable: false},
		{Name: "gpu-node", CPU: 1000, Memory: 1024, GPUCapable: true},
	}
	pod := Pod{NeedCPU: 100, NeedMem: 128, NeedGPU: true}
	got := Filter(pod, nodes)
	if len(got) != 1 || got[0].Name != "gpu-node" {
		t.Fatalf("需要 GPU 的 Pod 应只剩 gpu-node，实际 %+v", got)
	}
	t.Log("GPU 过滤验证通过")
}

func TestFilterTaint(t *testing.T) {
	nodes := []Node{
		{Name: "tainted", CPU: 1000, Memory: 1024, HasTaint: true},
		{Name: "clean", CPU: 1000, Memory: 1024},
	}
	// 不容忍污点 → 排除 tainted
	if got := Filter(Pod{NeedCPU: 100, NeedMem: 128, Tolerates: false}, nodes); len(got) != 1 || got[0].Name != "clean" {
		t.Fatalf("不容忍污点的 Pod 应只剩 clean，实际 %+v", got)
	}
	// 容忍污点 → 两个都保留
	if got := Filter(Pod{NeedCPU: 100, NeedMem: 128, Tolerates: true}, nodes); len(got) != 2 {
		t.Fatalf("容忍污点的 Pod 应保留全部节点，实际 %+v", got)
	}
	t.Log("污点/容忍过滤验证通过")
}

func TestScorePrefersRichNode(t *testing.T) {
	candidates := []Node{
		{Name: "poor", CPU: 100, Memory: 100},
		{Name: "rich", CPU: 900, Memory: 900, PreferredScore: 50},
	}
	got := Score(candidates)
	if got == nil || got.Name != "rich" {
		t.Fatalf("打分应选 rich，实际 %+v", got)
	}
	t.Log("打分偏好验证通过")
}

func TestScoreEmpty(t *testing.T) {
	if got := Score(nil); got != nil {
		t.Fatalf("空候选应返回 nil，实际 %+v", got)
	}
	t.Log("空候选验证通过")
}

func TestScheduleEndToEnd(t *testing.T) {
	nodes := []Node{
		{Name: "small", CPU: 100, Memory: 128},
		{Name: "gpu-node", CPU: 2000, Memory: 4096, GPUCapable: true},
		{Name: "tainted-rich", CPU: 5000, Memory: 8192, HasTaint: true},
	}
	pod := Pod{NeedCPU: 1000, NeedMem: 2048, NeedGPU: true, Tolerates: false}
	got := Schedule(pod, nodes)
	if got == nil || got.Name != "gpu-node" {
		t.Fatalf("端到端应调度到 gpu-node，实际 %+v", got)
	}
	t.Log("端到端调度验证通过")
}
