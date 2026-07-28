# 交替打印奇偶数

## 难度：⭐⭐ 中等

## 考点
- 两个 goroutine 交替执行
- channel 作为同步信号
- 无缓冲 channel 的阻塞特性

## 题目描述

使用两个 goroutine 交替打印 1~n 的数字，一个打印奇数，一个打印偶数，保证输出有序。

要求：
1. `PrintOddEven(n)` 返回按序排列的 []int{1, 2, 3, ..., n}
2. 必须使用两个 goroutine 协作完成，不能在单个 goroutine 中完成
3. 奇数 goroutine 负责 1, 3, 5...，偶数 goroutine 负责 2, 4, 6...

## 函数签名

```go
func PrintOddEven(n int) []int
```

## 提示
1. 用两个 channel 交替通知对方"轮到你了"
2. 奇数 goroutine 完成后通知偶数 goroutine，反之亦然
3. 收集结果可以用一个共享的 channel 或 slice + mutex
