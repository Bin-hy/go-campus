# 组合总和

## 难度：⭐⭐⭐ 面试高频

## 考点
- 回溯 + 剪枝
- 元素可重复选取（start 不变）
- 排序后剪枝优化

## 题目描述

给你一个无重复元素的整数数组 `candidates` 和一个目标整数 `target`，找出所有和为 `target` 的组合。`candidates` 中的数字可以无限制重复选取。结果不能包含重复组合。

## 函数签名

```go
func combinationSum(candidates []int, target int) [][]int
```

## 示例

```
输入：candidates = [2,3,6,7], target = 7
输出：[[2,2,3],[7]]

输入：candidates = [2,3,5], target = 8
输出：[[2,2,2,2],[2,3,3],[3,5]]

输入：candidates = [2], target = 1
输出：[]
```

## 提示
- 排序后可剪枝：当前候选数 > remain 时直接 break
- 允许重复选取：递归时 start 不加 1（`backtrack(i, ...)`）
- 避免重复组合：每次只往后选（不回头选之前的数）
