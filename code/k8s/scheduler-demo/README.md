# 练习：调度器过滤 + 打分模拟

## 考点
- K8s 调度两阶段：**Filter（过滤，硬约束）→ Score（打分，软偏好）**
- 硬约束：资源足够（requests）、GPU 需求、污点容忍
- 软偏好：资源均衡 / 节点亲和加分

## 题目
1. `Filter`：返回满足硬约束的候选节点
2. `Score`：剩余资源越多分越高（CPU + 内存 + 偏好分）
3. `Schedule`：Filter → Score 组合

## 运行测试

```bash
cd code/k8s/scheduler-demo
go test -v
```

## 与真实调度器的对应

| 模拟 | 真实 kube-scheduler |
|------|---------------------|
| Filter | 预选阶段（Predicates）：NodeResourcesFit、NodeSelector、TaintToleration… |
| Score | 优选阶段（Priorities）：LeastRequestedPriority、NodeAffinity、ImageLocality… |
| Schedule | 选定 Node 后写入 `spec.nodeName`，kubelet 看到后创建容器 |

## 面试追问
- 为什么"先过滤再打分"而不是直接打分？→ 打分基于软偏好，可能把 Pod 放到不满足硬约束的节点，必须先排除。
- requests 和实际使用的区别？→ 调度看 requests（预留），运行时可能超用（受 limits 约束），所以打分通常看"剩余可分配 - 新 Pod requests"。
