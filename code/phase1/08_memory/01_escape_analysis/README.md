# 逃逸分析

## 难度：⭐⭐ 中等

## 考点
- 理解栈分配 vs 堆分配
- 哪些操作会导致逃逸
- 如何优化减少堆分配

## 题目描述
实现多组对比函数，一个会逃逸到堆，一个留在栈上。
通过 benchmark + `go test -gcflags="-m"` 验证。

### 函数对1：CreateOnStack vs CreateOnHeap
### 函数对2：SliceNoEscape vs SliceEscape
### 函数对3：InterfaceEscape vs NoInterfaceEscape
