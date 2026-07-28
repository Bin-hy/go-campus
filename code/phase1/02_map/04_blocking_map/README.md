# 阻塞读 Map

## 难度：⭐⭐⭐ 困难

## 考点
- channel 与 map 结合
- 阻塞等待某个 key 被写入
- 超时控制

## 题目描述

实现一个支持阻塞读的并发安全 Map。当 Get 某个不存在的 key 时，调用者会阻塞等待，
直到有其他 goroutine Put 了该 key，或者超时。

这是字节跳动面试真题，考察 channel + map + context 的综合使用。

要求：
1. `Put(key, value)` — 设置值，如果有阻塞等待该 key 的 goroutine，通知它们
2. `Get(key, timeout)` — 如果 key 存在直接返回；不存在则阻塞等待，超时返回错误
3. 并发安全

## 函数签名

```go
type BlockingMap struct { ... }

func NewBlockingMap() *BlockingMap
func (m *BlockingMap) Put(key string, value interface{})
func (m *BlockingMap) Get(key string, timeout time.Duration) (interface{}, error)
```

## 提示
1. 每个被等待的 key 关联一个 channel（或 channel 列表）
2. Put 时检查是否有等待者，通过 close channel 或 send 通知
3. Get 时如果 key 不存在，创建/获取该 key 的等待 channel，然后 select 等待
4. 注意：多个 goroutine 可能同时等待同一个 key
