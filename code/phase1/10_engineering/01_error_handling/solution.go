package error_handling

import "fmt"

// AppError 应用级错误
type AppError struct {
	Code    int
	Message string
	Err     error // 内部错误
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	// TODO: 格式："[code] message: inner error" 或 "[code] message"（无内部错误时）
	panic("not implemented")
}

// Unwrap 支持 errors.Is / errors.As 链式解包
func (e *AppError) Unwrap() error {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// 预定义错误码
var (
	ErrNotFound     = &AppError{Code: 404, Message: "not found"}
	ErrUnauthorized = &AppError{Code: 401, Message: "unauthorized"}
	ErrInternal     = &AppError{Code: 500, Message: "internal error"}
)

// NewAppError 创建新的应用错误（包装内部错误）
func NewAppError(code int, message string, err error) *AppError {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// IsNotFound 判断 err 是否是 404 错误（支持 wrapped error）
func IsNotFound(err error) bool {
	// TODO: 用 errors.As 实现
	panic("not implemented")
}

// GetCode 从 error 中提取错误码，非 AppError 返回 -1
func GetCode(err error) int {
	// TODO: 用 errors.As 实现
	panic("not implemented")
}

// WrapWithContext 包装错误并添加上下文
func WrapWithContext(context string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}
