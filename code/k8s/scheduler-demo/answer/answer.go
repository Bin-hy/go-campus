// Package answer 参考答案（自包含，可独立编译对照阅读）。
package answer

// Node 一台可调度节点。
type Node struct {
	Name           string
	CPU            int64
	Memory         int64
	GPUCapable     bool
	HasTaint       bool
	PreferredScore int64
}

// Pod 待调度的 Pod。
type Pod struct {
	Name      string
	NeedCPU   int64
	NeedMem   int64
	NeedGPU   bool
	Tolerates bool
}

// Filter 过滤：资源够 + GPU 要求 + 污点容忍。
func Filter(pod Pod, nodes []Node) []Node {
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if n.CPU < pod.NeedCPU || n.Memory < pod.NeedMem {
			continue
		}
		if pod.NeedGPU && !n.GPUCapable {
			continue
		}
		if n.HasTaint && !pod.Tolerates {
			continue
		}
		out = append(out, n)
	}
	return out
}

// Score 打分：剩余资源越多分越高（CPU 权重 1，内存权重按 1Mi=1 分），加偏好分。
func Score(candidates []Node) *Node {
	if len(candidates) == 0 {
		return nil
	}
	best := &candidates[0]
	bestScore := int64(-1)
	for i := range candidates {
		n := &candidates[i]
		score := n.CPU + n.Memory + n.PreferredScore
		if score > bestScore {
			best = n
			bestScore = score
		}
	}
	return best
}

// Schedule 完整调度。
func Schedule(pod Pod, nodes []Node) *Node {
	return Score(Filter(pod, nodes))
}
