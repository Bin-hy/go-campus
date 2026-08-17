package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucket(t *testing.T) {
	b := NewTokenBucket(3, 10) // 容量 3，每秒补 10

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("第 %d 个请求应放行（桶内有令牌）", i)
		}
	}
	if b.Allow() {
		t.Fatal("桶耗尽后应立即拒绝")
	}
	t.Log("令牌桶验证通过：突发 3 个后拒绝")
}

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
	t.Log("滑动窗口验证通过：1 秒窗口内限流 3 次")
}
