# Go 微服务实战教程 Checklist

## 代码可运行性

- [x] `cd code/microservice && go build ./...`
- [x] `go vet ./...`
- [x] `go test ./...`：14 个练习全部通过
- [x] `go test -race ./01_grpc_basics/... ./02_governance/... ./03_messaging/... ./04_observability/...`
- [x] `go run ./lab/uservice -addr :9001` 可启动
- [x] `go run ./lab/gateway` 可编译并访问三实例
- [x] `make proto` 可重新生成且无 diff

## 内容质量

- [x] 每章有开头比喻 + 问题引入
- [x] 每章至少 1 个 mermaid 图
- [x] 每章有代码片段与真实路径
- [x] 每章有「练习」「面试追问」「串起来」
- [x] 四阶段与用户学习计划一一对应
- [x] 17 篇之间上一篇/下一篇链接正确
- [x] 与 S4/S8/K8s 互链
- [x] 8 项验收每条有复现命令、预期、失败模式

## 站点集成

- [x] `docs/.vitepress/config.mts` 顶栏 + 侧边栏
- [x] `docs/后端技术栈强化/index.md` 增加实战入口
- [x] `docs/index.md` 推荐顺序增加条目
- [x] `code/README.md` 目录树更新
- [x] `npm run docs:build` 构建通过（已验证，46s）
- [x] `kubectl apply --dry-run=client -f code/microservice/lab/k8s/` 通过

## 待人工确认

- [ ] minikube 上 `kubectl apply -f code/microservice/lab/k8s/` 全量验证
- [ ] 真实 Kafka（Docker）下的手工重投实验
- [ ] Grafana 看板导入验证
