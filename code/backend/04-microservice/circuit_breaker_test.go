// T32 实验（上）：熔断器状态机 —— Closed → Open → Half-Open。
package microservice_lab

import (
	"sync"
	"testing"
	"time"
)

type circuitState int

const (
	stateClosed circuitState = iota
	stateOpen
	stateHalfOpen
)

// CircuitBreaker 简单熔断器：连续失败达阈值进入 Open，冷却后 Half-Open 探测。
type CircuitBreaker struct {
	mu        sync.Mutex
	state     circuitState
	failures  int
	threshold int           // 连续失败阈值
	cooldown  time.Duration // Open 冷却时间
	openedAt  time.Time
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{state: stateClosed, threshold: threshold, cooldown: cooldown}
}

// Allow 判断请求是否放行。Open 状态在冷却期内直接拒绝。
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case stateClosed:
		return true
	case stateOpen:
		if time.Since(cb.openedAt) >= cb.cooldown {
			cb.state = stateHalfOpen // 冷却结束，进入半开探测
			return true
		}
		return false
	case stateHalfOpen:
		return true // 半开只放行一个探测请求
	}
	return false
}

// RecordResult 记录一次调用的结果。
func (cb *CircuitBreaker) RecordResult(ok bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case stateClosed:
		if !ok {
			cb.failures++
			if cb.failures >= cb.threshold {
				cb.state = stateOpen
				cb.openedAt = time.Now()
			}
		} else {
			cb.failures = 0
		}
	case stateHalfOpen:
		if ok {
			cb.state = stateClosed // 探测成功，恢复
			cb.failures = 0
		} else {
			cb.state = stateOpen // 探测失败，重新熔断
			cb.openedAt = time.Now()
		}
	}
}

func (cb *CircuitBreaker) State() circuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// TestCircuitBreaker 验证：连续失败 → Open（拒绝）→ 冷却 → Half-Open → 成功恢复 Closed。
func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// Closed 阶段：放行，连续失败 3 次触发熔断
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("Closed 阶段第 %d 次应放行", i)
		}
		cb.RecordResult(false)
	}
	if cb.State() != stateOpen {
		t.Fatalf("连续 3 次失败后应进入 Open，实际 %v", cb.State())
	}

	// Open 阶段：拒绝请求
	if cb.Allow() {
		t.Fatal("Open 冷却期内应拒绝请求")
	}

	// 等待冷却结束
	time.Sleep(120 * time.Millisecond)

	// Half-Open：放行探测，成功则恢复 Closed
	if !cb.Allow() {
		t.Fatal("冷却结束后应放行探测请求")
	}
	cb.RecordResult(true)
	if cb.State() != stateClosed {
		t.Fatalf("探测成功后应恢复 Closed，实际 %v", cb.State())
	}
	t.Logf("熔断状态机验证通过：Closed → Open → Half-Open → Closed")
}
