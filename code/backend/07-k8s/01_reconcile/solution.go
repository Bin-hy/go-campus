package reconcile

// Pod 集群里的一个 Pod。
type Pod struct {
	Name string
	Age  int // 存活秒数
}

// Action 控制器执行的一个动作。
type Action struct {
	Op   string // "create" | "delete"
	Name string // create 时为新 Pod 名，delete 时为目标 Pod 名
}

// DesiredReplicas 模拟 HPA：按 CPU 利用率计算期望副本数。
func DesiredReplicas(currentReplicas, avgCPU, targetUtil, minReplicas, maxReplicas int) int {
	// TODO: 实现你的代码
	panic("not implemented")
}

// Reconcile 返回让实际状态收敛到期望状态的动作列表。
func Reconcile(desired int, pods []Pod) []Action {
	// TODO: 实现你的代码
	panic("not implemented")
}
