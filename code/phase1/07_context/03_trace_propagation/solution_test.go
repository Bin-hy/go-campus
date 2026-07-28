package trace_propagation

import (
	"context"
	"testing"
)

func TestWithTraceID(t *testing.T) {
	ctx := WithTraceID(context.Background(), "abc-123")
	id := GetTraceID(ctx)
	if id != "abc-123" {
		t.Errorf("期望 abc-123，得到 %s", id)
	}
}

func TestGetTraceID_Empty(t *testing.T) {
	id := GetTraceID(context.Background())
	if id != "" {
		t.Errorf("无 traceID 应返回空字符串，得到 %s", id)
	}
}

func TestWithUserID(t *testing.T) {
	ctx := WithUserID(context.Background(), "user-456")
	id := GetUserID(ctx)
	if id != "user-456" {
		t.Errorf("期望 user-456，得到 %s", id)
	}
}

func TestBothValues(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-1")
	ctx = WithUserID(ctx, "user-2")

	if GetTraceID(ctx) != "trace-1" {
		t.Error("traceID 丢失")
	}
	if GetUserID(ctx) != "user-2" {
		t.Error("userID 丢失")
	}
}

func TestMiddleware(t *testing.T) {
	result := Middleware("req-100", "admin", func(ctx context.Context) string {
		tid := GetTraceID(ctx)
		uid := GetUserID(ctx)
		return tid + ":" + uid
	})

	if result != "req-100:admin" {
		t.Errorf("期望 req-100:admin，得到 %s", result)
	}
}

func TestContextValueIsolation(t *testing.T) {
	parent := WithTraceID(context.Background(), "parent-trace")
	child := WithUserID(parent, "child-user")

	// child 能获取 parent 的值
	if GetTraceID(child) != "parent-trace" {
		t.Error("子 context 应能读取父 context 的值")
	}

	// parent 不能获取 child 的值
	if GetUserID(parent) != "" {
		t.Error("父 context 不应能读取子 context 的值")
	}
}
