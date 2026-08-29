# 练习 4：Workqueue 模式

## 考点
- 控制器经典架构：**Informer 只丢 key → Workqueue 缓冲 → Worker 取 key 干活**
- workqueue 的接口语义：`Get`（取一个）、`Done`（处理完归还）、`Add`（入队）
- 失败重试：返回 error 后重新入队（RateLimiting 的简化版）

## 题目
1. `StartWorkers`：启动 n 个 worker goroutine，每个循环 `Get → 处理 → Done`；处理失败把 key 重新 `Add`
2. `RunWorker`：启动 workers 并等待全部退出

## 运行测试

```bash
cd code/k8s/client/04_workqueue
go test -v
```

## 为什么是"队列"而不是"直接调用"（面试高频）
Informer 回调里直接干活有三个问题：
1. **阻塞回调**：informer 事件处理是串行的，一个慢任务卡住后面所有事件；
2. **重复处理**：同 key 多次事件（Added+Modified）会重复执行；
3. **无重试**：处理失败没有机制补偿。

Workqueue 的答案：
- 回调里只做 `q.Add(key)`，**立即返回**，不阻塞事件流；
- workqueue 对**相同 key 去重**（Add 已存在的不重复排队）；
- 处理失败 `AddRateLimited`，按指数退避重试。

这就是 K8s 所有控制器的标准骨架，下一篇（09 手写 Controller）会把它拼成完整闭环。

## 参考：真实 workqueue 用法

```go
import "k8s.io/client-go/util/workqueue"

q := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
q.Add("default/pod-1")           // 入队
item, shutdown := q.Get()        // 取一个
// ... 处理 ...
q.Done(item)                     // 标记完成
q.AddRateLimited(item)           // 失败：限速重试
```
