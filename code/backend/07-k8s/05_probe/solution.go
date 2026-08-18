package probe

// Probe 探针配置。
type Probe struct {
	FailureThreshold int // 连续失败多少次触发
}

// ProbeResult 一次探测结果。
type ProbeResult struct {
	OK bool
}

// ProbeAction 探针触发后的动作。
type ProbeAction int

const (
	ActionNone ProbeAction = iota
	ActionRestart          // liveness 失败 → 重启容器
	ActionUnready          // readiness 失败 → 摘出 Service
	ActionKill             // startup 失败 → 杀掉容器
)

// JudgeProbe 根据最近 failureThreshold 次结果判断是否触发动作。
func JudgeProbe(kind string, probe Probe, history []ProbeResult) ProbeAction {
	// TODO: 实现你的代码
	panic("not implemented")
}

// CheckOOM 内存使用达到 limits 的 90% 时预警。
func CheckOOM(memLimit, memUsageMiB int64) bool {
	// TODO: 实现你的代码
	panic("not implemented")
}
