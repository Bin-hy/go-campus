package scheduler

// Pod 待调度的 Pod（只关心 requests，limits 不参与调度）。
type Pod struct {
	CPUReq int // millicpu
	MemReq int // MiB
}

// Node 集群里的一个节点。
type Node struct {
	Name                string
	CPUCap, MemCap      int // 总容量
	CPUAlloc, MemAlloc  int // 已分配
}

// Filter 返回能容纳 pod 的节点列表。
func Filter(pod Pod, nodes []Node) []Node {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Score 在过滤结果中打分，返回最优节点名。
func Score(nodes []Node) string {
	// TODO: 实现你的代码
	panic("not implemented")
}
