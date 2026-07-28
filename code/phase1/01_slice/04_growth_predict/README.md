# Slice 扩容预测

## 难度：⭐⭐ 中等

## 考点
- slice 扩容策略（Go 1.18+ 新规则）
- 内存对齐对实际 cap 的影响
- 预分配 slice 优化性能

## 题目描述

### 函数1：PredictGrowth
给定当前 cap 和需要的最小 cap，预测扩容后的新 cap。
使用 Go 1.18+ 的扩容策略：
- 如果 newLen（需要的容量）> 2 * oldCap：直接使用 newLen
- 如果 oldCap < 256：翻倍
- 否则：newcap += (newcap + 3*256) / 4，循环直到 >= newLen

注意：这里只需要实现逻辑上的扩容策略，不需要考虑内存对齐。

### 函数2：OptimalPrealloc
给定一系列 append 操作的元素数量，计算如果预先分配 cap 可以避免多少次扩容。
返回：(不预分配的扩容次数, 预分配后的扩容次数)

### 函数3：BatchAppend
高效地将多个切片合并为一个。要求：只分配一次内存。

## 函数签名

```go
func PredictGrowth(oldCap, needCap int) int
func OptimalPrealloc(appendSizes []int) (withoutPrealloc, withPrealloc int)
func BatchAppend(slices ...[]int) []int
```

## 提示
1. 扩容策略的关键是 256 这个阈值
2. OptimalPrealloc 需要模拟逐步 append 的过程
3. BatchAppend 先算总长度，再一次性 make，最后逐个 copy
