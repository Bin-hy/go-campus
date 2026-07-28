//go:build ignore

package answer

import (
	"errors"
	"fmt"
)

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Err.Error())
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func IsNotFound(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == 404
	}
	return false
}

func GetCode(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return -1
}

func WrapWithContext(context string, err error) error {
	if err == nil { return nil }
	return fmt.Errorf("%s: %w", context, err)
}
