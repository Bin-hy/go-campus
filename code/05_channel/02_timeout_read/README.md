# 超时读取 Channel

## 难度：⭐⭐ 中等

## 考点
- select 多路复用
- time.After / time.NewTimer 超时控制
- context.WithTimeout 的等价实现

## 题目描述

实现几个与超时相关的 channel 工具函数：

### 函数1：ReadWithTimeout
从 channel 读取一个值，如果超时则返回错误。

### 函数2：ReadMultipleWithTimeout
从 channel 读取最多 n 个值，总超时限制。

### 函数3：FirstResult
并发执行多个函数，返回第一个完成的结果（其余丢弃）。

## 函数签名

```go
func ReadWithTimeout(ch <-chan int, timeout time.Duration) (int, error)
func ReadMultipleWithTimeout(ch <-chan int, n int, timeout time.Duration) []int
func FirstResult(fns ...func() int) int
```

## 提示
1. `select` + `case <-time.After(timeout)` 是经典超时模式
2. ReadMultipleWithTimeout 用 for 循环 + select
3. FirstResult 每个函数启动一个 goroutine，结果写入共享 channel
