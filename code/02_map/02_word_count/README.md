# 词频统计

## 难度：⭐ 基础

## 考点
- map 基本 CRUD 操作
- 字符串分割处理
- map 排序输出

## 题目描述

### 函数1：WordCount
统计文本中每个单词出现的次数。单词以空格分隔，需要转为小写后统计。

### 函数2：TopN
从词频 map 中找出出现次数最多的 N 个单词。
如果出现次数相同，按字母顺序排列。

### 函数3：UniqueWords
返回文本中只出现一次的单词列表（按字母顺序）。

## 函数签名

```go
func WordCount(text string) map[string]int
func TopN(freq map[string]int, n int) []string
func UniqueWords(text string) []string
```

## 提示
1. `strings.Fields` 按空白字符分割
2. `strings.ToLower` 转小写
3. 排序可以用 `sort.Slice`
