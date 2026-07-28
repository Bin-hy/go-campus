# 实现 sync.Once

## 难度：⭐⭐ 中等

## 考点
- atomic 原子操作
- 双重检查锁定（Double-Checked Locking）
- sync.Mutex 与 atomic 的配合

## 题目描述

实现一个简化版 sync.Once，保证传入的函数只执行一次，即使有多个 goroutine 并发调用。

要求：
1. `Do(f)` — 保证 f 只执行一次，并发安全
2. `Done()` — 返回 f 是否已经执行过
3. `Reset()` — 重置状态，允许下次 Do 再次执行

注意：标准库 sync.Once 没有 Reset，这里增加是为了加深理解。

## 函数签名

```go
type MyOnce struct { ... }

func (o *MyOnce) Do(f func())
func (o *MyOnce) Done() bool
func (o *MyOnce) Reset()
```

## 提示
1. 用 atomic.LoadUint32 做快速路径检查（fast path）
2. 慢速路径加 Mutex 保证只有一个 goroutine 执行 f
3. 执行完后用 atomic.StoreUint32 设置标志
4. 标准库实现：先 atomic 检查 → 加锁 → 再次检查 → 执行 → 设置标志
