# Operator：webapp-operator（controller-runtime 手写版）

> 教程第 10 章配套代码。展示"自定义资源（CRD）+ 控制器"的完整 Operator 开发模式，**不用 kubebuilder 脚手架**，手写 Reconciler，便于理解本质。

## 架构

```mermaid
flowchart LR
    CR[WebApp 自定义资源<br/>apps.example.com/v1] -->|For + Owns| R[Reconciler]
    R -->|CreateOrUpdate| D[Deployment]
    D -->|Owns 级联触发| R
    R -->|Status().Update| CR
```

- `For(&WebApp{})`：WebApp 任何变化触发 Reconcile
- `Owns(&Deployment{})`：本控制器创建的 Deployment 变化也触发（级联）
- `CreateOrUpdate`：期望状态写进 Deployment，不存在就创建、存在就更新（controller-runtime 的声明式"补差距"）

## 文件

```
operator/
├── main.go               # manager + Reconciler + 注册
└── api/v1/webapp_types.go # WebApp 类型定义（与 CRD 对应，手写 DeepCopy/AddToScheme）
```

## 运行

```bash
# 1. 安装 CRD + 示例资源
kubectl apply -f ../manifests/10_crd/crd-webapp.yaml
kubectl apply -f ../manifests/10_crd/webapp-sample.yaml

# 2. 启动 operator（本机跑，等价于集群内运行）
cd code/k8s/operator
go run .

# 3. 另开终端观察
kubectl get webapp,deploy,pods -l app=myapp -w
# 修改 WebApp 副本数，观察 Deployment 自动收敛
kubectl patch webapp myapp --type=merge -p '{"spec":{"replicas":5}}'
kubectl get webapp myapp -o yaml   # status.readyReplicas 被回写
```

## 与"手写 client-go Controller"（09 章）的区别

| | 09 章 client-go 控制器 | 本章 controller-runtime |
|---|---|---|
| 监听对象 | 手动 Informer + Workqueue | `For/Owns` 声明式注册 |
| 收敛动作 | 手写 Get/Update | `CreateOrUpdate` |
| 更新冲突 | 直接 Update | 自动处理（内部用 Patch 语义） |
| 适合 | 理解原理、轻量场景 | 生产 Operator、多资源编排 |

## 面试要点

1. **为什么 CRD 里声明 `status` 子资源？** 让控制器只能通过 `/status` 更新状态，防止普通用户/控制器覆盖彼此，也是 apiserver 校验的边界。
2. **为什么用 `Owns` 而不手动 Watch Deployment？** Owns 自动建立 ownerReference 链，资源删除级联清理（GC），且只关心"自己管的"资源。
3. **`CreateOrUpdate` 内部怎么保证幂等？** 先 Get，存在则 mutate 后 Update，不存在则 Create；错误时返回给 reconciler 重试。
4. **Operator 比普通脚本强在哪？** 声明式 + 事件驱动 + 自动重试 + 与集群状态闭环，天然具备自愈能力。
