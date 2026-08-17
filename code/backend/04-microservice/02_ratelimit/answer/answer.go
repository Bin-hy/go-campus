package ratelimit

import (
	"sync"
	"time"
)

type TokenBucket struct {
	mu       sync.Mutex
	capacity float64
	tokens   float64
	rate     float64
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
