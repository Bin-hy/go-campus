# LRU Cache

## 难度：⭐⭐⭐ 困难

## 考点
- HashMap + 双向链表实现 O(1) 的 Get/Put
- 链表节点的插入、删除、移动操作
- 容量淘汰策略

## 题目描述

实现一个 LRU（Least Recently Used）缓存，满足以下要求：
1. `Get(key)` — O(1) 获取 key 对应的 value，不存在返回 -1
2. `Put(key, value)` — O(1) 插入/更新 key-value，容量满时淘汰最久未使用的
3. 每次 Get 或 Put（已有的 key）都视为一次"使用"，需要更新到最新位置

这是字节面试最高频手撕题，务必熟练到 15 分钟内写完。

## 函数签名

```go
type LRUCache struct { ... }

func NewLRUCache(capacity int) *LRUCache
func (c *LRUCache) Get(key int) int
func (c *LRUCache) Put(key int, value int)
```

## 示例

```go
cache := NewLRUCache(2)
cache.Put(1, 1)
cache.Put(2, 2)
cache.Get(1)      // 返回 1
cache.Put(3, 3)   // 淘汰 key=2
cache.Get(2)      // 返回 -1（已淘汰）
cache.Put(4, 4)   // 淘汰 key=1? 不对，key=1 刚被 Get 过，应淘汰 key=3
cache.Get(1)      // 返回 1
cache.Get(3)      // 返回 -1
cache.Get(4)      // 返回 4
```

## 提示
1. 双向链表头部是最新的，尾部是最旧的（或反过来，保持一致即可）
2. 用哨兵节点（dummy head/tail）简化边界处理
3. map 存储 key → 链表节点指针，实现 O(1) 定位
4. 核心操作只有三个：moveToFront、removeNode、addToFront
