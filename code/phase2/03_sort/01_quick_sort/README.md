# 快速排序

## 难度：⭐⭐⭐ 面试必会手写

## 考点
- 分治思想
- partition 分区操作
- 随机 pivot 避免最坏情况
- 字节面试手撕基本功

## 题目描述

给你一个整数数组 `nums`，请你使用快速排序将其升序排列后返回。

## 函数签名

```go
func sortArray(nums []int) []int
```

## 示例

```
输入：nums = [5,2,3,1]
输出：[1,2,3,5]

输入：nums = [5,1,1,2,0,0]
输出：[0,0,1,1,2,5]
```

## 要求
1. 必须手写快速排序（不能用标准库 sort）
2. 使用随机 pivot 优化（避免有序输入退化为 O(n²)）
3. 原地排序

## 提示
- 选 pivot → partition 使小于 pivot 的在左、大于 pivot 的在右 → 递归左右
- 随机选 pivot：`rand.Intn(right-left+1) + left`
- partition 用 Lomuto 方案（单指针 i 维护"小于区域"的右边界）
