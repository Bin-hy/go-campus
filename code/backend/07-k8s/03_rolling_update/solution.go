package rolling_update

// Action 滚动更新的一步动作。
type Action struct {
	Op      string // "create" | "delete"
	Version string // "v2" | "v1"
}

// RollingUpdatePlan 生成从 v1Count 个 v1 Pod 全部滚动到 v2 的分轮动作计划。
func RollingUpdatePlan(desired, maxUnavailable, maxSurge, v1Count int) [][]Action {
	// TODO: 实现你的代码
	panic("not implemented")
}
