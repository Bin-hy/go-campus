# 调度器：先过滤（requests），再打分（资源均衡）

## 难度：⭐⭐ 中等

## 考点
- requests 是调度依据（limits 不参与调度，超了运行时才管）
- 过滤：节点剩余容量 >= Pod 的 requests（CPU + 内存都要满足）
- 打分：选剩余资源最多的节点（least-allocated，资源最均衡）

## 题目描述

实现 K8s 调度器的"过滤 + 打分"两阶段：

1. `Filter(pod, nodes)`：返回能容纳该 Pod 的节点列表——
   - 节点剩余容量 `Capacity - Allocated` 必须同时满足 CPU 和内存 requests
2. `Score(nodes)`：在过滤结果里打分，返回最优节点名——
   - 打分 = 剩余 CPU 占比 + 剩余内存占比（各 0~1 的浮点数），分数最高者胜出
   - 平局按名字字典序取小者；空列表返回空串

## 函数签名

```go
type Pod struct {
    CPUReq int // 单位：millicpu（千分之一核）
    MemReq int // 单位：MiB
}

type Node struct {
    Name                string
    CPUCap, MemCap      int // 总容量
    CPUAlloc, MemAlloc  int // 已分配
}

func Filter(pod Pod, nodes []Node) []Node
func Score(nodes []Node) string
```

## 提示

- 过滤条件：`CPUCap-CPUAlloc >= pod.CPUReq && MemCap-MemAlloc >= pod.MemReq`
- 剩余占比用 float64 计算：`(cap-alloc) / cap`
- 打分时先算完再比较，避免浮点误差干扰判断

## 运行测试

```bash
cd code/backend/07-k8s/02_scheduler && go test -v
```
