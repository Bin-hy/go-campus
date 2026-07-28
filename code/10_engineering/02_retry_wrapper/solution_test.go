package retry_wrapper

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_SuccessOnFirst(t *testing.T) {
	called := 0
	err := Retry(func() error {
		called++
		return nil
	}, DefaultConfig())

	if err != nil {
		t.Errorf("不应有错误: %v", err)
	}
	if called != 1 {
		t.Errorf("成功时应只调用1次，实际 %d", called)
	}
}

func TestRetry_SuccessOnThird(t *testing.T) {
	var count int64
	err := Retry(func() error {
		c := atomic.AddInt64(&count, 1)
		if c < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}, DefaultConfig())

	if err != nil {
		t.Errorf("第三次应成功: %v", err)
	}
	if count != 3 {
		t.Errorf("应调用3次，实际 %d", count)
	}
}

func TestRetry_AllFail(t *testing.T) {
	config := RetryConfig{MaxRetries: 3, InitialWait: time.Millisecond, MaxWait: time.Second, Multiplier: 2}
	called := 0

	err := Retry(func() error {
		called++
		return errors.New("always fails")
	}, config)

	if err == nil {
		t.Error("全部失败应返回 error")
	}
	if called != 4 { // 1 + 3 retries
		t.Errorf("应调用4次（1次原始 + 3次重试），实际 %d", called)
	}
}

func TestRetry_ExponentialBackoff(t *testing.T) {
	config := RetryConfig{
		MaxRetries:  3,
		InitialWait: 50 * time.Millisecond,
		MaxWait:     time.Second,
		Multiplier:  2.0,
	}

	start := time.Now()
	Retry(func() error {
		return errors.New("fail")
	}, config)
	elapsed := time.Since(start)

	// 等待时间：50 + 100 + 200 = 350ms 左右
	if elapsed < 300*time.Millisecond || elapsed > 600*time.Millisecond {
		t.Errorf("退避时间不正确：%v（期望约350ms）", elapsed)
	}
}

func TestRetry_MaxWaitCap(t *testing.T) {
	config := RetryConfig{
		MaxRetries:  3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     150 * time.Millisecond, // 第二次退避 200ms 应被限制为 150ms
		Multiplier:  2.0,
	}

	start := time.Now()
	Retry(func() error {
		return errors.New("fail")
	}, config)
	elapsed := time.Since(start)

	// 100 + 150 + 150 = 400ms
	if elapsed > 600*time.Millisecond {
		t.Errorf("MaxWait 未生效：%v", elapsed)
	}
}

func TestRetryWithResult(t *testing.T) {
	var count int64
	result, err := RetryWithResult(func() (string, error) {
		c := atomic.AddInt64(&count, 1)
		if c < 2 {
			return "", errors.New("not ready")
		}
		return "success", nil
	}, RetryConfig{MaxRetries: 3, InitialWait: time.Millisecond, MaxWait: time.Second, Multiplier: 2})

	if err != nil {
		t.Fatalf("应成功: %v", err)
	}
	if result != "success" {
		t.Errorf("期望 success，得到 %q", result)
	}
}

type nonRetryableError struct{ msg string }

func (e *nonRetryableError) Error() string      { return e.msg }
func (e *nonRetryableError) IsRetryable() bool { return false }

func TestShouldRetry(t *testing.T) {
	// 普通 error 默认可重试
	if !ShouldRetry(errors.New("normal")) {
		t.Error("普通错误应可重试")
	}

	// 实现了 Retryable 接口返回 false
	if ShouldRetry(&nonRetryableError{"permanent"}) {
		t.Error("nonRetryable 错误不应重试")
	}

	// nil 不应重试
	if ShouldRetry(nil) {
		t.Error("nil 不应重试")
	}
}
