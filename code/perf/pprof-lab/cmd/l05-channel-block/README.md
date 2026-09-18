# L05 · 无缓冲 channel + 单 worker：请求阻塞在 chansend / chanrecv

> 模块根目录 `code/perf/pprof-lab`，所有命令都在该目录下执行。零第三方依赖，只用标准库；`go run` 直接跑，加 `-fix` 切换修复实现。

## 这个样例演示什么

业务场景是**素材处理流水线**：`GET /api/pipeline` 每次投递一个 job，等 `compute`（对共享样本表上的 20 万个值做滚动哈希 + 求和 + 求最大值，约几百微秒真实 CPU 工作）返回结果。`buildJob` 只是从启动时加载好的共享样本表（`sharedSample`）里取一个滑动窗口，所以「提交 job」本身不分配内存——压测看到的开销都来自流水线调度与计算本身。

- **bug 模式**（默认）：`serialPipeline` 用**两个无缓冲 channel**（`in` / `out`）+ **只有 1 个 worker goroutine**。`Submit` 要经历两次阻塞——先 `p.in <- j` 等 worker 空出手，再 `<-p.out` 等结果；worker 侧同样阻塞在 `chanrecv1` / `chansend1` 上。整条流水线完全串行，并发请求只是在排队。
- **关键副作用（面试很值得讲）**：调用方一旦先超时退出（客户端断开、上游超时中间件取消 ctx），worker 会**永久阻塞在无缓冲的 `p.out <- compute(j)`** 上——既泄漏一个 goroutine，又让整条流水线彻底失去处理能力，之后所有请求只能等到 ctx 超时。
- 本实验包的 `cmd/load` 在 `-d` 到点后**只停止投放新请求**，在途请求正常跑完、不主动取消，所以常规压测**不会**触发这个泄漏；只有客户端断开或上游取消请求时才会触发。

## 启动

```bash
cd code/perf/pprof-lab

# bug 模式：业务 :18085，pprof :19085（Ctrl-C 优雅退出：先停业务入口 → 收敛 worker → 停 pprof）
go run ./cmd/l05-channel-block

# fix 模式：有缓冲 channel + worker pool，端口同上（同样 Ctrl-C 优雅退出）
go run ./cmd/l05-channel-block -fix

# 需要改端口时（两个 flag 就是全部可调项）
go run ./cmd/l05-channel-block -addr=127.0.0.1:18085 -pprof-addr=127.0.0.1:19085
```

端口分工：业务 mux 只挂在 `-addr`（默认 `127.0.0.1:18085`）上，只暴露 `/api/pipeline`；`net/http/pprof` 由 `labkit` **显式注册**到独立的 `-pprof-addr`（默认 `127.0.0.1:19085`）。单独分一个端口的原因：pprof 会暴露 goroutine 栈、堆内容、命令行参数，**绝不能挂在业务端口对公网开放**；生产上应绑内网/管理网卡，并用安全组或 NetworkPolicy 收紧来源。启动日志会把业务地址、pprof 地址和当前模式各打一行，照抄即可确认模式。

## 造压力

```bash
# 50 并发压 20 秒，单请求超时 10s；压测期间另开终端采集 profile
go run ./cmd/load -url='http://127.0.0.1:18085/api/pipeline' -c=50 -d=20s -timeout=10s
```

`-url` / `-c` / `-d` / `-timeout` 就是压测器的全部 flag。这个样例的经典现象是：**bug 模式把并发从 10 提到 50，QPS 几乎不动**（本机实测 5023 → 5089），延迟却从 P50 1.9ms 涨到 9.8ms——因为唯一那个 worker 的处理能力就是天花板，并发再高也只是排队。fix 模式同样的并发下 QPS 分别到 25192 / 26587。

## 采集与观测

```bash
# ① block profile（L05 主战场）：delay 就是采样到的阻塞时长
go tool pprof -http=:9090 'http://127.0.0.1:19085/debug/pprof/block?seconds=30'

# ② CPU profile：看真实 CPU 花在哪
go tool pprof -http=:9090 'http://127.0.0.1:19085/debug/pprof/profile?seconds=30'

# ③ goroutine profile：确认 worker 数量、有没有卡死的 goroutine
go tool pprof -http=:9090 'http://127.0.0.1:19085/debug/pprof/goroutine'
```

`labkit` 启动时执行了 `runtime.SetBlockProfileRate(1)`，block profile 是全量采样，`delay` 单位是纳秒，bug / fix 两组数字可以直接比。视图选择：**top** 看谁的 delay / flat 最高；**flame graph** 看调用链占比；**peek** 看某个函数的调用者与被调用者（定位"谁在等谁"最快）；选中函数后 **list** 看具体哪一行在阻塞。`go tool pprof -top -nodecount=15 <url>` 可直接在终端出文本版。

预期（bug 那一段是本机实测的 `?debug=1` 输出结构，fix 的数字以你的采集为准）：

```text
# bug 模式 block profile —— 本机实测（c=10 / d=6s）
--- contention:
cycles/second=1000000000
52181950427 30577 @ 0x1020d0f88 ...
#	runtime.selectgo+0x557
#	main.(*serialPipeline).Submit+0x7f	  cmd/l05-channel-block/main.go:120   <- 阻塞点就在这一行
#	main.newMux.func1 ...

# fix 模式同一位置：delay 从 521 亿 cycles / 30577 采样 = 单次约 1.7M cycles，
# 降到 459 亿 / 151345 采样 = 单次约 0.3M cycles（采集期间请求数多了 4 倍）
```

读法要点：**总 delay 会随请求数一起变，要比的是 delay / samples 的商值**（平均一次阻塞多久），或者配合 QPS 一起看。另外 fix 并没有让阻塞消失——调用方本来就要等自己的结果，改变的是"等多久"：bug 是所有请求共用 1 个 worker 排队，fix 是 10 个 worker 并行。

## 预期对比

| 指标 | bug 模式 | fix 模式 | 怎么测 |
| --- | --- | --- | --- |
| QPS（c=10） | 5023 | 25192 | `go run ./cmd/load -url='http://127.0.0.1:18085/api/pipeline' -c=10 -d=20s`，读 `QPS` 行，两种模式各跑一遍（本机实测示例值） |
| QPS（c=50） | 5089（并发翻 5 倍几乎不动 = 串行天花板的铁证） | 26587 | 同上，把 `-c` 改成 50 |
| P50（c=50） | 9.76ms | 0.99ms | 同一条 `cmd/load` 命令，读 `P50` 行 |
| P95 / P99（c=50） | 10.05ms / 10.88ms | 4.91ms / 14.33ms（尾延迟来自 10 个 worker 抢 10 核） | 同上，读 `P95` / `P99` 行 |
| 分配量 | 每次提交 job 本身不分配（job 只是共享样本表的切片头）；heap 主要来自 HTTP 读写缓冲 | 与 bug 基本一致 | `go tool pprof -http=:9090 'http://127.0.0.1:19085/debug/pprof/heap'`，切 `alloc_space` 视图看 top |
| goroutine 数 | 1 个 `main.(*serialPipeline).worker` + 一堆卡在 `chansend`/`selectgo` 的 handler goroutine | `runtime.NumCPU()`（本机 10）个 `main.(*poolPipeline).worker` + handler | `curl -s 'http://127.0.0.1:19085/debug/pprof/goroutine?debug=1' \| head -30`，或看 pprof goroutine 视图 |
| 热点函数 | block delay 全部落在 `main.(*serialPipeline).Submit`（其下是 `runtime.selectgo` / `chansend1` / `chanrecv1`），单次阻塞约 1.7M cycles | block delay 仍存在但单次降到约 0.3M cycles；CPU 热点是 `main.compute`，并行分布在 10 个 `main.(*poolPipeline).worker` 上 | 上面 ① ② 两条 `go tool pprof` 命令 |

## 修复要点

1. **channel 加缓冲（4096）**：投递端不必等 worker 空出手，结果端也不必等调用方来取，绝大多数 `Submit` 不再发生 `chansend` / `chanrecv` 阻塞。
2. **单 worker 换成 worker pool**：`runtime.NumCPU()` 个 goroutine 消费同一个 `in`，天然负载均衡，job 真正并行执行。
3. **退出路径要能收敛**：`close(done)` + `close(in)` + `WaitGroup.Wait()`。`close(in)` 让空闲 worker 的 `range` 结束，`close(done)` 让正在写结果的 worker 立刻返回（不会被缓冲写挂住），`WaitGroup.Wait()` 确认全部退出。
4. **必须等没有在途 `Submit` 之后再 `Close()`**：否则先关 `in` 再发就会 `panic: send on closed channel`（本样例由 `labkit` 在业务入口 `Shutdown` 之后调用 `OnShutdown` 保证这个顺序）。
5. **顺手把"调用方退出"这条路径补上**：worker 写结果时必须 `select { case p.out <- r: case <-p.done: return }`，这样即使调用方已超时离开，worker 也不会永久挂在无缓冲写操作上。

## 面试话术

> 我压过一个素材处理流水线，接口是投递任务再等结果那种。bug 版用了两个无缓冲 channel 加一个 worker：调用方 `Submit` 一次请求要阻塞两次，先 `chansend` 投递、再 `chanrecv` 取结果，worker 那边也是两次对应阻塞，所以 50 并发打进来只是排队，QPS 上不去、P99 直接被串行度顶穿。pprof 的 block profile 非常直白——`top` 第一名就是 `main.(*serialPipeline).Submit`，它下面挂着 `runtime.selectgo`、`chansend1`、`chanrecv1`，peek 一下还能看到那个唯一的 `main.(*serialPipeline).worker`，goroutine profile 里 worker 就一个，其余全是卡在 `chansend1` 的 handler。更狠的是副作用：调用方一旦先超时退出，worker 会永久阻塞在无缓冲的 `p.out <- compute(j)` 上，goroutine 泄漏不说，整条流水线直接失去处理能力——因为 `cmd/load` 到点只停投放、不取消在途请求，常规压测反而不容易暴露，是客户端断开或上游超时中间件取消请求才会踩到。修复就是把 channel 缓冲调到 4096、worker 扩到 `runtime.NumCPU()`，退出用 `close(done)` + `close(in)` + `WaitGroup.Wait()` 收敛；实测 QPS 从 5023 提到 25192，而且 block profile 里单次阻塞从 1.7M cycles 降到 0.3M cycles、CPU 热点变成并行跑在 10 个 worker 上的 `main.compute`，说明瓶颈从"等 channel"回到了真实计算。

## 记录模板

```text
日期/机器：        2025-XX-XX / Apple M5、Go 1.26.2、darwin-arm64
样例：             L05 channel-block（业务 :18085 / pprof :19085）
模式：             bug | fix
命令：             go run ./cmd/l05-channel-block [-fix]
压测：             go run ./cmd/load -url='http://127.0.0.1:18085/api/pipeline' -c=50 -d=20s -timeout=10s
结果：             QPS=____  P50=____  P95=____  P99=____  成功/失败=____/____  错误分布=____
采集：             go tool pprof -http=:9090 'http://127.0.0.1:19085/debug/pprof/block?seconds=30'
                   go tool pprof -http=:9090 'http://127.0.0.1:19085/debug/pprof/profile?seconds=30'
                   go tool pprof -http=:9090 'http://127.0.0.1:19085/debug/pprof/goroutine'
pprof 结论：       block top = ____（delay=____）
                   goroutine 中 main.(*serialPipeline).worker / main.(*poolPipeline).worker 数量 = ____
                   CPU 热点 = ____
是否复现预期：     是/否；不一致的地方与猜测原因 = ____
一句话结论：       ____
```
