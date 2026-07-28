# 编辑距离

## 难度：⭐⭐⭐⭐ 面试 Hard 高频

## 考点
- 二维 DP 经典题
- 状态转移方程推导
- 字节面试高频 DP 题

## 题目描述

给你两个单词 `word1` 和 `word2`，计算出将 `word1` 转换成 `word2` 所使用的最少操作数。

允许的三种操作：
- 插入一个字符
- 删除一个字符
- 替换一个字符

## 函数签名

```go
func minDistance(word1 string, word2 string) int
```

## 示例

```
输入：word1 = "horse", word2 = "ros"
输出：3
horse → rorse（替换 h→r）→ rose（删除 r）→ ros（删除 e）

输入：word1 = "intention", word2 = "execution"
输出：5
```

## 要求
1. 时间复杂度 O(m*n)，空间复杂度 O(m*n)（可优化到 O(n)）

## 提示
- dp[i][j] = word1 前 i 个字符转换为 word2 前 j 个字符的最少操作数
- word1[i-1] == word2[j-1] → dp[i][j] = dp[i-1][j-1]（不需要操作）
- 否则 dp[i][j] = min(dp[i-1][j]+1, dp[i][j-1]+1, dp[i-1][j-1]+1)
  - dp[i-1][j]+1：删除 word1[i-1]
  - dp[i][j-1]+1：插入 word2[j-1]
  - dp[i-1][j-1]+1：替换 word1[i-1] 为 word2[j-1]
- 初始化：dp[i][0]=i, dp[0][j]=j
