# LRU 缓存

## 难度：⭐⭐⭐ 面试超高频

## 考点
- 哈希表 + 双向链表
- 数据结构设计能力
- 字节面试必考题（出现率极高）

## 题目描述

设计并实现一个满足 LRU（最近最少使用）缓存约束的数据结构。

实现 `LRUCache` 结构体：
- `NewLRUCache(capacity int) *LRUCache` — 以正整数作为容量初始化 LRU 缓存
- `Get(key int) int` — 如果 key 存在，返回 value；否则返回 -1
- `Put(key int, value int)` — 如果 key 已存在，更新 value；如果不存在，插入。当缓存容量达到上限时，在插入前淘汰最久未使用的 key

**Get 和 Put 必须以 O(1) 平均时间复杂度运行。**

## 函数签名

```go
type LRUCache struct {
    // 你的字段
}

func NewLRUCache(capacity int) *LRUCache
func (c *LRUCache) Get(key int) int
func (c *LRUCache) Put(key, value int)
```

## 示例

```
cache := NewLRUCache(2)
cache.Put(1, 1)     // 缓存：{1=1}
cache.Put(2, 2)     // 缓存：{1=1, 2=2}
cache.Get(1)        // 返回 1，缓存：{2=2, 1=1}（1 变成最近使用）
cache.Put(3, 3)     // 容量已满，淘汰 key=2，缓存：{1=1, 3=3}
cache.Get(2)        // 返回 -1（不存在）
cache.Put(4, 4)     // 容量已满，淘汰 key=1，缓存：{3=3, 4=4}
cache.Get(1)        // 返回 -1
cache.Get(3)        // 返回 3
cache.Get(4)        // 返回 4
```

## 要求
1. Get 和 Put 均为 O(1) 时间复杂度
2. 需要自己定义双向链表节点（不使用 container/list）

## 提示
1. 哈希表 map[int]*Node 提供 O(1) 查找
2. 双向链表维护访问顺序：头部是最近使用，尾部是最久未使用
3. 使用 dummy head 和 dummy tail 简化边界处理
4. Get 时将节点移到头部
5. Put 时：已存在则更新+移到头部；不存在则新建+加到头部，超容量则删尾部
