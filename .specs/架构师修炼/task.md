# 架构师修炼栏目 Task

> 依据：`spec.md` / `plan.md`。T 编号为交付项，括号内是验收动作。

## T1 文档层 · 方法论（2 篇）

- [x] T1.1 `index.md` 栏目总览：A1~A5 能力阶梯、六道坎总表与演进图、学习地图、每篇统一结构、里程碑 M1~M6、与现有内容衔接（渲染通过 + 含 mermaid）
- [x] T1.2 `00-架构师思维与设计方法论.md`：五步法、5 个估算公式、必背数量级表、完整算例（AI 剪辑平台）、找瓶颈 6 问、决策矩阵模板、演进四件套、一致性决策树、白板 6 步模板、反面清单、追问链、自测清单
- [x] T1.3 `01-QPS分级与架构演进地图.md`：六道坎量化特征、**跨量级做法对照总表**、成本表、触发指标阈值表、P0~P4 数据分级

## T2 文档层 · 演进六阶段（6 篇）

- [x] T2.1 `02-单体架构的极限与分层.md`（坎 0→1）
- [x] T2.2 `03-MySQL主从与读写分离落地.md`（坎 1→2）
- [x] T2.3 `04-Redis高可用与缓存体系落地.md`（坎 2→3）
- [x] T2.4 `05-Kafka削峰与可靠投递落地.md`（坎 3）
- [x] T2.5 `06-分库分表与在线迁移双写.md`（坎 3→4）
- [x] T2.6 `07-多活容灾与全球化架构.md`（坎 4→5）

## T3 文档层 · 一致性专题（4 篇）

- [x] T3.1 `08-缓存与DB一致性落地.md`
- [x] T3.2 `09-分布式事务与最终一致落地.md`
- [x] T3.3 `10-etcd与Raft选主租约落地.md`
- [x] T3.4 `11-幂等去重与ExactlyOnce.md`

## T4 文档层 · 稳定性工程（3 篇）

- [x] T4.1 `12-限流熔断降级与背压.md`
- [x] T4.2 `13-容量规划压测与故障演练.md`
- [x] T4.3 `14-备份恢复与故障复盘.md`

## T5 文档层 · 综合实战（4 篇）

- [x] T5.1 `15-案例-AI剪辑任务平台架构.md`（按五步法组织，含任务状态机 DDL 与成本控制）
- [x] T5.2 `16-案例-Feed与社区互动架构.md`（推拉结合、计数、热点、游标分页）
- [x] T5.3 `17-面试-白板架构设计与追问链.md`（7 步答题、6 道题、10 类追问模板、评分维度、7 天冲刺）
- [x] T5.4 `18-实验手册-Go落地实验.md`（15 个实验、记录模板、简历写法、常见坑）

## T6 代码层 · 实验编排

- [x] T6.1 `code/architect/docker-compose.yml`（`docker compose config -q` 通过；`--profile cluster` 通过）
- [x] T6.2 `conf/mysql/master.cnf`（GTID + ROW + 半同步 + sync_binlog=1）
- [x] T6.3 `conf/mysql/replica.cnf`（并行复制 + super_read_only）
- [x] T6.4 `conf/mysql/master-init.sql`（repl 账号 + account/big_table + 迁移库）
- [x] T6.5 `conf/redis/sentinel.conf`（模板，`__PORT__` 占位）
- [x] T6.6 `scripts/init-replication.sh`（可执行权限 + GTID 自动定位）
- [x] T6.7 `code/architect/README.md`（端口表 / 配置要点 / 常态命令 / 故障注入速查 / 记录模板）

## T7 站点集成

- [x] T7.1 `docs/.vitepress/config.mts`：新增 nav「架构师修炼」下拉（19 项）+ `/架构师修炼/` 侧边栏（开始 / A / B / C / D / E / 相关链接）
- [x] T7.2 `docs/index.md`：feature 卡片 + 推荐学习顺序第 14 条
- [x] T7.3 `docs/后端技术栈强化/index.md`：新增「下一步：架构师修炼」映射表，backlog 标记为已承接
- [x] T7.4 根 `README.md`：学习阶段表新增一行

## T8 验收

- [x] T8.1 栏目内相对链接与 `/架构师修炼/` 绝对链接全部命中现有文件（脚本校验）
- [x] T8.2 每篇含导航块 / 解决问题 / ≥2 mermaid / ≥3 表格 / 故障与一致性边界 / 面试追问链 / 自测清单（脚本抽查）
- [x] T8.3 `npm run docs:build` 成功（无死链、无 mermaid 报错）
- [x] T8.4 mermaid 围栏配对检查（每个文件 ``` 数量为偶数）
- [x] T8.5 交付说明：栏目入口、篇章清单、使用方式、实验环境启动方式

---

# 扩展：T9 子栏目「19 pprof 实战」（性能调优取证）

> 动机：T2.1（02 单体架构的极限与分层）给出了四个天花板与四个 pprof 命令，但「拿到现象该敲哪条命令、看哪个视图、怎么证明改对了」缺少可复现的教学载体；18 实验手册的 E01 只有一个实验，撑不起面试里「性能优化」的追问。

## T9.1 文档层（9 篇：index + 01~08）

- [x] T9.1.1 `19-pprof实战/index.md`：观测决策图（现象 → 五类 profile 分流）、8 篇依赖图、8 个实验清单（端口表）、五步闭环、**与架构学习关联映射表**（02/01/12/13/14/15/17/18 + S8 + K8s + 路线专题 04）、里程碑 P1~P6
- [x] T9.1.2 `01-观测体系与pprof原理.md`：metrics/expvar/trace/pprof 全景、profile 类型总表、CPU 采样原理（SIGPROF 100Hz、只统计 on-CPU）、heap 采样（MemProfileRate）与四视图、火焰图读法、命令速查
- [x] T9.1.3 `02-CPU火焰图实战.md`：top/top -cum/peek/list/traces 方法论、8 类 CPU 热点成因对照、`-base` 对比、L01 与 L07 全流程演练
- [x] T9.1.4 `03-内存与GC实战.md`：泄漏 vs churn 判定、三类泄漏判别、逃逸分析读法、减分配手段表、GOGC/GOMEMLIMIT 取舍、L02 与 L06 演练
- [x] T9.1.5 `04-goroutine与锁阻塞实战.md`：goroutine 泄漏 6 形态、block/mutex profile 开启与判读、锁优化对照表（分片锁/RWMutex/atomic/sync.Map/channel/batching）、死锁与全栈打印、L03/L04/L05 演练
- [x] T9.1.6 `05-常见问题排查手册.md`：27 条 FAQ（抓不到数据 / 图看不懂 / 环境与工具 / 优化验证），统一「现象 → 原因 → 解决 → 验证」四段式 + 30 秒定位表
- [x] T9.1.7 `06-生产环境pprof实践.md`：三条铁律、安全暴露 Go 代码、开销自测方法论、K8s 采集、持续 profiling 三档方案、排障 SOP、简历表述模板
- [x] T9.1.8 `07-实战案例集.md`：7 个完整案例（AI 剪辑业务语境，对应 L01~L07）+ 案例复盘总表
- [x] T9.1.9 `08-面试题与追问链.md`：30 道题（基础/定位/优化/生产/陷阱）+ 常见错误回答 + 10 条追问链 + 评分表 + 命令默写清单

## T9.2 代码层 · `code/architect/pprof-lab/`

- [x] T9.2.1 独立 module（`module gocampus/perf/pprof-lab`，go 1.22），**零第三方依赖**，`go build ./... && go vet ./... && go test ./...` 全绿
- [x] T9.2.2 8 个样例：L01 CPU 热点 / L02 分配与 GC / L03 goroutine 泄漏 / L04 锁竞争 / L05 channel 阻塞 / L06 内存滞留 / L07 IO 与序列化 + `cmd/load` 自建压测器
- [x] T9.2.3 统一约定：业务端口 `1808N`、pprof 管理端口 `1908N`（独立端口）、flag `-addr` / `-pprof-addr` / `-fix`（默认 false = bug 版）
- [x] T9.2.4 每个样例含 `README.md`（目标 / 启动 / 造压力 / 采集观测 / 预期对比 / 修复要点 / 面试话术 / 记录模板）—— 8 份：整包 + L01~L07
- [x] T9.2.5 L01/L02/L04 含成对 benchmark（`BenchmarkXxxBug` / `BenchmarkXxxFix`，带 `-benchmem`）；L03/L06/L07 含行为测试（收敛性 / LRU 淘汰与 TTL / 两模式内容一致）
- [x] T9.2.6 L07 提供 `GET /api/export?stats=1`（`countingWriter` 统计 Write 调用与字节数），字段契约 `mode/rows/bytes/write_calls/avg_write_bytes/elapsed_ms` 与 07 篇一致

## T9.3 站点集成与互链

- [x] T9.3.1 `config.mts`：nav 新增「pprof 性能调优」下拉（9 项）+ 架构师修炼下拉追加 19 项 + 侧边栏新增「F · 性能调优实战（pprof）」分组
- [x] T9.3.2 `架构师修炼/index.md`：学习地图新增 F 组、里程碑 M2 补 pprof 子栏目、第六节衔接图与产出补链接
- [x] T9.3.3 `02-单体架构的极限与分层.md`：pprof 命令块后加「展开篇」提示、2.2 节补生产实践与 FAQ 链接
- [x] T9.3.4 `18-实验手册-Go落地实验.md`：E01 表格行与实验卡补「展开版」链接
- [x] T9.3.5 `docs/index.md` 推荐顺序第 15 条、`docs/后端技术栈强化/index.md` 映射表新增一行、根 `README.md` 栏目表/阶段表/details/推荐顺序
- [x] T9.3.6 `code/architect/README.md` 新增「配套：性能定位实验包」小节
- [x] T9.3.7 `路线专题/04-简历项目改造与面试实战.md`：pprof 五步案例补完整版链接

## T9.4 验收（均为实跑结果，非声明）

- [x] T9.4.1 `gofmt -l .` 无输出；`go build ./...` / `go vet ./...` / `go test ./...` 全部通过（7 个样例 + load + labkit）
- [x] T9.4.2 死链校验：脚本扫描 21 个文件 574 条链接，0 缺失（含 8 篇正文、index、被改动的既有篇章与根 README）
- [x] T9.4.3 mermaid 围栏配对检查（每文件 ``` 数量为偶数）+ 结构自检（8 篇均含导航块 / ≥2 mermaid / ≥3 表格 / 面试追问链 / 自测清单）
- [x] T9.4.4 `npm run docs:build` 成功（build complete in 34.05s，无死链、无 mermaid 报错）
- [x] T9.4.5 **端到端实跑**：7 个样例全部启动成功，业务端点与 `/debug/pprof/{profile,heap,allocs,goroutine,block,mutex,threadcreate,cmdline,symbol}` 均返回 200
- [x] T9.4.6 **bug vs fix 实测**（Apple M5 / 10 核 / Go 1.26.2）：
  - L01 benchmark：`BenchmarkRenderBug` 15.03 ms/op、142.6 MB/op、96112 allocs/op → `BenchmarkRenderFix` 0.199 ms/op、0.34 MB/op、6506 allocs/op（**75× 提速、420× 分配下降**）
  - L01 HTTP：bug `-c=50 -d=10s` ≈ 77 QPS / P50 632ms（分配与 GC 主导，加压至 `-c=400` 仅 ~120 QPS 饱和）→ fix 同参数 ≈ **11320 QPS / P50 1.88ms**
  - L03：每次 `start?n=20` 泄漏 300 个 goroutine，`goroutine?debug=1` 显示 3 段各 100 的泄漏栈（带 `main.go:60/66` 行号）；bug 模式 `/api/task/stop` 返回 `stopped:false`（无法收敛）
  - L04：mutex profile 中 `sync.(*Mutex).Unlock` flat 100%、`main.(*bugCounter).Inc` cum 92.7%（**持锁者栈**）；block profile 中 `sync.(*Mutex).Lock`（**等待者栈**）——与 04 篇的判读口径一致
  - L06：灌 1000 条 × 64KB → entries 1000、`heap_inuse` 63MB；再灌一次 → entries 2000、126MB（线性上涨、`capacity:-1`）；fix 同参数 → entries 钉在 256、`evicted` 744、23MB
  - L07：`?stats=1` bug `{"rows":100000,"bytes":3689490,"write_calls":100000,"avg_write_bytes":36,"elapsed_ms":12.978}` → fix `{"write_calls":57,"avg_write_bytes":64727,"elapsed_ms":2.878}`，`rows`/`bytes` 两模式完全一致（证明只是少写了系统调用）
- [x] T9.4.7 **事实核查修正**（写作过程中发现并修掉）：
  - `SetBlockProfileRate` 的 rate 单位是纳秒，`10000` 是 10µs 而非 10ms（01 篇两处）
  - block profile **不记录网络/文件 IO 与 Sleep**，只覆盖 channel/select/信号量/锁（02、04、05、07 相关表述已统一）
  - `go tool pprof` 报 `no such tool` 的真因是 `GOCACHE` 不可写（Go 1.26 起 `go tool` 按需从 GOROOT 源码构建），不是「Go 移除了 pprof」——已改写为 05 篇 F27
  - `go tool pprof -graph` 不存在，导出 DOT 用 `-dot`（05 篇）
  - 已核实为真的细节：`goroutineleak` profile 为 Go 1.26 的 GOEXPERIMENT 实验特性、`memprofilerate` 是唯一与 profile 相关的 GODEBUG（block/mutex 无对应项）
