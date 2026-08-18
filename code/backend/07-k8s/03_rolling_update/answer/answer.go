package rolling_update

type Action struct {
	Op      string
	Version string
}

// RollingUpdatePlan 每轮先创建 v2（受 maxSurge 限制），再删除 v1（受 maxUnavailable 限制），
// 保证每一步都满足：总数 <= desired+maxSurge，可用数 >= desired-maxUnavailable。
func RollingUpdatePlan(desired, maxUnavailable, maxSurge, v1Count int) [][]Action {
	v1, v2 := v1Count, 0
	var plan [][]Action

	for v1 > 0 || v2 < desired {
		var round []Action

		// 1) 创建 v2：不超过目标副本数，且总数不超过 desired+maxSurge
		k := desired - v2
		if limit := desired + maxSurge - (v1 + v2); limit < k {
			k = limit
		}
		if k < 0 {
			k = 0
		}
		for i := 0; i < k; i++ {
			round = append(round, Action{Op: "create", Version: "v2"})
			v2++
		}

		// 2) 删除 v1：不超当前 v1 数，且总数不低于 desired-maxUnavailable
		m := v1
		if floor := (v1 + v2) - (desired - maxUnavailable); floor < m {
			m = floor
		}
		if m < 0 {
			m = 0
		}
		for i := 0; i < m; i++ {
			round = append(round, Action{Op: "delete", Version: "v1"})
			v1--
		}

		// 兜底：非法输入（如 maxUnavailable 与 maxSurge 同时为 0）避免死循环
		if len(round) == 0 {
			break
		}
		plan = append(plan, round)
	}
	return plan
}
