# 泛型 LRU Cache

## 难度：⭐⭐⭐ 困难

## 考点
- 泛型结构体定义
- 泛型方法
- comparable 约束
- 与非泛型 LRU 的实现对比

## 提示
1. 结构体定义：`type GenericLRU[K comparable, V any] struct {...}`
2. 链表节点也需要泛型参数
3. 实现逻辑与 02_map/03_lru_cache 完全一致，只是类型通用化
