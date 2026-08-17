package circuit_breaker

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	mu        sync.Mutex
	state     State
	failures  int
	threshold int
	cooldown  time.Duration
	openedAt  time.Time
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{state: StateClosed, threshold: threshold, cooldown: cooldown}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.openedAt) >= cb.cooldown {
			cb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

func (cb *CircuitBreaker) RecordResult(ok bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		if !ok {
			cb.failures++
			if cb.failures >= cb.threshold {
				cb.state = StateOpen
				cb.openedAt = time.Now()
			}
		} else {
			cb.failures = 0
		}
	case StateHalfOpen:
		if ok {
			cb.state = StateClosed
			cb.failures = 0
		} else {
			cb.state = StateOpen
			cb.openedAt = time.Now()
		}
	}
}

func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
