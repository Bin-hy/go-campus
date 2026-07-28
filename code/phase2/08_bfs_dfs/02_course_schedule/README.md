# 课程表

## 难度：⭐⭐⭐ 面试高频

## 考点
- 拓扑排序（BFS Kahn 算法）
- 有向图判环
- 字节面试高频图论题

## 题目描述

你这个学期必须选修 `numCourses` 门课程，记为 `0` 到 `numCourses-1`。给你一个数组 `prerequisites`，其中 `prerequisites[i] = [a, b]` 表示如果你想学课程 `a`，必须先学课程 `b`。

判断你是否可以完成所有课程的学习。（等价于：有向图中是否存在环）

## 函数签名

```go
func canFinish(numCourses int, prerequisites [][]int) bool
```

## 示例

```
输入：numCourses = 2, prerequisites = [[1,0]]
输出：true（先学 0 再学 1）

输入：numCourses = 2, prerequisites = [[1,0],[0,1]]
输出：false（存在环：0→1→0）
```

## 要求
1. 时间复杂度 O(V+E)

## 提示
### BFS 拓扑排序（Kahn 算法）
1. 构建邻接表 + 入度数组
2. 入度为 0 的节点入队
3. 每次出队一个，将其所有邻接节点入度 -1
4. 入度变 0 的节点入队
5. 最终出队数 == numCourses → 无环 → 可完成

### DFS 判环
- 三色标记：白(未访问)、灰(路径中)、黑(已完成)
- 遇到灰色节点 → 有环
