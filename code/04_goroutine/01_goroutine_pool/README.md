# Goroutine Pool 协程池

## 难度：⭐⭐⭐ 困难

## 考点
- 限制最大并发 goroutine 数量
- 任务队列与 worker 生命周期管理
- 优雅关闭（等待所有任务完成）

## 题目描述

实现一个 goroutine 池，限制最大并发数。

要求：
1. `NewPool(maxWorkers)` — 创建指定大小的协程池
2. `Submit(task)` — 提交任务到池中执行
3. `Wait()` — 等待所有已提交的任务完成
4. 同一时刻运行的 goroutine 不超过 maxWorkers
5. Submit 在池未关闭时不应阻塞（除非需要背压，这里不要求）

## 函数签名

```go
type Pool struct { ... }

func NewPool(maxWorkers int) *Pool
func (p *Pool) Submit(task func())
func (p *Pool) Wait()
```

## 提示
1. 用 buffered channel 作为任务队列
2. 启动固定数量的 worker goroutine，从 channel 读取任务执行
3. Wait 时关闭任务 channel，等待所有 worker 退出
4. 注意：Submit 在 Wait 之后调用应该是安全的（不 panic）
