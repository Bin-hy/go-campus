package circuit_breaker

import (
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// Closed：连续失败 3 次触发熔断
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("Closed 阶段第 %d 次应放行", i)
		}
		cb.RecordResult(false)
	}
	if cb.State() != StateOpen {
		t.Fatalf("连续 3 次失败后应为 StateOpen，实际 %v", cb.State())
	}

	// Open：冷却期内拒绝
	if cb.Allow() {
		t.Fatal("Open 冷却期内应拒绝请求")
	}

	// 等冷却结束 → Half-Open 放行探测，成功恢复 Closed
	time.Sleep(120 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("冷却结束后应放行探测请求")
	}
	cb.RecordResult(true)
	if cb.State() != StateClosed {
		t.Fatalf("探测成功后应恢复 StateClosed，实际 %v", cb.State())
	}
	t.Log("熔断状态机验证通过：Closed → Open → Half-Open → Closed")
}

func TestCircuitBreakerHalfOpenFail(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)
	for i := 0; i < 2; i++ {
		cb.Allow()
		cb.RecordResult(false)
	}
	if cb.State() != StateOpen {
		t.Fatalf("应进入 StateOpen，实际 %v", cb.State())
	}

	time.Sleep(120 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("冷却后应放行探测")
	}
	cb.RecordResult(false) // 探测失败 → 重新 Open
	if cb.State() != StateOpen {
		t.Fatalf("探测失败后应重新 StateOpen，实际 %v", cb.State())
	}
	t.Log("Half-Open 探测失败重新熔断验证通过")
}
