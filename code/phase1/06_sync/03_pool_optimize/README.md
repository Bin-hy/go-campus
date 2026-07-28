# sync.Pool 对象池优化

## 难度：⭐⭐ 中等

## 考点
- sync.Pool 使用方法
- 减少 GC 压力
- 对象复用模式

## 题目描述

实现一个使用 sync.Pool 优化的 Buffer Pool，用于高频创建/销毁 bytes.Buffer 的场景。

要求：
1. `GetBuffer()` — 从池中获取 Buffer（或新建）
2. `PutBuffer(buf)` — 将 Buffer 归还到池中（重置内容）
3. `ProcessRequests(data)` — 使用 Pool 高效处理多个请求

## 函数签名

```go
type BufferPool struct { ... }

func NewBufferPool() *BufferPool
func (p *BufferPool) GetBuffer() *bytes.Buffer
func (p *BufferPool) PutBuffer(buf *bytes.Buffer)
func ProcessRequests(requests [][]byte) []string
```

## 提示
1. sync.Pool 的 New 函数用于创建新对象
2. PutBuffer 前要 Reset，防止数据泄漏
3. ProcessRequests 对每个请求 Get → 使用 → Put
