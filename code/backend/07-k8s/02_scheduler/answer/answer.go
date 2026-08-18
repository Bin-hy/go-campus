package scheduler

type Pod struct {
	CPUReq int
	MemReq int
}

type Node struct {
	Name               string
	CPUCap, MemCap     int
	CPUAlloc, MemAlloc int
}

// Filter 阶段：硬约束，剩余容量必须同时满足 CPU 和内存 requests。
func Filter(pod Pod, nodes []Node) []Node {
	var out []Node
	for _, n := range nodes {
		if n.CPUCap-n.CPUAlloc >= pod.CPUReq && n.MemCap-n.MemAlloc >= pod.MemReq {
			out = append(out, n)
		}
	}
	return out
}

// Score 阶段：软偏好，剩余资源占比之和最大者胜出，平局取名字字典序小者。
func Score(nodes []Node) string {
	bestName := ""
	bestScore := -1.0
	for _, n := range nodes {
		cpuRemain := float64(n.CPUCap-n.CPUAlloc) / float64(n.CPUCap)
		memRemain := float64(n.MemCap-n.MemAlloc) / float64(n.MemCap)
		score := cpuRemain + memRemain
		if score > bestScore || (score == bestScore && n.Name < bestName) {
			bestScore = score
			bestName = n.Name
		}
	}
	return bestName
}
