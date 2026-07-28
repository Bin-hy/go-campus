package error_handling

import (
	"errors"
	"fmt"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	err := &AppError{Code: 404, Message: "user not found", Err: nil}
	s := err.Error()
	if s != "[404] user not found" {
		t.Errorf("期望 '[404] user not found'，得到 %q", s)
	}

	inner := fmt.Errorf("db connection failed")
	err2 := &AppError{Code: 500, Message: "query failed", Err: inner}
	s2 := err2.Error()
	if s2 != "[500] query failed: db connection failed" {
		t.Errorf("期望 '[500] query failed: db connection failed'，得到 %q", s2)
	}
}

func TestAppError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("original error")
	err := &AppError{Code: 500, Message: "wrapped", Err: inner}

	if !errors.Is(err, inner) {
		t.Error("errors.Is 应能找到内部错误")
	}
}

func TestNewAppError(t *testing.T) {
	inner := fmt.Errorf("timeout")
	err := NewAppError(503, "service unavailable", inner)

	if err.Code != 503 {
		t.Errorf("Code 应为503")
	}
	if err.Message != "service unavailable" {
		t.Errorf("Message 错误")
	}
	if !errors.Is(err, inner) {
		t.Error("应能解包到 inner error")
	}
}

func TestIsNotFound(t *testing.T) {
	err := NewAppError(404, "user not found", nil)
	if !IsNotFound(err) {
		t.Error("404 错误应被 IsNotFound 识别")
	}

	wrapped := fmt.Errorf("handler: %w", err)
	if !IsNotFound(wrapped) {
		t.Error("包装后的 404 也应被识别")
	}

	err500 := NewAppError(500, "server error", nil)
	if IsNotFound(err500) {
		t.Error("500 错误不应被识别为 NotFound")
	}

	if IsNotFound(nil) {
		t.Error("nil 不应被识别为 NotFound")
	}
}

func TestGetCode(t *testing.T) {
	err := NewAppError(403, "forbidden", nil)
	if GetCode(err) != 403 {
		t.Errorf("期望403，得到 %d", GetCode(err))
	}

	wrapped := fmt.Errorf("check: %w", err)
	if GetCode(wrapped) != 403 {
		t.Errorf("包装后期望403，得到 %d", GetCode(wrapped))
	}

	if GetCode(fmt.Errorf("plain error")) != -1 {
		t.Error("非 AppError 应返回 -1")
	}

	if GetCode(nil) != -1 {
		t.Error("nil 应返回 -1")
	}
}

func TestWrapWithContext_Nil(t *testing.T) {
	if WrapWithContext("ctx", nil) != nil {
		t.Error("nil error 包装后应仍为 nil")
	}
}

func TestWrapWithContext_Chain(t *testing.T) {
	base := NewAppError(404, "not found", nil)
	wrapped := WrapWithContext("userService", base)
	wrapped2 := WrapWithContext("handler", wrapped)

	if !IsNotFound(wrapped2) {
		t.Error("多层包装后仍应能识别 404")
	}
}
