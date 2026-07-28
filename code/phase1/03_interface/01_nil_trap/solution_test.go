package nil_trap

import (
	"testing"
)

func TestIsNilInterface_NilInterface(t *testing.T) {
	var v interface{} = nil
	if !IsNilInterface(v) {
		t.Error("nil interface 应返回 true")
	}
}

func TestIsNilInterface_NilPointerInInterface(t *testing.T) {
	var p *int = nil
	var v interface{} = p

	// v != nil (因为 type 不为 nil)，但值为 nil
	if v == nil {
		t.Fatal("前置条件错误：v 不应该 == nil")
	}

	if !IsNilInterface(v) {
		t.Error("持有 nil 指针的 interface 应返回 true")
	}
}

func TestIsNilInterface_NilMap(t *testing.T) {
	var m map[string]int = nil
	var v interface{} = m

	if !IsNilInterface(v) {
		t.Error("持有 nil map 的 interface 应返回 true")
	}
}

func TestIsNilInterface_NilSlice(t *testing.T) {
	var s []int = nil
	var v interface{} = s

	if !IsNilInterface(v) {
		t.Error("持有 nil slice 的 interface 应返回 true")
	}
}

func TestIsNilInterface_NilChannel(t *testing.T) {
	var ch chan int = nil
	var v interface{} = ch

	if !IsNilInterface(v) {
		t.Error("持有 nil channel 的 interface 应返回 true")
	}
}

func TestIsNilInterface_NilFunc(t *testing.T) {
	var fn func() = nil
	var v interface{} = fn

	if !IsNilInterface(v) {
		t.Error("持有 nil func 的 interface 应返回 true")
	}
}

func TestIsNilInterface_NonNil(t *testing.T) {
	x := 42
	var v interface{} = &x
	if IsNilInterface(v) {
		t.Error("非 nil 指针不应返回 true")
	}

	v = 0
	if IsNilInterface(v) {
		t.Error("零值 int 不应返回 true")
	}

	v = ""
	if IsNilInterface(v) {
		t.Error("空字符串不应返回 true")
	}

	v = false
	if IsNilInterface(v) {
		t.Error("false 不应返回 true")
	}
}

func TestSafeError_Nil(t *testing.T) {
	var err *MyError = nil
	result := SafeError(err)

	if result != nil {
		t.Errorf("SafeError(nil) 应返回 nil，得到 %v (type: %T)", result, result)
	}
}

func TestSafeError_NonNil(t *testing.T) {
	err := &MyError{Code: 404, Message: "not found"}
	result := SafeError(err)

	if result == nil {
		t.Fatal("SafeError(非nil) 不应返回 nil")
	}
	if result.Error() != "error 404: not found" {
		t.Errorf("错误消息不正确：%s", result.Error())
	}
}

func TestWrapError_NilError(t *testing.T) {
	result := WrapError("context", nil)
	if result != nil {
		t.Errorf("WrapError(msg, nil) 应返回 nil，得到 %v", result)
	}
}

func TestWrapError_NonNilError(t *testing.T) {
	inner := &MyError{Code: 500, Message: "internal"}
	result := WrapError("operation failed", inner)

	if result == nil {
		t.Fatal("WrapError 非 nil error 不应返回 nil")
	}

	// 包装后的错误消息应包含外部消息和内部错误
	msg := result.Error()
	if msg == "" {
		t.Error("包装后的错误消息不应为空")
	}
}

// 经典面试陷阱场景
func TestNilTrap_ClassicScenario(t *testing.T) {
	// 这是最经典的面试题场景
	var getError func() error = func() error {
		var err *MyError = nil
		// 直接返回 err，interface 会持有 (*MyError)(nil)
		// 导致调用者 if err != nil 为 true！
		return err // 这是一个 BUG！
	}

	err := getError()
	// 如果你理解了 nil 陷阱，就知道这里 err != nil
	if err == nil {
		t.Skip("如果你的 Go 版本行为不同，跳过此测试")
	}

	// 使用 IsNilInterface 可以正确判断
	if !IsNilInterface(err) {
		t.Error("IsNilInterface 应该能检测出这是 nil 值")
	}
}
