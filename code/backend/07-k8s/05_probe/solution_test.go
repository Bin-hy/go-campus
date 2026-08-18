package probe

import "testing"

func TestJudgeLivenessRestart(t *testing.T) {
	p := Probe{FailureThreshold: 3}
	// 连续 3 次失败 → 重启
	hist := []ProbeResult{{OK: false}, {OK: false}, {OK: false}}
	if got := JudgeProbe("liveness", p, hist); got != ActionRestart {
		t.Fatalf("liveness 连续 3 次失败应重启，实际 %v", got)
	}
	// 最近 3 次含成功 → 不触发
	hist = []ProbeResult{{OK: true}, {OK: false}, {OK: false}}
	if got := JudgeProbe("liveness", p, hist); got != ActionNone {
		t.Fatalf("最近 3 次含成功不应重启，实际 %v", got)
	}
	t.Log("liveness：连续失败触发重启验证通过")
}

func TestJudgeReadinessUnready(t *testing.T) {
	p := Probe{FailureThreshold: 2}
	hist := []ProbeResult{{OK: false}, {OK: false}}
	if got := JudgeProbe("readiness", p, hist); got != ActionUnready {
		t.Fatalf("readiness 连续 2 次失败应摘流量，实际 %v", got)
	}
	// 历史不足 threshold → 不触发
	if got := JudgeProbe("readiness", p, []ProbeResult{{OK: false}}); got != ActionNone {
		t.Fatalf("历史不足不应触发，实际 %v", got)
	}
	t.Log("readiness：摘流量不重启验证通过")
}

func TestJudgeStartupKill(t *testing.T) {
	p := Probe{FailureThreshold: 3}
	hist := []ProbeResult{{OK: false}, {OK: false}, {OK: false}}
	if got := JudgeProbe("startup", p, hist); got != ActionKill {
		t.Fatalf("startup 连续失败应杀掉重启，实际 %v", got)
	}
	t.Log("startup：启动失败杀掉重启验证通过")
}

func TestCheckOOM(t *testing.T) {
	cases := []struct {
		limit, usage int64
		want         bool
	}{
		{1000, 899, false},
		{1000, 900, true},
		{1000, 950, true},
		{2048, 1843, false}, // 1843*10=18430 < 2048*9=18432
		{2048, 1844, true},  // 1844*10=18440 >= 18432
	}
	for _, c := range cases {
		if got := CheckOOM(c.limit, c.usage); got != c.want {
			t.Errorf("CheckOOM(%d,%d)=%v, want %v", c.limit, c.usage, got, c.want)
		}
	}
	t.Log("OOM 预警边界验证通过")
}
