# 零钱兑换

## 难度：⭐⭐⭐ 面试高频

## 考点
- 完全背包问题
- 动态规划状态定义和转移
- 字节面试高频 DP 题

## 题目描述

给你一个整数数组 `coins` 表示不同面额的硬币，以及一个整数 `amount` 表示总金额。计算凑成总金额所需的最少硬币个数。如果无法凑成，返回 `-1`。每种硬币数量无限。

## 函数签名

```go
func coinChange(coins []int, amount int) int
```

## 示例

```
输入：coins = [1, 5, 10, 25], amount = 30
输出：2（25 + 5）

输入：coins = [2], amount = 3
输出：-1

输入：coins = [1], amount = 0
输出：0
```

## 要求
1. 时间复杂度 O(amount * len(coins))

## 提示
- dp[i] = 凑成金额 i 的最少硬币数
- dp[0] = 0
- dp[i] = min(dp[i-coin] + 1) for each coin
- 初始化 dp 为 amount+1（不可达标记）
