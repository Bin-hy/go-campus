# 并发安全 Map

## 难度：⭐⭐⭐ 困难

## 考点
- map 并发不安全的原因
- 用 sync.RWMutex 保护 map
- 读写锁与互斥锁的选择

## 题目描述

实现一个并发安全的泛型 Map，支持基本的 CRUD 操作。

要求：
1. 支持 Set、Get、Delete、Len 操作
2. Get 操作返回 (value, bool)，bool 表示 key 是否存在
3. 使用读写锁（RWMutex）优化读多写少场景
4. 额外实现 Range 方法，安全地遍历所有 kv 对

## 函数签名

```go
type SafeMap struct { ... }

func NewSafeMap() *SafeMap
func (m *SafeMap) Set(key string, value int)
func (m *SafeMap) Get(key string) (int, bool)
func (m *SafeMap) Delete(key string)
func (m *SafeMap) Len() int
func (m *SafeMap) Range(fn func(key string, value int) bool)
```

## 提示
1. RWMutex：读操作用 RLock/RUnlock，写操作用 Lock/Unlock
2. Range 回调返回 false 时应停止遍历
3. Range 遍历期间持有读锁，回调中不要调用写操作（会死锁）
