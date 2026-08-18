# 滚动更新：maxUnavailable / maxSurge 约束下的发布计划

## 难度：⭐⭐⭐ 困难

## 考点
- Deployment 滚动更新状态机
- maxUnavailable：更新过程中最多允许几个旧实例不可用（可用数 >= desired - maxUnavailable）
- maxSurge：更新过程中最多允许超出目标副本数几个实例（总数 <= desired + maxSurge）
- 生成一轮轮"先起新、再删旧"的动作计划

## 题目描述

实现 Deployment 滚动更新的计划生成器：给定目标副本数、当前 v1 数量，以及 maxUnavailable / maxSurge，输出一轮轮动作，每轮先创建 v2、再删除 v1，直到全部换成 v2。

状态建模：`v1` = 旧版本 Pod 数，`v2` = 新版本 Pod 数。每一步都必须满足：
- 总数 `v1+v2 <= desired + maxSurge`
- 可用数（简化模型里就是总数）`v1+v2 >= desired - maxUnavailable`

每轮动作：
1. 创建 k 个 v2：`k = min(desired - v2, desired + maxSurge - (v1+v2))`（不超过目标数、不超 surge 上限）
2. 删除 m 个 v1：`m = min(v1, (v1+v2) - (desired - maxUnavailable))`（不超当前 v1、不低于可用下限）

> 注：k8s 要求 maxUnavailable 与 maxSurge 不能同时为 0（API 校验会拒绝），本题不测该非法输入。

## 函数签名

```go
type Action struct {
    Op      string // "create" | "delete"
    Version string // "v2" | "v1"
}

func RollingUpdatePlan(desired, maxUnavailable, maxSurge, v1Count int) [][]Action
```

## 提示

- 循环条件：`v1 > 0 || v2 < desired`；每轮生成完动作后更新 v1/v2 计数，再进入下一轮
- `v1Count = 0` 表示纯扩容（只有创建动作）；`desired = 0` 表示缩容到零（只有删除动作）
- 为避免非法输入死循环，可加"本轮无动作则 break"的兜底

## 运行测试

```bash
cd code/backend/07-k8s/03_rolling_update && go test -v
```
