# K8s Code 编程教程 Spec

## 背景

GoCampus 仓库已有一套 K8s 理论文档（`docs/后端技术栈强化/07-k8s/`，偏面试与原理讲解）和纯 Go 模拟练习（`code/backend/07-k8s/`）。用户是 Go 开发者，目标是字节跳动 AI 应用开发实习，需要把 K8s 从"会用"提升到"能编程、能手写 Controller/Operator"的大师级水平。本机环境：Go 1.26.2、Docker、kubectl v1.34.1、minikube v1.38.1（运行中，K8s v1.35.1，单节点）、Go 模块代理可用（goproxy.cn）。

需求：新建一套**独立的 K8s Code 编程教程**——以"写代码 + 真集群实操"为主线，配合 mermaid 图与可运行练习，让用户完完全全成为 K8s 大师。

## 目标

- 提供一条从零到大师的 K8s 学习主线：**环境 → 核心对象 YAML → client-go 编程 → 手写 Controller → CRD/Operator → 调度/网络/存储深入 → 排障与生产实践 → 面试题集**。
- 每章都"有代码可写、有命令可跑、有练习可验"，代码在用户本机 minikube 上真实运行。
- 与现有 S7 理论文档互补互链：理论文档讲"为什么"，本教程讲"怎么写、怎么跑"。
- 覆盖 Go 工程师面试 K8s 高频考点（控制循环、Informer/Workqueue、Operator、调度、探针、RBAC 等）。

## 功能需求

- F1 环境准备章：minikube/kubectl 状态检查、kubeconfig/context、常用 kubectl 命令速查、集群信息核对，带可执行命令清单。
- F2 核心对象 YAML 章（3-4 章）：Pod（生命周期/探针/initContainer）、Deployment/ReplicaSet（滚动更新/回滚）、Service/Ingress/DNS、ConfigMap/Secret、PV/PVC/StorageClass、HPA、Job/CronJob；每节配可直接 apply 的 YAML 与预期输出。
- F3 client-go 编程章（2-3 章）：连接集群（in-cluster vs kubeconfig）、typed client CRUD、LabelSelector、Watch 与 Informer、DeltaFIFO/Workqueue 概念与代码。
- F4 手写 Controller 章：基于 client-go 从零写一个最小控制器（Informer + Workqueue + Reconcile），跑在真实集群上实现"监听自定义资源 → 收敛期望状态"。
- F5 CRD 与 Operator 章：定义 CRD、用 controller-runtime/kubebuilder 思路手写或脚手架搭建 Operator、Admission Webhook 概念与最小示例。
- F6 调度/网络/存储深入章：调度器流程与调度器扩展点、亲和性/污点容忍、requests/limits 与 QoS、CNI 与 Service 底层（iptables/IPVS 概念）、NetworkPolicy、CSI 与动态供给；配真实集群可观察的演示。
- F7 排障与生产实践章：探针与优雅退出、资源不足/OOM/ImagePullBackOff 等典型故障的 kubectl 排查剧本、日志与事件、可观测性（metrics）简介。
- F8 面试题集章：15+ 高频面试题，每题含"考察点 / 追问链 / 参考答案要点"，与现有 07-k8s/面试题集互补。
- F9 代码配套：`code/k8s/` 下每章有可运行 Go 代码（client-go 为主）或 YAML 清单，遵循仓库既有练习格式（`solution.go` 含 TODO + `answer/answer.go` 参考答案 + `solution_test.go` 测试 + `README.md`），`go test` 可验证（模拟部分）或连真集群运行（实操部分）。
- F10 集成：教程首页 + 章节索引页；在 `docs/.vitepress/config.mts` 注册侧边栏与导航入口；与 `docs/后端技术栈强化/07-k8s/`、`docs/后端技术栈强化/index.md`（S7 表格）互链。
- F11 图表：关键流程用 mermaid 图（架构图、控制循环、Informer 数据流、滚动更新、请求链路等）。

## 非功能需求

- N1 可运行性：所有 Go 代码在本机真实编译通过；连集群的代码在本机 minikube 上可跑（内网无需外网依赖，仅依赖已缓存/可拉取的 Go 模块）。
- N2 风格一致：沿用仓库文档风格——中文、口语化引入、比喻、章节末尾"串起来"总结、每章带练习与"面试追问"。
- N3 渐进难度：每章有明确的"前置知识 / 本章产出 / 验收动作"。
- N4 体积可控：章节正文每章 ≤ 600 行 markdown；代码文件保持最小可运行（不引入无关依赖）。

## 不做的事

- 不部署多节点生产集群、不涉及云厂商托管服务细节。
- 不重写 kube-scheduler/kubelet 源码级内容（只到"原理 + 扩展点"深度）。
- 不展开 Service Mesh（Istio）全量内容（仅提及与 Service 的关系）。
- 不覆盖 Windows 节点、边缘计算、多集群联邦等小众场景。
- 不在教程内引入收费服务或需外网注册的组件。

## 验收标准

- AC1 `docs/k8s-code教程/` 存在：首页 + 章节 ≥ 12 篇（含面试题集），每篇含 mermaid 图（如适用）与代码/命令，风格与仓库一致。
- AC2 `code/k8s/` 存在：模拟类练习 `go test ./...` 全部通过；实操类代码在 minikube 上可运行（提供运行命令与预期输出）。
- AC3 `docs/.vitepress/config.mts` 注册"K8s Code 教程"侧边栏/导航；与 `后端技术栈强化/07-k8s/`、`后端技术栈强化/index.md` 互链；`docs/index.md` 推荐顺序提及新教程。
- AC4 面试题集 ≥ 15 题，每题含考察点/追问链/答案要点。
- AC5 `npm run docs:build` 构建通过（mermaid 渲染无报错）。
