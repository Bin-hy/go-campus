# 路线专题 · 后端与计算机基础（Day 1-7）代码集

> 配套文档：[路线专题-后端与计算机基础夯实（Day 1-7）](/路线专题/02-后端与计算机基础)

本目录对应 30 天冲刺训练营「后端与计算机基础夯实」的**每日实战练习**，每题结构与 `code/phase1` 一致：`README.md`（题目 / 考点 / 函数签名 / 提示）+ `solution.go`（待实现）+ `solution_test.go`（测试）+ `answer/answer.go`（参考答案，文档站中默认折叠）。

## 练习地图

| 文档小节 | 代码练习目录 | 说明 |
| --- | --- | --- |
| Day 1 · 1.8 并发任务池 | `01_concurrency/01_worker_pool` | fan-out / fan-in、channel 关闭语义、context 取消 |
| Day 2 · 2.7 epoll TCP 服务 | `02_io_multiplexing/01_event_loop` | 事件循环：注册 → 就绪 → 分发，LT vs ET 触发语义（跨平台可测；真实 syscall 版见文档 2.7） |
| Day 3 · 3.7 SSE 服务端 + 客户端 | `03_network/01_sse_server`、`03_network/02_sse_client` | text/event-stream 写入与解析、`[DONE]`、ctx 取消 |
| Day 4 · 4.6 慢 SQL 优化 | `04_sql/01_slow_query` | 纯 SQL 练习（无 Go 测试），explain 自测 |
| Day 5 · 5.6 防击穿缓存重建 | `05_redis_cache/01_cache_breakdown` | 缓存接口抽象 + SETNX 互斥锁重建，离线可测 |
| Day 6 · 6.5 AI 剪辑任务异步流程 | `06_async_pipeline/01_task_pipeline` | 任务队列 + 多 worker 消费 + 失败重试 + 结果事件 |

## 运行

```bash
cd code/route
go test ./...                                   # 全部
go test ./01_concurrency/01_worker_pool -v      # 单题
```

> 说明：`04_sql` 为纯 SQL 练习，无 Go 测试；`05_redis_cache` 用 `Cache` 接口抽象替代真实 Redis，方便离线自测（生产实现参考文档 5.6 的 go-redis 写法与 `code/backend/02-redis`）。
