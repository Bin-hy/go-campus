# 控制器模式：HPA 算期望，Reconcile 收敛

## 难度：⭐⭐ 中等

## 考点
- 声明式 API：期望状态 vs 实际状态
- 控制循环（Reconcile Loop）：有差距就补齐
- HPA：按 CPU 利用率计算期望副本数（带 min/max 值域）

## 题目描述

K8s 控制器的内核是控制循环：不停对比"期望状态"和"实际状态"，有差距就执行动作补齐。实现一个简化版控制器，分两步：

1. `DesiredReplicas`：模拟 HPA 计算期望副本数——`desired = ceil(currentReplicas * avgCPU / targetUtil)`，结果钳制在 `[minReplicas, maxReplicas]`；`avgCPU` 为 0 视为无压力，直接返回 `minReplicas`。
2. `Reconcile`：给出让实际状态收敛到期望状态的动作列表——
   - 现有 Pod 数 < desired：补建缺失数量（`create`，名字用 `pod-{最大编号+1}` 递增，避免冲突）
   - 现有 Pod 数 > desired：缩容（`delete`，先删最老的，即 Age 最小的；Age 相同按名字字典序）
   - 相等：返回空列表

## 函数签名

```go
type Pod struct {
    Name string
    Age  int // 存活秒数
}

type Action struct {
    Op   string // "create" | "delete"
    Name string // create 时为新 Pod 名，delete 时为目标 Pod 名
}

func DesiredReplicas(currentReplicas, avgCPU, targetUtil, minReplicas, maxReplicas int) int
func Reconcile(desired int, pods []Pod) []Action
```

## 提示

- 向上取整的除法：`(currentReplicas*avgCPU + targetUtil - 1) / targetUtil`
- clamp：先算裸值，再与 min/max 比较收窄
- `Reconcile` 创建的名字要避免与现有 Pod 冲突：解析现有名字里最大的数字后缀，从 +1 开始

## 运行测试

```bash
cd code/backend/07-k8s/01_reconcile && go test -v
```
