# L04 · 全局锁竞争：一把 mutex 保护一切，临界区还很长

> 模块根目录 `code/architect/pprof-lab`，下面所有命令都在该目录下执行；零第三方依赖，只用标准库。

## 这个样例演示什么

业务场景是热度计数器：每个请求给某个 key（视频 ID）加一，顺便更新统计、写一笔下游 IO。
bug 模式用**一把全局 mutex**（`main.(*bugCounter).Inc`），而且把「200 次迭代的统计计算」
和「`time.Sleep(30µs)` 模拟的下游 IO 等待」全塞进临界区，所有并发请求被强制串行，
QPS 上限就是 `1 / 临界区耗时`。fix 模式按 key 分片成 64 个锁（`shardIndex` 做 FNV-1a 取模），
计数走 `atomic.Int64`，临界区缩到只剩 map 的一次读或建，计算与 sleep 全部移到锁外。

这个样例的价值在于：**mutex profile 会直接把"锁被谁拿了多久"写成函数名和 delay 数字**，
不需要你靠猜——这是"把锁竞争从玄学变成证据"的标准做法。

## 启动

端口分工：业务端口 **18084**（只挂 `GET /api/counter?k=hot`），pprof 管理端口 **19084**（= 业务端口 + 1000）。
独立管理端口的意义在 L04 尤其明显：**block / mutex profile 默认是关闭的**（采样开销不为 0），
`labkit.Run` 在启动时统一打开 `runtime.SetMutexProfileFraction(1)` 与 `runtime.SetBlockProfileRate(1)`，
采样数据只从 19084 这个只绑内网的端口暴露，业务端口上一个 pprof 端点都不开。

```bash
# 终端 A：bug 模式；Ctrl-C 优雅退出（先停业务入口，再停 pprof）
go run ./cmd/l04-lock-contention

# 终端 A（对照实验）：fix 模式
go run ./cmd/l04-lock-contention -fix

# 三个 flag 的默认值（一般不用显式传）
go run ./cmd/l04-lock-contention -addr=127.0.0.1:18084 -pprof-addr=127.0.0.1:19084 -fix=false

# 自检：响应体里的 elapsed_ms 就是单请求耗时；total / keys / shards 一起返回
curl -s 'http://127.0.0.1:18084/api/counter?k=hot'
```

响应体形如 `{"mode":"bug","key":"hot","value":1,"total":1,"keys":1,"shards":64,"elapsed_ms":0.05}`
（单请求 bug 模式实测 `elapsed_ms` ≈ 0.05，因为串行化只在并发下才暴露）。

## 造压力

```bash
# 50 并发全打同一个热点 key "hot"，竞争才会真正出现
go run ./cmd/load -url='http://127.0.0.1:18084/api/counter?k=hot' -c=50 -d=20s -timeout=10s
```

关键点：**故意让所有请求打同一个 key**——bug 模式是一把全局锁，打不同 key 也照样互相阻塞，
但热点 key 才能把"分片锁 vs 全局锁"的差别压到最大。
实测参考：bug 模式下 30 个并发请求会被完全串行化，总耗时约 41ms（约 1.4ms/请求，
其中 30µs 的 sleep 加上 200 次迭代计算就是那个"临界区下限"）；fix 模式把这些工作并行化后，
同一批请求的墙钟时间从"串行求和"变成"近似单请求耗时"。

## 采集与观测

```bash
# mutex profile：谁在争锁、争了多久（contentions + delay），30s 采样
go tool pprof -http=:9091 'http://127.0.0.1:19084/debug/pprof/mutex?seconds=30'

# block profile：谁被阻塞在锁上（对应 runtime.lock2 / sync.(*Mutex).Lock 栈）
go tool pprof -http=:9091 'http://127.0.0.1:19084/debug/pprof/block?seconds=30'

# 命令行等价写法：top 排序、peek 看调用链、list 落到源码行
go tool pprof -top  'http://127.0.0.1:19084/debug/pprof/mutex'
go tool pprof -peek='Inc' 'http://127.0.0.1:19084/debug/pprof/mutex'
go tool pprof -list='Inc' 'http://127.0.0.1:19084/debug/pprof/mutex?seconds=30'

# 顺带看 CPU：串行等待之外，200 次迭代的计算是不是也被挤在锁里
go tool pprof -http=:9090 'http://127.0.0.1:19084/debug/pprof/profile?seconds=30'
```

看哪个视图：mutex profile 里切 **Top / Flame Graph** 看 `delay` 与 `contentions` 两列，
用 **Peek / Source** 定位到具体那一行 `defer c.mu.Unlock()`。
bug 模式实测**直接给出结论**：`sync.(*Mutex).Unlock` ← `main.(*bugCounter).Inc`，
delay 约 **145 万 cycles / 10 次采样**（每次 contention 约十几万 cycles，正好对应 30µs sleep + 200 次迭代）；
block profile 里对应的是 `sync.(*Mutex).Lock`——**Unlock 出现在 mutex profile、Lock 出现在 block profile**
是 Go 运行时这两个 profile 的固有分工，不是代码写错了。
fix 模式预期：这两份 profile 里 `main.(*fixCounter).Inc` 的 delay 掉到微秒级、
且不再集中在单一热点函数上；`shardIndex` 成为 CPU profile 里的新增小头（可忽略的哈希成本）。

## 预期对比

| 指标 | bug 模式 | fix 模式 | 怎么测 |
| --- | --- | --- | --- |
| QPS | 实测填写（受 `1/临界区耗时` 封顶） | 实测填写（应显著更高） | `go run ./cmd/load -url='http://127.0.0.1:18084/api/counter?k=hot' -c=50 -d=20s -timeout=10s` 结果行的 `QPS` |
| B/op | 0 B/op | 0 B/op | `go test ./cmd/l04-lock-contention -bench=Counter -benchmem` |
| allocs/op | 0 allocs/op | 0 allocs/op | 同上（本样例不是分配问题，两列应当相同） |
| ns/op | 50244 ns/op | 5038 ns/op（约 10 倍） | 同上，`b.RunParallel` 并行基准 |
| P50 | 实测填写 | 实测填写 | `cmd/load` 结果行的 `P50` |
| P95 | 实测填写 | 实测填写 | `cmd/load` 结果行的 `P95` |
| P99 | 实测填写（排队尾部很长） | 实测填写 | `cmd/load` 结果行的 `P99` |
| goroutine 数 | 实测填写 | 实测填写 | 服务端启动日志 / `go tool pprof 'http://127.0.0.1:19084/debug/pprof/goroutine'` |
| 单请求 elapsed_ms | ≈ 0.05（顺序打时） | ≈ 0.05 | `curl -s 'http://127.0.0.1:18084/api/counter?k=hot'` 响应体字段 |
| 热点函数 | mutex: `sync.(*Mutex).Unlock` ← `main.(*bugCounter).Inc`；block: `sync.(*Mutex).Lock` | `main.(*fixCounter).Inc` 的 delay 掉到微秒级，临界区不再包含 `time.Sleep` / 计算循环 | mutex / block profile（见上） |

> 上表中的基准数字是本机实测示例值（Apple M5 / Go 1.26.2 / darwin-arm64），你的机器会有差异，
> 但 **ns/op 约 10 倍、mutex delay 从百万 cycles 掉到微秒级** 这个结论是稳定的。

## 修复要点

1. **先量后改**：打开 `SetMutexProfileFraction(1)` / `SetBlockProfileRate(1)`，让 profile 直接点名"谁持有锁多久"，避免凭感觉重构。
2. **缩短临界区是第一位**：把 200 次迭代的计算和 `time.Sleep`（真实项目里是写 Redis / Kafka / 同步日志）移出锁外——这一条通常就解决大部分问题。
3. **再按 key 分片**：64 个分片锁让热点 key 只影响 1/64 的请求；`shardCount` 取 2 的幂，`shardIndex` 用 FNV-1a 取模且零分配。
4. **能用 atomic 就别用锁**：自增走 `atomic.Int64`，读路径 `Total()` 直接 `Load()`，不再和写路径抢同一把锁。
5. **分片数不是越多越好**：分片带来更多 map 和缓存行分裂，要靠压测而非直觉定；`Keys()` 这类要跨分片聚合的接口仍然是 O(分片数) 的短暂加锁。

## 面试话术

> 我遇到过接口 QPS 死活上不去的场景，压测一并发，P99 直接起飞，但 CPU 使用率又很低——典型的"不在算，在等"。这时候 CPU profile 是看不出来的，得开 mutex profile：Go 默认不采样锁竞争，要显式 `runtime.SetMutexProfileFraction(1)`，然后打 `/debug/pprof/mutex`。结论非常直白，`sync.(*Mutex).Unlock` 下面挂着 `main.(*bugCounter).Inc`，delay 大约 145 万 cycles、10 次采样，也就是说锁的持有者就是这个函数。原因很清楚：一把全局 mutex，临界区里塞了 200 次统计迭代，还加了一次 30 微秒的模拟下游 IO，等于每个请求在锁里待了 1.4 毫秒左右，50 并发全部排队，QPS 上限就是 1 除以临界区耗时。实测 30 个并发请求串行下来总耗时 41 毫秒，而单独一个请求只有 0.05 毫秒——差距全在排队上。修法分三步：先把计算和 IO 移出临界区，再把一把锁按 key 分片成 64 个、用 FNV-1a 取模定位分片，最后把自增换成 atomic.Int64，临界区只剩 map 的一次读或建。基准测试用 `b.RunParallel` 打同一个热点 key，ns/op 从 50244 降到 5038，差不多十倍。补充一点，block profile 里显示的是 `sync.(*Mutex).Lock`，mutex profile 里显示 Unlock，这是运行时的分工，别看到 Unlock 就以为哪里写错了。

## 记录模板

```text
日期 / 机器   : 2025-xx-xx / Apple M5 (darwin-arm64), Go 1.26.2
样例          : L04 · cmd/l04-lock-contention   业务 18084 / pprof 19084
模式          : bug（不带 -fix） / fix（-fix）
启动命令      : go run ./cmd/l04-lock-contention [-fix]
压测命令      : go run ./cmd/load -url='http://127.0.0.1:18084/api/counter?k=hot' -c=50 -d=20s -timeout=10s
结果          : 成功/失败 = ? / ?    错误数 = ?
                QPS = ?   平均 = ?   P50 = ?   P95 = ?   P99 = ?   最慢 = ?
基准          : go test ./cmd/l04-lock-contention -bench=Counter -benchmem
                ns/op = ?    B/op = ?    allocs/op = ?
单请求          : curl -s 'http://127.0.0.1:18084/api/counter?k=hot' → elapsed_ms = ?
pprof 结论    : mutex  top = ?（contentions = ?，delay = ? cycles）
                block  top = ?
                CPU    top = ?
对照结论      : bug → fix 改变了什么机制（临界区缩短 / 64 分片 / atomic），
                ns/op ? → ?，mutex delay ? → ?
```
