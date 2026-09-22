# L01 · CPU 打满：反射 + JSON 序列化 + 字符串拼接

> 模块根目录 `code/architect/pprof-lab`，下面所有命令都在该目录下执行；零第三方依赖，只用标准库。

## 这个样例演示什么

业务场景是剪映「一键成片」的**同步预渲染接口**：把一个素材时间线序列化成合成引擎能吃的描述文本并算总分。
`GET /api/render?n=2000` 处理 n 条素材记录，bug / fix **算出的分数与内容完全一致**（`TestRenderModesProduceSameScore` 守着这一点），
唯一差别是写法：

- **bug 模式**：每条记录都做四件浪费 CPU 的事 ——
  1. `reflect.ValueOf(it)` + `Field(j).Interface()` 反射遍历字段（还顺带把值装箱到 `interface{}`）；
  2. `fmt.Sprintf("%v", ...)` 再走一遍反射格式化；
  3. `json.Marshal` 序列化后立刻 `json.Unmarshal` 回 `map[string]any`，**只为了读一个 `Score` 字段**；
  4. `out += item + "|"` 字符串累加，n 条就是 O(n²) 的整体拷贝。
- **fix 模式**：字段直取 + `strings.Builder`（`Grow` 预分配）+ `strconv.AppendFloat` 追加，
  只在最后 `String()` 一次，零反射、零中间字符串。

要观察的是一条完整因果链：**反射/格式化/拼接的调用次数 → on-CPU 时间 → QPS 与 P99**。
这也是第 3 类 CPU 热点（分配引发 GC）的入口，量大了要接着看 [L02](../l02-alloc-gc/)。

## 启动

端口分工：业务端口 **18081**（只挂 `GET /api/render`），pprof 管理端口 **19081**（= 业务端口 + 1000）。
`labkit.Run` 把 `net/http/pprof` 显式注册到**独立端口的 mux** 上，而不是 `import _ "net/http/pprof"` 挂上
`http.DefaultServeMux`——pprof 会泄漏 goroutine 栈、堆内容与命令行参数，绝不能跟着业务端口对公网开放。

```bash
# 终端 A：bug 模式；Ctrl-C 优雅退出（先停业务入口，再关 pprof）
go run ./cmd/l01-cpu-hotspot

# 终端 A（对照实验）：fix 模式，同端口重跑
go run ./cmd/l01-cpu-hotspot -fix

# 三个 flag 的默认值（一般不用显式传）
go run ./cmd/l01-cpu-hotspot -addr=127.0.0.1:18081 -pprof-addr=127.0.0.1:19081 -fix=false

# 自检：单请求（稳态、无并发），响应里有 mode / n / elapsed_ms / out_bytes / score
curl -s 'http://127.0.0.1:18081/api/render?n=2000'
```

`n` 的默认值 2000、上限 20000（`labkit.QueryInt` 会做上下限收敛，防止一个请求把机器打爆）。

## 造压力

```bash
# 50 并发打 10 秒（bug 模式这是"沉重"负载：约 77 QPS、P50 600ms+）
go run ./cmd/load -url='http://127.0.0.1:18081/api/render?n=2000' -c=50 -d=10s

# 逐级加压找拐点（bug 模式会很快饱和，见下表）
go run ./cmd/load -url='http://127.0.0.1:18081/api/render?n=2000' -c=200 -d=10s
go run ./cmd/load -url='http://127.0.0.1:18081/api/render?n=2000' -c=400 -d=10s
```

压测器直接输出 QPS、平均延迟、P50 / P95 / P99、成功失败数与状态码分布。
**先跑 bug 再跑 fix，URL 与参数一字不改**，只有服务端多了 `-fix`——这是单变量对照的全部纪律。

> ⚠️ **采集时不要和压测抢同一份注意力**：先让压测跑起来（它占终端），再用另一个终端采 profile。
> 采集窗口必须落在压测期间，否则 profile 里只有"空闲进程"，会得出错误结论。

## 采集与观测

```bash
# ① CPU 30s（压测进行中执行）
go tool pprof -http=:9090 'http://127.0.0.1:19081/debug/pprof/profile?seconds=30'

# ② 不开网页：先 cum 找链路，再 flat 找自耗，最后 list 落到源码行
go tool pprof -top -cum -nodecount=30 'http://127.0.0.1:19081/debug/pprof/profile?seconds=30'
go tool pprof -peek='renderBug' 'http://127.0.0.1:19081/debug/pprof/profile?seconds=30'
go tool pprof -list='renderBug' 'http://127.0.0.1:19081/debug/pprof/profile?seconds=30'

# ③ 分配视角（bug 模式每请求 142MB，顺手把因果链补全）
go tool pprof -http=:9090 -sample_index=alloc_space 'http://127.0.0.1:19081/debug/pprof/allocs'

# ④ benchmark：没有网络噪声，ns/op 与分配量最可比
go test ./cmd/l01-cpu-hotspot -bench=. -benchmem -benchtime=3x
```

**预期看到什么**（bug 模式，本机实测）：

- `top -cum` 往下找第一处**自己的代码**：`main.render` → `main.renderBug`（cum ≈ 19%），
  它下面挂着 `runtime.concatstrings` / `runtime.concatstring5`（cum ≈ 14%）——**这就是 O(n²) 字符串累加的宽度**。
  更下面还能看到 `encoding/json`、`reflect`、`fmt` 的栈。
- 反面清单：`encoding/json.*`、`reflect.*`、`fmt.Sprintf` 这类"元编程"函数本身 `flat` 通常很小，
  它们的时间在**被调用的次数**上 —— 所以**别只看 flat 排名，要顺着 cum 数调用量**。
- ⚠️ **darwin 上的噪声**：macOS 抓出来的 CPU profile 顶部常年是 `runtime.pthread_cond_wait`、`runtime.usleep`、
  `runtime.madvise`、`runtime.pthread_cond_signal`、`runtime.pthread_kill`（运行时空转 + 采样器发信号自身）。
  **这是平台特性，不是你的业务热点**，用 `-focus`/`-ignore` 剪掉，或直接 `top -cum | grep main.` 追到业务链路。
- fix 模式：`main.renderFix` 变成一条很窄的栈，`concatstrings` 基本消失，剩下的是 `memmove`（`Builder` 的连续写）。

## 预期对比

| 指标 | bug 模式 | fix 模式 | 怎么测 |
| --- | --- | --- | --- |
| ns/op | **15030764 ns/op**（15.0ms）| **199194 ns/op**（0.199ms）| `go test ./cmd/l01-cpu-hotspot -bench=. -benchmem -benchtime=3x` |
| B/op | **142638576 B/op**（142.6MB）| **340309 B/op**（0.34MB）| 同上（`-benchmem`）|
| allocs/op | **96112** | **6506** | 同上 |
| QPS（`-c=50`）| **77.5** | **11320.5** | `go run ./cmd/load -url='http://127.0.0.1:18081/api/render?n=2000' -c=50 -d=10s` |
| QPS（`-c=200`）| 113.5 | 实测填写 | 同上，改 `-c=200` |
| QPS（`-c=400`）| 121.8（**已饱和**）| 实测填写 | 同上，改 `-c=400` |
| P50（`-c=50`）| **632ms** | **1.88ms** | `cmd/load` 输出的 `P50` |
| P99（`-c=50`）| **778ms** | **23.8ms** | `cmd/load` 输出的 `P99` |
| top1 热点 | `main.renderBug` 下挂 `runtime.concatstrings` / `encoding/json` / `reflect` | `main.renderFix`（窄栈，`memmove` 为主）| `go tool pprof -top -cum` 对比两份 profile |

> **数字怎么读**：上表是本机实测示例值（Apple M5 / Go 1.26.2 / darwin-arm64），你的机器会有差异，
> 但**量级差是稳定的**：`ns/op` 差 ~75 倍、每请求分配差 ~420 倍、同负载 QPS 差 ~146 倍。
>
> **为什么 bug 模式 QPS 只有两位数**：它的瓶颈**不只是 CPU**，还有每请求 142MB 的分配把进程拖进了
> "分配 → 回收 → 再分配"的循环（`-c=400` 时 QPS 仍停在 ~120，P50 反而涨到 3.06s）。
> 这正好说明一条纪律：**看到"加并发不提吞吐、只提延迟"时，说明瓶颈不在并发度上，要去 profile 里找**。

## 修复要点

1. **能直取字段就不要反射**：结构体在编译期完全已知，`reflect` + `Interface()` 只为通用性付费，而这里并不需要通用。
2. **`fmt.Sprintf` 是最贵的通用格式化**：拼接固定结构用 `strings.Builder` + `strconv.AppendXxx`（`appendFloat` 用 `strconv.AppendFloat` 写进栈上 `[24]byte`，零分配）。
3. **别为了读一个字段做 JSON 往返**：`Marshal` 再 `Unmarshal` 是两次反射编码 + 一次 `map` 分配，直接访问字段即可。
4. **字符串累加要预分配**：`sb.Grow(n * 64)` 一次到位，避免 Builder 反复扩容与拷贝。
5. **别只看 flat 排名下结论**：反射/序列化这类"元编程"函数 `flat` 低、调用次数高，必须结合 `cum` 与调用量判断。

## 面试话术

> 我压测过一键成片的同步预渲染接口，QPS 只有两位数、P50 六百多毫秒，第一反应是"加机器"，但先采了 profile：`top -cum` 顺着调用链看到 `main.renderBug` 下面挂着 14% 的 `runtime.concatstrings`，再往下是 `encoding/json` 和 `reflect`——原来每条素材记录都在做反射遍历字段、`fmt.Sprintf` 格式化、JSON 序列化后立刻反序列化只为了读一个字段，最后还用 `out += ...` 做字符串累加，n 条记录就是 O(n²) 的整体拷贝，每请求分配 142MB。改法很直接：字段直取 + `strings.Builder` 预分配 + `strconv.AppendFloat`，`ns/op` 从 15.0ms 降到 0.199ms，每请求分配从 142.6MB 降到 0.34MB，同一负载 QPS 从 77 到 11320、P50 从 632ms 到 1.88ms。**这件事我记住的教训是：性能问题不能靠猜，`top` 的 flat 排名会骗人，要顺着 cum 数调用量；而且"加并发不提吞吐只提延迟"就是瓶颈不在并发度的信号。**

## 记录模板

```text
日期 / 机器   : 2025-xx-xx / Apple M5 (darwin-arm64), Go 1.26.2
样例          : L01 · cmd/l01-cpu-hotspot   业务 18081 / pprof 19081
模式          : bug（不带 -fix） / fix（-fix）
启动命令      : go run ./cmd/l01-cpu-hotspot [-fix]
压测命令      : go run ./cmd/load -url='http://127.0.0.1:18081/api/render?n=2000' -c=50 -d=10s
结果          : 成功/失败 = ? / ?    错误数 = ?
                QPS = ?   平均 = ?   P50 = ?   P95 = ?   P99 = ?   最慢 = ?
基准          : go test ./cmd/l01-cpu-hotspot -bench=. -benchmem -benchtime=3x
                ns/op = ?    B/op = ?    allocs/op = ?
pprof 结论    : top -cum 里第一处自己的代码 = ?（cum ?%）
                它下面是：runtime.concatstrings ?% / encoding/json ?% / reflect ?%
                噪声（darwin 平台特性）：pthread_cond_wait / usleep / madvise 占比 = ?
对照结论      : bug → fix 改变了什么机制（?），数字变化（? → ?）
                加并发是否还提吞吐（拐点在哪）：
遗留疑问      :
```
