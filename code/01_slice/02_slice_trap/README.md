# Slice 陷阱：共享底层数组

## 难度：⭐⭐ 中等

## 考点
- slice 底层数组共享机制
- append 扩容时机
- 子切片修改影响原切片的场景

## 题目描述

实现以下函数，它们考察你对 slice 底层行为的理解：

### 函数1：AppendNoEffect
给定一个切片 s 和一个值 val，向 s append val 后返回原切片 s 是否受影响。
- 如果 `len(s) < cap(s)`（有剩余容量），append 会修改底层数组，原切片对应位置会变
- 如果 `len(s) == cap(s)`（无剩余容量），append 触发扩容，原切片不受影响

### 函数2：SafeSubSlice
从切片中截取子切片，确保对子切片的 append 操作不会影响原切片。

### 函数3：RemoveElement
从切片中删除指定索引的元素（保持顺序），返回新切片。

## 函数签名

```go
func AppendNoEffect(s []int, val int) bool
func SafeSubSlice(s []int, start, end int) []int
func RemoveElement(s []int, index int) []int
```

## 提示
1. `s[low:high:max]` 三下标切片可以限制 cap，阻止 append 覆盖后续元素
2. 删除元素常用 `append(s[:i], s[i+1:]...)` 模式
3. AppendNoEffect 的关键判断：`len(s) == cap(s)`
