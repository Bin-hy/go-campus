# K8s Code 编程教程 Checklist

> 每一项通过运行命令或观察行为验证，聚焦系统行为。

## 实现完整性
- [ ] 教程首页存在且含 mermaid 路线图（验证：`docs/k8s-code教程/index.md` 存在，构建后页面渲染）
- [ ] 14 个章节文档全部存在，文件名与 task.md 一致（验证：`ls docs/k8s-code教程/*.md`）
- [ ] 每章含代码/命令与"面试追问"小节（验证：抽查 3 章）
- [ ] 每章含 mermaid 图（如适用）（验证：抽查含流程的章节，构建无 mermaid 报错）

## 代码层
- [ ] `code/k8s/go.mod` 存在，`go build ./...` 零错误（验证：`cd code/k8s && go build ./...`）
- [ ] 模拟类练习 `go test ./...` 全部通过（验证：`cd code/k8s && go test ./...`）
- [ ] 练习遵循 solution.go(TODO) + answer/answer.go + solution_test.go + README.md 格式（验证：抽查 client/01_connect、client/04_workqueue、scheduler-demo）
- [ ] manifests 各章 YAML 语法正确（验证：`kubectl apply --dry-run=client -f` 逐个检查）
- [ ] 连集群代码有 README 运行命令与预期输出（验证：client/02_crud、controller/counter-controller 的 README）

## 真集群实操
- [ ] 02-06 章 YAML 可在 minikube 上 apply 成功（验证：在 minikube 实际 apply 代表性清单）
- [ ] 09 章手写 Controller 演示步骤可行（验证：按 README 在 minikube 跑一遍，改注解后副本数变化）
- [ ] 07 章 client-go 连接程序在 minikube 可运行（验证：`go run` 列出 Pod 成功）

## 集成
- [ ] config.mts 注册 `/k8s-code教程/` 侧边栏分组（14 条）与导航入口（验证：`grep k8s-code教程 docs/.vitepress/config.mts`）
- [ ] `docs/index.md` 推荐顺序包含 K8s Code 教程（验证：grep）
- [ ] `docs/后端技术栈强化/index.md` S7 表格含互链（验证：grep）
- [ ] `npm run docs:build` 构建通过（验证：运行构建命令，退出码 0）

## 内容质量
- [ ] 面试题集 ≥ 15 题，每题含考察点/追问链/答案要点（验证：`grep -c '^### ' docs/k8s-code教程/14-面试题集.md` ≥ 15）
- [ ] 与 07-k8s 理论文档互链（验证：grep 互链链接）
- [ ] 所有示例命令在本机环境可解释/可复现（验证：抽查 3 个命令执行）

## 端到端场景
- [ ] 场景 1：新读者从首页 → 01 章环境检查 → 02 章 apply 一个 Pod → `kubectl get pod` 看到 Running（验证：实际执行）
- [ ] 场景 2：学完 07-09 章后，`go run` counter-controller，在 minikube 上改 ConfigMap 注解，观察到 Deployment 副本数被控制器收敛（验证：实际执行）
- [ ] 场景 3：`npm run docs:build` 后本地预览，侧边栏出现"K8s Code 教程"且章节可点击（验证：构建+预览）
