# L02 · 每请求 MB 级分配：GC CPU 高、P99 抖

> 模块根目录 `code/perf/pprof-lab`，下面所有命令都在该目录下执行；零第三方依赖，只用标准库。

## 这个样例演示什么

业务场景是缩略图生成：4MB 原图 → 2MB 中间结果 → 1MB 输出。bug / fix 的**计算完全相同**
（都走 `main.transform`：解码铺块 → 缩放取样 → 编码拷贝），唯一差别是工作内存从哪来：

- **bug 模式**：`main.renderThumb` 每次请求都 `make([]byte, 4MB) + 2MB + 1MB`，7MB 临时缓冲写完即垃圾，
  请求一多就变成「分配 → 变垃圾 → GC → 再分配」的循环；
- **fix 模式**：`sync.Pool`（`scratchPool`）复用 `scratch{raw, mid, out}` 三块缓冲，稳态 0 allocs/op。

要观察的是一条完整因果链：**分配量 → GC 频率 → GC assist / STW → P99 抖动**，而不是"某段代码慢"。

## 启动

端口分工：业务端口 **18082**（只挂 `GET /api/thumb?id=1`），pprof 管理端口 **19082**（= 业务端口 + 1000）。
`labkit.Run` 是显式把 `net/http/pprof` 的 handler 注册到**独立端口的 mux** 上，而不是
`import _ "net/http/pprof"` 挂上 `http.DefaultServeMux`——这样 pprof 才能只绑 127.0.0.1 / 内网，
不会随业务端口对公网暴露（它会泄漏 goroutine 栈、堆内容与命令行参数）。

```bash
# 终端 A：bug 模式；Ctrl-C 优雅退出（先停业务入口 → 收敛 stats goroutine → 最后关 pprof）
go run ./cmd/l02-alloc-gc

# 终端 A（对照实验）：fix 模式
go run ./cmd/l02-alloc-gc -fix

# 三个 flag 的默认值（一般不用显式传）
go run ./cmd/l02-alloc-gc -addr=127.0.0.1:18082 -pprof-addr=127.0.0.1:19082 -fix=false

# 自检：响应头带 X-Mode / X-Bytes / X-Checksum / X-Elapsed-Us
curl -s -D- -o /dev/null 'http://127.0.0.1:18082/api/thumb?id=1'
```

启动后每 3 秒打印一行堆与 GC 快照（`startGCStatsLogger`），bug / fix 的差别肉眼可见：

```text
[stats] heap_inuse=...MB heap_objects=... gc_cycles=... gc_cpu=...% goroutines=...
```

## 造压力

```bash
# 50 并发打 20 秒；压测器 flag 只有 -url / -c / -d / -timeout 四个
go run ./cmd/load -url='http://127.0.0.1:18082/api/thumb?id=1' -c=50 -d=20s -timeout=10s
```

压测器直接输出 QPS、平均延迟、P50 / P95 / P99、成功失败数与状态码分布；
先跑 bug 再跑 fix，用同一个 URL、同一组参数，只有服务端加了 `-fix` 这一处不同。

## 采集与观测

```bash
# CPU 30s：看 runtime.mallocgc / gcBgMarkWorker 是不是压过了 main.renderThumb
go tool pprof -http=:9090 'http://127.0.0.1:19082/debug/pprof/profile?seconds=30'

# 累计分配量（问题本体）：alloc_space；常驻内存：inuse_space
go tool pprof -http=:9090 -sample_index=alloc_space 'http://127.0.0.1:19082/debug/pprof/allocs'
go tool pprof -http=:9090 -sample_index=inuse_space  'http://127.0.0.1:19082/debug/pprof/heap'

# 不开网页时：top 看排序，peek 看调用链上下游，list 落到源码行
go tool pprof -top  -sample_index=alloc_space 'http://127.0.0.1:19082/debug/pprof/allocs'
go tool pprof -peek='renderThumb' 'http://127.0.0.1:19082/debug/pprof/allocs'
go tool pprof -list='renderThumb' 'http://127.0.0.1:19082/debug/pprof/allocs'
```

Web UI 里切 **Top**（排序看函数名）、**Flame Graph**（看 GC 相关栈有多宽）、**Peek / Source**（看调用链与源码行）。
bug 模式预期：CPU top 由 `runtime.mallocgc`、`runtime.gcBgMarkWorker`、`runtime.memclrNoHeapPointers`
（大块新内存要清零）占据，`main.renderThumb` 这条路径下挂着 3 次分配、约 7MB/请求；
`alloc_space` 视图里 `main.renderThumb`（含其内联的 `make`）是绝对大头，且随压测时长线性增长。
fix 模式预期：`main.transform` 回到 CPU 主体，`alloc_space` 只剩零星来源——
`main.retainSample` 首次建环形缓冲时的一次性分配，以及 `sync.Pool` 被 GC 清空后的少量重建。
注意结果头部 8KB 会复制进环形缓冲 `sampleRing`（保留最近 64 次，约 512KB 常驻），
这是**有意保留**的业务缓存，不是问题本身，别把它当优化对象。

## 预期对比

| 指标 | bug 模式 | fix 模式 | 怎么测 |
| --- | --- | --- | --- |
| QPS | 实测填写 | 实测填写 | `go run ./cmd/load -url='http://127.0.0.1:18082/api/thumb?id=1' -c=50 -d=20s -timeout=10s` 结果行的 `QPS` |
| B/op | 7342709 B/op | 36707 B/op | `go test ./cmd/l02-alloc-gc -bench=Thumb -benchmem -benchtime=200x` |
| allocs/op | 3 allocs/op | 0 allocs/op | 同上（`benchmem` 输出） |
| ns/op | 590304 ns/op | 189110 ns/op | 同上（≈3 倍差距） |
| P50 | 实测填写 | 实测填写 | `cmd/load` 结果行的 `P50` |
| P95 | 实测填写 | 实测填写 | `cmd/load` 结果行的 `P95` |
| P99 | 实测填写 | 实测填写（抖幅应明显更小） | `cmd/load` 结果行的 `P99` |
| goroutine 数 | 实测填写 | 实测填写 | 服务端每 3 秒的 `[stats]` 行，或 `go tool pprof 'http://127.0.0.1:19082/debug/pprof/goroutine'` |
| gc_cycles / gc_cpu | 实测填写（增长快、占比高） | 实测填写（明显更低） | 同上 `[stats]` 行 |
| 热点函数 | `runtime.mallocgc`、`runtime.gcBgMarkWorker`、`runtime.memclrNoHeapPointers` | `main.transform`，分配来源仅 `main.retainSample` | CPU profile（见上） |

> 上表中的基准数字是本机实测示例值（Apple M5 / Go 1.26.2 / darwin-arm64），你的机器会有差异，
> 但 **B/op 从 ~7MB 降到 ~37KB、allocs/op 从 3 降到 0** 这个量级差是稳定的。

## 修复要点

1. **用 `sync.Pool` 复用工作缓冲**：`scratchPool` 持有 `scratch{raw, mid, out}`，把"每请求分配"变成"每请求借用"，稳态 allocs/op 归零。
2. **归还时机要在 body 写完、且不再引用之后**：handler 里 `defer scratchPool.Put(pooled)`，池化缓冲绝不能跨请求持有。
3. **接受池会被 GC 清空**：`sync.Pool` 不保证保留，所以 fix 模式偶尔仍会分配——这不是回归，是设计取舍（`bench_test.go` 里 `TestThumbModesProduceSameBytes` 就是在守这一点）。
4. **复用缓冲要防脏数据**：池里拿到的内存可能残留上次内容，凡是"必须为零/必须整块覆盖"的字段要显式处理，否则 bug 会从"慢"变成"错"。
5. **区分"有意保留"和"泄漏"**：`sampleRing` 那约 512KB 是缓存语义，压测时应关注 `inuse_space` 是否随时间单调上涨，而不是看绝对值。

## 面试话术

> 我压测过一个缩略图接口，4MB 原图加水印输出 1MB，QPS 上不去、P99 抖得厉害。CPU profile 一打，排在最前面的不是我的业务函数，而是 `runtime.mallocgc`、`runtime.gcBgMarkWorker`，再加一个 `memclrNoHeapPointers`——因为每请求 `make` 了 4MB+2MB+1MB 三块缓冲，用完立刻变垃圾，进程整天在"分配—回收—再分配"，GC assist 让业务 goroutine 也被拉去帮忙标记，尾延迟就炸了。heap profile 加 `-sample_index=alloc_space` 一看，累计分配量随压测时长线性涨，`renderThumb` 就是唯一大头。改法很直接：`sync.Pool` 复用这三块 scratch，稳态 allocs/op 从 3 降到 0，每请求分配从 7342709 B/op 降到 36707 B/op，benchmark 的 ns/op 从 590304 降到 189110，差不多三倍。顺带一个坑我专门写了测试守：`sync.Pool` 会在 GC 时被清空，所以 fix 之后不是"零分配"而是"接近零分配"，而且池里缓冲可能带脏数据，必须保证整块覆盖写。最后提醒一句，接口里那个保留最近 64 次结果头部的环形缓冲约 512KB，是有意留的缓存，不算泄漏——排查内存时得先把这两类东西分清楚。

## 记录模板

```text
日期 / 机器   : 2025-xx-xx / Apple M5 (darwin-arm64), Go 1.26.2
样例          : L02 · cmd/l02-alloc-gc   业务 18082 / pprof 19082
模式          : bug（不带 -fix） / fix（-fix）
启动命令      : go run ./cmd/l02-alloc-gc [-fix]
压测命令      : go run ./cmd/load -url='http://127.0.0.1:18082/api/thumb?id=1' -c=50 -d=20s -timeout=10s
结果          : 成功/失败 = ? / ?    错误数 = ?
                QPS = ?   平均 = ?   P50 = ?   P95 = ?   P99 = ?   最慢 = ?
基准          : go test ./cmd/l02-alloc-gc -bench=Thumb -benchmem -benchtime=200x
                ns/op = ?    B/op = ?    allocs/op = ?
运行时快照    : [stats] heap_inuse = ?MB   gc_cycles = ?   gc_cpu = ?%   goroutines = ?
pprof 结论    : CPU top3 = ? / ? / ?
                alloc_space 最大来源 = ?（约 ? MB/请求，共 ? 次分配）
对照结论      : bug → fix 改变了什么机制（?），数字变化（? → ?），
                残留分配来自哪里（sync.Pool 被 GC 清空 / retainSample 首次建 ring）
```
