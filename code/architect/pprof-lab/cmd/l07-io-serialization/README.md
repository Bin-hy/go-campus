# L07 · 逐行 fmt.Fprintf 导出 CSV：反射格式化 + 海量分配

> 模块根目录 `code/architect/pprof-lab`，所有命令都在该目录下执行。零第三方依赖，只用标准库；`go run` 直接跑，加 `-fix` 切换修复实现。

## 这个样例演示什么

业务场景是**剪辑任务的 CSV 导出**：`GET /api/export?n=20000` 生成 n 行记录（默认 20000，`labkit.QueryInt` 收敛到 1~200000），响应 `Content-Type: text/csv`，同时用 `X-Mode` 头标出当前模式。

- **bug 模式**（默认）：`exportBug` 逐行 `fmt.Fprintf(w, "%d,%s,%s,%d,%d,%d\n", ...)`。每行都要走一次反射格式化——解析格式串、把 6 个参数装箱成 `[]any`、再分配一个临时字符串；而且每行都是一次独立的写，没有任何缓冲层把小写合并成大写。
- **fix 模式**：`exportFix` 用 `bufio.NewWriterSize(w, 64<<10)` + 预分配的 `line`（64B）与 `batch`（32KB）+ `strconv.AppendInt` 手写编码，攒够 32KB 才真正批量写一次；整个导出过程几乎只在开头分配那两块内存，之后全部复用。
- **诚实说明**：`net/http` 自身对 `ResponseWriter` 有 2KB~4KB 的缓冲，所以"逐行写"的代价**主要来自 fmt 的反射格式化和每次分配**，而不是"每行真的发一次 syscall"——这点不要夸大，profile 会替你说话：bug 模式的 CPU 时间集中在 `fmt` 内部和分配器上，不在 `write(2)` 上。
- 有一个测试专门守住"优化不改变行为"：`l07_test.go` 的 `TestExportModesProduceSameCSV` 断言 fix 与 bug 两种实现的输出**逐字节一致**（字节数相同、内容 `bytes.Equal`、行数等于 rows）。

## 启动

```bash
cd code/architect/pprof-lab

# bug 模式：业务 :18087，pprof :19087（Ctrl-C 优雅退出：停业务入口 → 停 pprof）
go run ./cmd/l07-io-serialization

# fix 模式：bufio + 预分配 + 批量写，端口同上（同样 Ctrl-C 优雅退出）
go run ./cmd/l07-io-serialization -fix

# 需要改端口时（两个 flag 就是全部可调项）
go run ./cmd/l07-io-serialization -addr=127.0.0.1:18087 -pprof-addr=127.0.0.1:19087
```

端口分工：业务 mux 只挂在 `-addr`（默认 `127.0.0.1:18087`）上，只暴露 `/api/export`；`net/http/pprof` 由 `labkit` **显式注册**到独立的 `-pprof-addr`（默认 `127.0.0.1:19087`）。单独分一个端口的原因：pprof 会暴露 goroutine 栈、堆内容、命令行参数，**绝不能挂在业务端口对公网开放**；生产上应绑内网/管理网卡，并用安全组或 NetworkPolicy 收紧来源。启动日志会打印业务地址、pprof 地址和当前模式各一行。

先做一次冒烟（两种模式都应返回 200 + `text/csv`）：

```bash
curl -sS -D- -o/dev/null 'http://127.0.0.1:18087/api/export?n=1000'
```

## 造压力

```bash
# 50 并发压 20 秒，单请求超时 10s；压测期间另开终端采集 profile
go run ./cmd/load -url='http://127.0.0.1:18087/api/export?n=20000' -c=50 -d=20s -timeout=10s
```

`-url` / `-c` / `-d` / `-timeout` 就是压测器的全部 flag；`n` 通过 URL query 控制（默认 20000）。`-d` 到点后压测器**只停止投放新请求**，在途请求正常跑完、不主动取消，所以收尾不会被误记成失败——但这也意味着收尾阶段会多出几个请求，比较两组数时看 QPS 与分位数即可。

## 采集与观测

```bash
# ① CPU profile：bug 模式的热点全在 fmt 与分配器里
go tool pprof -http=:9090 'http://127.0.0.1:19087/debug/pprof/profile?seconds=30'

# ② heap / allocs：看 alloc_objects 有多离谱
go tool pprof -http=:9090 'http://127.0.0.1:19087/debug/pprof/heap'

# ③ goroutine profile：确认没有异常堆积（L07 不是 goroutine 问题，做对照用）
go tool pprof -http=:9090 'http://127.0.0.1:19087/debug/pprof/goroutine'
```

视图选择：**top** 看 flat / cum 排名；**flame graph** 看 `fmt` 内部展开的层级与占比；**peek** 看 `main.exportBug` / `main.exportFix` 的调用者与被调用者；选中函数后 **list** 直接定位到具体行（比如 `exportBug` 里那行 `fmt.Fprintf`）。heap profile 记得切到 **`alloc_objects` / `alloc_space`**：GC 之后 `inuse` 会掉下来，但 `alloc_objects` 记录的是累计分配次数，正好对应"每行分配一次临时字符串"。分配速率高时，CPU profile 里还会看到 **GC assist**（`runtime.gcAssistAlloc`）被算进用户态 CPU——那是分配把 GC 的活摊派给了业务 goroutine。

```text
# bug 模式 CPU profile —— top（示意结构，实际数字以你的采集为准）
      flat  flat%   cum
      1.4s    35%   fmt.Fprintf
      1.1s    27%   fmt.(*pp).doPrintf
      0.6s    15%   runtime.convT64          ← 参数装箱成 []any
      0.5s    12%   runtime.mallocgc         ← 每行一个临时字符串
                                       ...   main.exportBug

# fix 模式：热点变成 main.exportFix 里的批量写路径（strconv.AppendInt / bufio 写），
# fmt 相关帧基本消失，alloc_objects 从六位数掉到个位数
```

## 预期对比

基准测试（本机实测示例值，Apple M5 / Go 1.26.2 / darwin-arm64，`n=20000` 行）：

```bash
go test ./cmd/l07-io-serialization -bench=Export -benchmem -benchtime=30x
```

| 指标 | bug 模式 | fix 模式 | 怎么测 |
| --- | --- | --- | --- |
| QPS | 实测填写 | 实测填写 | `go run ./cmd/load -url='http://127.0.0.1:18087/api/export?n=20000' -c=50 -d=20s -timeout=10s`，读 `QPS` 行，两种模式各跑一遍 |
| 分配量（B/op、allocs/op） | **1115358 B/op，99353 allocs/op**（本机实测示例值） | **147456 B/op，3 allocs/op**（本机实测示例值） | 上面那条 `go test -bench=Export -benchmem -benchtime=30x`；HTTP 场景可用 heap profile 的 `alloc_objects` 交叉验证 |
| 单次导出耗时（benchmark，非 HTTP） | **4028021 ns/op（约 4.0ms）**（本机实测示例值） | **1010532 ns/op（约 1.0ms）**（本机实测示例值） | 同上 benchmark（`discard{}` 零分配 Writer，差异来自格式化与缓冲而非下游） |
| P50 / P95 / P99 | 实测填写 | 实测填写 | `cmd/load` 那条命令，读 `P50` / `P95` / `P99` 三行（建议同一台机器、同一次会话内连跑两组） |
| goroutine 数 | 约等于在途 handler 数 | 同左（L07 不是 goroutine 问题） | `curl -s 'http://127.0.0.1:19087/debug/pprof/goroutine?debug=1' \| head -30` |
| 热点函数 | CPU：`fmt.Fprintf` / `fmt.(*pp).doPrintf`，分配侧 `runtime.convT64` / `runtime.mallocgc`；heap `alloc_objects` 巨大 | CPU：`main.exportFix` 的批量写路径（`strconv.AppendInt` + `bufio` 写）；`fmt` 帧消失，allocs/op 降到 3 | 上面 ① ② 两条 `go tool pprof` 命令 |

行为正确性：单次请求实测 `/api/export?n=1000` 返回 200 + `text/csv`，正常；`go test ./cmd/l07-io-serialization -run TestExportModesProduceSameCSV -v` 可确认 fix 与 bug 输出逐字节一致。

## 修复要点

1. **去掉逐行的反射格式化**：用 `strconv.AppendInt` 把数字直接追加进 `[]byte`，不经过 `fmt` 的格式串解析与 `[]any` 装箱，也省掉了每行的临时字符串。
2. **预分配并复用缓冲区**：`line` 容量 64B、`batch` 容量 32KB，一次分配、循环里 `line = line[:0]` 复用，分配从"每行一次"降到"整个导出 3 次"（本机实测 99353 → 3 allocs/op）。
3. **加显式缓冲 + 批量写**：`bufio.NewWriterSize(w, 64<<10)` 兜住零散写入，`batch` 攒够 32KB 才 `bw.Write` 一次，最后 `bw.Flush()` 收尾——写入次数从"每行一次"降到"每 32KB 一次"。
4. **优化不能改语义**：用 `TestExportModesProduceSameCSV` 做逐字节等价性回归，`X-Mode` / `Content-Type` 等响应头在写 body 之前设置（写出去之后改就无效了）。
5. **还有优化空间（可讲但不必做）**：行格式固定时可以先扫一遍预估总长度、提前 `Content-Length`，避免 chunked 传输；再进一步可以让生产者 + 消费者并发，一边生成一边写，但要小心别把分配和 goroutine 数又推回去。

## 面试话术

> 我压过一个 CSV 导出接口，`/api/export?n=20000`，bug 版是逐行 `fmt.Fprintf(w, ...)`。看起来很自然，但每行都干三件重活：解析格式串走反射、把 6 个参数装箱成 `[]any`、再分配一个临时字符串。benchmark 里 `n=20000` 行是 4028021 ns/op、1115358 B/op、**99353 allocs/op**，QPS 上不去，P99 也很毛。CPU profile 一打就明白：top 是 `fmt.Fprintf` 和 `fmt.(*pp).doPrintf`，下面挂着 `runtime.convT64` 和 `runtime.mallocgc`；heap profile 切到 `alloc_objects` 是六位数，分配速率一高，CPU 里还会冒出 GC assist。这里有个细节我会主动说清楚：`net/http` 自己对 `ResponseWriter` 有 2KB~4KB 缓冲，所以逐行写的代价主要是反射和分配，不是真的每行一次 syscall，别把话说满。修复就是 `bufio.NewWriterSize(w, 64<<10)` + 预分配 64B 的 `line` 和 32KB 的 `batch` + `strconv.AppendInt` 手写编码 + 攒够 32KB 批量写，结果变成 1010532 ns/op、147456 B/op、**3 allocs/op**，`fmt` 的帧基本从 profile 里消失。而且我用 `TestExportModesProduceSameCSV` 保证两种实现输出逐字节一致，性能优化不动语义；再往上还有空间，比如行格式固定时预估总长度提前设 `Content-Length`，避免 chunked。

## 记录模板

```text
日期/机器：        2025-XX-XX / Apple M5、Go 1.26.2、darwin-arm64
样例：             L07 io-serialization（业务 :18087 / pprof :19087）
模式：             bug | fix
命令：             go run ./cmd/l07-io-serialization [-fix]
压测：             go run ./cmd/load -url='http://127.0.0.1:18087/api/export?n=20000' -c=50 -d=20s -timeout=10s
benchmark：        go test ./cmd/l07-io-serialization -bench=Export -benchmem -benchtime=30x
结果：             QPS=____  P50=____  P95=____  P99=____  成功/失败=____/____  错误分布=____
                   ns/op=____  B/op=____  allocs/op=____
采集：             go tool pprof -http=:9090 'http://127.0.0.1:19087/debug/pprof/profile?seconds=30'
                   go tool pprof -http=:9090 'http://127.0.0.1:19087/debug/pprof/heap'
pprof 结论：       CPU top = ____（flat/cum=____）
                   heap alloc_objects 顶部 = ____
正确性：           TestExportModesProduceSameCSV 通过/失败；curl n=1000 状态码 = ____
是否复现预期：     是/否；不一致的地方与猜测原因 = ____
一句话结论：       ____
```
