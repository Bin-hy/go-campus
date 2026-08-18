package probe

type Probe struct {
	FailureThreshold int
}

type ProbeResult struct {
	OK bool
}

type ProbeAction int

const (
	ActionNone ProbeAction = iota
	ActionRestart
	ActionUnready
	ActionKill
)

// JudgeProbe 只看最近 failureThreshold 条：不足不触发，全部失败按类型触发。
func JudgeProbe(kind string, probe Probe, history []ProbeResult) ProbeAction {
	if probe.FailureThreshold <= 0 || len(history) < probe.FailureThreshold {
		return ActionNone
	}
	tail := history[len(history)-probe.FailureThreshold:]
	for _, r := range tail {
		if r.OK {
			return ActionNone
		}
	}
	switch kind {
	case "liveness":
		return ActionRestart
	case "readiness":
		return ActionUnready
	case "startup":
		return ActionKill
	}
	return ActionNone
}

// CheckOOM 内存使用达到 limits 的 90% 即预警。
func CheckOOM(memLimit, memUsageMiB int64) bool {
	return memUsageMiB*10 >= memLimit*9
}
