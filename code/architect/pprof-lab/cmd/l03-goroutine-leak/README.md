# L03 · goroutine 只增不减：Ticker 未停 + 没有退出信号

> 模块根目录 `code/perf/pprof-lab`，下面所有命令都在该目录下执行；零第三方依赖，只用标准库。

## 这个样例演示什么

业务场景是转码任务的**进度上报 / 心跳**：每次 `GET /api/task/start` 都拉起一组后台 goroutine。
bug / fix 每任务起**同样数量（3 个）**的 goroutine，唯一差别是**它们有没有退出条件**：

- **bug 模式**：`leakTask()` 每次泄漏 3 个 goroutine ——
  1. `time.NewTicker(10ms)` 驱动、`for range t.C` 永不 break，且 **Ticker 从不 `Stop()`**（goroutine 与 runtime timer 双泄漏）；
  2. 阻塞在**永远不会被 close** 的 channel 上（`<-done`）；
  3. 阻塞在**永远不会有人写入**的 channel 上。
- **fix 模式**：所有后台 goroutine 都挂 `context.Context`，用 `sync.WaitGroup` 记账，
  `GET /api/task/stop` 一次 `cancel + Wait` 全部收敛。

要观察的是三条可复现的判据：**① goroutine 数随时间单调上升；② 泄漏栈在 `debug=1` 里成段重复（带 `main.go:行号`）；
③ 有没有"收敛开关"** —— 数大不是病，**能不能被收回才是病**。

## 启动

端口分工：业务端口 **18083**（`/api/task/start`、`/api/task/count`、`/api/task/stop`），pprof 管理端口 **19083**。

```bash
# 终端 A：bug 模式
go run ./cmd/l03-goroutine-leak

# 终端 A（对照实验）：fix 模式
go run ./cmd/l03-goroutine-leak -fix

# 三个 flag 的默认值
go run ./cmd/l03-goroutine-leak -addr=127.0.0.1:18083 -pprof-addr=127.0.0.1:19083 -fix=false
```

三个端点的语义（`n` 上限 100，避免一个请求打出十万 goroutine）：

| 端点 | 作用 | bug 模式响应要点 | fix 模式响应要点 |
| --- | --- | --- | --- |
| `GET /api/task/start?n=20` | 拉起 n 个任务 | `leaked_per_start:3`、`leaked_total` 累加 | `active`、`stop_hint` |
| `GET /api/task/count` | 观测快照 | `goroutines` / `baseline` / `leaked_total` / `num_cpu` | 同左，另有 `active_workers` |
| `GET /api/task/stop` | 收敛开关 | **`stopped:false`**（没有任何退出信号）| `goroutines_before/after`、`released` |

## 造压力

```bash
# ① bug 模式：基线
curl -s 'http://127.0.0.1:18083/api/task/count'
# {"active_workers":0,"baseline":1,"goroutines":7,"leaked_total":0,"mode":"bug","num_cpu":10}

# ② 提交 100 个任务（5 次 × n=20）→ 泄漏 300 个
for i in 1 2 3 4 5; do curl -s -o /dev/null 'http://127.0.0.1:18083/api/task/start?n=20'; done
curl -s 'http://127.0.0.1:18083/api/task/count'
# {"active_workers":0,"baseline":1,"goroutines":307,"leaked_total":300,...}   ← 7 + 300，公式精确吻合

# ③ 试着收敛：bug 模式收不回来
curl -s 'http://127.0.0.1:18083/api/task/stop'
# {"goroutines":307,"mode":"bug","stopped":false,"note":"⚠️ bug 模式没有任何退出信号…"}
```

fix 模式的对照（同样的 5 次 × n=20）：

```bash
curl -s 'http://127.0.0.1:18083/api/task/start?n=20'   # {"active_workers":60,"goroutines":67,...}
curl -s 'http://127.0.0.1:18083/api/task/stop'
# {"goroutines_before":67,"goroutines_after":7,"released":60,"mode":"fix",...}  ← 回到基线
```

## 采集与观测

```bash
# ① 聚合视图：数哪段栈在涨（这是定位泄漏最快的视图）
curl -s 'http://127.0.0.1:19083/debug/pprof/goroutine?debug=1' | head -40

# ② 逐栈视图：看每一个 goroutine 到底卡在哪一行（数大时输出很巨，别在 10w goroutine 时打）
curl -s 'http://127.0.0.1:19083/debug/pprof/goroutine?debug=2' | head -60

# ③ 或交给 pprof 聚合
go tool pprof -top 'http://127.0.0.1:19083/debug/pprof/goroutine'
```

**预期看到什么**（bug 模式，本机实测）：`debug=1` 的第一行是 `goroutine profile: total 307`，
随后**三段各 100 个**的重复栈，且都带源码行号——正是三处泄漏点：

```text
goroutine profile: total 307
100 @ … main.leakTask.func3+0x23   .../cmd/l03-goroutine-leak/main.go:66
100 @ … main.leakTask.func2+0x23   .../cmd/l03-goroutine-leak/main.go:60
100 @ … main.leakTask.func1+0x23   .../cmd/l03-goroutine-leak/main.go:5x
```

栈状态词就是分诊答案：三处都是 `[chan receive]`（等 channel）或 ticker 循环；
如果是 `[IO wait]` + `internal/poll.runtime_pollWait`，那是等下游，**不要在这条线上找泄漏**。

> **判据模板**：`goroutines ≈ baseline + leaked_per_start × 已提交任务数`。
> **能写出公式的假设才是好假设**——它把"看起来在涨"变成了可证伪的预测。

## 预期对比

| 指标 | bug 模式 | fix 模式 | 怎么测 |
| --- | --- | --- | --- |
| 每任务 goroutine 数 | 3（`leaked_per_start`）| 3（`workerPerTask`，与 bug 对齐便于对比）| 响应字段 |
| 提交 100 任务后 goroutine 数 | **307**（基线 7 + 300）| 提交 20 任务：**67**（基线 7 + 60）| `GET /api/task/count` 的 `goroutines` |
| `/api/task/stop` 结果 | **`stopped:false`**（收不回）| **`released:60`，67 → 7** | `GET /api/task/stop` |
| 泄漏栈数量（`debug=1`）| 三段重复栈，各 100 | 无重复泄漏栈（全部可收敛）| `curl … /goroutine?debug=1` |
| 泄漏栈行号 | `main.go:60` / `main.go:66` / `main.go:5x`（`leakTask` 内）| —— | 同上 |
| 内存 / FD | 随时间缓涨（栈 + timer + 闭包引用）| 平稳 | `GET /api/task/count` + `ps -o rss= -p <pid>` |
| 收敛后 | 只能杀进程 | 回到基线 7 | 先 `stop` 再 `count` |

> **数字怎么读**：上表是本机实测示例值（Apple M5 / Go 1.26.2 / darwin-arm64）。
> 关键是**趋势与公式**，不是绝对值：`7 + 3 × 任务数` 这个关系在 bug 模式永远成立、在 fix 模式永远不成立。

## 修复要点

1. **每个 goroutine 都要能回答"它怎么退出"**：`for range t.C` 不是退出条件，`select { case <-ctx.Done(): return }` 才是。
2. **`time.Ticker` 必须 `defer t.Stop()`**：否则泄漏的不只是 goroutine，还有 runtime timer（定时器堆会持续增长）。
3. **别等一个没人 close 的 channel**：等待方要有 `ctx.Done()` 兜底，发送/关闭方要有明确的生命周期负责人。
4. **用 `WaitGroup` 记账 + `context` 取消**：`Stop()` 里先 `cancel()` 再 `wg.Wait()`，保证返回时后台 goroutine 已经真的退出。
5. **把"能不能收敛"做成可观测接口**：`/api/task/stop` 返回 `released` 数量，
   这样"泄漏修复"就有了一个当场可验证的断言，而不是"感觉好了"。

## 面试话术

> 我遇到过一个转码任务平台的进度上报模块，上线三天内存持续上涨、重启就恢复。我没有先猜内存，而是先看 goroutine：`/api/task/count` 显示 goroutine 数从基线 7 单调涨到几千，而且**能用公式对上**——每提交一个任务涨 3 个。接着用 `goroutine?debug=1` 看聚合栈，三段重复栈各 100 个、都带源码行号，一眼定位到三处：一个 `for range ticker.C` 没有退出条件而且 Ticker 从没 `Stop`、一个在等永远不会 close 的 channel、一个在等永远不会有人写入的 channel。修法是给每个后台 goroutine 挂 `context`、用 `WaitGroup` 记账，`/api/task/stop` 里 `cancel` 后 `Wait`，修复后同样的提交量 goroutine 数能从 67 收敛回 7。**我记住的纪律是：每个 goroutine 都要问一句"它怎么退出"；而且判断泄漏不要只看数字大，要看它能不能被收回。**

## 记录模板

```text
日期 / 机器   : 2025-xx-xx / Apple M5 (darwin-arm64), Go 1.26.2
样例          : L03 · cmd/l03-goroutine-leak   业务 18083 / pprof 19083
模式          : bug（不带 -fix） / fix（-fix）
启动命令      : go run ./cmd/l03-goroutine-leak [-fix]
复现命令      : curl 'http://127.0.0.1:18083/api/task/start?n=20'  × 5 次
观测命令      : curl 'http://127.0.0.1:18083/api/task/count'
采集命令      : curl -s 'http://127.0.0.1:19083/debug/pprof/goroutine?debug=1' | head -40
基线 / 复现后 : goroutines = ?  →  ?
拟合公式      : goroutines ≈ baseline + ? × 任务数   （是否吻合）
泄漏栈        : 段数 = ?，每段数量 = ?，行号 = ?
收敛验证      : GET /api/task/stop → stopped = ? / released = ? ；收敛后 goroutines = ?
对照结论      : bug → fix 改变了什么（退出信号 / 取消传播 / WaitGroup 记账），数字变化（? → ?）
遗留疑问      :
```
