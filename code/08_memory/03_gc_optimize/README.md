# GC 优化实战

## 难度：⭐⭐⭐ 困难

## 考点
- 减少堆分配的具体手法
- strings.Builder vs += 拼接
- sync.Pool 对象复用
- 预分配 vs 动态增长

## 提示
1. ProcessData：一次性 make([]int, len) 代替反复 append
2. ConcatStrings：用 strings.Builder
3. ObjectPool：sync.Pool + Get/Put 复用 buffer
4. 用 benchmark 的 -benchmem 验证分配次数减少
