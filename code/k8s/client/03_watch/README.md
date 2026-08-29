# 练习 3：Watch 与 Informer

## 考点
- watch 事件流：ADDED / MODIFIED / DELETED
- SharedInformer：本地缓存 + 事件回调，避免每次都全量 List
- informers 工厂：`NewSharedInformerFactory`

## 题目
1. `ClassifyEvent`：把 "ADDED"/"MODIFIED"/"DELETED"（大小写不敏感）归类
2. `DescribeEvent`：给每种事件一句中文说明
3. `Summarize`：把原始事件列表转成带说明的 PodEvent 列表
4. `BuildInformer`：构建监听 Pod 的 SharedIndexInformer（返回 informer 即可）

## 运行测试

```bash
cd code/k8s/client/03_watch
go test -v
```

## 真集群实操（main.go）

```bash
cd code/k8s/client/03_watch
go run .               # 监听 default 的 Pod 事件
# 另开终端：
kubectl run tmp --image=nginx:1.27
kubectl delete pod tmp
# 观察程序打印 Added/Modified/Deleted 事件
```

## 为什么需要 Informer（面试高频）
直接 Watch 有两个问题：重连会漏事件、每次都要全量 List。Informer 的答案：
1. **先 List 全量进本地缓存，再 Watch 增量**，事件 + 缓存保证"不漏不重"；
2. 多个消费者共享**同一个 informer 实例**（Shared 的含义），List/Watch 只做一次；
3. 事件回调里**不要直接干活**，只把 key 丢进 Workqueue（下一篇的伏笔）。
