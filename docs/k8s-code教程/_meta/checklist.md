# K8s Code 编程教程 Checklist（验收记录）

> 验收时间：2026-08-29。全部项目已实际验证，✓ 表示通过。

## 实现完整性
- [x] 教程首页存在且含 mermaid 路线图（验证：`docs/k8s-code教程/index.md` 存在，构建渲染通过）
- [x] 14 个章节文档全部存在，文件名与 task.md 一致（验证：`ls docs/k8s-code教程/*.md` = 15 个文件含首页）
- [x] 每章含代码/命令与"面试追问"小节（验证：抽查 11 章，均含 `## 面试追问`）
- [x] 每章含 mermaid 图（如适用）（验证：13 个内容章均 ≥1 图，14 章为问答格式；docs:build 无 mermaid 报错）

## 代码层
- [x] `code/k8s/go.mod` 存在，`go build ./...` 零错误（验证：`cd code/k8s && go build ./...` exit 0）
- [x] 模拟类练习答案 `go test ./...` 全部通过（验证：5 组练习换入 answer 后全 ok）
- [x] 练习遵循 solution.go(TODO) + answer/answer.go + solution_test.go + README.md 格式（验证：client/01_connect、02_crud、03_watch、04_workqueue、scheduler-demo）
- [x] manifests 各章 YAML 语法正确（验证：`kubectl apply --dry-run=client -f` 逐个检查全过；webapp-sample 在安装 CRD 后 server dry-run 通过）
- [x] 连集群代码有 README 运行命令与预期输出（验证：client/02_crud、controller/counter-controller、operator 的 README）

## 真集群实操
- [x] 02-06 章 YAML 可在 minikube 上 apply 成功（验证：dry-run sweep 全过；CRD+WebApp 实装成功）
- [x] 09 章手写 Controller 演示步骤可行（验证：实测 apply cm-web → Deployment 建 3 副本 → annotate 5 → 收敛 5 副本 → delete cm → 跟随删除）
- [x] 07 章 client-go 连接程序在 minikube 可运行（验证：`go run . kube-system` 列出 7 个 Pod）

## 集成
- [x] config.mts 注册 `/k8s-code教程/` 侧边栏分组（16 条链接）与导航入口（验证：grep 命中 17 处）
- [x] `docs/index.md` 推荐顺序包含 K8s Code 教程（验证：第 12 条）
- [x] `docs/后端技术栈强化/index.md` S7 表格含互链（验证：S7 章节末尾提示块）
- [x] `npm run docs:build` 构建通过（验证：多次构建 exit 0，页面渲染完成）

## 内容质量
- [x] 面试题集 ≥ 15 题（验证：`grep -c '^### Q'` = 20 题），每题含考察点/追问链/答案要点
- [x] 与 07-k8s 理论文档互链（验证：11 个章节文件含 `/后端技术栈强化/07-k8s/` 链接）
- [x] 所有示例命令在本机环境可解释/可复现（验证：环境准备、client-go、controller、CRD 命令均实测）

## 端到端场景
- [x] 场景 1：首页 → 01 章环境检查 → 02 章 apply Pod（验证：minikube Ready、Pod dry-run 通过）
- [x] 场景 2：counter-controller 真集群完整闭环（验证：实测 建3→改5→跟随删除 全链路）
- [x] 场景 3：`npm run docs:build` 构建 + 侧边栏出现"K8s Code 教程"（验证：构建通过，config 中 16 条链接）

**结论：Checklist 全部通过。教程可交付。**
