# 接口 nil 陷阱

## 难度：⭐⭐ 中等

## 考点
- eface（空接口）与 iface（非空接口）的底层区别
- 接口持有 nil 指针时不等于 nil
- 安全的 nil 检查方式

## 题目描述

### 函数1：IsNilInterface
判断一个 interface{} 值是否真正为 nil（包括接口本身为 nil，或接口持有的值为 nil 指针）。
用 reflect 包实现对任意接口值的深度 nil 检查。

### 函数2：SafeError
安全地从可能返回 nil 指针的函数中获取 error。
避免 "nil pointer in interface" 陷阱。

### 函数3：WrapError
实现一个安全的 error 包装函数。如果内部 error 为 nil，返回 nil interface（而不是持有 nil 的 interface）。

## 函数签名

```go
func IsNilInterface(v interface{}) bool
func SafeError(err *MyError) error
func WrapError(msg string, err error) error
```

## 提示
1. `reflect.ValueOf(v).IsNil()` 可以检查接口中的值是否为 nil（但要先判断 Kind）
2. 可以用 `reflect.ValueOf(v).IsValid()` 检查 reflect.Value 本身是否有效
3. SafeError 的关键：如果 *MyError 为 nil，直接返回 nil（不是返回 err）
