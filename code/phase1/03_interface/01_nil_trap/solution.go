package nil_trap

import "fmt"

// MyError 自定义错误类型
type MyError struct {
	Code    int
	Message string
}

func (e *MyError) Error() string {
	return fmt.Sprintf("error %d: %s", e.Code, e.Message)
}

// IsNilInterface 判断 interface{} 是否"真正为 nil"
// 以下情况都返回 true：
// - v 本身是 nil interface
// - v 持有的是 nil 指针/nil map/nil slice/nil channel/nil func
func IsNilInterface(v interface{}) bool {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// SafeError 安全地将 *MyError 转为 error 接口
// 如果 err 为 nil，返回 nil error（避免 interface 持有 nil 指针的陷阱）
func SafeError(err *MyError) error {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// WrapError 包装错误，添加上下文消息
// 如果 err 为 nil，返回 nil（不是包装一个 nil）
func WrapError(msg string, err error) error {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
