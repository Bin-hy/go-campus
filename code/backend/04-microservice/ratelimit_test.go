// T32 实验（下）：限流 —— 令牌桶（允许突发）与滑动窗口（平滑）。
package microservice_lab

import (
	"sync"
	"testing"
	"time"
)

// TokenBucket 令牌桶：匀速放令牌，请求取令牌，允许突发。
type TokenBucket struct {
	mu       sync.Mutex
	capacity float64 // 桶容量
	tokens   float64 // 当前令牌数
	rate     float64 // 每秒补充令牌数
	last     time.Time
}

func NewTokenBucket(capacity, rate float64) *TokenBucket {
	return &TokenBucket{capacity: capacity, tokens: capacity, rate: rate, last: time.Now()}
}

func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.tokens = min(b.capacity, b.tokens+elapsed*b.rate)
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// SlidingWindow 滑动窗口：统计最近 window 内请求数，超过 limit 拒绝。
type SlidingWindow struct {
	mu         sync.Mutex
	window     time.Duration
	limit      int
	timestamps []time.Time
}

func NewSlidingWindow(window time.Duration, limit int) *SlidingWindow {
	return &SlidingWindow{window: window, limit: limit}
}

func (w *SlidingWindow) Allow() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-w.window)
	// 淘汰窗口外的旧时间戳
	kept := w.timestamps[:0]
	for _, ts := range w.timestamps {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	w.timestamps = kept
	if len(w.timestamps) >= w.limit {
		return false
	}
	w.timestamps = append(w.timestamps, now)
	return true
}

// TestTokenBucket 验证：初始可突发消费 capacity 个，耗尽后需等待补充。
func TestTokenBucket(t *testing.T) {
	b := NewTokenBucket(3, 10) // 容量 3，每秒补 10

	// 突发消费 3 个（桶内初始 3 个）
	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("第 %d 个请求应放行（桶内有令牌）", i)
		}
	}
	if b.Allow() {
		t.Fatal("桶耗尽后应立即拒绝")
	}
	t.Logf("令牌桶验证通过：突发 3 个后拒绝")
}

// TestSlidingWindow 验证：窗口内超过 limit 拒绝。
func TestSlidingWindow(t *testing.T) {
	w := NewSlidingWindow(time.Second, 3)

	for i := 0; i < 3; i++ {
		if !w.Allow() {
			t.Fatalf("窗口内第 %d 个请求应放行", i)
		}
	}
	if w.Allow() {
		t.Fatal("窗口内超过 limit 应拒绝")
	}
	t.Logf("滑动窗口验证通过：1 秒窗口内限流 3 次")
}
