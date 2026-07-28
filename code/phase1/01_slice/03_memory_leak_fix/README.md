# 修复 Slice 内存泄漏

## 难度：⭐⭐ 中等

## 考点
- slice 截取后底层数组无法被 GC 回收
- 大切片截取小片段的内存泄漏问题
- 正确的截取方式避免内存泄漏

## 题目描述

在实际开发中，经常遇到从一个大切片中截取少量数据的场景。如果直接用 `s[start:end]`，
返回的子切片仍然引用原来的底层大数组，导致整个大数组无法被 GC 回收。

实现以下函数，安全地从大数据中提取部分内容，不造成内存泄漏：

### 函数1：GetFirstN
从一个可能很大的 byte 切片中提取前 N 个字节，确保原始大切片可被 GC。

### 函数2：FilterLargeSlice
从大切片中筛选满足条件的元素，返回的结果不应引用原始大切片的底层数组。

### 函数3：TrimMessage
从消息数据（[]byte）中提取有效载荷部分（去掉前4字节头和后2字节尾），
确保返回值不持有原始数据的引用。

## 函数签名

```go
func GetFirstN(data []byte, n int) []byte
func FilterLargeSlice(data []int, predicate func(int) bool) []int
func TrimMessage(msg []byte) []byte
```

## 提示
1. 关键思路：创建新切片 + copy，而不是直接截取
2. `append([]T(nil), slice...)` 也是一种脱离底层数组的方式
3. 注意边界条件：n 大于 len(data) 时怎么办？msg 长度不足6时怎么办？
