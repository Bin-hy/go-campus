# Vistack 面试深挖 · 项目入口（字节跳动专项）

> **本页是本项目的入口页**（属于「[面试项目深挖](/面试项目深挖/)」栏目）。
>
> 使用方式：先读本页建立全局认知 → 读 `01` 背下三版口述稿 → 按模块读 `02`-`07` 补细节 → 用 `08` 做追问链实战演练 → 用 `10`/`11` 做临考速记 → **务必读完 `09`**（简历风险点与诚实话术，决定你会不会被当场问崩）。
>
> 全部内容基于 `/Users/binhy/Binhy-Projects/Vistack` 真实代码，标注了文件与函数；凡代码里没有的能力，一律在 `09` 里列为"不能吹的点"。

---

## 一、这个项目到底是什么（一句话）

**Vistack 是一个 Go 写的云原生分布式视频平台：单二进制按角色拆成 `api` / `worker` / `transcoder` / `auth` 四个进程，用 Kafka 解耦上传与转码、用 gRPC + etcd 把 FFmpeg 做成可水平扩容的远程转码服务、用 MinIO 做分片直传与 DASH 切片分发，再叠加一套 Redis 高并发能力（缓存三件套 / 分布式限流 / 点赞收藏播放量计数 / 弹幕）。**

被问"介绍一下这个项目"，永远先给这句，再按 `01` 的口述稿展开。

---

## 二、事实底盘（必须记牢的 12 个数字/事实）

| 事实 | 值 | 来源 |
|------|----|------|
| 角色 | api / worker / transcoder / auth（另有 migrate） | `cmd/vistack/main.go` `resolveRole` |
| 角色选择 | `VISTACK_ROLE` 环境变量或首个位置参数 | 同上 |
| 上传分片大小 | 8 MB（前端切片）；哈希块 2 MB | `web/web-client/src/views/Creator/index.vue` |
| 上传并发 | 6 个 worker 抢同一个分片队列 | 同上 |
| 预签名有效期 | 1 小时 | `internal/api/v1/Video.go` `GetUploadPartURL` |
| 转码调用超时 | 25 分钟 | `internal/core/message_queue/transcode/worker.go` `transcodeCallTimeout` |
| 转码租约 | Redis `SetNX lease:transcode:{id}` 30 分钟 | 同上 |
| 重试 | Redis ZSet 延迟队列，指数退避 2^(n-1) 分钟 + 抖动，上限 8 小时，最多 7 次 | `transcode/retry.go`、`worker.go` `markFailed` |
| Watchdog | 每分钟扫；processing 超 15 分钟且无租约 / pending 超 10 分钟 → 重投 | `transcode/watchdog.go` |
| etcd 注册 | 租约 TTL 10 s，每 3 s 续约，key `/vistack/transcoders/{uuid}` | `internal/transcoder/registry/etcd.go` |
| Kafka 并发 | 每实例 `concurrency = 4`（同 group 多 reader），手动提交 offset | `internal/core/kafka.go`、`conf/app.toml` |
| DASH 切片 | `seg_duration = 4` s，GOP `-g 120` @30fps（4 秒一个关键帧），240p–4K 七档 | `internal/transcoder/ffmpeg.go` |
| 限流 | 滑动窗口 60 s / 100 次（或令牌桶 10 rate / 20 burst），按 user_id，Redis 挂 fail-open | `conf/app.toml`、`internal/middlewares/ratelimit` |
| 缓存 | TTL 300–600 s 随机、空值 60 s、锁 5 s、布隆 1000 万位 7 哈希 | `conf/app.toml`、`internal/core/cache` |
| 计数落库 | 事件进 Redis List，flusher 每 5 s 批量 200 条 | `internal/interaction/flusher.go` |

> 面试时数字比形容词有力：**"分片 8 MB、并发 6、转码超时 25 分钟、租约 30 分钟、重试 7 次"** 这类细节能直接证明你真写过。

---

## 三、简历描述 vs 代码实现（自查表）

请把这张表背下来，它决定了你在哪些地方**可以深挖**、哪些地方**必须提前打补丁**。

| 简历原文 | 代码事实 | 面试风险 | 处置 |
|---------|---------|---------|------|
| 三角色微服务拆分（API / Worker / Transcoder） | ✅ 真实存在，且实际是**四角色**（还有 auth 服务） | 无 | 主动说"实际是四个角色"，展示超出简历的深度 |
| 支持实时直播与视频点播 | ⚠️ **VOD 完整实现；直播只有 README 描述（live777 Rust SFU），仓库里没有任何直播代码** | 极高：一问推流密钥、拉流协议、SFU 架构就崩 | **见 `09`，必须改简历或彻底准备好话术** |
| DASH ABR 240p–4K，dash.js 无缝切换 | ✅ 七档参数表 + 前端 autoSwitchBitrate | 低 | 把 `-g`/`seg_duration`/`-sc_threshold` 讲透 |
| MinIO Multipart + MD5 秒传 + 断点续传 | ✅ 全部真实 | 低 | 讲清秒传的 ref_count 与并发问题 |
| 远程转码隔离，gRPC ProcessVideo，无状态水平扩容 | ✅ 真实，etcd 注册 + round_robin | 低 | 讲清 25 分钟超时/租约/watchdog 三者配合 |
| 防盗链：预签名 URL + STS 临时凭证 + JWT | ✅ 真实（STS 用于播放分片，预签名用于上传分片） | 中：要能区分两者用途 | 见 `07` |
| （未写但项目里有）缓存三件套 / 限流 / 计数 / 弹幕 / 评论 / 领导选举 | ✅ 真实且是加分项 | 无 | **主动拿出来讲**，比简历上的点更能体现深度 |

---

## 三·补、🔥 十大"只有真作者才知道"的发现（背下来，这是你最强的武器）

这些是逐行核对代码后找出来的**真实缺陷/真相**，它们比任何"我用了 Kafka"都更能证明你读过自己的代码。面试时**主动抛出 2–3 条**，效果远好于被动挨打。

| # | 发现 | 事实依据 | 现场怎么用 |
|---|------|---------|-----------|
| 1 | **`Bitrate` 字段从未传进 ffmpeg** —— 三档表里写的 `500k/4000k/35000k` 只出现在日志里，实际编码是 **CRF + maxrate 的 capped CRF**，所以 MPD 里的 `bandwidth` 和档位表不是严格对应 | `internal/transcoder/ffmpeg.go:316-341` 只用 Profile/Preset/CRF/MaxRate/BufSize；Bitrate 仅见 `:290` 日志 | "我一开始以为是固定码率，后来发现是 CRF 封顶模式——这是刻意的，CRF 保质量、maxrate 限制峰值，但要注意 MPD 的 bandwidth 是估算值" |
| 2 | **etcd 注册保活有个自愈失败的洞** —— `KeepAliveOnce` 失败后重新 `Grant`，但随后的 `Put` 错误被 `_, _ =` 丢弃；若这次 Put 失败，下轮续约会作用于新租约"成功"，于是**永不重试 Put**，实例从发现列表永久消失，只能重启 | `internal/transcoder/registry/etcd.go:52-56` | "我这块保活是自己写的，严格说应该用官方流式 `KeepAlive` 通道，避免续约和重新注册之间的状态不一致" |
| 3 | **转码超时不会杀掉 ffmpeg** —— `exec.Command` 不接收 context，worker 侧 25 分钟超时只取消了 gRPC，转码机上的 ffmpeg 还在跑；worker 重试时可能有两份转码在烧 CPU | `internal/transcoder/ffmpeg.go:370` | 见 `08` 链 2 Q2.6，这条是"我知道我的超时是假的"级别的加分回答 |
| 4 | **`transcode` topic 只有 1 分区 → 全集群同时只有一路消费** —— 并发度 = `min(concurrency=4, 分区数)`，加 worker 副本只提升可用性、不提升转码吞吐 | `internal/core/kafka.go` `EnsureTopic`、`conf/app.toml` | 见 `08` 链 2 Q2.7，配合"扩到 8 分区后并发从 1 → N"（做完作业 1 就能讲实测） |
| 5 | **Watchdog 会烧完重试预算** —— processing 分支在重投后**没有 touch `updated_at`**，下一分钟会再次扫到同一行并 `INCR attempts`，与 `markFailed` 共享同一个计数键，**7 分钟就把 7 次预算烧完**，之后静默跳过（无 DLQ） | `internal/core/message_queue/transcode/watchdog.go:30-49` | "看门狗这里有个真实 bug：它和失败重试共用 attempts 键，会提前耗光重试次数，修法是给看门狗独立的计数或者加状态守卫" |
| 6 | **Watchdog 把 Redis 错误当成"没有租约"** —— 判断是 `Get(leaseKey)` 返回 `err == nil` 才算有主，连接超时/抖动都会被判定为"无人处理"从而重复投递 | 同上 `:32-34` | "严格说应该区分 `redis.Nil` 和连接错误，前者才代表没有租约" |
| 7 | **删除链路重投不幂等** —— 重复调 DELETE 或 Kafka 重复消费会重复递减 `ref_count`；且文件仍被共享（`allZero=false`）时，TX2 里那套"四表复核修正 ref_count"的逻辑根本不会执行 | `internal/api/v1/Video.go:466-497`、`delete_video_worker.go:99-121`、`:180-185` | "软删除没有状态守卫，是幂等洞；但我做了'物理删除前用真实引用数复核'的兜底，方向是对的" |
| 8 | **秒传用 MD5 且服务端不校验内容** —— 只按 `hash + status` 查库，不比对大小/内容，理论上存在撞哈希伪造引用他人原始文件、以及"文件存在性预言机"两个缺口 | `internal/api/v1/Video.go:97-179`（Entity 注释里写的是 SHA-256，实际前端用 SparkMD5） | "正确做法是 SHA-256 + 抽样字节校验 + 对 active 状态建唯一索引" |
| 9 | **两个播放接口不校验可见性** —— `/videos/:id/manifest.mpd` 与 `/videos/:id/segments/signature` 挂在公开路由组，只查视频存在、不查 `visibility`/作者；STS 还用的是 MinIO root 凭证 | `internal/routers/api/v1/video.go:33-34`、`Video.go:780-784`、`:811-815` | 见 `08` 链 6 Q6.2，主动认下这条"越权缺口"极加分 |
| 10 | **布隆过滤器的 best-effort 会造成真视频 404** —— 新建视频后 `addVideoBloom` 是提交事务后的 best-effort，失败只打日志；漏加后 `Exists` 返回 false，**真实存在的视频会被直接返回 404**（这是布隆最不该出现的假阴性） | `internal/api/v1/video_cache.go:56-63`、`cache/bloom.go:83-107` | "布隆只应该用来'挡住一定不存在的'，我这里的降级方向反了，应该宁可放行也不能拦真实数据" |

> 另外两条与"高并发一致性"相关，建议一并记住：**① `syncCounts` 以 Redis 为权威回写 DB 冗余列，但 compose 里 Redis 只有 RDB 没开 AOF，重启可能把错值/0 覆盖进 DB；② flusher 用 `LPopCount` 弹出即丢（`applyEvents` 失败事件就没了），没有回塞或死信。**
>
> 这两条最值钱的用法是：**"如果让你重做，你先修哪个？"** ——答"先修'Redis 是权威但可能丢数据'这条，因为计数错乱用户会直接看到，比后端的重试问题更早暴露"。

---

## 四、面试时间分配建议（45 分钟技术面）

| 环节 | 时长 | 你要做的事 |
|------|------|-----------|
| 自我介绍 | 3 min | 用 `01` 的 3 分钟版，项目占 60% 时间 |
| 项目深挖 | 20–25 min | 主链路讲上传→转码→播放；主动抛出"我知道的不足" |
| 场景/设计题 | 10 min | 用 `08` 追问链里的思路答（限流、幂等、一致性、扩容） |
| 反问 | 5 min | 用 `10` 里的反问清单 |

**核心策略：不要等面试官挖，自己先挖。** 每讲一个亮点，主动补一句"这里我踩过一个坑/有个已知不足"，面试官会觉得你真实且有工程判断力。

---

## 五、文档清单与阅读顺序

| 文件 | 内容 | 优先级 |
|------|------|--------|
| `index.md`（本页） | 全局认知、事实底盘、简历对照 | ⭐⭐⭐ |
| `01-项目全景与三版口述稿.md` | 架构图 + 1/3/8 分钟口述稿（可直接背） | ⭐⭐⭐ |
| `02-上传链路与对象存储.md` | MinIO 分片直传、预签名、MD5 秒传、断点续传、引用计数删除 | ⭐⭐⭐ |
| `03-DASH转码与ABR自适应码率.md` | FFmpeg 参数逐项拆解、档位算法、dash.js ABR | ⭐⭐⭐ |
| `04-gRPC远程转码与etcd服务发现.md` | proto 契约、无状态设计、etcd 租约、gRPC resolver + round_robin | ⭐⭐⭐ |
| `05-Kafka异步编排与任务可靠性.md` | 消费语义、幂等三件套、重试/看门狗、领导选举、优雅停机 | ⭐⭐⭐ |
| `06-缓存限流与高并发计数.md` | 缓存三件套、布隆、令牌桶 vs 滑动窗口、Lua 计数与异步落库 | ⭐⭐⭐ |
| `07-鉴权与安全防线.md` | RS256/JWKS、auth 拆分、STS + 前端 SigV4、防盗链链路 | ⭐⭐ |
| `08-面试官追问链实战.md` | 6 条完整追问链（每条 5–8 轮），含"翻车信号" | ⭐⭐⭐ |
| `09-风险点与诚实话术.md` | 简历必须改的点、被问"是不是 AI 写的"怎么答、不能吹的边界 | ⭐⭐⭐（先读） |
| `10-速答题库与背诵卡.md` | 70+ 题速答表、数字速记、自我介绍三版 | ⭐⭐⭐ |
| `11-代码地图与事实索引.md` | 模块→文件→函数索引、三条链路逐跳追踪、已知缺口清单 | ⭐⭐ |

---

## 六、三条必须能徒手画出来的图

1. **部署/架构图**：客户端 → Traefik → api/auth → (Kafka / PostgreSQL / Redis / MinIO / etcd) ← worker → gRPC → transcoder → MinIO。
2. **上传→转码状态机**：`pending → processing → completed / failed →（重试队列）→ 回到 processing`，并标注每一步的幂等手段（DB 状态、Redis 租约）。
3. **播放鉴权链路**：`manifest.mpd` 走 API 代理 → `segments/signature` 拿 STS → dash.js 拦截 `.m4s` 请求 → 前端 SigV4 签名 → 直连 MinIO。

> 白板画图时把"数字"标上去（8 MB / 6 并发 / 25 min / 30 min / 7 次 / 4 s 切片），面试官一眼就知道这不是背的。

---

➡️ 开始阅读：[`01-项目全景与三版口述稿`](./01-项目全景与三版口述稿) ｜ 返回栏目：[面试项目深挖总览](/面试项目深挖/) ｜ 同栏项目：[BinRag](/面试项目深挖/BinRag/) · [EasyCoding](/面试项目深挖/EasyCoding/)
