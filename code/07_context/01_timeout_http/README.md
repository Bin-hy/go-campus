# Context 超时控制

## 难度：⭐⭐ 中等

## 考点
- context.WithTimeout 使用
- 超时取消传播
- 父 context 取消影响子 context

## 题目描述
实现带超时控制的请求函数，以及并发请求多个 URL 的函数。

## 提示
1. `context.WithTimeout` 返回带截止时间的子 context
2. 记得 `defer cancel()` 释放资源
3. FetchMultiple 中每个 URL 启动一个 goroutine
