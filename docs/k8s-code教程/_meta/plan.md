# K8s Code 编程教程 Plan

> 依据：已批准的 `spec.md`。本文档定义教程的章节结构、代码目录与关键技术决策。

## 架构概览

教程 = **文档层（docs/k8s-code教程/）+ 代码层（code/k8s/）+ 站点集成（config.mts 侧边栏/导航）** 三部分，互相引用：

```
docs/k8s-code教程/
├── index.md            # 教程首页：路线图（mermaid）、怎么用、章节导航
├── 01-环境准备与kubectl速查.md
├── 02-Pod与容器生命周期.md
├── 03-Deployment与滚动发布.md
├── 04-Service-Ingress与DNS.md
├── 05-配置与存储-ConfigMap-Secret-PV.md
├── 06-资源管理与HPA-Job.md
├── 07-client-go编程-连接与CRUD.md
├── 08-client-go编程-Informer与Workqueue.md
├── 09-手写Controller实战.md
├── 10-CRD与Operator开发.md
├── 11-调度器深入.md
├── 12-网络与安全深入.md
├── 13-故障排查与生产实践.md
├── 14-面试题集.md
└── _meta/              # spec/plan/task/checklist（不参与导航）
```

```
code/k8s/
├── go.mod              # module gocampus/k8s；依赖 client-go v0.35.x（匹配 minikube v1.35.1）
├── manifests/          # 每章可 apply 的 YAML（按章分子目录）
│   ├── 02_pod/ 03_deployment/ 04_service/ 05_config_storage/ 06_hpa_job/
│   ├── 09_controller/ 10_crd/ 11_scheduler/ 12_network/
├── client/             # 07-08 章：client-go 编程练习
│   ├── 01_connect/     # 连接集群 + 列出 Pod（solution.go + answer + test）
│   ├── 02_crud/        # typed client 增删改查 Deployment
│   ├── 03_watch/       # Watch + Informer 最小示例
│   └── 04_workqueue/   # Workqueue 模式（模拟版可 go test）
├── controller/         # 09 章：手写 Controller（真集群）
│   └── counter-controller/   # 监听自定义 ConfigMap 注解 → 维护 Deployment 副本数
├── operator/           # 10 章：CRD + controller-runtime 风格 Operator 骨架
│   └── ...
└── scheduler-demo/     # 11 章：调度器扩展点演示（可独立 go test 的模拟）
```

## 核心章节设计

| 章 | 标题 | 本章产出（可验收） | 代码位置 |
|----|------|-------------------|---------|
| 01 | 环境准备与 kubectl 速查 | 核对集群状态、kubeconfig/context、kubectl 常用命令清单 | manifests/01 |
| 02 | Pod 与容器生命周期 | Pod YAML、探针、initContainer 跑通；go test 探针判定模拟 | manifests/02_pod + client 或模拟 |
| 03 | Deployment 与滚动发布 | 滚动更新/回滚实操；go test 模拟滚动更新节奏 | manifests/03_deployment |
| 04 | Service/Ingress/DNS | ClusterIP/NodePort/Ingress 实操；go test Service 转发选择 | manifests/04_service |
| 05 | 配置与存储 | ConfigMap/Secret/PV/PVC/StorageClass 实操 | manifests/05_config_storage |
| 06 | 资源管理与 HPA/Job | requests/limits、HPA、Job/CronJob 实操；复用现有 HPA 模拟 | manifests/06_hpa_job |
| 07 | client-go：连接与 CRUD | 真集群列出/创建/删除 Deployment 的 Go 程序 | client/01_connect、02_crud |
| 08 | client-go：Informer 与 Workqueue | 真集群 Watch 事件；Workqueue 模拟 go test | client/03_watch、04_workqueue |
| 09 | 手写 Controller | 真集群最小控制器（Informer+Workqueue+Reconcile）跑通 | controller/counter-controller |
| 10 | CRD 与 Operator | CRD apply 成功；controller-runtime 骨架 Reconciler 编译通过 | operator/ |
| 11 | 调度器深入 | 亲和性/污点容忍实操；调度打分模拟 go test | manifests/11_scheduler + scheduler-demo |
| 12 | 网络与安全深入 | NetworkPolicy/RBAC/ServiceAccount 实操 | manifests/12_network |
| 13 | 故障排查与生产实践 | 排障剧本（常见故障→命令→结论） | manifests（故障注入示例） |
| 14 | 面试题集 | 15+ 题：考察点/追问链/答案要点 | — |

## 模块交互

```
读者路径：docs 章节（概念 + mermaid）→ 代码层（manifests apply / go run / go test）→ 章节练习 → 面试题集
站点集成：config.mts 增加 '/k8s-code教程/' 侧边栏分组 + 导航项；index.md 与 07-k8s 互链
```

## 关键技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| client-go 版本 | v0.35.1（与集群 v1.35.1 同大版本） | 避免 discovery/版本不匹配 |
| Go 模块代理 | GOPROXY=goproxy.cn,direct（仓库 README/练习文档注明） | 本机 proxy.golang.org 超时，goproxy.cn 可用 |
| 练习格式 | 沿用仓库惯例：`solution.go`(TODO) + `answer/answer.go` + `solution_test.go` + `README.md` | 与现有 code/backend 一致，.gitignore 已忽略 solution.go |
| 真集群代码的测试策略 | 模拟逻辑可 go test；连集群代码用 `go run ./cmd` + README 给预期输出 | 测试不依赖集群在线，实操由用户手动运行 |
| Operator 方案 | controller-runtime 手写 Reconciler（不强制 kubebuilder 脚手架，避免 codegen 依赖） | 教学清晰、依赖少、可控 |
| mermaid 使用 | vitepress-plugin-mermaid 已配置，章节图直接用 mermaid 代码块 | 与 CLAUDE.md 规则一致 |
| 与 S7 理论文档关系 | 互链不复制：理论深度引用 07-k8s 各篇 | 避免内容重复 |

## 章节导航与集成

- 侧边栏新增 `/k8s-code教程/` 分组（1 个分组，14 个条目）。
- 顶部导航"代码练习/知识"下加"K8s Code 教程"入口。
- `docs/index.md` 推荐顺序补充第 12 条：学习 K8s Code 教程。
- `docs/后端技术栈强化/index.md` S7 表格追加互链提示。
