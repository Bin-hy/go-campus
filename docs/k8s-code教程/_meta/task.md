# K8s Code 编程教程 Tasks

> 依据：已批准的 spec.md + plan.md。按序执行，每任务带验证。

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `code/k8s/go.mod` | 模块 gocampus/k8s，依赖 client-go v0.35.x |
| 新建 | `docs/k8s-code教程/index.md` | 教程首页（mermaid 路线图 + 使用说明 + 章节导航） |
| 新建 | `docs/k8s-code教程/01-环境准备与kubectl速查.md` | 环境/context/kubectl 速查 |
| 新建 | `docs/k8s-code教程/02-Pod与容器生命周期.md` | Pod/探针/initContainer |
| 新建 | `docs/k8s-code教程/03-Deployment与滚动发布.md` | Deployment/RS/滚动更新回滚 |
| 新建 | `docs/k8s-code教程/04-Service-Ingress与DNS.md` | Service/Ingress/DNS |
| 新建 | `docs/k8s-code教程/05-配置与存储-ConfigMap-Secret-PV.md` | ConfigMap/Secret/PV/PVC/StorageClass |
| 新建 | `docs/k8s-code教程/06-资源管理与HPA-Job.md` | requests/limits/HPA/Job/CronJob |
| 新建 | `docs/k8s-code教程/07-client-go编程-连接与CRUD.md` | client-go 连接/CRUD |
| 新建 | `docs/k8s-code教程/08-client-go编程-Informer与Workqueue.md` | Watch/Informer/Workqueue |
| 新建 | `docs/k8s-code教程/09-手写Controller实战.md` | 手写最小控制器 |
| 新建 | `docs/k8s-code教程/10-CRD与Operator开发.md` | CRD/controller-runtime Reconciler |
| 新建 | `docs/k8s-code教程/11-调度器深入.md` | 调度流程/亲和性/污点容忍 |
| 新建 | `docs/k8s-code教程/12-网络与安全深入.md` | NetworkPolicy/RBAC/ServiceAccount |
| 新建 | `docs/k8s-code教程/13-故障排查与生产实践.md` | 排障剧本/优雅退出/可观测性 |
| 新建 | `docs/k8s-code教程/14-面试题集.md` | 15+ 高频题 |
| 新建 | `code/k8s/manifests/*` | 各章 YAML（按章分子目录） |
| 新建 | `code/k8s/client/01_connect/` | 连接+列出 Pod（solution/answer/test/README） |
| 新建 | `code/k8s/client/02_crud/` | typed client CRUD Deployment |
| 新建 | `code/k8s/client/03_watch/` | Watch/Informer 示例 |
| 新建 | `code/k8s/client/04_workqueue/` | Workqueue 模拟（可 go test） |
| 新建 | `code/k8s/controller/counter-controller/` | 手写控制器（Informer+Workqueue+Reconcile） |
| 新建 | `code/k8s/operator/` | CRD YAML + Reconciler 骨架 |
| 新建 | `code/k8s/scheduler-demo/` | 调度打分模拟（可 go test） |
| 修改 | `docs/.vitepress/config.mts` | 注册侧边栏/导航 |
| 修改 | `docs/index.md` | 推荐顺序补 K8s Code 教程 |
| 修改 | `docs/后端技术栈强化/index.md` | S7 表格互链 |

## T1: 初始化 code/k8s 模块

**文件：** `code/k8s/go.mod`
**依赖：** 无
**步骤：**
1. `mkdir -p code/k8s`，`go mod init gocampus/k8s`
2. `GOPROXY=https://goproxy.cn,direct go get k8s.io/client-go@v0.35.1 k8s.io/apimachinery@v0.35.1 k8s.io/api@v0.35.1 k8s.io/klog/v2`
3. 建 manifests 子目录骨架

**验证：** `go build ./...` 通过（空模块）

## T2: 教程首页 index.md

**文件：** `docs/k8s-code教程/index.md`
**依赖：** T1（引用 code 路径）
**步骤：**
1. mermaid 路线图（14 章线性 + 进阶分支）
2. 使用说明（需要 minikube/kubectl/go；GOPROXY 提示）
3. 章节表格导航（每章：内容/代码位置/验收）

**验证：** `npm run docs:build` 该页渲染无报错

## T3: 第 01 章 环境准备

**文件：** `docs/k8s-code教程/01-环境准备与kubectl速查.md`
**依赖：** T2
**步骤：** 集群状态核对命令、kubeconfig/context 讲解（mermaid）、kubectl 命令分类速查表、常见坑
**验证：** 文档中的命令在本机执行输出正确（kubectl get nodes 等）

## T4: 第 02 章 Pod 与容器生命周期

**文件：** `docs/k8s-code教程/02-Pod与容器生命周期.md` + `code/k8s/manifests/02_pod/*.yaml`
**依赖：** T3
**步骤：**
1. Pod YAML 逐字段讲解；探针（liveness/readiness/startup）mermaid 状态图
2. manifests：pod-basic、probe、initContainer 三个 YAML
3. 练习：探针判定模拟（可并入 client 或独立模拟，go test）

**验证：** `kubectl apply -f manifests/02_pod/` 成功；`kubectl get pod` 状态符合预期

## T5: 第 03 章 Deployment 与滚动发布

**文件：** `docs/k8s-code教程/03-Deployment与滚动发布.md` + `manifests/03_deployment/*.yaml`
**依赖：** T4
**步骤：** Deployment/RS 层级 mermaid、滚动更新参数、回滚实操 YAML
**验证：** apply 后 rollout status 成功；模拟练习 `go test`（如含）

## T6: 第 04 章 Service/Ingress/DNS

**文件：** `docs/k8s-code教程/04-Service-Ingress与DNS.md` + `manifests/04_service/*.yaml`
**依赖：** T5
**步骤：** ClusterIP/NodePort/Headless/Ingress/DNS 讲解 + YAML 实操
**验证：** NodePort 访问通；CoreDNS 解析 `服务名.命名空间.svc` 成功

## T7: 第 05 章 配置与存储

**文件：** `docs/k8s-code教程/05-配置与存储-ConfigMap-Secret-PV.md` + `manifests/05_config_storage/*.yaml`
**依赖：** T6
**步骤：** ConfigMap/Secret 注入方式、PV/PVC/StorageClass 动态供给实操（minikube 自带 default-storageclass）
**验证：** PVC Bound；容器内读到配置/挂载文件

## T8: 第 06 章 资源管理与 HPA/Job

**文件：** `docs/k8s-code教程/06-资源管理与HPA-Job.md` + `manifests/06_hpa_job/*.yaml`
**依赖：** T7
**步骤：** requests/limits 与 QoS（mermaid）、HPA 实操（需 metrics-server）、Job/CronJob
**验证：** HPA 状态 OK；Job 完成（Succeeded）

## T9: 第 07 章 client-go 连接与 CRUD

**文件：** `docs/k8s-code教程/07-client-go编程-连接与CRUD.md` + `code/k8s/client/01_connect/`、`02_crud/`
**依赖：** T1
**步骤：**
1. 01_connect：rest.InClusterConfig vs clientcmd，列出 Pod（solution/answer/test：mock 或纯编译）
2. 02_crud：Create/Get/Update/Delete Deployment，README 给 `go run` 命令
3. 练习测试：用 fake clientset 或纯逻辑测试

**验证：** `go build ./...`；`go test ./client/...` 通过；README 命令在 minikube 可跑

## T10: 第 08 章 Informer 与 Workqueue

**文件：** `docs/k8s-code教程/08-client-go编程-Informer与Workqueue.md` + `client/03_watch/`、`04_workqueue/`
**依赖：** T9
**步骤：**
1. 03_watch：Watch 事件流 + SharedInformer 示例（含 cache 同步）
2. 04_workqueue：RateLimitingQueue 模拟练习（可 go test，不连集群）
3. mermaid：Informer 数据流图

**验证：** `go test ./client/...` 通过

## T11: 第 09 章 手写 Controller

**文件：** `docs/k8s-code教程/09-手写Controller实战.md` + `controller/counter-controller/`
**依赖：** T10
**步骤：**
1. 设计：监听 ConfigMap 注解 `counter.example.com/desired` → 调整同 namespace Deployment 副本数
2. 实现 main.go（Informer + Workqueue + Reconcile 函数）
3. README：真集群演示步骤

**验证：** `go build ./...`；README 演示步骤在 minikube 可跑（改注解 → 副本数变化）

## T12: 第 10 章 CRD 与 Operator

**文件：** `docs/k8s-code教程/10-CRD与Operator开发.md` + `operator/`
**依赖：** T11
**步骤：**
1. CRD YAML（apps.example.com/v1 WebApp，含 status 子资源）
2. Reconciler 骨架（controller-runtime 手写，编译通过即可，不强制运行）
3. mermaid：Operator 控制循环

**验证：** CRD apply 成功（`kubectl get crd`）；`go build ./...` 通过

## T13: 第 11 章 调度器深入

**文件：** `docs/k8s-code教程/11-调度器深入.md` + `manifests/11_scheduler/` + `scheduler-demo/`
**依赖：** T8
**步骤：**
1. 调度流程 mermaid；nodeSelector/亲和性/污点容忍实操 YAML
2. scheduler-demo：过滤+打分模拟（可 go test）

**验证：** 污点/容忍 YAML 生效（Pod Pending 原因可解释）；`go test ./scheduler-demo/...`

## T14: 第 12 章 网络与安全深入

**文件：** `docs/k8s-code教程/12-网络与安全深入.md` + `manifests/12_network/`
**依赖：** T13
**步骤：** NetworkPolicy 实操（minikube 默认 CNI 支持情况说明）、RBAC/ServiceAccount/ClusterRole 示例、Secret 安全
**验证：** NetworkPolicy 拒绝/放行可观察（minikube 默认需说明或跳过）；RBAC YAML apply 成功

## T15: 第 13 章 故障排查与生产实践

**文件：** `docs/k8s-code教程/13-故障排查与生产实践.md`
**依赖：** T14
**步骤：** 故障→命令→结论剧本表（CrashLoopBackOff/ImagePullBackOff/Pending/OOMKilled/Evicted）、优雅退出、事件与日志、metrics 简介
**验证：** 每个剧本在本机可复现或可解释

## T16: 第 14 章 面试题集

**文件：** `docs/k8s-code教程/14-面试题集.md`
**依赖：** 全部章节
**步骤：** 15+ 题：考察点/追问链/答案要点；与 07-k8s/面试题集 互链
**验证：** 题数 ≥ 15；每题三要素齐全

## T17: 站点集成

**文件：** `docs/.vitepress/config.mts`、`docs/index.md`、`docs/后端技术栈强化/index.md`
**依赖：** T2-T16
**步骤：**
1. config.mts：sidebar 增加 `/k8s-code教程/` 分组（14 条）；nav 加入口
2. index.md 推荐顺序补一条
3. 后端技术栈强化/index.md S7 表格加互链行

**验证：** `npm run docs:build` 通过；本地预览侧边栏/导航正常

## T18: 收尾验收

**依赖：** T17
**步骤：** 对照 checklist.md 逐项验证；修复问题
**验证：** checklist 全部通过

## 执行顺序

```
T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8
                  ↘（T9 依赖 T1，可并行于 T4-T8）
T9 → T10 → T11 → T12
T13（依赖 T8）→ T14 → T15 → T16（依赖前面全部）
T17 → T18
```
