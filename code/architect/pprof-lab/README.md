# pprof 调优实验包（code/architect/pprof-lab）

> 配套文档：[架构师修炼 · 19 pprof 实战](/架构师修炼/19-pprof实战/)（观测体系 · CPU 火焰图 · 内存与 GC · goroutine 与锁 · 常见问题手册 · 生产实践 · 7 个案例 · 30 道面试题）

一套**能亲手造出性能问题、再用 pprof 定位、最后用量化数字证明修复有效**的 Go 实验包。
7 个服务样例 + 1 个自带压测器，**零第三方依赖（只用标准库）**、不需要 Docker、不需要 `wrk`，
`go run` 直接跑，每个样例加 `-fix` 就是修复版，**同一组压测参数前后对比**。

```text
你不只是在"学工具"，而是在建立一条闭环：
写下预期 → 起服务造压力 → 采集 profile → 读图定位到行 → 改一处 → 同负载复测 → 拿出数字
```

---

## 快速开始（30 秒）

```bash
cd code/architect/pprof-lab

# 终端 A：起一个有问题的服务（bug 模式）
go run ./cmd/l01-cpu-hotspot

# 终端 B：造压力
go run ./cmd/load -url='http://127.0.0.1:18081/api/render?n=2000' -c=50 -d=10s

# 终端 C：采集 + 看火焰图（注意 pprof 在 19081，不是业务端口）
go tool pprof -http=:9090 'http://127.0.0.1:19081/debug/pprof/profile?seconds=30'

# 终端 A：Ctrl-C 停掉，换修复版重跑同样的压测与采集，对比数字
go run ./cmd/l01-cpu-hotspot -fix
```

**端口约定（记住这一条就不会敲错）**：业务端口 `1808N`，**pprof 管理端口 `1908N`（= 业务端口 + 1000）**。
pprof 永远挂在独立端口上、默认只绑 `127.0.0.1`——它会泄漏 goroutine 栈、堆内容与命令行参数，不能跟业务端口一起对公网开放。

**通用 flag**：`-addr`（业务监听）、`-pprof-addr`（管理监听）、`-fix`（默认 `false` 跑问题实现，加上就跑修复实现）。

---

## 样例总表

| ID | 演示的问题 | 启动 | 业务端点 | 该抓哪个 profile | 观测重点 | 文档 |
| --- | --- | --- | --- | --- | --- | --- |
| **L01** | CPU 打满：反射 + JSON 往返 + 字符串累加 | `go run ./cmd/l01-cpu-hotspot` | `:18081` · `GET /api/render?n=2000` | `profile` | `top -cum` 追到 `main.renderBug`，其下挂着 `runtime.concatstrings` | [02](/架构师修炼/19-pprof实战/02-CPU火焰图实战) |
| **L02** | 每请求 7MB 分配 → GC CPU 高、P99 抖 | `go run ./cmd/l02-alloc-gc` | `:18082` · `GET /api/thumb?id=1` | `allocs` → `heap` | CPU 顶部是 `mallocgc`/`gcBgMarkWorker`；`alloc_space` 线性增长 | [03](/架构师修炼/19-pprof实战/03-内存与GC实战) |
| **L03** | goroutine 只增不减（Ticker 未停、无退出信号） | `go run ./cmd/l03-goroutine-leak` | `:18083` · `GET /api/task/start?n=20` · `/api/task/count` · `/api/task/stop` | `goroutine` | `?debug=1` 里三段各 100 的重复栈（带行号）；`count` 能用公式对上 | [04](/架构师修炼/19-pprof实战/04-goroutine与锁阻塞实战) |
| **L04** | 全局 mutex 竞争（临界区里做计算与 IO） | `go run ./cmd/l04-lock-contention` | `:18084` · `GET /api/counter?k=hot` | `mutex` + `profile` | mutex 记**持锁者**栈（`Unlock` 侧），block 记**等待者**栈（`Lock` 侧） | [04](/架构师修炼/19-pprof实战/04-goroutine与锁阻塞实战) |
| **L05** | 无缓冲 channel + 单 worker 串行 | `go run ./cmd/l05-channel-block` | `:18085` · `GET /api/pipeline` | `block` | CPU 图上"没有热点"，`delay` 全落在 `chan send`/`chan recv` | [04](/架构师修炼/19-pprof实战/04-goroutine与锁阻塞实战) |
| **L06** | 全局 map 无上限 → heap 只涨不降 | `go run ./cmd/l06-memory-retention` | `:18086` · `GET /api/cache/put?n=1000&kb=64` · `/api/cache/stats` | `heap`（两次做差） | `entries` 单调上涨 + `-base` 增长点落在业务容器 = **强引用滞留**（不是泄漏） | [03](/架构师修炼/19-pprof实战/03-内存与GC实战) |
| **L07** | 逐行 `fmt.Fprintf`、无缓冲、未预分配 | `go run ./cmd/l07-io-serialization` | `:18087` · `GET /api/export?n=20000` · `GET /api/export?stats=1` | `profile` + `runtime/trace` | `?stats=1` 的 `write_calls` ≈ `rows`；block profile 这里**预期为空**（IO 不埋点） | [02](/架构师修炼/19-pprof实战/02-CPU火焰图实战) · [07](/架构师修炼/19-pprof实战/07-实战案例集) |
| **load** | 自建压测器（不依赖 `wrk`） | `go run ./cmd/load -url=… -c=50 -d=20s` | —— | —— | 输出 QPS / 平均 / P50 / P95 / P99 / 成功率 / 状态码分布 | 全部 |

**L07 的 `?stats=1` 字段契约**（文档依赖，勿改）：`mode` / `rows` / `bytes` / `write_calls` / `avg_write_bytes` / `elapsed_ms`。

### 本机实测的量级差（用于对照，你的机器会有差异）

| 样例 | 指标 | bug | fix | 备注 |
| --- | --- | --- | --- | --- |
| L01 | `ns/op` / `B/op` / `allocs/op` | 15.03ms / 142.6MB / 96112 | 0.199ms / 0.34MB / 6506 | `go test -bench=. -benchmem -benchtime=3x` |
| L01 | QPS / P50（`-c=50 -d=10s`）| 77.5 / 632ms | 11320 / 1.88ms | bug 在 `-c=400` 仍 ~122 QPS（分配与 GC 主导）|
| L03 | 提交 100 任务后 goroutine 数 | 307（基线 7 + 300，`stop` 收不回）| 20 任务：67 → `stop` 后 7 | 泄漏栈 3 段 × 100 |
| L04 | mutex profile 热点 | `sync.(*Mutex).Unlock` 下 `main.(*bugCounter).Inc` cum 92.7% | 分片锁 + `atomic` 后争抢消失 | 持锁者 vs 等待者 |
| L06 | 灌 2000 条 × 64KB | entries 2000、`heap_inuse` 126MB（线性上涨）| entries 钉在 256、`evicted` 744、23MB | 有上界 |
| L07 | `write_calls` / `avg_write_bytes` / `elapsed_ms` | 100000 / 36B / 12.98ms | 57 / 63KB / 2.88ms | `rows` 与 `bytes` 两者完全一致 |

> 环境：Apple M5（10 核）/ Go 1.26.2 / darwin-arm64。**结论看的是量级与趋势，不是这些具体数字。**

---

## 标准观测套路（三步，所有样例通用）

```mermaid
flowchart LR
    A["① 起服务<br/>go run ./cmd/lNN-xxx [-fix]"] --> B["② 造压力<br/>go run ./cmd/load -url=… -c=50 -d=10s"]
    B --> C["③ 采集<br/>go tool pprof -http=:9090 'http://127.0.0.1:1908N/debug/pprof/…'"]
    C --> D["④ 读图<br/>top -cum → peek → list 落到行"]
    D --> E["⑤ 修复复测<br/>同参数跑 -fix，数字对比"]
    style B fill:#fff9db,stroke:#f59f00
    style E fill:#e6fcf5,stroke:#087f5b,stroke-width:2px
```

**三条纪律**（比命令更重要）：

1. **采集窗口必须与压测重叠**：先让压测跑起来，再采 profile。空载采出来的 profile 只有空闲进程。
2. **单变量对照**：URL、并发、时长一字不改，只有服务端 `-fix` 这一处不同。
3. **先写预期再采集**：先写下"我应该看到什么函数"，采完再对答案——否则容易变成看图说话。

---

## pprof 常用命令速查

```bash
# ── CPU（默认 30s；必须与压测重叠）
go tool pprof -http=:9090 'http://127.0.0.1:1908N/debug/pprof/profile?seconds=30'
go tool pprof -top -cum -nodecount=30 <profile>      # 先按 cum 找链路
go tool pprof -top -nodecount=20 <profile>           # 再按 flat 找自耗
go tool pprof -list='renderBug' <profile>            # 落到源码行
go tool pprof -peek='renderBug' <profile>            # 看上下游
go tool pprof -traces <profile>                      # 看完整调用栈

# ── 内存（inuse = 现在持有谁；alloc = 谁分配得最多）
go tool pprof -top -sample_index=inuse_space  'http://127.0.0.1:1908N/debug/pprof/heap'
go tool pprof -top -sample_index=alloc_space 'http://127.0.0.1:1908N/debug/pprof/allocs'
curl -s 'http://127.0.0.1:1908N/debug/pprof/heap?gc=1' -o h1.pb.gz     # 先 GC 再采样，判泄漏的前提
go tool pprof -top -base h1.pb.gz h2.pb.gz                            # 两次快照做差：谁在涨
go tool pprof -http=:9090 -diff_base h1.pb.gz h2.pb.gz                # 带负值：谁被释放了

# ── goroutine（数量与栈：泄漏/阻塞的第一现场）
curl -s 'http://127.0.0.1:1908N/debug/pprof/goroutine?debug=1' | head -40   # 聚合栈（最快）
curl -s 'http://127.0.0.1:1908N/debug/pprof/goroutine?debug=2' | head -60   # 每个 goroutine 的完整栈

# ── block / mutex（默认关闭！本实验包已在 labkit 里统一开启采样）
go tool pprof -top -sample_index=delay       'http://127.0.0.1:1908N/debug/pprof/mutex'
go tool pprof -top -sample_index=contentions 'http://127.0.0.1:1908N/debug/pprof/mutex'
go tool pprof -top -sample_index=delay       'http://127.0.0.1:1908N/debug/pprof/block'

# ── 执行追踪（回答"时间到底去哪了"，开销大，只短窗口用）
curl -s -o trace.out 'http://127.0.0.1:1908N/debug/pprof/trace?seconds=5'
go tool trace trace.out

# ── benchmark（没有网络噪声，ns/op 与分配量最可比）
go test ./cmd/lNN-xxx -bench=. -benchmem -benchtime=3x
```

**为什么 block / mutex 在这里"一抓就有"**：`internal/labkit` 在启动时统一调用了
`runtime.SetBlockProfileRate(1)` 与 `runtime.SetMutexProfileFraction(1)`（默认都是 0 = 关闭）。
生产环境不要照抄这个全采样配置，见 [06 生产环境 pprof 实践](/架构师修炼/19-pprof实战/06-生产环境pprof实践)。

---

## 与文档的对应关系

| 你想学 | 读这篇 | 做这些样例 |
| --- | --- | --- |
| 建立观测框架：现象 → 该抓哪个 profile | [01 观测体系与 pprof 原理](/架构师修炼/19-pprof实战/01-观测体系与pprof原理) | 先读，再动手 |
| CPU 热点定位到源码行 | [02 CPU 火焰图实战](/架构师修炼/19-pprof实战/02-CPU火焰图实战) | L01、L07 |
| 内存与 GC：泄漏 / 滞留 / churn 三分类 | [03 内存与 GC 实战](/架构师修炼/19-pprof实战/03-内存与GC实战) | L02、L06 |
| goroutine 泄漏、锁竞争、channel 阻塞 | [04 goroutine 与锁阻塞实战](/架构师修炼/19-pprof实战/04-goroutine与锁阻塞实战) | L03、L04、L05 |
| 采不到数据 / 图看不懂 / 数字对不上 | [05 常见问题排查手册](/架构师修炼/19-pprof实战/05-常见问题排查手册) | 全部（当字典查） |
| 线上怎么安全地采 | [06 生产环境 pprof 实践](/架构师修炼/19-pprof实战/06-生产环境pprof实践) | L01、L02 当被观测对象 |
| 7 个完整排障故事（含面试话术） | [07 实战案例集](/架构师修炼/19-pprof实战/07-实战案例集) | L01–L07 |
| 自测与面试追问 | [08 面试题与追问链](/架构师修炼/19-pprof实战/08-面试题与追问链) | 全部，合上文档默写命令 |

上级栏目：[架构师修炼](/架构师修炼/)（QPS 六道坎、A1~A5 能力阶梯）· 相邻实验环境：[`code/architect/`](../../architect/README.md)（MySQL 主从 / Redis 哨兵 / etcd / Kafka，需要 Docker）。

---

## 常见故障

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| `go: no such tool "pprof"` | **不是 Go 移除了 pprof**（Go 1.26 起 `go tool` 是按需从 `$GOROOT/src/cmd/pprof` 源码构建）；多半是 `GOCACHE` 不可写（容器 / CI / 受限沙箱高发）| `go env GOCACHE` 确认可写，或 `GOCACHE=$(mktemp -d) go tool pprof …`；详见 [05 · F27](/架构师修炼/19-pprof实战/05-常见问题排查手册) |
| `listen tcp 127.0.0.1:1808N: bind: address already in use` | 上一次的样例进程没退干净（`go run` 的子进程可能还在）| `pgrep -fl 'l0[1-7]-'` 找到后 `kill`；或换 `-addr=127.0.0.1:28081` 重新起 |
| 业务端口能访问，`1908N/debug/pprof` 打不开 | pprof 在**独立管理端口**（业务端口 + 1000），不是业务端口 | 确认 URL 端口；容器里还要确认该端口已映射 |
| profile 里几乎全是 `runtime.*` | ① 采集时没有流量（忘了先起压测）；② macOS/darwin 上运行时与采样器自身的栈（`pthread_cond_wait` / `usleep` / `madvise` / `pthread_kill`）本来就占据顶部；③ 二进制被 strip 掉符号 | 让压测与采集重叠；用 `top -cum \| grep main.` 追业务链路；不 strip 构建 |
| `block` / `mutex` 抓出来是空的 | 默认 rate = 0 不采样 | 本实验包已在 `labkit` 里开启（`SetBlockProfileRate(1)` / `SetMutexProfileFraction(1)`）；自己写服务时要在启动处打开 |
| 压测 QPS 很低、P50 很大 | bug 模式确实很慢（这是设计目的）；也可能是压测端自己打满了 | 先看 `cmd/load` 进程自身的 CPU；bug 与 fix 用同一组参数对比，看**相对差** |
| 数字和文档表格里的实测值不同 | 机器、核数、Go 版本不同 | 正常。**只认量级与趋势**：写完自己的数字填进各样例 README 的「记录模板」 |
