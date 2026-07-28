//go:build ignore

package answer

import (
	"fmt"
	"reflect"
)

type MyError struct {
	Code    int
	Message string
}

func (e *MyError) Error() string {
	return fmt.Sprintf("error %d: %s", e.Code, e.Message)
}

func IsNilInterface(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

func SafeError(err *MyError) error {
	if err == nil {
		return nil
	}
	return err
}

func WrapError(msg string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}
