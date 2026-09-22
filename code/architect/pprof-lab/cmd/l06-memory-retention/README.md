# L06 · 内存只涨不降：全局 map 长期持有

> 模块根目录 `code/architect/pprof-lab`，下面所有命令都在该目录下执行；零第三方依赖，只用标准库（LRU 用 `container/list`）。

## 这个样例演示什么

业务场景是素材缩略图的**进程内缓存**：`GET /api/cache/put?kb=256&n=100` 写入 n 条、每条 kb KB 的数据。
两种实现的差别只有一个：**有没有上界**。

- **bug 模式**：一个全局 `map[string][]byte`——**没有容量上限、没有 TTL、没有淘汰**。
  只要 key 不同就一直长。注意这些内存**严格来说不是"泄漏"**（GC 完全看得见它们，
  `heap_objects` 也在正常统计），而是被业务代码**一直强引用**着，所以 GC 无能为力。
  这正是"内存只涨不降"最常见的一类：**不是运行时的问题，是容器的生命周期问题。**
- **fix 模式**：带**容量上限 + TTL + 字节上限**的 LRU（`container/list` 实现），
  超限就淘汰最久未使用的条目、过期条目由后台 janitor 清理，占用有明确上界。

要观察的是**判别方法**：先用 `/api/cache/stats` + `heap` 两次快照做差，判断到底是
**"真泄漏"** 还是 **"被强引用的滞留"** 还是 **"分配太猛（churn）"**——这三类的修法完全不同。

## 启动

端口分工：业务端口 **18086**（`/api/cache/put`、`/api/cache/stats`），pprof 管理端口 **19086**。

```bash
# 终端 A：bug 模式
go run ./cmd/l06-memory-retention

# 终端 A（对照实验）：fix 模式
go run ./cmd/l06-memory-retention -fix

# 三个 flag 的默认值
go run ./cmd/l06-memory-retention -addr=127.0.0.1:18086 -pprof-addr=127.0.0.1:19086 -fix=false
```

fix 模式的三个常量就是"上界"本身：`maxEntries = 256`、`maxBytes = 32MB`、`ttl = 30s`、`sweepEvery = 5s`。

| 端点 | 作用 | 关键字段 |
| --- | --- | --- |
| `GET /api/cache/put?kb=64&n=1000` | 写 n 条、每条 kb KB | `written_mb`、`evicted`、`cache_entries`、`capacity` |
| `GET /api/cache/put?kb=64&n=1&key=fixed` | 固定 key（反复覆盖同一条，用来对比"新 key 才涨"）| 同上 |
| `GET /api/cache/stats` | 内存快照 | `entries`、`capacity`、`unbounded`、`cache_mb`、`evictions`、`heap_inuse_mb`、`heap_objects`、`num_gc` |

> `/api/cache/stats` 里的 `runtime.ReadMemStats` **会短暂 STW**，所以它只挂在观测接口上、绝不放进业务热路径。

## 造压力

```bash
# bug 模式：灌 1000 条 × 64KB = 62.5MB，`capacity:-1` 表示无上限
curl -s 'http://127.0.0.1:18086/api/cache/put?n=1000&kb=64'
# {"cache_entries":1000,"cache_mb":-1,"capacity":-1,"entries":1000,"evicted":0,"mode":"bug","written_mb":62.5}
curl -s 'http://127.0.0.1:18086/api/cache/stats'
# {"entries":1000,"heap_inuse_mb":63,"heap_objects":2912,"num_gc":5,"unbounded":true,...}

# 再灌一次 → 只涨不降（这条曲线就是问题本身）
curl -s 'http://127.0.0.1:18086/api/cache/put?n=1000&kb=64' >/dev/null
curl -s 'http://127.0.0.1:18086/api/cache/stats'
# {"entries":2000,"heap_inuse_mb":126,"heap_objects":4907,...}     ← 63MB → 126MB，线性上涨
```

fix 模式的对照（同样的 1000 条 × 64KB）：

```bash
curl -s 'http://127.0.0.1:18086/api/cache/put?n=1000&kb=64'
# {"cache_entries":256,"cache_mb":16,"capacity":256,"entries":1000,"evicted":744,"mode":"fix","written_mb":62.5}
curl -s 'http://127.0.0.1:18086/api/cache/stats'
# {"entries":256,"capacity":256,"unbounded":false,"evictions":744,"heap_inuse_mb":23,...}  ← 有上界
```

## 采集与观测

```bash
# ① 先判性质：条目数在涨还是对象数在涨？——"引用滞留"看条目，"churn"看分配量
curl -s 'http://127.0.0.1:18086/api/cache/stats'

# ② 快照 1（先 GC 再采样，去掉"已死未清扫"的干扰）
curl -s 'http://127.0.0.1:19086/debug/pprof/heap?gc=1' -o /tmp/l06-heap-1.pb.gz

# ③ 灌数据（bug 模式再灌 1000 条）
curl -s -o /dev/null 'http://127.0.0.1:18086/api/cache/put?n=1000&kb=64'

# ④ 快照 2，然后做差：负值代表"新版本变少了"，用 -base 看"谁在涨"最干净
curl -s 'http://127.0.0.1:19086/debug/pprof/heap?gc=1' -o /tmp/l06-heap-2.pb.gz
go tool pprof -http=:9090 -base /tmp/l06-heap-1.pb.gz /tmp/l06-heap-2.pb.gz
go tool pprof -top -sample_index=inuse_space -base /tmp/l06-heap-1.pb.gz /tmp/l06-heap-2.pb.gz

# ⑤ 常驻视角 vs 分配视角（判断是"滞留"还是"churn"）
go tool pprof -top -sample_index=inuse_space  'http://127.0.0.1:19086/debug/pprof/heap'
go tool pprof -top -sample_index=alloc_space 'http://127.0.0.1:19086/debug/pprof/allocs'
```

**预期看到什么**：

- bug 模式：`-base` 差值的最大来源是 `main.(*mapStore).Put` → `make([]byte, kb<<10)` 的分配点，
  也就是**业务代码主动持有**的那块内存；`inuse_space` 随灌数据线性上涨，`alloc_space` 也大但两者同步。
- fix 模式：`inuse_space` 很快稳定在 `maxBytes` 量级（32MB 以内），`evictions` 随写入增长而增长，
  `entries` 钉在 `capacity=256`。等 30s（`ttl`）后再看 `stats`，条目会进一步被 janitor 清掉。
- **别把 `cache_mb:-1` 当成 bug**：`-1` 是"这条实现不统计字节数"的约定值（bug 模式无上界，没必要统计）。

## 预期对比

| 指标 | bug 模式 | fix 模式 | 怎么测 |
| --- | --- | --- | --- |
| 写入 1000 × 64KB 后 `entries` | **1000** | **256**（= `capacity`）| `GET /api/cache/stats` |
| `capacity` / `unbounded` | `-1` / `true` | `256` / `false` | 同上 |
| `evicted` | **0**（永不淘汰）| **744** | `GET /api/cache/put` 响应 |
| `heap_inuse_mb`（1 次灌入）| **63MB** | **23MB** | `GET /api/cache/stats` |
| `heap_inuse_mb`（2 次灌入）| **126MB**（线性上涨）| 稳定在 ~32MB 以内 | 同上，灌两次 |
| `heap` 两次快照做差 | 增长点 = `mapStore.Put` 的 `make([]byte,…)` | 无明显正增长 | `go tool pprof -base` 对比 |
| 等待 `ttl`（30s）后 | 不变 | janitor 清理过期条目 | 等 30s 再 `stats` |
| 结论分类 | 不是"泄漏"，是**被强引用的滞留** | 有上界，可预测 | —— |

> **数字怎么读**：上表是本机实测示例值（Apple M5 / Go 1.26.2 / darwin-arm64）。
> 要记住的是**判别方法**：`entries` 单调上涨 + `-base` 差值落在业务容器上 = 滞留；
> `inuse_space` 平稳而 `alloc_space` 巨大 = churn（去 [L02](../l02-alloc-gc/)）；
> `-base` 差值落在 goroutine 持有/未关闭资源上 = 真泄漏（去 [L03](../l03-goroutine-leak/)）。

## 修复要点

1. **任何进程内缓存都必须有上界**：条目数上限 + 字节上限 + TTL，三者缺一不可（只有条目上限时，大 value 照样打爆内存）。
2. **淘汰要真的解除引用**：LRU 淘汰时必须把 `[]byte` 从 map 与链表里都摘掉，只挪链表不删 map 等于没淘汰。
3. **TTL 需要一个清理者**：读取时校验 TTL（惰性）+ 后台 janitor 定期 sweep（主动），否则"没人访问的过期条目"会一直占内存。
4. **容量按内存算，不按直觉算**：`maxEntries × 单条大小` 就是最坏占用，本样例 256 × 64KB ≈ 16MB，配 `maxBytes` 兜底。
5. **区分"有意保留"和"泄漏"**：调试用的环形缓冲、连接池、热数据缓存都是有意的常驻内存；
   排查时看的是**斜率**（是否随时间单调上涨）与**上界**（有没有天花板），不是绝对值。

## 面试话术

> 我排查过一个缩略图缓存服务，内存曲线只涨不降、深夜 OOM。第一步不是去看 heap 里哪个函数最大，而是先判性质：`/api/cache/stats` 显示缓存条目数在单调上涨、`capacity` 是 `-1`——说明**这块数据是被业务代码一直强引用的，GC 其实看得见它，只是收不掉**，所以它不是"内存泄漏"，是"容器没有生命周期管理"。第二步才去取证：用 `heap?gc=1` 采两次快照做 `-base` 差，增长点精确落在 `mapStore.Put` 里那次 `make([]byte, kb<<10)` 上，增量正好等于我灌进去的 62.5MB。修法就是给缓存装上界：条目上限 256 + 字节上限 32MB + 30 秒 TTL，超出淘汰最久未使用，后台 janitor 清过期。修完同样灌 1000 条 × 64KB，`entries` 停在 256、`evicted` 744、`heap_inuse` 从 126MB 降到 23MB 并有明显天花板。**我记住的教训是：内存问题的第一刀是分性质——泄漏、滞留、churn 三类的修法完全不同，先分清再优化。**

## 记录模板

```text
日期 / 机器   : 2025-xx-xx / Apple M5 (darwin-arm64), Go 1.26.2
样例          : L06 · cmd/l06-memory-retention   业务 18086 / pprof 19086
模式          : bug（不带 -fix） / fix（-fix）
启动命令      : go run ./cmd/l06-memory-retention [-fix]
写数据命令    : curl -s 'http://127.0.0.1:18086/api/cache/put?n=1000&kb=64'
观测命令      : curl -s 'http://127.0.0.1:18086/api/cache/stats'
快照做差      : curl -s 'http://127.0.0.1:19086/debug/pprof/heap?gc=1' -o heap-{1,2}.pb.gz
                go tool pprof -top -sample_index=inuse_space -base heap-1.pb.gz heap-2.pb.gz
第一次灌入后  : entries = ?   capacity = ?   evicted = ?   heap_inuse_mb = ?
第二次灌入后  : entries = ?                    heap_inuse_mb = ?
性质判定      : 泄漏 / 强引用滞留 / churn（依据：entries 斜率 ? 与 -base 增长点 ?）
增长点        : 函数 = ?（行号 = ?）
修复上界      : maxEntries = ?   maxBytes = ?   ttl = ?
等待 ttl 后   : entries = ?   evictions = ?
对照结论      : bug → fix 改变了什么机制（?），数字变化（? → ?）
遗留疑问      :
```
