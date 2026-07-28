# 单词拆分

## 难度：⭐⭐⭐ 面试高频

## 考点
- DP + 哈希集合
- 字符串分割问题
- 字节面试高频题

## 题目描述

给你一个字符串 `s` 和一个字符串列表 `wordDict` 作为字典。判断是否可以利用字典中出现的单词拼接出 `s`。字典中的单词可以重复使用。

## 函数签名

```go
func wordBreak(s string, wordDict []string) bool
```

## 示例

```
输入：s = "leetcode", wordDict = ["leet","code"]
输出：true（"leet" + "code"）

输入：s = "applepenapple", wordDict = ["apple","pen"]
输出：true（"apple" + "pen" + "apple"）

输入：s = "catsandog", wordDict = ["cats","dog","sand","and","cat"]
输出：false
```

## 要求
1. 时间复杂度 O(n²)，n = len(s)

## 提示
- dp[i] = s[0:i] 能否被字典中的单词拼接
- dp[0] = true（空串）
- dp[i] = true 如果存在 j < i 使得 dp[j] == true 且 s[j:i] 在字典中
- 用 map/set 存储字典，O(1) 查找
