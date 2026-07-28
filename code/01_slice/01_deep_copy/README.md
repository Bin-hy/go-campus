# Slice 深拷贝

## 难度：⭐ 基础

## 考点
- slice 底层结构（共享底层数组）
- 深拷贝 vs 浅拷贝
- nil slice 与 empty slice 的区别

## 题目描述

实现一个 `DeepCopy` 函数，对 int 切片进行深拷贝。

要求：
1. 返回一个全新的切片，修改返回值不影响原切片
2. 如果输入为 nil，返回 nil
3. 如果输入为空切片（len=0），返回空切片（非 nil）

## 函数签名

```go
func DeepCopy(src []int) []int
```

## 示例

```go
src := []int{1, 2, 3}
dst := DeepCopy(src)
dst[0] = 100
// src[0] 仍然是 1
```

## 提示
1. `copy` 内置函数可以拷贝切片内容
2. 注意区分 nil slice 和 empty slice：`var s []int` 是 nil，`[]int{}` 是 empty
3. 用 `make` 创建新切片时，len 应该与 src 相同
