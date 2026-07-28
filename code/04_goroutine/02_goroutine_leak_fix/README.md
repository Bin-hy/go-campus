# 修复 Goroutine 泄漏

## 难度：⭐⭐ 中等

## 考点
- goroutine 泄漏的常见场景
- 用 done channel/context 控制 goroutine 生命周期
- 排查泄漏的方法

## 题目描述

以下代码存在 goroutine 泄漏问题，请修复：

### 函数1：FixedSearch
同时向多个服务发起请求，只要任何一个返回结果就立即返回。
原始版本会导致其余 goroutine 永远阻塞。

### 函数2：FixedGenerator
生成无限序列的生成器，调用者可能只需要前几个值。
原始版本中如果调用者提前停止读取，生成器 goroutine 会泄漏。

### 函数3：FixedWorker
一个定时执行任务的 worker，需要支持优雅关闭。

## 函数签名

```go
func FixedSearch(query string, backends ...func(string) string) string
func FixedGenerator(done <-chan struct) <-chan int
func FixedWorker(done <-chan struct{}, interval time.Duration, task func())
```

## 提示
1. FixedSearch：用 buffered channel 或 done channel 确保落选的 goroutine 不阻塞
2. FixedGenerator：在 select 中同时监听 done 和输出 channel
3. FixedWorker：用 select 监听 done 和 ticker
