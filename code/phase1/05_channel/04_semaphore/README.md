# 信号量（Semaphore）

## 难度：⭐⭐ 中等

## 考点
- buffered channel 实现信号量
- 并发控制
- Acquire/Release 语义

## 题目描述

用 buffered channel 实现一个计数信号量，用于限制并发访问数。

要求：
1. `NewSemaphore(n)` — 创建允许最多 n 个并发的信号量
2. `Acquire()` — 获取一个许可，如果没有许可则阻塞
3. `TryAcquire(timeout)` — 尝试获取，超时返回 false
4. `Release()` — 释放一个许可
5. `Available()` — 返回当前可用许可数

## 函数签名

```go
type Semaphore struct { ... }

func NewSemaphore(n int) *Semaphore
func (s *Semaphore) Acquire()
func (s *Semaphore) TryAcquire(timeout time.Duration) bool
func (s *Semaphore) Release()
func (s *Semaphore) Available() int
```

## 提示
1. buffered channel 的容量就是信号量的计数
2. Acquire = 向 channel 发送（满了就阻塞）
3. Release = 从 channel 接收（释放一个位置）
4. Available = cap - len
